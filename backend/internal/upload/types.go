// Wire types for the upload domain. Swag reads the JSON
// tags to populate the OpenAPI spec; orval turns the
// generated schemas into TypeScript interfaces.
package upload

import "time"

// MediaFileResponse is one recorded upload. StorageKey, UserID and the owner
// relation stay server-side.
type MediaFileResponse struct {
	ID        uint      `json:"id" example:"17"`
	URL       string    `json:"url" example:"/uploads/01a15240.png"`
	Size      int64     `json:"size" example:"20480"`
	Type      string    `json:"type" example:"image/png"`
	CreatedAt time.Time `json:"createdAt"`
}

// UploadResponse is the body of POST /api/upload (single file
// upload).
type UploadResponse struct {
	Message string            `json:"message" example:"File uploaded successfully"`
	File    MediaFileResponse `json:"file"`
}

// MultiUploadResponse is the body of POST /api/upload/multiple.
// Any per-file failures are silently dropped and the response
// only includes the ones that succeeded.
type MultiUploadResponse struct {
	Message string              `json:"message" example:"File upload completed"`
	Files   []MediaFileResponse `json:"files"`
}

// FilesListResponse is the body of GET /api/upload/my. Lists
// the authenticated user's uploaded files (most recent first).
type FilesListResponse struct {
	Files []MediaFileResponse `json:"files"`
}
