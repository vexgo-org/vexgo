package asset

import (
	"io"

	"github.com/google/uuid"
)

type Actor struct {
	ID   uint
	Role string
}

type UploadResponse struct {
	Message     string    `json:"message" example:"File uploaded successfully"`
	PublicID    uuid.UUID `json:"assetPublicID" example:"01a15240-3847-71a4-b8bb-f8448ef3a952"`
	OriginalURL string    `json:"assetOriginalURL" example:"/api/assets/01a15240-3847-71a4-b8bb-f8448ef3a952/original"`
}

type MessageResponse struct {
	Message string `json:"message" example:"File deleted"`
}

type OriginalContent struct {
	Reader io.ReadSeekCloser

	MIMEType string
	Filename string
}
