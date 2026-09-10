package public

import (
	"strings"
	"testing"
)

// viteManifestFixture mirrors the structure Vite 7 emits in
// .vite/manifest.json: the entry is keyed "index.html" with isEntry:true,
// manualChunks carry their chunk name, and code-split modules (codemirror
// language modes) also carry the name "index".
const viteManifestFixture = `{
  "_react-vendor-BmqGXi6J.js": {
    "file": "assets/react-vendor-BmqGXi6J.js",
    "name": "react-vendor"
  },
  "_ui-vendor-CEsCEvQe.js": {
    "file": "assets/ui-vendor-CEsCEvQe.js",
    "name": "ui-vendor",
    "imports": ["_react-vendor-BmqGXi6J.js"]
  },
  "_utils-vendor-42ANG6Sg.js": {
    "file": "assets/utils-vendor-42ANG6Sg.js",
    "name": "utils-vendor"
  },
  "index.html": {
    "file": "assets/index-DsXrKOv2.js",
    "name": "index",
    "src": "index.html",
    "isEntry": true,
    "imports": ["_ui-vendor-CEsCEvQe.js", "_react-vendor-BmqGXi6J.js", "_utils-vendor-42ANG6Sg.js"],
    "css": ["assets/index-BCv7Z314.css"]
  },
  "node_modules/@codemirror/lang-python/dist/index.js": {
    "file": "assets/index-JCx2MGp4.js",
    "name": "index"
  },
  "node_modules/@codemirror/lang-vue/dist/index.js": {
    "file": "assets/index-CMEKYH0O.js",
    "name": "index"
  }
}`

func TestParseViteManifest(t *testing.T) {
	m, err := parseViteManifest([]byte(viteManifestFixture))
	if err != nil {
		t.Fatalf("parseViteManifest returned error: %v", err)
	}

	// The entry chunk must map to the "index" assets.
	if got := m.JS["index"]; got != "/assets/index-DsXrKOv2.js" {
		t.Fatalf("JS[index] = %q, want %q", got, "/assets/index-DsXrKOv2.js")
	}
	if got := m.CSS["index"]; got != "/assets/index-BCv7Z314.css" {
		t.Fatalf("CSS[index] = %q, want %q", got, "/assets/index-BCv7Z314.css")
	}

	// ManualChunks vendors must map by their chunk name.
	for name, want := range map[string]string{
		"react-vendor": "/assets/react-vendor-BmqGXi6J.js",
		"ui-vendor":    "/assets/ui-vendor-CEsCEvQe.js",
		"utils-vendor": "/assets/utils-vendor-42ANG6Sg.js",
	} {
		if got := m.JS[name]; got != want {
			t.Fatalf("JS[%s] = %q, want %q", name, got, want)
		}
	}
}

// The original bug: code-split chunks (codemirror language modes) are emitted
// as index-<hash>.js and also carry the name "index". They must never
// override the entry chunk, no matter the map iteration order.
func TestParseViteManifestCodeSplitIndexDoesNotOverrideEntry(t *testing.T) {
	m, err := parseViteManifest([]byte(viteManifestFixture))
	if err != nil {
		t.Fatalf("parseViteManifest returned error: %v", err)
	}

	if got := m.JS["index"]; got != "/assets/index-DsXrKOv2.js" {
		t.Fatalf("JS[index] = %q, want entry chunk %q", got, "/assets/index-DsXrKOv2.js")
	}
	for _, split := range []string{"/assets/index-JCx2MGp4.js", "/assets/index-CMEKYH0O.js"} {
		if got := m.JS["index"]; got == split {
			t.Fatalf("JS[index] = %q, code-split chunk must not win", got)
		}
	}
}

func TestParseViteManifestMissingEntry(t *testing.T) {
	// No isEntry chunk: maps must be non-nil and empty, not an error.
	m, err := parseViteManifest([]byte(`{
		"_react-vendor-BmqGXi6J.js": {"file": "assets/react-vendor-BmqGXi6J.js", "name": "react-vendor"}
	}`))
	if err != nil {
		t.Fatalf("parseViteManifest returned error: %v", err)
	}
	if m.JS == nil || m.CSS == nil {
		t.Fatalf("maps should be initialized, got JS=%v CSS=%v", m.JS, m.CSS)
	}
	if got := m.JS["index"]; got != "" {
		t.Fatalf("JS[index] = %q, want empty when no entry", got)
	}
	if got := m.JS["react-vendor"]; got != "/assets/react-vendor-BmqGXi6J.js" {
		t.Fatalf("JS[react-vendor] = %q, want %q", got, "/assets/react-vendor-BmqGXi6J.js")
	}
}

func TestParseViteManifestInvalidJSON(t *testing.T) {
	if _, err := parseViteManifest([]byte("not json")); err == nil {
		t.Fatal("parseViteManifest should error on invalid JSON")
	}
}

func TestParseViteManifestCSSChunk(t *testing.T) {
	m, err := parseViteManifest([]byte(`{
		"some-style.css": {"file": "assets/theme-Da3fG1.css", "name": "theme"}
	}`))
	if err != nil {
		t.Fatalf("parseViteManifest returned error: %v", err)
	}
	if got := m.CSS["theme"]; got != "/assets/theme-Da3fG1.css" {
		t.Fatalf("CSS[theme] = %q, want %q", got, "/assets/theme-Da3fG1.css")
	}
}

func TestBuildAssetManifestFromURLsPrefersReferencedEntry(t *testing.T) {
	// A stale build leaves extra index-<hash>.js files around; the file
	// referenced by dist/index.html is the true entry and must win.
	urls := []string{
		"/assets/index-A1b2C3d4.js", // stale old build
		"/assets/index-DsXrKOv2.js", // referenced by dist/index.html
		"/assets/index-JCx2MGp4.js", // code-split chunk sharing the "index" name
	}
	referenced := map[string]bool{"/assets/index-DsXrKOv2.js": true}

	m := buildAssetManifestFromURLs(urls, referenced)
	if got := m.JS["index"]; got != "/assets/index-DsXrKOv2.js" {
		t.Fatalf("JS[index] = %q, want referenced entry %q", got, "/assets/index-DsXrKOv2.js")
	}
}

func TestBuildAssetManifestFromURLsReferencedEntryFirst(t *testing.T) {
	urls := []string{
		"/assets/index-DsXrKOv2.js", // referenced by dist/index.html
		"/assets/index-A1b2C3d4.js", // stale old build
	}
	referenced := map[string]bool{"/assets/index-DsXrKOv2.js": true}

	m := buildAssetManifestFromURLs(urls, referenced)
	if got := m.JS["index"]; got != "/assets/index-DsXrKOv2.js" {
		t.Fatalf("JS[index] = %q, want referenced entry %q", got, "/assets/index-DsXrKOv2.js")
	}
}

func TestBuildAssetManifestFromURLsNoReferenceKeepsFirst(t *testing.T) {
	// Without any index.html reference (defensive fallback), the first
	// alphabetical candidate wins deterministically.
	urls := []string{
		"/assets/index-A1b2C3d4.js",
		"/assets/index-DsXrKOv2.js",
	}

	m := buildAssetManifestFromURLs(urls, map[string]bool{})
	if got := m.JS["index"]; got != "/assets/index-A1b2C3d4.js" {
		t.Fatalf("JS[index] = %q, want first candidate %q", got, "/assets/index-A1b2C3d4.js")
	}
}

func TestBuildAssetManifestFromURLsSkipsNonAssets(t *testing.T) {
	urls := []string{
		"/assets/index-DsXrKOv2.js",
		"/assets/index-DsXrKOv2.js.map", // source map: not css/js
		"/assets/logo.svg",              // not css/js
		"/assets/no-hash.js",            // no "-" separator: no asset name
		"/assets/react-vendor-BmqGXi6J.js",
	}

	m := buildAssetManifestFromURLs(urls, map[string]bool{})
	if got := m.JS["index"]; got != "/assets/index-DsXrKOv2.js" {
		t.Fatalf("JS[index] = %q, want %q", got, "/assets/index-DsXrKOv2.js")
	}
	if got := m.JS["react-vendor"]; got != "/assets/react-vendor-BmqGXi6J.js" {
		t.Fatalf("JS[react-vendor] = %q, want %q", got, "/assets/react-vendor-BmqGXi6J.js")
	}
	if _, ok := m.JS["logo"]; ok {
		t.Fatal("non js/css assets must not be added to the manifest")
	}
}

func TestBuildAssetManifestFromURLsSeparatesCSS(t *testing.T) {
	urls := []string{
		"/assets/index-DsXrKOv2.js",
		"/assets/index-BCv7Z314.css",
	}

	m := buildAssetManifestFromURLs(urls, map[string]bool{})
	if got := m.CSS["index"]; got != "/assets/index-BCv7Z314.css" {
		t.Fatalf("CSS[index] = %q, want %q", got, "/assets/index-BCv7Z314.css")
	}
}

// TestLoadAssetManifestFromEmbeddedDist guards the whole SSR asset-resolution
// pipeline: after loading, the "index" JS must be the entry chunk referenced
// by the vite-built dist/index.html.
func TestLoadAssetManifestFromEmbeddedDist(t *testing.T) {
	if err := LoadAssetManifest(); err != nil {
		t.Fatalf("LoadAssetManifest returned error: %v", err)
	}

	entry := GetAssetURL("js", "index")
	if entry == "" {
		t.Fatal("GetAssetURL(js, index) is empty")
	}
	if !strings.HasPrefix(entry, "/assets/index-") {
		t.Fatalf("GetAssetURL(js, index) = %q, want /assets/index-*", entry)
	}
	if !assetReferencesFromIndexHTML()[entry] {
		t.Fatalf("entry %q is not referenced by dist/index.html", entry)
	}

	if css := GetAssetURL("css", "index"); css == "" || !strings.HasPrefix(css, "/assets/") {
		t.Fatalf("GetAssetURL(css, index) = %q, want /assets/*", css)
	}
	for _, name := range []string{"react-vendor", "ui-vendor", "utils-vendor"} {
		if u := GetAssetURL("js", name); u == "" || !strings.HasPrefix(u, "/assets/") {
			t.Fatalf("GetAssetURL(js, %s) = %q, want /assets/*", name, u)
		}
	}
}

func TestGetAssetURLUnknown(t *testing.T) {
	manifest = AssetManifest{
		CSS: map[string]string{"index": "/assets/index-BCv7Z314.css"},
		JS:  map[string]string{"index": "/assets/index-DsXrKOv2.js"},
	}
	if got := GetAssetURL("js", "nope"); got != "" {
		t.Fatalf("GetAssetURL(js, nope) = %q, want empty", got)
	}
	if got := GetAssetURL("html", "index"); got != "" {
		t.Fatalf("GetAssetURL(html, index) = %q, want empty", got)
	}
}
