// Package user implements the user-management and creator-application domain.
package user

import (
	"context"
	"errors"
	"fmt"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	// User lookup errors.
	ErrUserNotFound        = errors.New("user not found")
	ErrApplicationNotFound = errors.New("application not found")

	// UpdateRole errors.
	ErrCannotModifySelf    = errors.New("cannot modify own role")
	ErrModifySuperAdmin    = errors.New("no permission to modify super admin role")
	ErrInvalidRole         = errors.New("invalid role")
	ErrSuperAdminOwnRole   = errors.New("super admin cannot modify own role")
	ErrAdminRoleRestricted = errors.New("admin can only set user roles to author, contributor, or guest")
	ErrNoPermission        = errors.New("no permission to modify user role")

	// DeleteUser errors.
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
func (s *Service) ListUsers(ctx context.Context, search string, page, limit int) ([]model.User, int64, error) {
	offset := (page - 1) * limit
	return s.repo.ListUsers(ctx, search, offset, limit)
}

// UpdateRole changes the target user's role under the acting user's
// permissions, and notifies the target user of the change.
func (s *Service) UpdateRole(ctx context.Context, actor model.User, targetID uint, newRole string) (*model.User, error) {
	user, err := s.repo.FindByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Cannot modify own role
	if user.ID == actor.ID {
		return nil, ErrCannotModifySelf
	}
	// Cannot modify super admin's role (unless current user is also super admin)
	if model.IsSuperAdmin(user.Role) && !model.IsSuperAdmin(actor.Role) {
		return nil, ErrModifySuperAdmin
	}

	// Validate role is valid
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

	// Capture the old role before it is overwritten below, so the
	// notification can report the actual before/after values.
	oldRole := user.Role

	// Permission check
	// Super admin can set any role (including making other users super admin)
	// But cannot downgrade own super admin privileges
	switch actor.Role {
	case model.RoleSuperAdmin:
		if user.ID == actor.ID && newRole != model.RoleSuperAdmin {
			return nil, ErrSuperAdminOwnRole
		}
		user.Role = newRole
	case model.RoleAdmin:
		// Admin can only set user roles to author, contributor, or guest
		if newRole != model.RoleAuthor && newRole != model.RoleContributor && newRole != model.RoleGuest {
			return nil, ErrAdminRoleRestricted
		}
		user.Role = newRole
	default:
		return nil, ErrNoPermission
	}

	// Save updates
	if err := s.repo.UpdateUserRole(ctx, user); err != nil {
		return nil, err
	}

	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:  user.ID,
		Type:    "role",
		Title:   "role changed",
		Content: fmt.Sprintf("Your role has been changed from \"%s\" to \"%s\"", oldRole, newRole),
	}); err != nil {
		logrus.WithError(err).Warn("failed to create role change notification")
	}

	return user, nil
}

// DeleteUser deletes a user and all their posts, comments, likes and media
// files inside a transaction.
func (s *Service) DeleteUser(ctx context.Context, actor model.User, targetID uint) error {
	user, err := s.repo.FindByID(ctx, targetID)
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
		if model.IsAdmin(user.Role) {
			return ErrAdminDeleteRestricted
		}
	default:
		return ErrNoPermissionToDelete
	}

	// Cascade-delete inside a single transaction, then remove the referenced
	// files only after it commits: a rollback must not leave DB rows pointing
	// at deleted files (or files deleted for rows that were rolled back).
	fileURLs, err := s.repo.DeleteUserCascade(ctx, user.ID)
	if err != nil {
		return err
	}

	for _, url := range fileURLs {
		if err := s.files.Delete(url); err != nil {
			logrus.WithError(err).WithField("url", url).Warn("Failed to delete media file")
		}
	}
	return nil
}

// ApplyForCreator submits a creator application for the given user and
// notifies all admins. It returns the new application ID.
func (s *Service) ApplyForCreator(ctx context.Context, user model.User, reason string) (uint, error) {
	if model.IsAuthor(user.Role) {
		return 0, ErrRoleNotEligible
	}

	if _, err := s.repo.FindPendingApplication(ctx, user.ID); err == nil {
		return 0, ErrAlreadyPending
	}

	application := model.CreatorApplication{
		UserID: user.ID,
		Status: model.CreatorApplicationStatusPending,
		Reason: reason,
	}
	if err := s.repo.CreateApplication(ctx, &application); err != nil {
		return 0, err
	}

	targetRole := ""
	switch user.Role {
	case model.RoleGuest:
		targetRole = model.RoleContributor
	case model.RoleContributor:
		targetRole = model.RoleAuthor
	}

	admins, err := s.repo.FindAdmins(ctx)
	if err == nil {
		for _, admin := range admins {
			if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
				UserID:      admin.ID,
				Type:        "role",
				Title:       "New Role Application",
				Content:     fmt.Sprintf("User %s has applied for %s role", user.Username, targetRole),
				RelatedID:   fmt.Sprintf("%d", application.ID),
				RelatedType: "creator_application",
			}); err != nil {
				logrus.WithError(err).Warn("failed to create role application notification")
			}
		}
	}

	return application.ID, nil
}

// ListCreatorApplicationsQuery carries the pagination and status filter.
type ListCreatorApplicationsQuery struct {
	ActorRole string
	Status    model.CreatorApplicationStatus
	Page      int
	Limit     int
}

// ListCreatorApplications returns the paginated creator applications with the
// applicant preloaded, filtered by status.
func (s *Service) ListCreatorApplications(ctx context.Context, q ListCreatorApplicationsQuery) ([]model.CreatorApplication, int64, error) {
	if !model.IsAdmin(q.ActorRole) {
		return nil, 0, ErrNoPermissionAccessApps
	}
	offset := (q.Page - 1) * q.Limit
	return s.repo.ListApplications(ctx, q.Status, offset, q.Limit)
}

// ReviewCreatorApplicationRequest carries the review action inputs.
type ReviewCreatorApplicationRequest struct {
	Actor  model.User
	AppID  uint
	Action string
	Reason string
}

// ReviewCreatorApplication approves or rejects a pending creator application,
// upgrading the applicant's role on approval and notifying the applicant.
func (s *Service) ReviewCreatorApplication(ctx context.Context, req ReviewCreatorApplicationRequest) error {
	if !model.IsAdmin(req.Actor.Role) {
		return ErrNoPermissionReviewApps
	}

	application, err := s.repo.FindApplicationByID(ctx, req.AppID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrApplicationNotFound
		}
		return err
	}

	if application.Status != model.CreatorApplicationStatusPending {
		return ErrApplicationProcessed
	}

	if req.Action != model.CreatorApplicationActionApprove && req.Action != model.CreatorApplicationActionReject {
		return ErrInvalidAction
	}

	if req.Action == model.CreatorApplicationActionApprove {
		application.Status = model.CreatorApplicationStatusApproved
		switch application.User.Role {
		case model.RoleGuest:
			application.User.Role = model.RoleContributor
		case model.RoleContributor:
			application.User.Role = model.RoleAuthor
		}
		if err := s.repo.UpdateUserRole(ctx, &application.User); err != nil {
			return err
		}
	} else {
		application.Status = model.CreatorApplicationStatusRejected
	}

	application.ReviewerID = &req.Actor.ID
	application.ReviewReason = req.Reason

	if err := s.repo.SaveApplication(ctx, application); err != nil {
		return err
	}

	var notificationTitle, notificationContent string
	if req.Action == model.CreatorApplicationActionApprove {
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
		if req.Reason != "" {
			notificationContent += " Reason: " + req.Reason
		}
	}

	if err := s.notifier.CreateNotification(ctx, model.NotificationInput{
		UserID:  application.UserID,
		Type:    "role",
		Title:   notificationTitle,
		Content: notificationContent,
	}); err != nil {
		logrus.WithError(err).Warn("failed to create creator application notification")
	}

	return nil
}
