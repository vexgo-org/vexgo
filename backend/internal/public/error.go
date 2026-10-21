package public

import "errors"

var (
	// ErrNoThemeTemplates reports a theme that provides no template files at all.
	ErrNoThemeTemplates = errors.New("theme has no template files")
	ErrDevTheme         = errors.New("dev theme error")
	// ErrNoPreviewSecret means the service has no JWT secret, so it can neither
	// sign nor verify preview links.
	ErrNoPreviewSecret = errors.New("theme preview is unavailable: no signing secret configured")
)
