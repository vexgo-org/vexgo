package asset

import "errors"

var (
	// ErrNotFound means the media file does not exist.
	ErrNotFound = errors.New("file not found")
	// ErrForbidden means the acting user may not delete this file.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidExtension means the file extension is not in `allowedExts`.
	ErrInvalidExtension = errors.New("invalid file extension")
	// ErrAssetNotDeleted means an asset is removed without soft deletion.
	ErrAssetNotDeleted = errors.New("asset is not deleted")
	// ErrEmptyStorageKey means the asset has empty storage key.
	ErrEmptyStorageKey = errors.New("empty storage key")
)
