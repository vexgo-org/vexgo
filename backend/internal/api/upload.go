package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// UploadResponse is the body of POST /api/upload/file and POST /api/upload/files.
// Only the fields that belong to the invoked endpoint are present.
type UploadResponse struct {
	Message string            `json:"message"`
	File    *model.MediaFile  `json:"file,omitempty"`
	Files   []model.MediaFile `json:"files,omitempty"`
}

// FilesResponse is the body of GET /api/upload/my-files.
type FilesResponse struct {
	Files []model.MediaFile `json:"files"`
}
