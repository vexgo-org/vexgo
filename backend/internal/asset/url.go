package asset

import (
	"path"

	"github.com/google/uuid"
)

const originalPath = "/api/assets"

func OriginalURL(publicID uuid.UUID) string {
	return path.Join(
		originalPath,
		publicID.String(),
		"original",
	)
}
