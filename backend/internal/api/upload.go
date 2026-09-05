package api

import "github.com/vexgo-org/vexgo/backend/internal/model"

// UploadFileResponse is the body of POST /api/upload/file.
type UploadFileResponse struct {
	Message string         `json:"message" doc:"Human-readable outcome"`
	File    model.MediaFile `json:"file" doc:"The persisted file record"`
}

// UploadFilesResponse is the body of POST /api/upload/files.
type UploadFilesResponse struct {
	Message string          `json:"message"`
	Files   []model.MediaFile `json:"files"`
}

// ListFilesResponse is the body of GET /api/upload/my-files.
type ListFilesResponse struct {
	Files []model.MediaFile `json:"files"`
}

// UploadMessageResponse is the body of DELETE /api/upload/{id}.
type UploadMessageResponse struct {
	Message string `json:"message"`
}
