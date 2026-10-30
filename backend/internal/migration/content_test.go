package migration

import (
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

func reloadPage(t *testing.T, db *gorm.DB, id uint) model.Page {
	t.Helper()

	var page model.Page
	if err := db.First(&page, id).Error; err != nil {
		t.Fatalf("reload page: %v", err)
	}
	return page
}

func TestMigrateDB_RewritesLegacyMediaURLs(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "content-local.png"
	const legacyURL = "/uploads/" + key
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: legacyURL})

	post := model.Post{
		Slug:    "content-post",
		Title:   "Content post",
		Content: "![image](" + legacyURL + ")\n<img src=\"" + legacyURL + "\">\n\n[external](https://example.com/image.png)",
	}
	if err := db.Omit("Author").Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}
	page := model.Page{
		Slug:    "content-page",
		Title:   "Content page",
		Content: "Body with " + legacyURL + ".",
	}
	if err := db.Omit("Author").Create(&page).Error; err != nil {
		t.Fatalf("seed page: %v", err)
	}

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	migrated := findAssetByOriginalName(t, db, key)
	assetURL := asset.OriginalURL(migrated.PublicID)

	gotPost := reloadPost(t, db, post.ID)
	wantPost := "![image](" + assetURL + ")\n<img src=\"" + assetURL + "\">\n\n[external](https://example.com/image.png)"
	if gotPost.Content != wantPost {
		t.Fatalf("post content = %q, want %q", gotPost.Content, wantPost)
	}

	gotPage := reloadPage(t, db, page.ID)
	wantPage := "Body with " + assetURL + "."
	if gotPage.Content != wantPage {
		t.Fatalf("page content = %q, want %q", gotPage.Content, wantPage)
	}

	// The rewrite is idempotent: a second pass finds nothing to change.
	if err := s.rewriteLegacyMediaURLs(); err != nil {
		t.Fatalf("second rewrite: %v", err)
	}
	if got := reloadPost(t, db, post.ID); got.Content != wantPost {
		t.Fatalf("second rewrite changed post content: %q", got.Content)
	}
}

func TestMigrateDB_RewritesS3MediaURLs(t *testing.T) {
	s, db, local := newMigrationService(t)

	const key = "content-s3.png"
	const legacyURL = "https://cdn.example.com/blog/" + key
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: legacyURL, StorageKey: key})

	post := model.Post{Slug: "s3-post", Title: "S3 post", Content: "![image](" + legacyURL + ")"}
	if err := db.Omit("Author").Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	migrated := findAssetByOriginalName(t, db, key)
	want := "![image](" + asset.OriginalURL(migrated.PublicID) + ")"
	if got := reloadPost(t, db, post.ID); got.Content != want {
		t.Fatalf("post content = %q, want %q", got.Content, want)
	}
}

// Media rows that produced no asset must keep their legacy URLs: the file is
// still served by the legacy route, and rewriting it would only break content.
func TestMigrateDB_LeavesUnmappedMediaURLs(t *testing.T) {
	s, db, local := newMigrationService(t)

	const missingKey = "missing.png"
	const missingURL = "/uploads/" + missingKey

	const documentKey = "document.pdf"
	const documentURL = "/uploads/" + documentKey
	putObject(t, local, documentKey, textPayload)
	seedMediaFile(t, db, model.MediaFile{URL: documentURL})

	content := "![missing](" + missingURL + ")\n[document](" + documentURL + ")"
	post := model.Post{Slug: "unmapped-post", Title: "Unmapped post", Content: content}
	if err := db.Omit("Author").Create(&post).Error; err != nil {
		t.Fatalf("seed post: %v", err)
	}

	if err := s.MigrateDB(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if got := reloadPost(t, db, post.ID); got.Content != content {
		t.Fatalf("content changed for unmapped media: %q", got.Content)
	}
}

func TestLegacyURLReplacer_PrefersLongestMatch(t *testing.T) {
	replacer := newLegacyURLReplacer(map[string]string{
		"/uploads/a":     "/api/assets/short/original",
		"/uploads/a.png": "/api/assets/long/original",
	})

	got := replacer.Replace("![image](/uploads/a.png)")
	if want := "![image](/api/assets/long/original)"; got != want {
		t.Fatalf("Replace() = %q, want %q", got, want)
	}
	if strings.Contains(got, "short") {
		t.Fatalf("shorter prefix won the match: %q", got)
	}
}
