package asset

import "errors"

var (
	// ErrNotFound means the media file does not exist.
	ErrNotFound = errors.New("file not found")
	// ErrForbidden means the acting user may not delete this file.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidExtension means the file extension is not in `allowedExts`.
	ErrInvalidExtension = errors.New("invalid file extension")
	// ErrEmptyFile identifies the file to read is empty.
	ErrEmptyFile = errors.New("empty file")
	// ErrInvalidFileSize means the declared file size is invalid.
	ErrInvalidFileSize = errors.New("invalid file size")
	// ErrEmptyFilename means the filename is empty.
	ErrEmptyFilename = errors.New("filename is empty")
	// ErrFilenameTooLong means the filename exceeds `model.MaxAssetFilenameLength`.
	ErrFilenameTooLong = errors.New("filename is too long")
	// ErrContentUnavailable means that an asset and a blob records exist in database,
	// while the physical file is unavailable.
	ErrContentUnavailable = errors.New("file is not unavailable")
)
