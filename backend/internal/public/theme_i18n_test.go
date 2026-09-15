package public

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestNormalizeLanguage maps locale variants to primary subtags.
func TestNormalizeLanguage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"en", "en"},
		{"EN", "en"},
		{"en-US", "en"},
		{"zh-CN", "zh"},
		{"zh_CN", "zh"},
		{"zh", "zh"},
		{"", ""},
		{"  ", ""},
		{"12", ""},
		{"e", ""},
		{"toolonglanguagecode", ""},
	}
	for _, c := range cases {
		if got := NormalizeLanguage(c.in); got != c.want {
			t.Errorf("NormalizeLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestResolveLanguage checks the ?lang > cookie > Accept-Language > site
// default > en priority chain.
func TestResolveLanguage(t *testing.T) {
	cases := []struct {
		name          string
		query, cookie string
		accept, def   string
		want          string
	}{
		{"query wins", "zh", "en", "en", "en", "zh"},
		{"cookie second", "", "zh", "en", "en", "zh"},
		{"accept third", "", "", "zh-CN,zh;q=0.9", "en", "zh"},
		{"site default", "", "", "", "zh", "zh"},
		{"ultimate en", "", "", "", "", "en"},
		{"invalid query falls through", "!!", "zh", "", "", "zh"},
	}
	for _, c := range cases {
		if got := ResolveLanguage(c.query, c.cookie, c.accept, c.def); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// writeI18nTheme creates a disk theme with one template using t and the
// given language files.
func writeI18nTheme(t *testing.T, r *Renderer, id, template string, langs map[string]string) {
	t.Helper()
	dir := filepath.Join(r.dataDir, ThemesDir, id)
	if err := os.MkdirAll(filepath.Join(dir, "i18n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, IndexTemplate), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	for lang, content := range langs {
		if err := os.WriteFile(filepath.Join(dir, "i18n", lang+".json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestMergedDictFallback ensures per-key fallback: visitor > site default >
// en > key itself, without failing the page.
func TestMergedDictFallback(t *testing.T) {
	r := newTestRenderer(t)
	writeI18nTheme(t, r, "i18nfb", `<html lang="{{.Site.Language}}">{{t "a"}}|{{t "b"}}|{{t "missing.key"}}</html>`, map[string]string{
		"en": `{"a": "A-en", "b": "B-en"}`,
		"zh": `{"a": "A-zh"}`,
	})

	site := &SiteData{Name: "T", Language: "zh", DefaultLanguage: "zh"}
	dict := r.loadMergedDict("i18nfb", "zh", "zh")
	out, err := r.renderThemeWithDict("i18nfb", IndexTemplate, IndexData{Site: site}, dict)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)
	// a comes from zh, b falls back to en, missing key echoes itself.
	for _, want := range []string{`lang="zh"`, "A-zh", "B-en", "missing.key"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

// TestRenderDefaultTheme_Zh guards the shipped zh dictionary: the default
// theme must render Chinese chrome and a dynamic <html lang>.
func TestRenderDefaultTheme_Zh(t *testing.T) {
	r := newTestRenderer(t)
	site := &SiteData{Name: "VexGo", URL: "http://vexgo.example", ItemsPerPage: 20, Language: "zh", DefaultLanguage: "zh"}
	dict := r.loadMergedDict(DefaultTheme, "zh", "zh")
	out, err := r.renderThemeWithDict(DefaultTheme, IndexTemplate, IndexData{
		Site:       site,
		Pagination: &PaginationData{CurrentPage: 1, TotalPages: 2, HasNext: true, NextURL: "/?page=2"},
	}, dict)
	if err != nil {
		t.Fatalf("render zh index: %v", err)
	}
	got := string(out)
	for _, want := range []string{`lang="zh"`, "首页", "下一页", "热门文章", "暂无热门标签"} {
		if !strings.Contains(got, want) {
			t.Errorf("zh index missing %q", want)
		}
	}
	if strings.Contains(got, "{{") {
		t.Errorf("zh index has unrendered placeholders")
	}
}

// TestRenderTheme_UnknownLangFallsBackToEn ensures a language the theme does
// not ship still renders English instead of failing.
func TestRenderTheme_UnknownLangFallsBackToEn(t *testing.T) {
	r := newTestRenderer(t)
	site := &SiteData{Name: "VexGo", URL: "http://vexgo.example", ItemsPerPage: 20, Language: "ja", DefaultLanguage: "en"}
	dict := r.loadMergedDict(DefaultTheme, "ja", "en")
	out, err := r.renderThemeWithDict(DefaultTheme, IndexTemplate, IndexData{
		Site:       site,
		Pagination: &PaginationData{CurrentPage: 1, TotalPages: 1},
	}, dict)
	if err != nil {
		t.Fatalf("render unknown lang: %v", err)
	}
	if !strings.Contains(string(out), "Popular Posts") {
		t.Errorf("expected English fallback, got:\n%s", out)
	}
}

// TestAvailableLanguages lists what the embedded default theme ships.
func TestAvailableLanguages(t *testing.T) {
	r := newTestRenderer(t)
	langs := r.AvailableLanguages(DefaultTheme)
	found := map[string]bool{}
	for _, l := range langs {
		found[l] = true
	}
	if !found["en"] || !found["zh"] {
		t.Errorf("default theme should ship en and zh, got %v", langs)
	}
	if got := r.AvailableLanguages("does-not-exist"); len(got) != 0 {
		t.Errorf("unknown theme should list no languages, got %v", got)
	}
}

// TestResolveRequestLanguage_SetsCookie ensures an explicit ?lang= persists
// for follow-up requests (pagination, next visit).
func TestResolveRequestLanguage_SetsCookie(t *testing.T) {
	r := newTestRenderer(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)

	if got := r.resolveRequestLanguage(c, "en"); got != "zh" {
		t.Fatalf("resolved = %q, want zh", got)
	}
	cookies := w.Result().Cookies()
	found := false
	for _, ck := range cookies {
		if ck.Name == LangCookieName && ck.Value == "zh" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s=zh cookie to be set", LangCookieName)
	}
}
