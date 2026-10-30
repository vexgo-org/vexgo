package asset

import (
	"testing"

	"github.com/google/uuid"
)

func TestPublicIDFromURL(t *testing.T) {
	publicID := uuid.New()
	original := OriginalURL(publicID)

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"original path", original, true},
		{"absolute URL", "https://example.com" + original, true},
		{"query string", original + "?v=2", true},
		{"legacy upload", "/uploads/image.png", false},
		{"external URL", "https://example.com/image.png", false},
		{"missing suffix", "/api/assets/" + publicID.String(), false},
		{"wrong prefix", "/api/blobs/" + publicID.String() + "/original", false},
		{"invalid public ID", "/api/assets/not-a-uuid/original", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PublicIDFromURL(tt.raw)
			if ok != tt.want {
				t.Fatalf("PublicIDFromURL(%q) ok = %v, want %v", tt.raw, ok, tt.want)
			}
			if ok && got != publicID {
				t.Fatalf("PublicIDFromURL(%q) = %s, want %s", tt.raw, got, publicID)
			}
		})
	}
}
