package asset

import (
	"net/url"
	"path"
	"strings"

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

// PublicIDFromURL extracts the public ID from an original asset URL such as
// /api/assets/<publicID>/original. Absolute URLs are accepted because content
// may carry a full link copied from a browser; any other shape reports false.
func PublicIDFromURL(raw string) (uuid.UUID, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 ||
		parts[0] != "api" ||
		parts[1] != "assets" ||
		parts[3] != "original" {
		return uuid.Nil, false
	}

	publicID, err := uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, false
	}

	return publicID, true
}
