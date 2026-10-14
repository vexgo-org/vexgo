package asset

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestKeyGenerator_Generate(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantExt  string
		wantErr  bool
	}{
		// Allowed extensions survive, in the allowlist's own casing.
		{name: "jpg", filename: "photo.jpg", wantExt: ".jpg"},
		{name: "jpeg", filename: "photo.jpeg", wantExt: ".jpeg"},
		{name: "png", filename: "photo.png", wantExt: ".png"},
		{name: "gif", filename: "loop.gif", wantExt: ".gif"},
		{name: "webp", filename: "photo.webp", wantExt: ".webp"},
		{name: "avif", filename: "photo.avif", wantExt: ".avif"},
		{name: "bmp", filename: "photo.bmp", wantExt: ".bmp"},
		{name: "ico", filename: "favicon.ico", wantExt: ".ico"},
		{name: "tiff", filename: "scan.tiff", wantExt: ".tiff"},
		{name: "svg", filename: "icon.svg", wantExt: ".svg"},
		{name: "mp4", filename: "clip.mp4", wantExt: ".mp4"},
		{name: "webm", filename: "clip.webm", wantExt: ".webm"},
		{name: "mov", filename: "clip.mov", wantExt: ".mov"},
		{name: "mp3", filename: "song.mp3", wantExt: ".mp3"},
		{name: "wav", filename: "song.wav", wantExt: ".wav"},
		{name: "ogg", filename: "song.ogg", wantExt: ".ogg"},
		{name: "m4a", filename: "song.m4a", wantExt: ".m4a"},
		{name: "pdf", filename: "paper.pdf", wantExt: ".pdf"},
		{name: "txt", filename: "notes.txt", wantExt: ".txt"},
		{name: "md", filename: "notes.md", wantExt: ".md"},
		{name: "csv", filename: "table.csv", wantExt: ".csv"},

		// Casing is normalized, including mixed case.
		{name: "uppercase extension", filename: "PHOTO.JPG", wantExt: ".jpg"},
		{name: "mixed case extension", filename: "Song.Mp3", wantExt: ".mp3"},
		{name: "uppercase svg", filename: "ICON.SVG", wantExt: ".svg"},

		// Only the final segment's extension counts.
		{name: "multiple dots", filename: "report.final.pdf", wantExt: ".pdf"},
		{name: "dot in directory-like prefix", filename: "v1.2/photo.png", wantExt: ".png"},
		{name: "spaces in name", filename: "my holiday photo.png", wantExt: ".png"},
		{name: "unicode name", filename: "写真.png", wantExt: ".png"},

		// Path traversal cannot survive a UUID key: the name is dropped, so the
		// stored object is `uuid.ext` and never a client-controlled path.
		{name: "traversal attempt", filename: "../../etc/passwd.png", wantExt: ".png"},
		{name: "absolute path", filename: "/etc/shadow.png", wantExt: ".png"},
		{name: "backslash path", filename: `..\..\windows\photo.png`, wantExt: ".png"},

		// Rejected: missing or unusable extension.
		{name: "empty filename", filename: "", wantErr: true},
		{name: "no extension", filename: "README", wantErr: true},
		{name: "trailing dot", filename: "archive.", wantErr: true},
		{name: "dotfile", filename: ".gitignore", wantErr: true},
		{name: "extension only", filename: ".jpg", wantExt: ".jpg"},
		{name: "extensionless in directory", filename: "docs/notes", wantErr: true},
		{name: "trailing space", filename: "photo.png ", wantErr: true},

		// Rejected: script-bearing document types are a stored XSS vector
		// because local uploads are served from the site's own origin.
		{name: "html", filename: "page.html", wantErr: true},
		{name: "htm", filename: "page.htm", wantErr: true},
		{name: "xhtml", filename: "page.xhtml", wantErr: true},
		{name: "xml", filename: "feed.xml", wantErr: true},
		{name: "js", filename: "app.js", wantErr: true},
		{name: "html with double extension", filename: "photo.png.html", wantErr: true},
		{name: "uppercase html", filename: "PAGE.HTML", wantErr: true},

		// Rejected: everything outside the allowlist.
		{name: "exe", filename: "installer.exe", wantErr: true},
		{name: "gz", filename: "archive.tar.gz", wantErr: true},
		{name: "zip", filename: "archive.zip", wantErr: true},
		{name: "sh", filename: "run.sh", wantErr: true},
		{name: "php", filename: "index.php", wantErr: true},
		{name: "svgz", filename: "icon.svgz", wantErr: true},
		{name: "apk", filename: "app.apk", wantErr: true},
		{name: "docx", filename: "report.docx", wantErr: true},
	}

	generator := NewKeyGenerator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := generator.Generate(tt.filename)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Generate(%q) = %q, want an error", tt.filename, key)
				}
				if !errors.Is(err, ErrInvalidExtension) {
					t.Errorf("Generate(%q) error = %v, want ErrInvalidExtension", tt.filename, err)
				}
				if key != "" {
					t.Errorf("Generate(%q) returned key %q alongside an error, want empty", tt.filename, key)
				}
				// The offending extension is named in the error so the handler
				// can report what was rejected.
				if want := strings.ToLower(path.Ext(tt.filename)); !strings.Contains(err.Error(), want) {
					t.Errorf("Generate(%q) error = %q, want it to name %q", tt.filename, err.Error(), want)
				}
				return
			}

			if err != nil {
				t.Fatalf("Generate(%q) error = %v", tt.filename, err)
			}
			if !strings.HasSuffix(key, tt.wantExt) {
				t.Errorf("Generate(%q) = %q, want suffix %q", tt.filename, key, tt.wantExt)
			}

			// The stem must be a bare UUID: no directory separators, no client
			// text, nothing that can escape the storage root.
			stem := strings.TrimSuffix(key, tt.wantExt)
			if _, err := uuid.Parse(stem); err != nil {
				t.Errorf("Generate(%q) stem %q is not a UUID: %v", tt.filename, stem, err)
			}
			if strings.ContainsAny(key, `/\`) {
				t.Errorf("Generate(%q) = %q, want a key without path separators", tt.filename, key)
			}
			if stem == "" {
				t.Errorf("Generate(%q) = %q, want a UUID stem", tt.filename, key)
			}
		})
	}
}

// TestKeyGenerator_GenerateIsUnique guards the only thing that keeps two
// uploads of the same filename apart: a fresh UUID per call. A fixed UUID would
// collide on the unique StorageKey index and fail the second upload.
func TestKeyGenerator_GenerateIsUnique(t *testing.T) {
	generator := NewKeyGenerator()

	seen := make(map[string]struct{}, 100)
	for range 100 {
		key, err := generator.Generate("photo.png")
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		if _, dup := seen[key]; dup {
			t.Fatalf("Generate returned the duplicate key %q", key)
		}
		seen[key] = struct{}{}
	}
}

// TestKeyGenerator_DoesNotMutateAllowlist makes sure the map backing the
// allowlist is only ever read: a write would let one upload widen what every
// later upload accepts.
func TestKeyGenerator_DoesNotMutateAllowlist(t *testing.T) {
	before := len(allowedExts)

	for _, name := range []string{"evil.html", "evil.sh", "evil.exe", "ok.png"} {
		_, _ = NewKeyGenerator().Generate(name)
	}

	if after := len(allowedExts); after != before {
		t.Errorf("allowedExts changed size: %d -> %d", before, after)
	}
	if _, ok := allowedExts[".html"]; ok {
		t.Error(".html leaked into allowedExts")
	}
}
