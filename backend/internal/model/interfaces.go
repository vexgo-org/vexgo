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
