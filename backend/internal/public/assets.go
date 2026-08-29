package public

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"strings"
)

// AssetManifest holds the mapping of logical asset names to their hashed filenames
type AssetManifest struct {
	CSS map[string]string `json:"css"`
	JS  map[string]string `json:"js"`
}

var manifest AssetManifest

// viteManifestEntry mirrors a single entry of Vite's manifest.json (build.manifest).
// Vite keys the manifest by chunk id (e.g. "index.html" for the entry and
// "_react-vendor-<hash>.js" for manualChunks), so the entry must be located via
// IsEntry rather than by guessing the asset name from the filename.
type viteManifestEntry struct {
	File    string   `json:"file"`
	Name    string   `json:"name"`
	CSS     []string `json:"css"`
	IsEntry bool     `json:"isEntry"`
	Src     string   `json:"src"`
}

// LoadAssetManifest loads the asset manifest from the embedded filesystem.
// The frontend build copies Vite's manifest to dist/manifest.json (Vite writes
// it to the hidden .vite/ directory, which //go:embed excludes). If it is
// missing we fall back to scanning the assets directory.
func LoadAssetManifest() error {
	manifestData, err := staticFS.ReadFile("dist/manifest.json")
	if err != nil {
		return buildAssetManifest()
	}

	parsed, err := parseViteManifest(manifestData)
	if err != nil {
		slog.Warn("failed to parse Vite manifest, falling back to directory scan", "err", err)
		return buildAssetManifest()
	}
	manifest = parsed
	return nil
}

// parseViteManifest parses Vite's manifest.json (build.manifest) into an
// AssetManifest. The entry chunk is the only authoritative source for the
// "index" assets: code-split chunks (e.g. codemirror language modes) are also
// emitted as index-<hash>.js files and carry the name "index", so name-based
// matching would pick a wrong chunk.
func parseViteManifest(data []byte) (AssetManifest, error) {
	var viteManifest map[string]viteManifestEntry
	if err := json.Unmarshal(data, &viteManifest); err != nil {
		return AssetManifest{}, err
	}

	m := AssetManifest{
		CSS: make(map[string]string),
		JS:  make(map[string]string),
	}

	// The entry chunk defines the "index" assets.
	for _, entry := range viteManifest {
		if !entry.IsEntry {
			continue
		}
		if entry.File != "" && strings.HasSuffix(entry.File, ".js") {
			m.JS["index"] = "/" + entry.File
		}
		for _, css := range entry.CSS {
			if m.CSS["index"] == "" {
				m.CSS["index"] = "/" + css
			}
		}
	}

	// Named chunks (manualChunks vendors like react-vendor, ui-vendor,
	// utils-vendor). Skip the name "index": the entry above is the only source
	// for it.
	for _, entry := range viteManifest {
		if entry.IsEntry || entry.Name == "" || entry.Name == "index" || entry.File == "" {
			continue
		}
		switch {
		case strings.HasSuffix(entry.File, ".js"):
			m.JS[entry.Name] = "/" + entry.File
		case strings.HasSuffix(entry.File, ".css"):
			m.CSS[entry.Name] = "/" + entry.File
		}
	}

	return m, nil
}

// assetReferencesFromIndexHTML returns the asset URLs referenced by the
// vite-built dist/index.html (the authoritative entry references).
func assetReferencesFromIndexHTML() map[string]bool {
	refs := map[string]bool{}
	re := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(string(indexHTML), -1) {
		refs[m[1]] = true
	}
	return refs
}

// buildAssetManifest builds the manifest by scanning the assets directory.
// This is only a fallback when no Vite manifest.json is embedded. Multiple
// builds may leave stale index-<hash>.js files around, and a single build
// already emits several index-<hash>.js chunks (code-split modules), so when
// several candidates share a name we prefer the one referenced by the
// vite-built index.html, which names the true entry chunk.
func buildAssetManifest() error {
	var urls []string
	err := fs.WalkDir(staticFS, "dist/assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := path.Ext(p)
		if ext != ".css" && ext != ".js" {
			return nil
		}
		urls = append(urls, "/assets/"+path.Base(p))
		return nil
	})
	if err != nil {
		return err
	}

	manifest = buildAssetManifestFromURLs(urls, assetReferencesFromIndexHTML())
	return nil
}

// buildAssetManifestFromURLs builds an AssetManifest from a list of asset URLs
// (e.g. "/assets/index-DsXrKOv2.js"). When several assets share the same
// logical name, the one referenced by the vite-built index.html wins;
// otherwise the first one encountered is kept.
func buildAssetManifestFromURLs(urls []string, referenced map[string]bool) AssetManifest {
	m := AssetManifest{
		CSS: make(map[string]string),
		JS:  make(map[string]string),
	}

	for _, assetURL := range urls {
		filename := path.Base(assetURL)
		ext := path.Ext(filename)
		if ext != ".css" && ext != ".js" {
			continue
		}

		// Extract asset name from filename pattern: {name}-{hash}.{ext}
		// e.g., "index-DR2bYJZO.js" -> name="index", ext="js"
		base := strings.TrimSuffix(filename, ext)
		idx := strings.LastIndex(base, "-")
		if idx == -1 {
			continue
		}
		assetName := base[:idx]

		if ext == ".css" {
			if existing, ok := m.CSS[assetName]; ok && existing != assetURL && !referenced[assetURL] {
				slog.Warn(
					"duplicate CSS asset",
					"asset", assetName,
					"existing", existing,
					"duplicate", assetURL,
				)
				continue
			}
			m.CSS[assetName] = assetURL
		} else {
			if existing, ok := m.JS[assetName]; ok && existing != assetURL && !referenced[assetURL] {
				slog.Warn(
					"duplicate JS asset",
					"asset", assetName,
					"existing", existing,
					"duplicate", assetURL,
				)
				continue
			}
			m.JS[assetName] = assetURL
		}
	}

	return m
}

// GetAssetURL returns the actual URL for a logical asset name
func GetAssetURL(assetType, name string) string {
	var manifestMap map[string]string
	switch assetType {
	case "css":
		manifestMap = manifest.CSS
	case "js":
		manifestMap = manifest.JS
	default:
		return ""
	}

	return manifestMap[name]
}
