package public

import "testing"

// TestThemeCachesAreInitialized pins the package-level caches to a usable zero
// state. themeCache and themeSourcesCache are each filled by their own init
// function, and a nil map panics on its first write — so a refactor that drops
// one of the initializers (or merges the two incorrectly) turns the first theme
// render into a panic instead of a parse. Both must therefore start non-nil and
// writable.
func TestThemeCachesAreInitialized(t *testing.T) {
	if themeCache.themes == nil {
		t.Fatal("themeCache.themes is nil: the first loadTheme write would panic")
	}
	if themeSourcesCache.themes == nil {
		t.Fatal("themeSourcesCache.themes is nil: the first loadThemeSources write would panic")
	}

	// Non-nil is not enough: each map must accept a write under its lock.
	themeCache.Lock()
	themeCache.themes["__init_test__"] = cachedTheme{}
	delete(themeCache.themes, "__init_test__")
	themeCache.Unlock()

	themeSourcesCache.Lock()
	themeSourcesCache.themes["__init_test__"] = themeSources{}
	delete(themeSourcesCache.themes, "__init_test__")
	themeSourcesCache.Unlock()
}
