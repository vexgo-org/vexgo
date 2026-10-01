package asset

import "github.com/vexgo-org/vexgo/backend/internal/model"

type UploadResponse struct {
	Message string       `json:"message" example:"File uploaded successfully"`
	File    *model.Asset `json:"file"`
}

type MessageResponse struct {
	Message string `json:"message" example:"File deleted"`
}
