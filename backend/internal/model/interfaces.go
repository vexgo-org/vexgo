package model

import "context"

// NotificationInput is the content of a notification to create. It groups
// the values that were previously passed as positional arguments.
type NotificationInput struct {
	UserID        uint
	Type          NotificationType
	Title         string
	Content       string
	RelatedID     string
	RelatedType   NotificationRelatedType
	RelatedPostID *uint // Owning post ID for reply/comment notifications
}

// Notifier is the seam for creating notifications; implemented by the
// notification domain and injected so it can be faked in tests.
type Notifier interface {
	CreateNotification(ctx context.Context, input NotificationInput) error
}

// FileRemover deletes a stored file by its public URL; implemented by
// upload.Storage and injected so it can be faked in tests.
type FileRemover interface {
	Delete(url string) error
}

// Mailer is the seam for sending transactional email and managing email
// verification/password-reset tokens; implemented by the mailer domain and
// injected so it can be faked in tests.
type Mailer interface {
	IsEmailEnabled() (bool, error)
	GenerateVerificationToken(userID uint) (string, error)
	SendVerificationEmail(toEmail, toName, verificationLink string) error
	VerifyEmail(token string) error
	GenerateEmailChangeToken(userID uint, newEmail string) (string, error)
	SendEmailChangeEmail(toEmail, toName, newEmail, verificationLink string) error
	GeneratePasswordResetToken(userID uint) (string, error)
	SendPasswordResetEmail(toEmail, toName, resetLink string) error
	ConfirmEmailChange(token string) error
}
