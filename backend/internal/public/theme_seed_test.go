package public

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSeedTestRenderer(t *testing.T) (*Renderer, *gorm.DB, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.Page{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dataDir := t.TempDir()
	return NewRenderer(db, "http://localhost", dataDir), db, dataDir
}

func seedAdminUser(t *testing.T, db *gorm.DB) model.User {
	t.Helper()
	u := model.User{Username: "admin", Email: "admin@example.com", Role: model.RoleSuperAdmin}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return u
}

func writeCustomThemeSeed(t *testing.T, dataDir, theme, name, content string) {
	t.Helper()
	dir := filepath.Join(dataDir, ThemesDir, theme, seedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	// Custom themes require a manifest for ThemeExists, though seeding itself
	// only needs the seed directory.
	meta := filepath.Join(dataDir, ThemesDir, theme, ThemeMetaFile)
	if _, err := os.Stat(meta); os.IsNotExist(err) {
		if err := os.WriteFile(meta, []byte(`{"id":"`+theme+`","name":"`+theme+`"}`), 0o644); err != nil {
			t.Fatalf("write theme meta: %v", err)
		}
	}
}

func TestParseSeedFile_Frontmatter(t *testing.T) {
	seed := parseSeedFile("links", []byte("---\ntitle: Links\nshowInNav: true\nsortOrder: 101\nstatus: published\n---\n\nBody here.\n"))
	if seed.Title != "Links" || !seed.ShowInNav || seed.SortOrder != 101 {
		t.Errorf("frontmatter not parsed: %+v", seed)
	}
	if seed.Status != model.PageStatusPublished {
		t.Errorf("status = %q", seed.Status)
	}
	if seed.Content != "Body here." {
		t.Errorf("content = %q", seed.Content)
	}
}

func TestParseSeedFile_DefaultsAndBadValues(t *testing.T) {
	seed := parseSeedFile("about", []byte("Just body, no frontmatter.\n"))
	if seed.Title != "about" || seed.Content != "Just body, no frontmatter." {
		t.Errorf("defaults wrong: %+v", seed)
	}
	if seed.Status != model.PageStatusPublished || seed.ShowInNav || seed.SortOrder != 0 {
		t.Errorf("default flags wrong: %+v", seed)
	}

	seed = parseSeedFile("x", []byte("---\ntitle:\nshowInNav: yes\nsortOrder: lots\nstatus: archived\n---\nBody\n"))
	if seed.Title != "x" || seed.ShowInNav || seed.SortOrder != 0 {
		t.Errorf("bad values should fall back: %+v", seed)
	}
	if seed.Status != model.PageStatusPublished {
		t.Errorf("bad status should fall back to published, got %q", seed.Status)
	}
}

func TestLoadThemeSeeds_CustomTheme(t *testing.T) {
	r, _, dataDir := newSeedTestRenderer(t)
	writeCustomThemeSeed(t, dataDir, "blue", "about.md", "---\ntitle: About\n---\nHi\n")
	writeCustomThemeSeed(t, dataDir, "blue", "admin.md", "---\ntitle: Admin\n---\nReserved\n")
	writeCustomThemeSeed(t, dataDir, "blue", "Uppercase.md", "---\ntitle: Upper\n---\nSkipped\n")
	writeCustomThemeSeed(t, dataDir, "blue", "note.txt", "not a seed")

	seeds, err := r.LoadThemeSeeds("blue")
	if err != nil {
		t.Fatalf("LoadThemeSeeds: %v", err)
	}
	if len(seeds) != 2 || seeds[0].Slug != "about" || seeds[1].Slug != "admin" {
		t.Fatalf("unexpected seeds: %+v", seeds)
	}
}

func TestLoadThemeSeeds_MissingDir(t *testing.T) {
	r, _, _ := newSeedTestRenderer(t)
	seeds, err := r.LoadThemeSeeds("default")
	if err != nil {
		t.Fatalf("default theme seeds: %v", err)
	}
	// The embedded default theme ships timeline + links seeds.
	if len(seeds) != 2 || seeds[0].Slug != "links" || seeds[1].Slug != "timeline" {
		t.Fatalf("unexpected default seeds: %+v", seeds)
	}
	if seeds[1].Title != "Timeline" || !seeds[1].ShowInNav || seeds[1].SortOrder != 100 {
		t.Errorf("timeline seed wrong: %+v", seeds[1])
	}

	r2, _, _ := newSeedTestRenderer(t)
	// A missing theme directory (or one without seed/) means "no seeds",
	// never an error: EnsureThemeSeeds stays a safe no-op there.
	seeds, err = r2.LoadThemeSeeds("no-such-theme")
	if err != nil || seeds != nil {
		t.Errorf("missing theme should yield no seeds, got seeds=%v err=%v", seeds, err)
	}
	if _, err := r2.LoadThemeSeeds("../escape"); err == nil {
		t.Errorf("invalid theme id should error")
	}
}

func TestEnsureThemeSeeds_CreatesSkipsAndPreserves(t *testing.T) {
	r, db, dataDir := newSeedTestRenderer(t)
	admin := seedAdminUser(t, db)
	ctx := context.Background()

	writeCustomThemeSeed(t, dataDir, "blue", "about.md", "---\ntitle: About\nshowInNav: true\nsortOrder: 5\n---\nTheme body\n")
	writeCustomThemeSeed(t, dataDir, "blue", "contact.md", "---\ntitle: Contact\n---\nMail me\n")
	writeCustomThemeSeed(t, dataDir, "blue", "admin.md", "---\ntitle: Admin\n---\nReserved slug\n")

	// Pre-existing user-edited page must never be overwritten.
	existing := model.Page{Slug: "about", Title: "About", Content: "User body", Status: model.PageStatusPublished, AuthorID: admin.ID}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}

	created, err := r.EnsureThemeSeeds(ctx, "blue")
	if err != nil {
		t.Fatalf("EnsureThemeSeeds: %v", err)
	}
	if created != 1 {
		t.Errorf("expected 1 created (contact), got %d", created)
	}

	var about model.Page
	if err := db.Where("slug = ?", "about").First(&about).Error; err != nil {
		t.Fatal(err)
	}
	if about.Content != "User body" {
		t.Errorf("existing page overwritten: %q", about.Content)
	}
	var contact model.Page
	if err := db.Where("slug = ?", "contact").First(&contact).Error; err != nil {
		t.Fatalf("contact not created: %v", err)
	}
	if contact.Title != "Contact" || contact.AuthorID != admin.ID || contact.Status != model.PageStatusPublished {
		t.Errorf("contact wrong: %+v", contact)
	}
	var reserved int64
	if err := db.Model(&model.Page{}).Where("slug = ?", "admin").Count(&reserved).Error; err != nil {
		t.Fatal(err)
	}
	if reserved != 0 {
		t.Errorf("reserved slug must be skipped")
	}

	// Second run is a no-op.
	created, err = r.EnsureThemeSeeds(ctx, "blue")
	if err != nil || created != 0 {
		t.Errorf("second run should create nothing, got created=%d err=%v", created, err)
	}
}

func TestEnsureThemeSeeds_DraftAndNoAdmin(t *testing.T) {
	r, db, dataDir := newSeedTestRenderer(t)
	ctx := context.Background()
	writeCustomThemeSeed(t, dataDir, "blue", "soon.md", "---\ntitle: Soon\nstatus: draft\n---\nLater\n")

	// Without any admin user there is nobody to attribute pages to.
	created, err := r.EnsureThemeSeeds(ctx, "blue")
	if err != nil || created != 0 {
		t.Errorf("expected skip without admin, got created=%d err=%v", created, err)
	}

	seedAdminUser(t, db)
	created, err = r.EnsureThemeSeeds(ctx, "blue")
	if err != nil || created != 1 {
		t.Fatalf("expected 1 created, got %d err=%v", created, err)
	}
	var page model.Page
	if err := db.Where("slug = ?", "soon").First(&page).Error; err != nil {
		t.Fatal(err)
	}
	if page.Status != model.PageStatusDraft {
		t.Errorf("draft status not honored: %q", page.Status)
	}
}
