package asset

type Actor struct {
	ID   uint
	Role string
}

type UploadResponse struct {
	Message string `json:"message" example:"File uploaded successfully"`
	ID      uint   `json:"asset_id" example:"1"`
}

type MessageResponse struct {
	Message string `json:"message" example:"File deleted"`
}
