package public

import "testing"

// TestThemeCachesAreUsableWithoutInitialization pins the zero-value contract of
// the two package-level theme caches. Each map is created lazily on its first
// write, so a process that never runs an initializer still renders a theme
// instead of panicking on a nil map write. The maps are reset to nil here to
// exercise exactly that path.
func TestThemeCachesAreUsableWithoutInitialization(t *testing.T) {
	themeCache.Lock()
	savedThemes := themeCache.themes
	themeCache.themes = nil
	themeCache.Unlock()

	themeSourcesCache.Lock()
	savedSources := themeSourcesCache.themes
	themeSourcesCache.themes = nil
	themeSourcesCache.Unlock()

	t.Cleanup(func() {
		themeCache.Lock()
		themeCache.themes = savedThemes
		themeCache.Unlock()

		themeSourcesCache.Lock()
		themeSourcesCache.themes = savedSources
		themeSourcesCache.Unlock()
	})

	r := newTestRenderer(t)
	if _, err := r.loadTheme(DefaultTheme); err != nil {
		t.Fatalf("loadTheme on an uninitialized cache: %v", err)
	}
	if _, err := r.loadThemeSources(DefaultTheme); err != nil {
		t.Fatalf("loadThemeSources on an uninitialized cache: %v", err)
	}

	// The first write must not only avoid panicking but also populate the map,
	// or every request re-parses the theme.
	themeCache.Lock()
	_, cachedTemplate := themeCache.themes[DefaultTheme]
	themeCache.Unlock()
	if !cachedTemplate {
		t.Error("loadTheme did not cache the parsed template set")
	}

	themeSourcesCache.Lock()
	_, cachedSources := themeSourcesCache.themes[DefaultTheme]
	themeSourcesCache.Unlock()
	if !cachedSources {
		t.Error("loadThemeSources did not cache the theme sources")
	}
}
