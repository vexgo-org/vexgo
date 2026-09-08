// Wire types for the upload domain. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package upload

import (
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// UploadResponse is the body of POST /api/upload (single file
// upload). The `file` field is the recorded MediaFile row.
type UploadResponse struct {
	Message string          `json:"message" example:"File uploaded successfully"`
	File    model.MediaFile `json:"file"`
}

// MultiUploadResponse is the body of POST /api/upload/multiple.
// The `files` array contains the recorded MediaFile rows;
// any per-file failures are silently dropped and the response
// only includes the ones that succeeded.
type MultiUploadResponse struct {
	Message string            `json:"message" example:"File upload completed"`
	Files   []model.MediaFile `json:"files"`
}

// FilesListResponse is the body of GET /api/upload/my. Lists
// the authenticated user's uploaded files (most recent first).
type FilesListResponse struct {
	Files []model.MediaFile `json:"files"`
}

// MessageResponse is a uniform `{ "message": "..." }` body
// for endpoints that don't return data.
type MessageResponse struct {
	Message string `json:"message" example:"File deleted"`
}
