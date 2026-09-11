package public

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultLanguage is the fallback when no language can be resolved. It keeps
// pre-i18n behavior (English-only themes) unchanged for existing sites.
const DefaultLanguage = "en"

// LangCookieName carries the visitor's explicit language choice so SSR can
// read it. localStorage is not visible to the server and is only used as a
// progressive enhancement by client scripts.
const LangCookieName = "vexgo_lang"

// LangQueryParam sets the language for the current request and persists it
// into the LangCookieName cookie for subsequent visits.
const LangQueryParam = "lang"

// NormalizeLanguage maps inputs like "zh-CN", "zh_CN" or "EN" to the primary
// lowercase subtag ("zh", "en"). It returns "" for empty/invalid input so
// callers can continue down the fallback chain.
func NormalizeLanguage(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return ""
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	if i := strings.Index(lang, ","); i >= 0 {
		lang = lang[:i]
	}
	if i := strings.Index(lang, ";"); i >= 0 {
		lang = lang[:i]
	}
	lang = strings.TrimSpace(lang)
	if i := strings.Index(lang, "-"); i >= 0 {
		lang = lang[:i]
	}
	lang = strings.TrimSpace(lang)
	if len(lang) < 2 || len(lang) > 10 {
		return ""
	}
	for _, r := range lang {
		if r < 'a' || r > 'z' {
			return ""
		}
	}
	return lang
}

// parseAcceptLanguage returns the first usable language from an
// Accept-Language header value (e.g. "zh-CN,zh;q=0.9,en;q=0.8").
func parseAcceptLanguage(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		if lang := NormalizeLanguage(part); lang != "" {
			return lang
		}
	}
	return ""
}

// ResolveLanguage implements the agreed priority:
// ?lang= > cookie > Accept-Language > site default > en.
func ResolveLanguage(queryLang, cookieLang, acceptHeader, siteDefault string) string {
	if lang := NormalizeLanguage(queryLang); lang != "" {
		return lang
	}
	if lang := NormalizeLanguage(cookieLang); lang != "" {
		return lang
	}
	if lang := parseAcceptLanguage(acceptHeader); lang != "" {
		return lang
	}
	if lang := NormalizeLanguage(siteDefault); lang != "" {
		return lang
	}
	return DefaultLanguage
}

// resolveRequestLanguage resolves the theme language for one SSR request. A
// valid ?lang= value is persisted into a 1-year cookie so pagination and
// follow-up visits keep the choice without repeating the query param.
func (r *Renderer) resolveRequestLanguage(c *gin.Context, siteDefault string) string {
	lang := ResolveLanguage(
		c.Query(LangQueryParam),
		cookieValue(c, LangCookieName),
		c.GetHeader("Accept-Language"),
		siteDefault,
	)
	if query := NormalizeLanguage(c.Query(LangQueryParam)); query != "" {
		c.SetCookie(LangCookieName, query, 365*24*3600, "/", "", false, false)
	}
	return lang
}

// cookieValue reads one cookie without treating a missing cookie as an error.
func cookieValue(c *gin.Context, name string) string {
	value, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return value
}

// Dict is one language's flat key -> text lookup for a theme.
type Dict map[string]string

// loadLangDict reads i18n/<lang>.json from a theme. The second return value
// reports whether the file existed and parsed.
func (r *Renderer) loadLangDict(themeID, lang string) (Dict, bool) {
	content, ok := r.readThemeFile(themeID, "i18n/"+lang+".json")
	if !ok {
		return nil, false
	}
	var dict Dict
	if err := json.Unmarshal(content, &dict); err != nil {
		return nil, false
	}
	return dict, true
}

// loadMergedDict merges dictionaries along the fallback chain
// (en < site default < visitor language) so missing keys degrade per key
// instead of failing the whole page. The backend never hardcodes any
// language: whatever the theme ships under i18n/ is what can be served.
func (r *Renderer) loadMergedDict(themeID, lang, siteDefault string) Dict {
	merged := Dict{}
	for _, candidate := range []string{DefaultLanguage, NormalizeLanguage(siteDefault), NormalizeLanguage(lang)} {
		if candidate == "" {
			continue
		}
		if dict, ok := r.loadLangDict(themeID, candidate); ok {
			for k, v := range dict {
				merged[k] = v
			}
		}
	}
	return merged
}

// translate looks up one key with per-key fallback to the key itself, so
// third-party themes missing a key render something debuggable instead of
// failing the page.
func (d Dict) translate(key string) string {
	if d == nil {
		return key
	}
	if v, ok := d[key]; ok && v != "" {
		return v
	}
	return key
}

// AvailableLanguages lists the language codes a theme ships under i18n/.
// It powers the admin site-language dropdown; an empty list means the theme
// predates i18n and just renders its hardcoded strings.
func (r *Renderer) AvailableLanguages(themeID string) []string {
	base, err := r.themeFS(themeID)
	if err != nil {
		return nil
	}
	entries, err := fs.ReadDir(base, "i18n")
	if err != nil {
		return nil
	}
	var langs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if lang := NormalizeLanguage(strings.TrimSuffix(name, ".json")); lang != "" {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	return langs
}
