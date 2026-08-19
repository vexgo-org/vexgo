// Package user implements the user-management and creator-application domain.
package user

import (
	"errors"
	"fmt"

	"vexgo/backend/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler. Each error carries
// the exact message of the original handler response it replaces.
var (
	// ErrUserNotFound means the target user does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrApplicationNotFound means the creator application does not exist.
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

	// Creator application errors.
	ErrRoleNotEligible        = errors.New("only guest and contributor users can apply for role upgrade")
	ErrAlreadyPending         = errors.New("you already have a pending application")
	ErrNoPermissionAccessApps = errors.New("no permission to access creator applications")
	ErrNoPermissionReviewApps = errors.New("no permission to review creator applications")
	ErrApplicationProcessed   = errors.New("application has already been processed")
	ErrInvalidAction          = errors.New("invalid action")
)

// Deps holds the dependencies required by the user domain.
type Deps struct {
	DB       *gorm.DB
	Notifier Notifier
	Files    FileRemover
}

// Notifier is an alias for model.Notifier kept for backward compatibility.
type Notifier = model.Notifier

// FileRemover is an alias for model.FileRemover kept for backward compatibility.
type FileRemover = model.FileRemover

// Service contains the business logic of the user domain.
type Service struct {
	db       *gorm.DB
	notifier Notifier
	files    FileRemover
}

// NewService creates a user service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{db: deps.DB, notifier: deps.Notifier, files: deps.Files}
}

// ListUsers returns the paginated user list with an optional search term.
func (s *Service) ListUsers(search string, page, limit int) ([]model.User, int64, error) {
	query := s.db.Model(&model.User{})

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", searchTerm, searchTerm)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	if err := query.Offset((page - 1) * limit).
		Limit(limit).
		Order("id ASC").
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateRole changes the target user's role under the acting user's
// permissions, and notifies the target user of the change.
func (s *Service) UpdateRole(actor model.User, targetID uint, newRole string) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, targetID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// Cannot modify own role
	if user.ID == actor.ID {
		return nil, ErrCannotModifySelf
	}

	// Cannot modify super admin's role (unless current user is also super admin)
	if user.Role == model.RoleSuperAdmin && actor.Role != model.RoleSuperAdmin {
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

	// Permission check
	// Super admin can set any role (including making other users super admin)
	// But cannot downgrade own super admin privileges
	switch actor.Role {
	case model.RoleSuperAdmin:
		// If current user is super admin, can set any role
		// Note: super admin cannot downgrade own role
		if user.ID == actor.ID && newRole != model.RoleSuperAdmin {
			return nil, ErrSuperAdminOwnRole
		}
		user.Role = newRole
	case model.RoleAdmin:
		// Admin can only set user roles to author, contributor, or guest (cannot set to admin or super admin)
		if newRole == model.RoleAuthor || newRole == model.RoleContributor || newRole == model.RoleGuest {
			user.Role = newRole
		} else {
			return nil, ErrAdminRoleRestricted
		}
	default:
		return nil, ErrNoPermission
	}

	// Save updates
	// Note: mirrors the original ordering where oldRole is captured after the
	// role was already assigned in the branches above.
	oldRole := user.Role
	user.Role = newRole
	if err := s.db.Model(&user).Select("Role").Updates(&user).Error; err != nil {
		return nil, err
	}

	// Create notification for user when role changes
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

	return &user, nil
}

// DeleteUser deletes a user and all their posts, comments, likes and media
// files inside a transaction.
func (s *Service) DeleteUser(actor model.User, targetID uint) error {
	var user model.User
	if err := s.db.First(&user, targetID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Cannot delete yourself
	if user.ID == actor.ID {
		return ErrCannotDeleteSelf
	}

	// Permission check
	// Super admin can delete any user except themselves
	// Admin can only delete users with role author, contributor, or guest
	switch actor.Role {
	case model.RoleSuperAdmin:
		// Super admin can delete any user
	case model.RoleAdmin:
		if user.Role != model.RoleAuthor && user.Role != model.RoleContributor && user.Role != model.RoleGuest {
			return ErrAdminDeleteRestricted
		}
	default:
		return ErrNoPermissionToDelete
	}

	// Start transaction
	tx := s.db.Begin()

	// Delete user's comments
	if err := tx.Where("user_id = ?", user.ID).Delete(&model.Comment{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete user comments: %w", err)
	}

	// Delete user's likes
	if err := tx.Where("user_id = ?", user.ID).Delete(&model.Like{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete user likes: %w", err)
	}

	// Delete user's media files
	var mediaFiles []model.MediaFile
	if err := tx.Where("user_id = ?", user.ID).Find(&mediaFiles).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("query user media files: %w", err)
	}

	// Delete physical media files
	for _, media := range mediaFiles {
		if err := s.files.Delete(media.URL); err != nil {
			// Log error but continue execution
			logrus.WithError(err).WithField("url", media.URL).Warn("Failed to delete media file")
		}
	}

	// Delete media files from database
	if err := tx.Where("user_id = ?", user.ID).Delete(&model.MediaFile{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete user media files: %w", err)
	}

	// Get user's posts
	var posts []model.Post
	if err := tx.Where("author_id = ?", user.ID).Find(&posts).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("query user posts: %w", err)
	}

	// Delete comments and likes for each post, and the post cover image
	for _, post := range posts {
		// Delete post cover image if exists
		if post.CoverImage != "" {
			if err := s.files.Delete(post.CoverImage); err != nil {
				logrus.WithError(err).WithField("url", post.CoverImage).Warn("Failed to delete cover image")
			}
		}

		if err := tx.Where("post_id = ?", post.ID).Delete(&model.Comment{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("delete post comments: %w", err)
		}

		if err := tx.Where("post_id = ?", post.ID).Delete(&model.Like{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("delete post likes: %w", err)
		}
	}

	// Delete user's posts
	if err := tx.Where("author_id = ?", user.ID).Delete(&model.Post{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete user posts: %w", err)
	}

	// Delete user
	if err := tx.Delete(&user).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete user: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ApplyForCreator submits a creator application for the given user and
// notifies all admins. It returns the new application ID.
func (s *Service) ApplyForCreator(user model.User, reason string) (uint, error) {
	// Only guest and contributor users can apply for role upgrade
	if user.Role != model.RoleGuest && user.Role != model.RoleContributor {
		return 0, ErrRoleNotEligible
	}

	// Check if user already has a pending application
	var existingApplication model.CreatorApplication
	if err := s.db.Where("user_id = ? AND status = ?", user.ID, model.CreatorApplicationStatusPending).First(&existingApplication).Error; err == nil {
		return 0, ErrAlreadyPending
	}

	// Create new application
	application := model.CreatorApplication{
		UserID: user.ID,
		Status: model.CreatorApplicationStatusPending,
		Reason: reason,
	}

	if err := s.db.Create(&application).Error; err != nil {
		return 0, err
	}

	// Determine target role based on current role
	targetRole := ""
	switch user.Role {
	case model.RoleGuest:
		targetRole = model.RoleContributor
	case model.RoleContributor:
		targetRole = model.RoleAuthor
	}

	// Send notification to admins and super admins
	var admins []model.User
	if err := s.db.Where("role IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).Find(&admins).Error; err == nil {
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
	// Only admins and super admins can access this endpoint
	if actorRole != model.RoleAdmin && actorRole != model.RoleSuperAdmin {
		return nil, 0, ErrNoPermissionAccessApps
	}

	query := s.db.Model(&model.CreatorApplication{}).Preload("User")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var applications []model.CreatorApplication
	if err := query.Offset((page - 1) * limit).
		Limit(limit).
		Order("created_at DESC").
		Find(&applications).Error; err != nil {
		return nil, 0, err
	}

	return applications, total, nil
}

// ReviewCreatorApplication approves or rejects a pending creator application,
// upgrading the applicant's role on approval and notifying the applicant.
func (s *Service) ReviewCreatorApplication(actor model.User, appID uint, action, reason string) error {
	// Only admins and super admins can review applications
	if actor.Role != model.RoleAdmin && actor.Role != model.RoleSuperAdmin {
		return ErrNoPermissionReviewApps
	}

	var application model.CreatorApplication
	if err := s.db.Preload("User").First(&application, appID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrApplicationNotFound
		}
		return err
	}

	// Check if application is still pending
	if application.Status != model.CreatorApplicationStatusPending {
		return ErrApplicationProcessed
	}

	// Validate action
	if action != model.CreatorApplicationActionApprove && action != model.CreatorApplicationActionReject {
		return ErrInvalidAction
	}

	// Update application status
	if action == model.CreatorApplicationActionApprove {
		application.Status = model.CreatorApplicationStatusApproved
		// Update user role based on current role
		switch application.User.Role {
		case model.RoleGuest:
			application.User.Role = model.RoleContributor
		case model.RoleContributor:
			application.User.Role = model.RoleAuthor
		}
		if err := s.db.Model(&application.User).Select("Role").Updates(&application.User).Error; err != nil {
			return err
		}
	} else {
		application.Status = model.CreatorApplicationStatusRejected
	}

	application.ReviewerID = &actor.ID
	application.ReviewReason = reason

	if err := s.db.Save(&application).Error; err != nil {
		return err
	}

	// Send notification to the applicant
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
