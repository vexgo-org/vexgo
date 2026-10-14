package asset

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// allowedExts is the allowlist of extensions kept on the stored
// file. Local uploads are served from this origin, so a file the browser would
// render as an executable document is a stored XSS vector: html, htm, xhtml,
// xml and js are therefore deliberately absent. Any other extension is dropped;
// the handler then serves the resulting extensionless file as
// application/octet-stream, which cannot execute.
//
// SVG is the one document-like type on the list, because an <img> never runs
// script inside an SVG and the media routes neutralize direct navigation by
// serving it with a sandboxing CSP (see public.mediaSVGContentType). Dropping
// its extension instead is what made uploaded SVG icons invisible: an
// extensionless file is an opaque byte stream the browser refuses to render as
// an image.
var allowedExts = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".avif": {},
	".bmp": {}, ".ico": {}, ".tif": {}, ".tiff": {}, ".svg": {},
	".mp4": {}, ".webm": {}, ".mov": {}, ".mp3": {}, ".wav": {}, ".ogg": {}, ".m4a": {},
	".pdf": {}, ".txt": {}, ".md": {}, ".csv": {},
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
