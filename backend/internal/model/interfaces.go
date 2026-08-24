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
