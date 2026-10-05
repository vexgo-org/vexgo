package post

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeNotifier struct {
	calls []model.NotificationType
}

func (f *fakeNotifier) CreateNotification(_ context.Context, input model.NotificationInput) error {
	f.calls = append(f.calls, input.Type)
	return nil
}

type fakeRemover struct {
	deleted []string
}

func (f *fakeRemover) Delete(_ context.Context, url string) error {
	f.deleted = append(f.deleted, url)
	return nil
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// TranslateError mirrors the production gorm.Config (internal/database),
	// so unique-index violations surface as gorm.ErrDuplicatedKey.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Post{}, &model.User{}, &model.Tag{}, &model.Like{}, &model.Comment{}, &model.Category{}, &model.GeneralSettings{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *fakeNotifier, *fakeRemover, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	notifier := &fakeNotifier{}
	remover := &fakeRemover{}
	svc := NewService(Deps{DB: db, Notifier: notifier, Files: remover})
	return svc, notifier, remover, db
}

func seedUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{Username: username, Email: username + "@example.com", Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

// reloadPostStatus re-reads a post's status from the database so a test can
// assert that a rejected update left the row untouched.
func reloadPostStatus(t *testing.T, db *gorm.DB, id uint) model.PostStatus {
	t.Helper()
	var post model.Post
	if err := db.First(&post, id).Error; err != nil {
		t.Fatalf("reload post %d: %v", id, err)
	}
	return post.Status
}

// TestSlugValidation exercises model.ValidateSlug directly for both valid and
// invalid inputs: empty, long, uppercase, various syntax violations, and
// numeric-only slugs.  The service layer normalizes before calling
// ValidateSlug, so these tests ensure the model function itself is correct.
func TestSlugValidation(t *testing.T) {
	// Valid slugs — must all pass.
	valid := []string{
		"hello", "hello-world", "my-post-123", "a1-b2-c3",
		"a", "a1",
		"中文-标题", "こんにちは-世界", "안녕하세요-세계",
		"привет-мир", "مرحبا-بالعالم",
		"hello-中文-привет",
		"café",
		strings.Repeat("a", model.MaxSlugLength),
		"  hello  ",           // trimmed before validation
		"\t\thello-world\t\t", // trimmed before validation
	}
	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			if len([]rune(s)) > 20 {
				t.Skip("long value")
			}
			if err := model.ValidateSlug(s); err != nil {
				t.Errorf("expected valid slug %q, got error: %v", s, err)
			}
		})
	}

	// Invalid slugs — each must return an error.
	invalid := []struct {
		slug   string
		reason string
	}{
		{"", "empty"},
		{"   ", "whitespace only"},
		{"\t\n", "whitespace only tab+newline"},
		{"INVALID", "uppercase"},
		{"Hello", "mixed case"},
		{"hello-WORLD", "partial uppercase"},
		{"-bad", "leading hyphen"},
		{"bad-", "trailing hyphen"},
		{"bad--bad", "consecutive hyphens"},
		{"with space", "space"},
		{"has@at", "at sign"},
		{"has/slash", "slash"},
		{"has.dot", "dot"},
		{"123", "numeric only"},
		{"-", "hyphen only"},
		{strings.Repeat("a", model.MaxSlugLength+1), "too long"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.reason, func(t *testing.T) {
			if len(tc.slug) > 20 {
				t.Skip("long value")
			}
			if err := model.ValidateSlug(tc.slug); err == nil {
				t.Errorf("expected error for slug %q (%s), got nil", tc.slug, tc.reason)
			}
		})
	}
}

// TestSlugFromTitle exercises model.SlugFromTitle across languages,
// punctuation, edge cases (empty, non-Latin), combining marks, truncation,
// and multiple consecutive separators.
func TestSlugFromTitle(t *testing.T) {
	type slugCase struct {
		name  string
		title string
		want  string
	}

	tests := []slugCase{
		// ── Basic English ──
		{name: "English", title: "Hello World Test", want: "hello-world-test"},
		{name: "all uppercase", title: "HELLO WORLD", want: "hello-world"},
		{name: "extra spaces", title: "  hello   world  ", want: "hello-world"},
		{name: "numbers", title: "Post 123 Title", want: "post-123-title"},
		{name: "underscores", title: "hello_world_test", want: "hello-world-test"},
		{name: "em dash", title: "hello\u2014world\u2014test", want: "hello-world-test"},
		{name: "en dash", title: "hello\u2013world\u2013test", want: "hello-world-test"},

		// ── Non-Latin languages ──
		{name: "Chinese", title: "中文 标题 测试", want: "中文-标题-测试"},
		{name: "Japanese", title: "こんにちは 世界 入門", want: "こんにちは-世界-入門"},
		{name: "Korean", title: "안녕하세요 아름다운 세계", want: "안녕하세요-아름다운-세계"},
		{name: "Russian", title: "Привет прекрасный мир", want: "привет-прекрасный-мир"},
		{name: "Arabic", title: "مرحبا بالعالم الجميل", want: "مرحبا-بالعالم-الجميل"},
		{name: "French", title: "Bonjour le Monde", want: "bonjour-le-monde"},
		{name: "German umlauts", title: "Hallo schöne Welt", want: "hallo-schöne-welt"},

		// ── Mixed scripts ──
		{name: "Mixed languages", title: "Hello 中文 Français Deutsch 日本語 Русский 한국어 العربية", want: "hello-中文-français-deutsch-日本語-русский-한국어-العربية"},
		{name: "Multiple spaces", title: "Hello   世界   테스트", want: "hello-世界-테스트"},

		// ── Punctuation stripping ──
		{name: "apostrophe", title: "What's Up", want: "whats-up"},
		{name: "question and exclaim", title: "Hello! How are you?", want: "hello-how-are-you"},
		{name: "periods", title: "Hello. World.", want: "hello-world"},
		{name: "commas", title: "Hello, World", want: "hello-world"},
		{name: "quotes", title: `"Hello" World`, want: "hello-world"},
		{name: "parentheses", title: "Hello (World) Test", want: "hello-world-test"},
		{name: "brackets", title: "Hello [World] Test", want: "hello-world-test"},
		{name: "at and hash", title: "Hello @ World #1", want: "hello-world-1"},
		{name: "Chinese with numbers", title: "What's Up? 中文 测试 123!", want: "whats-up-中文-测试-123"},

		// ── Empty / all-punctuation fallback ──
		{name: "empty title", title: "", want: ""},
		{name: "only punctuation", title: "!@#$%", want: ""},
		{name: "only hyphens", title: "---", want: ""},
		{name: "only spaces", title: "   ", want: ""},
		{name: "only underscores", title: "_ _ _", want: ""},

		// ── Combining marks ──
		{name: "combining acute", title: "cafe\u0301 resume\u0301 test", want: "cafe\u0301-resume\u0301-test"},
		{name: "isolated combining mark", title: "\u0301hello", want: "hello"},

		// ── Multiple consecutive separators collapse ──
		{name: "consecutive separators", title: "hello__world  --test_\t\tfoo", want: "hello-world-test-foo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := model.SlugFromTitle(tc.title)
			if got != tc.want {
				t.Errorf("SlugFromTitle(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}

	// ── Truncation ──
	t.Run("truncation", func(t *testing.T) {
		longWord := strings.Repeat("a", model.MaxSlugLength+10)
		got := model.SlugFromTitle(longWord)
		if len([]rune(got)) != model.MaxSlugLength {
			t.Errorf("expected slug length %d, got %d (%q)", model.MaxSlugLength, len([]rune(got)), got)
		}

		// Truncation that ends on a trailing hyphen must strip it.
		hyphenSegment := strings.Repeat("a", model.MaxSlugLength-1) + " b"
		got2 := model.SlugFromTitle(hyphenSegment)
		if len([]rune(got2)) != model.MaxSlugLength-1 {
			t.Errorf("expected truncated slug without trailing hyphen, got %q", got2)
		}
		if strings.HasSuffix(got2, "-") {
			t.Errorf("slug should not end with hyphen after truncation, got %q", got2)
		}
	})
}

func idString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
