package model

import "context"

// Notifier is the seam for creating notifications; implemented by the
// message domain and injected so it can be faked in tests.
type Notifier interface {
	CreateNotification(ctx context.Context, userID uint, notificationType, title, content, relatedID, relatedType string) error
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
