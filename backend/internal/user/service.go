// Package user implements the user-management and creator-application domain.
package user

import (
	"errors"
	"fmt"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrApplicationNotFound = errors.New("application not found")

	ErrCannotModifySelf    = errors.New("cannot modify own role")
	ErrModifySuperAdmin    = errors.New("no permission to modify super admin role")
	ErrInvalidRole         = errors.New("invalid role")
	ErrSuperAdminOwnRole   = errors.New("super admin cannot modify own role")
	ErrAdminRoleRestricted = errors.New("admin can only set user roles to author, contributor, or guest")
	ErrNoPermission        = errors.New("no permission to modify user role")

	ErrCannotDeleteSelf      = errors.New("cannot delete yourself")
	ErrAdminDeleteRestricted = errors.New("admin can only delete users with role author, contributor, or guest")
	ErrNoPermissionToDelete  = errors.New("no permission to delete user")

	ErrRoleNotEligible        = errors.New("only guest and contributor users can apply for role upgrade")
	ErrAlreadyPending         = errors.New("you already have a pending application")
	ErrNoPermissionAccessApps = errors.New("no permission to access creator applications")
	ErrNoPermissionReviewApps = errors.New("no permission to review creator applications")
	ErrApplicationProcessed   = errors.New("application has already been processed")
	ErrInvalidAction          = errors.New("invalid action")
)

// Deps holds the dependencies required by the user domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	Notifier  Notifier
	Files     FileRemover
}

// Notifier is an alias for model.Notifier kept for backward compatibility.
type Notifier = model.Notifier

// FileRemover is an alias for model.FileRemover kept for backward compatibility.
type FileRemover = model.FileRemover

// Service contains the business logic of the user domain.
type Service struct {
	repo     Repository
	notifier Notifier
	files    FileRemover
}

// NewService creates a user service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB), notifier: deps.Notifier, files: deps.Files}
}

// ListUsers returns the paginated user list with an optional search term.
func (s *Service) ListUsers(search string, page, limit int) ([]model.User, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListUsers(search, offset, limit)
}

// UpdateRole changes the target user's role under the acting user's
// permissions, and notifies the target user of the change.
func (s *Service) UpdateRole(actor model.User, targetID uint, newRole string) (*model.User, error) {
	user, err := s.repo.FindByID(targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if user.ID == actor.ID {
		return nil, ErrCannotModifySelf
	}
	if user.Role == model.RoleSuperAdmin && actor.Role != model.RoleSuperAdmin {
		return nil, ErrModifySuperAdmin
	}

	validRoles := map[string]bool{
		model.RoleSuperAdmin:  true,
		model.RoleAdmin:       true,
		model.RoleAuthor:      true,
		model.RoleContributor: true,
		model.RoleGuest:       true,
	}
	if !validRoles[newRole] {
		return nil, ErrInvalidRole
	}

	switch actor.Role {
	case model.RoleSuperAdmin:
		if user.ID == actor.ID && newRole != model.RoleSuperAdmin {
			return nil, ErrSuperAdminOwnRole
		}
		user.Role = newRole
	case model.RoleAdmin:
		if newRole == model.RoleAuthor || newRole == model.RoleContributor || newRole == model.RoleGuest {
			user.Role = newRole
		} else {
			return nil, ErrAdminRoleRestricted
		}
	default:
		return nil, ErrNoPermission
	}

	oldRole := user.Role
	user.Role = newRole
	if err := s.repo.UpdateUserRole(user); err != nil {
		return nil, err
	}

	if err := s.notifier.CreateNotification(
		user.ID,
		"role",
		"role changed",
		fmt.Sprintf("Your role has been changed from \"%s\" to \"%s\"", oldRole, newRole),
		"",
		"",
	); err != nil {
		logrus.WithError(err).Warn("failed to create role change notification")
	}

	return user, nil
}

// DeleteUser deletes a user and all their posts, comments, likes and media
// files inside a transaction.
func (s *Service) DeleteUser(actor model.User, targetID uint) error {
	user, err := s.repo.FindByID(targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	if user.ID == actor.ID {
		return ErrCannotDeleteSelf
	}

	switch actor.Role {
	case model.RoleSuperAdmin:
		// can delete any user
	case model.RoleAdmin:
		if user.Role != model.RoleAuthor && user.Role != model.RoleContributor && user.Role != model.RoleGuest {
			return ErrAdminDeleteRestricted
		}
	default:
		return ErrNoPermissionToDelete
	}

	return s.repo.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.DeleteCommentsByUserIDTx(tx, user.ID); err != nil {
			return fmt.Errorf("delete user comments: %w", err)
		}
		if err := s.repo.DeleteLikesByUserIDTx(tx, user.ID); err != nil {
			return fmt.Errorf("delete user likes: %w", err)
		}

		mediaFiles, err := s.repo.FindMediaByUserIDTx(tx, user.ID)
		if err != nil {
			return fmt.Errorf("query user media files: %w", err)
		}
		for _, media := range mediaFiles {
			if err := s.files.Delete(media.URL); err != nil {
				logrus.WithError(err).WithField("url", media.URL).Warn("Failed to delete media file")
			}
		}
		if err := s.repo.DeleteMediaFilesByUserIDTx(tx, user.ID); err != nil {
			return fmt.Errorf("delete user media files: %w", err)
		}

		posts, err := s.repo.FindPostsByAuthorIDTx(tx, user.ID)
		if err != nil {
			return fmt.Errorf("query user posts: %w", err)
		}
		for _, post := range posts {
			if post.CoverImage != "" {
				if err := s.files.Delete(post.CoverImage); err != nil {
					logrus.WithError(err).WithField("url", post.CoverImage).Warn("Failed to delete cover image")
				}
			}
			if err := s.repo.DeleteCommentsByPostIDTx(tx, post.ID); err != nil {
				return fmt.Errorf("delete post comments: %w", err)
			}
			if err := s.repo.DeleteLikesByPostIDTx(tx, post.ID); err != nil {
				return fmt.Errorf("delete post likes: %w", err)
			}
		}

		if err := s.repo.DeletePostsByAuthorIDTx(tx, user.ID); err != nil {
			return fmt.Errorf("delete user posts: %w", err)
		}
		if err := s.repo.DeleteUserTx(tx, user); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		return nil
	})
}

// ApplyForCreator submits a creator application for the given user and
// notifies all admins. It returns the new application ID.
func (s *Service) ApplyForCreator(user model.User, reason string) (uint, error) {
	if user.Role != model.RoleGuest && user.Role != model.RoleContributor {
		return 0, ErrRoleNotEligible
	}

	if _, err := s.repo.FindPendingApplication(user.ID); err == nil {
		return 0, ErrAlreadyPending
	}

	application := model.CreatorApplication{
		UserID: user.ID,
		Status: model.CreatorApplicationStatusPending,
		Reason: reason,
	}
	if err := s.repo.CreateApplication(&application); err != nil {
		return 0, err
	}

	targetRole := ""
	switch user.Role {
	case model.RoleGuest:
		targetRole = model.RoleContributor
	case model.RoleContributor:
		targetRole = model.RoleAuthor
	}

	admins, err := s.repo.FindAdmins()
	if err == nil {
		for _, admin := range admins {
			if err := s.notifier.CreateNotification(
				admin.ID,
				"role",
				"New Role Application",
				fmt.Sprintf("User %s has applied for %s role", user.Username, targetRole),
				fmt.Sprintf("%d", application.ID),
				"creator_application",
			); err != nil {
				logrus.WithError(err).Warn("failed to create role application notification")
			}
		}
	}

	return application.ID, nil
}

// ListCreatorApplications returns the paginated creator applications with the
// applicant preloaded, filtered by status.
func (s *Service) ListCreatorApplications(actorRole string, status model.CreatorApplicationStatus, page, limit int) ([]model.CreatorApplication, int64, error) {
	if actorRole != model.RoleAdmin && actorRole != model.RoleSuperAdmin {
		return nil, 0, ErrNoPermissionAccessApps
	}
	offset := (page - 1) * limit
	return s.repo.ListApplications(status, offset, limit)
}

// ReviewCreatorApplication approves or rejects a pending creator application,
// upgrading the applicant's role on approval and notifying the applicant.
func (s *Service) ReviewCreatorApplication(actor model.User, appID uint, action, reason string) error {
	if actor.Role != model.RoleAdmin && actor.Role != model.RoleSuperAdmin {
		return ErrNoPermissionReviewApps
	}

	application, err := s.repo.FindApplicationByID(appID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if application.Status != model.CreatorApplicationStatusPending {
		return ErrApplicationProcessed
	}

	if action != model.CreatorApplicationActionApprove && action != model.CreatorApplicationActionReject {
		return ErrInvalidAction
	}

	if action == model.CreatorApplicationActionApprove {
		application.Status = model.CreatorApplicationStatusApproved
		switch application.User.Role {
		case model.RoleGuest:
			application.User.Role = model.RoleContributor
		case model.RoleContributor:
			application.User.Role = model.RoleAuthor
		}
		if err := s.repo.UpdateUserRole(&application.User); err != nil {
			return err
		}
	} else {
		application.Status = model.CreatorApplicationStatusRejected
	}

	application.ReviewerID = &actor.ID
	application.ReviewReason = reason

	if err := s.repo.SaveApplication(application); err != nil {
		return err
	}

	var notificationTitle, notificationContent string
	if action == model.CreatorApplicationActionApprove {
		if application.User.Role == model.RoleAuthor {
			notificationTitle = "Author Application Approved"
			notificationContent = "Your author application has been approved. You are now an author."
		} else {
			notificationTitle = "Contributor Application Approved"
			notificationContent = "Your contributor application has been approved. You are now a contributor."
		}
	} else {
		notificationTitle = "Role Application Rejected"
		notificationContent = "Your role application has been rejected."
		if reason != "" {
			notificationContent += " Reason: " + reason
		}
	}

	if err := s.notifier.CreateNotification(
		application.UserID,
		"role",
		notificationTitle,
		notificationContent,
		"",
		"",
	); err != nil {
		logrus.WithError(err).Warn("failed to create creator application notification")
	}

	return nil
}
