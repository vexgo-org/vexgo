package asset

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// allowedExts is the allowlist of extensions kept on the stored file. An
// extension that is not listed is rejected with ErrInvalidExtension before
// anything is written, so this list is the complete set of formats an upload
// can be stored under.
//
// Two filters decide membership. Local uploads are served from this origin, so
// a file the browser would render as an executable document is a stored XSS
// vector: html, htm, xhtml, xml and js are deliberately absent. The other is
// the blob type: model.BlobTypeFromMIME only maps image, video and audio, so a
// format whose sniffed MIME type falls outside those three — pdf, plain text,
// svg, and the containers Go's sniffer cannot recognize — is absent too, since
// an upload of it could never become a blob.
//
// TestAllowedExtsAreAcceptedByBlobType enforces both halves: an extension may
// not be added here unless the sniffer gives its content a blob type.
var allowedExts = map[string]struct{}{
	".bmp":  {},
	".gif":  {},
	".ico":  {},
	".jpeg": {},
	".jpg":  {},
	".mp3":  {},
	".mp4":  {},
	".png":  {},
	".wav":  {},
	".webm": {},
	".webp": {},
}

type KeyGenerator interface {
	Generate(filename string) (string, error)
}

type keyGenerator struct{}

func NewKeyGenerator() KeyGenerator {
	return &keyGenerator{}
}

// Generate generates UUID storage key with file extension.
//
// The format of filename is `name.ext`, and the result will be
// `uuid.ext`.
//
// If the extension is not listed in `allowedExts`, this function will
// return an error.
func (g *keyGenerator) Generate(filename string) (string, error) {
	ext := strings.ToLower(path.Ext(filename))
	if _, ok := allowedExts[ext]; !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidExtension, ext)
	}

	id := uuid.NewString()
	return id + ext, nil
}
