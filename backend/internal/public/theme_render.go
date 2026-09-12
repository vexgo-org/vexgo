package public

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Template file names a theme may provide. Every template is a complete HTML
// document; shared fragments use Go's {{define}}/{{template}} mechanism.
// PageTemplate is the generic fallback for custom pages; any <slug>.html file
// in the theme is additionally usable as the dedicated template of the page
// with that slug (discovered dynamically, not listed here).
const (
	IndexTemplate    = "index.html"
	PostTemplate     = "post.html"
	PageTemplate     = "page.html"
	UserTemplate     = "user.html"
	NotFoundTemplate = "404.html"
)

// ThemeTemplateNames lists the base templates a theme may provide, in the
// order they are parsed into the shared template set. Extra <slug>.html
// files are discovered dynamically by parseThemeTemplates.
var ThemeTemplateNames = []string{IndexTemplate, PostTemplate, PageTemplate, UserTemplate, NotFoundTemplate}

// ErrNoThemeTemplates reports a theme that provides no template files at all.
var ErrNoThemeTemplates = errors.New("theme has no template files")

// gold parses post markdown into HTML for server-side rendering. Raw HTML in
// the markdown is escaped (safe defaults, matching the client-side renderer),
// while GFM tables/strikethrough are enabled for parity with remark-gfm.
var gold = goldmark.New(goldmark.WithExtensions(extension.GFM))

// RenderMarkdown converts markdown into safe HTML. The result is typed as
// template.HTML so themes can emit it with {{.ContentHTML}} without Go
// re-escaping it.
func RenderMarkdown(src string) template.HTML {
	var buf bytes.Buffer
	if err := gold.Convert([]byte(src), &buf); err != nil {
		// Rendering failures are extremely unlikely; fall back to escaped
		// plain text so the page still renders something safe.
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(buf.String())
}

// SiteData is the site-wide context available to every theme template.
type SiteData struct {
	Name         string
	Description  string
	Icon         string
	URL          string
	ItemsPerPage int
	// Language is the resolved visitor language for this request
	// (e.g. "en", "zh"), after the ?lang=/cookie/Accept-Language/site
	// default chain. Templates emit it as <html lang> and the t function
	// uses it implicitly.
	Language string
	// DefaultLanguage is the configured site default from settings.
	DefaultLanguage string
}

// NavPageData is one custom page shown in the theme navigation.
type NavPageData struct {
	Title     string
	Slug      string
	URL       string
	SortOrder int
}

// PostCardData is the public summary of a post for list pages (home, user).
type PostCardData struct {
	ID            uint
	Title         string
	Slug          string
	Excerpt       string
	CoverImage    string
	Category      string
	Tags          []string
	AuthorName    string
	AuthorID      uint
	AuthorAvatar  string
	CreatedAt     time.Time
	ViewCount     int
	CommentsCount int64
	LikesCount    int64
	URL           string
}

// PageLinkData is one numbered pagination entry. Ellipsis entries carry no
// number or URL; they render as a literal "..." separator.
type PageLinkData struct {
	Number    int
	URL       string
	IsCurrent bool
	Ellipsis  bool
}

// PaginationData drives the navigation of list pages: prev/next links plus
// the windowed list of numbered page links (first, last, current ± 1).
type PaginationData struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
	Pages       []PageLinkData
}

// IndexQueryData carries the active filters of the home page.
type IndexQueryData struct {
	Search   string
	Category string
}

// PopularTagData is one entry of the hot-tags sidebar: a tag name and how
// many published posts carry it.
type PopularTagData struct {
	Name  string
	Count int64
}

// IndexData is the context of the home page template.
type IndexData struct {
	Site         *SiteData
	Posts        []PostCardData
	Pages        []NavPageData
	Pagination   *PaginationData
	Query        IndexQueryData
	Categories   []string
	PopularPosts []PostCardData
	PopularTags  []PopularTagData
}

// PostData is the context of the post detail template.
type PostData struct {
	Site  *SiteData
	Pages []NavPageData
	Post  struct {
		ID            uint
		Title         string
		Slug          string
		Excerpt       string
		CoverImage    string
		Category      string
		Tags          []string
		AuthorName    string
		AuthorID      uint
		AuthorAvatar  string
		CreatedAt     time.Time
		UpdatedAt     time.Time
		ViewCount     int
		CommentsCount int64
		LikesCount    int64
		ContentHTML   template.HTML
		URL           string
	}
}

// UserData is the context of the user profile template.
type UserData struct {
	Site  *SiteData
	Pages []NavPageData
	User  struct {
		ID         uint
		Username   string
		Avatar     string
		Bio        string
		CreatedAt  time.Time
		PostsCount int64
	}
	Posts      []PostCardData
	Pagination *PaginationData
}

// NotFoundData is the context of the optional 404 template.
type NotFoundData struct {
	Site  *SiteData
	Pages []NavPageData
}

// PageData is the context of custom page templates (page.html and any
// <slug>.html dedicated template).
type PageData struct {
	Site  *SiteData
	Pages []NavPageData
	Page  struct {
		ID          uint
		Title       string
		Slug        string
		CreatedAt   time.Time
		UpdatedAt   time.Time
		ContentHTML template.HTML
		URL         string
	}
}

// baseTemplateFuncs holds the language-independent helpers. The translatable
// t function is bound per request (see boundTemplateFuncs) so themes declare
// {{t "key"}} without naming a language; the backend resolves it.
func baseTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"date": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
		// add sums two integers; used to render 1-based list positions.
		"add": func(a, b int) int {
			return a + b
		},
		// first clamps a slice to at most n items; unlike the builtin slice it
		// tolerates short/empty inputs (used to cap the tag pills on cards).
		"first": func(items []string, n int) []string {
			if n < 1 || len(items) <= n {
				return items
			}
			return items[:n]
		},
		"truncate": func(s string, max int) string {
			s = strings.TrimSpace(s)
			if len(s) <= max {
				return s
			}
			return s[:max] + "..."
		},
		// userURL builds the public user profile URL. Helper funcs keep attribute
		// actions free of double quotes, which React would escape to &quot; and
		// break Go template parsing.
		"userURL": func(id uint) string {
			return "/user/" + strconv.FormatUint(uint64(id), 10)
		},
		// categoryURL builds the home page category filter URL.
		"categoryURL": func(name string) string {
			return "/?category=" + url.QueryEscape(name)
		},
		// searchURL builds the home page search URL for a tag or keyword.
		"searchURL": func(name string) string {
			return "/?search=" + url.QueryEscape(name)
		},
	}
}

// boundTemplateFuncs copies the base helpers and binds t to one request's
// merged dictionary. Unknown keys fall back to the key itself so
// third-party themes with incomplete translations degrade per key.
func boundTemplateFuncs(dict Dict) template.FuncMap {
	funcs := baseTemplateFuncs()
	funcs["t"] = func(key string) string {
		return dict.translate(key)
	}
	return funcs
}

// languageOf extracts the requested language from template data, defaulting
// to en for data assembled without i18n (tests, legacy callers).
func languageOf(data any) string {
	var site *SiteData
	switch d := data.(type) {
	case IndexData:
		site = d.Site
	case *IndexData:
		if d != nil {
			site = d.Site
		}
	case PostData:
		site = d.Site
	case *PostData:
		if d != nil {
			site = d.Site
		}
	case UserData:
		site = d.Site
	case *UserData:
		if d != nil {
			site = d.Site
		}
	case NotFoundData:
		site = d.Site
	case *NotFoundData:
		if d != nil {
			site = d.Site
		}
	case PageData:
		site = d.Site
	case *PageData:
		if d != nil {
			site = d.Site
		}
	}
	if site == nil {
		return DefaultLanguage
	}
	if lang := NormalizeLanguage(site.Language); lang != "" {
		return lang
	}
	return DefaultLanguage
}

// cachedTheme is a parsed template set plus the newest file mtime it was
// parsed from, so custom themes are re-parsed only when a template changes.
type cachedTheme struct {
	tmpl   *template.Template
	modSum time.Time
}

var themeCache struct {
	sync.Mutex
	themes map[string]cachedTheme
}

func init() {
	themeCache.themes = make(map[string]cachedTheme)
}

// buildSiteData loads the site-wide settings for theme rendering, falling
// back to safe defaults when no settings row exists. lang is the already
// resolved visitor language; empty means "use the site default".
func (r *Renderer) buildSiteData(ctx context.Context, lang string) *SiteData {
	site := &SiteData{
		Name:            "VexGo",
		Description:     "",
		URL:             r.baseURL,
		ItemsPerPage:    20,
		Language:        DefaultLanguage,
		DefaultLanguage: DefaultLanguage,
	}
	var settings model.GeneralSettings
	if r.db != nil {
		if err := r.db.WithContext(ctx).First(&settings).Error; err == nil {
			if settings.SiteName != "" {
				site.Name = settings.SiteName
			}
			site.Description = settings.SiteDescription
			site.Icon = settings.SiteIcon
			if settings.ItemsPerPage > 0 {
				site.ItemsPerPage = settings.ItemsPerPage
			}
			if normalized := NormalizeLanguage(settings.SiteLanguage); normalized != "" {
				site.DefaultLanguage = normalized
			}
		}
	}
	if normalized := NormalizeLanguage(lang); normalized != "" {
		site.Language = normalized
	} else {
		site.Language = site.DefaultLanguage
	}
	return site
}

// toPostCard maps a post row to the public card shape consumed by templates.
func toPostCard(post model.Post) PostCardData {
	card := PostCardData{
		ID:            post.ID,
		Title:         post.Title,
		Slug:          post.Slug,
		Excerpt:       post.Excerpt,
		CoverImage:    post.CoverImage,
		Category:      post.Category,
		CreatedAt:     post.CreatedAt,
		ViewCount:     post.ViewCount,
		CommentsCount: int64(post.CommentsCount),
		URL:           "/post/" + post.Slug,
	}
	for _, tag := range post.Tags {
		card.Tags = append(card.Tags, tag.Name)
	}
	if post.Author.ID != 0 {
		card.AuthorID = post.Author.ID
		card.AuthorName = post.Author.Username
		card.AuthorAvatar = post.Author.Avatar
	}
	return card
}

// countLikesBatch returns the like count per post id in a single query.
func (r *Renderer) countLikesBatch(ctx context.Context, postIDs []uint) map[uint]int64 {
	counts := make(map[uint]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var rows []result
	if err := r.db.WithContext(ctx).Model(&model.Like{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&rows).Error; err != nil {
		return counts
	}
	for _, row := range rows {
		counts[row.PostID] = row.Count
	}
	return counts
}

// popularPoolLimit bounds the pool of posts considered for the hot-posts and
// hot-tags sidebars. The SSR path must stay cheap: only the most recent
// published posts compete for the sidebar slots, matching the old client-side
// sidebar which tallied the latest 200 posts.
const popularPoolLimit = 200

// popularPostsData returns the top published posts by likes*5 + views,
// matching the public /stats/popular-posts endpoint. Like and comment counts
// are batch-fetched; on query failure an empty list is returned so the home
// page still renders.
func (r *Renderer) popularPostsData(ctx context.Context, limit int) []PostCardData {
	if limit < 1 {
		limit = 5
	}
	var posts []model.Post
	if err := r.db.WithContext(ctx).Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", model.PostStatusPublished).
		Order("created_at DESC").
		Limit(popularPoolLimit).
		Find(&posts).Error; err != nil {
		return nil
	}

	ids := make([]uint, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	likes := r.countLikesBatch(ctx, ids)
	comments := r.countCommentsBatch(ctx, ids)

	// Score by likes*5 + views, exactly like the public API's Popular.
	sort.SliceStable(posts, func(i, j int) bool {
		scoreI := int(likes[posts[i].ID])*5 + posts[i].ViewCount
		scoreJ := int(likes[posts[j].ID])*5 + posts[j].ViewCount
		return scoreI > scoreJ
	})

	cards := make([]PostCardData, 0, min(len(posts), limit))
	for i := range posts {
		if len(cards) >= limit {
			break
		}
		card := toPostCard(posts[i])
		card.CommentsCount = comments[posts[i].ID]
		card.LikesCount = likes[posts[i].ID]
		cards = append(cards, card)
	}
	return cards
}

// popularTagsData returns the tags most used by published posts, ordered by
// usage count, via a single grouped join query. On failure an empty list is
// returned so the home page still renders.
func (r *Renderer) popularTagsData(ctx context.Context, limit int) []PopularTagData {
	if limit < 1 {
		limit = 10
	}
	type row struct {
		Name  string
		Count int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("tags").
		Select("tags.name, COUNT(*) AS count").
		Joins("JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("JOIN posts ON posts.id = post_tags.post_id").
		Where("posts.status = ?", model.PostStatusPublished).
		Group("tags.id").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil
	}
	tags := make([]PopularTagData, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, PopularTagData(row))
	}
	return tags
}

// countCommentsBatch returns the comment count per post id in a single query.
func (r *Renderer) countCommentsBatch(ctx context.Context, postIDs []uint) map[uint]int64 {
	counts := make(map[uint]int64, len(postIDs))
	if len(postIDs) == 0 {
		return counts
	}
	type result struct {
		PostID uint
		Count  int64
	}
	var rows []result
	if err := r.db.WithContext(ctx).Model(&model.Comment{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Find(&rows).Error; err != nil {
		return counts
	}
	for _, row := range rows {
		counts[row.PostID] = row.Count
	}
	return counts
}

// listPageData fetches a published-post page with pagination and returns the
// post cards, the total page count and the total post count. search filters
// title/content, category filters the post category column exactly like the
// public API does, and authorID (0 = any author) narrows to one user's posts.
func (r *Renderer) listPageData(ctx context.Context, base string, page, limit int, search, category string, authorID uint) ([]PostCardData, *PaginationData, int64, error) {
	page = max(page, 1)
	if limit < 1 {
		limit = 20
	}

	query := r.db.WithContext(ctx).Model(&model.Post{}).
		Preload("Author").
		Preload("Tags").
		Where("status = ?", model.PostStatusPublished)
	if authorID != 0 {
		query = query.Where("author_id = ?", authorID)
	}
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, 0, err
	}

	var posts []model.Post
	if err := query.Order("created_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, nil, 0, err
	}

	totalPages := max((int(total)+limit-1)/limit, 1)

	postIDs := make([]uint, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	commentCounts := r.countCommentsBatch(ctx, postIDs)
	likes := r.countLikesBatch(ctx, postIDs)

	cards := make([]PostCardData, 0, len(posts))
	for _, p := range posts {
		card := toPostCard(p)
		if n, ok := commentCounts[p.ID]; ok {
			card.CommentsCount = n
		}
		if n, ok := likes[p.ID]; ok {
			card.LikesCount = n
		}
		cards = append(cards, card)
	}

	pagination := &PaginationData{
		CurrentPage: page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
	}
	if pagination.HasPrev {
		pagination.PrevURL = pageURL(base, page-1, search, category)
	}
	if pagination.HasNext {
		pagination.NextURL = pageURL(base, page+1, search, category)
	}
	pagination.Pages = buildPageLinks(page, totalPages, func(p int) string {
		return pageURL(base, p, search, category)
	})
	return cards, pagination, total, nil
}

// buildPageLinks returns the windowed page links shown by list templates:
// always the first and last page plus the pages around the current one, with
// ellipsis markers between the gaps. This mirrors the client-side pagination
// of the previous SPA home page.
func buildPageLinks(current, total int, urlFor func(int) string) []PageLinkData {
	if total <= 1 {
		return nil
	}
	pages := make([]PageLinkData, 0, min(total, 7))
	for i := 1; i <= total; i++ {
		switch {
		case i == 1 || i == total || abs(i-current) <= 1:
			pages = append(pages, PageLinkData{
				Number:    i,
				URL:       urlFor(i),
				IsCurrent: i == current,
			})
		case len(pages) > 0 && !pages[len(pages)-1].Ellipsis:
			pages = append(pages, PageLinkData{Ellipsis: true})
		}
	}
	return pages
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// pageURL builds a paginated list URL, preserving the active filters.
func pageURL(base string, page int, search, category string) string {
	q := url.Values{}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if search != "" {
		q.Set("search", search)
	}
	if category != "" {
		q.Set("category", category)
	}
	if len(q) == 0 {
		return base
	}
	return base + "?" + q.Encode()
}

// buildNavPages loads published pages flagged for navigation, ordered by
// sortOrder. On query failure an empty list is returned so pages still render.
func (r *Renderer) buildNavPages(ctx context.Context) []NavPageData {
	if r.db == nil {
		return nil
	}
	var pages []model.Page
	if err := r.db.WithContext(ctx).
		Where("status = ? AND show_in_nav = ?", model.PageStatusPublished, true).
		Order("sort_order ASC, id ASC").
		Find(&pages).Error; err != nil {
		return nil
	}
	nav := make([]NavPageData, 0, len(pages))
	for _, p := range pages {
		nav = append(nav, NavPageData{
			Title: p.Title, Slug: p.Slug, URL: "/" + p.Slug, SortOrder: p.SortOrder,
		})
	}
	return nav
}

// friendsBlockRe matches a fenced ```friends code block in page markdown.
// Each non-empty line inside is "name | url | avatar | description".
var friendsBlockRe = regexp.MustCompile("(?m)^```friends[ \t]*\n([\r\\s\\S]*?)^```[ \t]*$")

// renderFriendsCards converts the body of a friends block into card HTML.
// Fields are HTML-escaped; rows without a name or a valid http(s) URL are
// skipped so one bad line cannot break the whole block.
func renderFriendsCards(body string) string {
	type friend struct {
		name, url, avatar, desc string
	}
	var friends []friend
	for line := range strings.Lines(strings.TrimSpace(body)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		name, rawURL, avatar, desc := parts[0], parts[1], parts[2], parts[3]
		u, err := url.ParseRequestURI(rawURL)
		if name == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			continue
		}
		friends = append(friends, friend{name: name, url: rawURL, avatar: avatar, desc: desc})
	}
	if len(friends) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(`<div class="vexgo-friends">`)
	for _, f := range friends {
		sb.WriteString(`<a class="vexgo-friend-card" href="`)
		sb.WriteString(template.HTMLEscapeString(f.url))
		sb.WriteString(`" target="_blank" rel="noopener">`)
		if f.avatar != "" {
			sb.WriteString(`<img class="vexgo-friend-avatar" src="`)
			sb.WriteString(template.HTMLEscapeString(f.avatar))
			sb.WriteString(`" alt="`)
			sb.WriteString(template.HTMLEscapeString(f.name))
			sb.WriteString(`" loading="lazy">`)
		} else {
			sb.WriteString(`<span class="vexgo-friend-avatar vexgo-friend-initial">`)
			name := []rune(f.name)
			sb.WriteString(template.HTMLEscapeString(string(name[:1])))
			sb.WriteString(`</span>`)
		}
		sb.WriteString(`<span class="vexgo-friend-meta"><span class="vexgo-friend-name">`)
		sb.WriteString(template.HTMLEscapeString(f.name))
		sb.WriteString(`</span>`)
		if f.desc != "" {
			sb.WriteString(`<span class="vexgo-friend-desc">`)
			sb.WriteString(template.HTMLEscapeString(f.desc))
			sb.WriteString(`</span>`)
		}
		sb.WriteString(`</span></a>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// RenderPageContent converts page markdown into safe HTML, expanding fenced
// ```friends blocks into link-card HTML shared by all themes (themes only
// provide the .vexgo-friends CSS). Placeholders survive the markdown pass
// and are swapped for the card HTML afterwards so goldmark never escapes it.
func RenderPageContent(src string) template.HTML {
	cards := []string{}
	withPlaceholders := friendsBlockRe.ReplaceAllStringFunc(src, func(block string) string {
		m := friendsBlockRe.FindStringSubmatch(block)
		body := ""
		if len(m) == 2 {
			body = m[1]
		}
		html := renderFriendsCards(body)
		if html == "" {
			return ""
		}
		cards = append(cards, html)
		return "\n\nVEXGOFRIENDS" + strconv.Itoa(len(cards)-1) + "\n\n"
	})
	var buf bytes.Buffer
	if err := gold.Convert([]byte(withPlaceholders), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	out := buf.String()
	for i, html := range cards {
		out = strings.ReplaceAll(out, "<p>VEXGOFRIENDS"+strconv.Itoa(i)+"</p>", html)
		out = strings.ReplaceAll(out, "VEXGOFRIENDS"+strconv.Itoa(i), html)
	}
	return template.HTML(out)
}

// themeFS resolves a theme id to its file system root: the embedded default
// theme for the built-in id, a directory under data/theme for uploaded ones.
// Untrusted ids are rejected before touching the file system.
func (r *Renderer) themeFS(themeID string) (fs.FS, error) {
	if themeID == DefaultTheme {
		// The embed keeps the defaulttheme/ directory prefix; expose the
		// theme root so all paths are relative to it.
		return fs.Sub(defaultThemeFS, "default-theme")
	}
	if strings.ContainsAny(themeID, `/\`) || themeID == "." || themeID == ".." {
		return nil, fmt.Errorf("invalid theme id %q", themeID)
	}
	return os.DirFS(filepath.Join(r.dataDir, ThemesDir, themeID)), nil
}

// readThemeFile reads a single file from a theme, reporting whether it exists.
func (r *Renderer) readThemeFile(themeID, relPath string) ([]byte, bool) {
	base, err := r.themeFS(themeID)
	if err != nil {
		return nil, false
	}
	clean := path.Clean(strings.TrimPrefix(relPath, "/"))
	if !fs.ValidPath(clean) {
		return nil, false
	}
	content, err := fs.ReadFile(base, clean)
	if err != nil {
		return nil, false
	}
	return content, true
}

// themeSources is the raw template text of a theme, cached so per-request
// parsing (needed to bind t to the visitor language) never touches disk.
type themeSources struct {
	files  map[string]string
	modSum time.Time
}

var themeSourcesCache struct {
	sync.Mutex
	themes map[string]themeSources
}

func init() {
	themeSourcesCache.themes = make(map[string]themeSources)
}

// loadThemeSources reads every template file the theme provides into memory.
// Besides the base ThemeTemplateNames, any extra <slug>.html file at the
// theme root is a dedicated custom-page template. Custom themes are re-read
// when a template changes on disk; the embedded default theme is read once.
//
// The returned map is the cache's copy: callers must only read it, never
// mutate it, or every subsequent request sees the corruption.
func (r *Renderer) loadThemeSources(themeID string) (map[string]string, error) {
	themeSourcesCache.Lock()
	defer themeSourcesCache.Unlock()

	if cached, ok := themeSourcesCache.themes[themeID]; ok {
		if themeID == DefaultTheme {
			return cached.files, nil
		}
		if modSum, err := r.templateModSum(themeID); err == nil && modSum.Equal(cached.modSum) {
			return cached.files, nil
		}
	}

	base, err := r.themeFS(themeID)
	if err != nil {
		return nil, err
	}
	known := map[string]struct{}{}
	names := append([]string{}, ThemeTemplateNames...)
	for _, name := range ThemeTemplateNames {
		known[name] = struct{}{}
	}
	names = append(names, listExtraPageTemplates(base, known)...)
	files := make(map[string]string, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(base, name)
		if err != nil {
			continue
		}
		files[name] = string(content)
	}
	if len(files) == 0 {
		return nil, ErrNoThemeTemplates
	}
	modSum, _ := r.templateModSum(themeID)
	themeSourcesCache.themes[themeID] = themeSources{files: files, modSum: modSum}
	return files, nil
}

// parseThemeSources parses cached sources with one FuncMap so {{define}}
// fragments stay shared across pages. Names are parsed in sorted order so a
// theme's cross-file {{define}} overrides resolve deterministically.
func parseThemeSources(themeID string, files map[string]string, funcs template.FuncMap) (*template.Template, error) {
	tmpl := template.New("theme").Funcs(funcs)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if _, err := tmpl.New(name).Parse(files[name]); err != nil {
			return nil, fmt.Errorf("parse theme %q template %s: %w", themeID, name, err)
		}
	}
	if tmpl.Lookup(IndexTemplate) == nil &&
		tmpl.Lookup(PostTemplate) == nil &&
		tmpl.Lookup(PageTemplate) == nil &&
		tmpl.Lookup(UserTemplate) == nil &&
		tmpl.Lookup(NotFoundTemplate) == nil {
		return nil, ErrNoThemeTemplates
	}
	return tmpl, nil
}

// parseThemeTemplates reads every template file the theme provides into one
// parsed set, so {{define}} fragments can be shared across pages. Besides the
// base ThemeTemplateNames, any extra <slug>.html file at the theme root is
// parsed as a dedicated custom-page template.
//
// The fallback t returns the key itself so legacy cached renders never fail
// on translatable templates; request renders re-parse with the bound dict.
func (r *Renderer) parseThemeTemplates(themeID string) (*template.Template, error) {
	files, err := r.loadThemeSources(themeID)
	if err != nil {
		return nil, err
	}
	funcs := baseTemplateFuncs()
	funcs["t"] = func(key string) string { return key }
	return parseThemeSources(themeID, files, funcs)
}

// parseThemeWithDict parses a theme with t bound to one request's merged
// dictionary.
func (r *Renderer) parseThemeWithDict(themeID string, dict Dict) (*template.Template, error) {
	files, err := r.loadThemeSources(themeID)
	if err != nil {
		return nil, err
	}
	return parseThemeSources(themeID, files, boundTemplateFuncs(dict))
}

// listExtraPageTemplates returns theme-root *.html files beyond the known
// base set (e.g. timeline.html, links.html, about.html). Names with slashes,
// dot segments or uppercase are ignored so only safe <slug>.html files qualify.
func listExtraPageTemplates(base fs.FS, known map[string]struct{}) []string {
	entries, err := fs.ReadDir(base, ".")
	if err != nil {
		return nil
	}
	var extra []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := known[name]; ok {
			continue
		}
		if !strings.HasSuffix(name, ".html") || strings.Contains(name, "/") || strings.HasPrefix(name, ".") {
			continue
		}
		slug := strings.TrimSuffix(name, ".html")
		if slug == "" || slug != strings.ToLower(slug) {
			continue
		}
		extra = append(extra, name)
	}
	sort.Strings(extra)
	return extra
}

// themeProvides reports whether the theme supplies the named template.
func (r *Renderer) themeProvides(themeID, name string) bool {
	files, err := r.loadThemeSources(themeID)
	if err != nil {
		return false
	}
	_, ok := files[name]
	return ok
}

// templateModSum returns the newest modification time of the theme's template
// files, or the zero time when none exist.
func (r *Renderer) templateModSum(themeID string) (time.Time, error) {
	base, err := r.themeFS(themeID)
	if err != nil {
		return time.Time{}, err
	}
	known := map[string]struct{}{}
	for _, name := range ThemeTemplateNames {
		known[name] = struct{}{}
	}
	names := append([]string{}, ThemeTemplateNames...)
	names = append(names, listExtraPageTemplates(base, known)...)
	var newest time.Time
	for _, name := range names {
		info, err := fs.Stat(base, name)
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, nil
}

// loadTheme returns the parsed template set for a theme, re-parsing custom
// themes whenever one of their template files changes on disk. The embedded
// default theme is parsed once and cached forever.
func (r *Renderer) loadTheme(themeID string) (*template.Template, error) {
	themeCache.Lock()
	defer themeCache.Unlock()

	if cached, ok := themeCache.themes[themeID]; ok {
		if themeID == DefaultTheme {
			return cached.tmpl, nil
		}
		modSum, err := r.templateModSum(themeID)
		if err == nil && modSum.Equal(cached.modSum) {
			return cached.tmpl, nil
		}
	}

	tmpl, err := r.parseThemeTemplates(themeID)
	if err != nil {
		return nil, err
	}
	modSum, _ := r.templateModSum(themeID)
	themeCache.themes[themeID] = cachedTheme{tmpl: tmpl, modSum: modSum}
	return tmpl, nil
}

// renderTheme renders one page of a theme with the given data. The page name
// is one of the ThemeTemplateNames constants. The language comes from
// data's Site.Language (defaulting to en); t is bound to the theme's merged
// dictionary so translatable templates render without extra plumbing.
func (r *Renderer) renderTheme(themeID, page string, data any) ([]byte, error) {
	lang := languageOf(data)
	dict := r.loadMergedDict(themeID, lang, lang)
	return r.renderThemeWithDict(themeID, page, data, dict)
}

// renderThemeWithDict renders with an explicitly merged dictionary (used by
// request handlers that resolve the full visitor > site-default > en chain).
func (r *Renderer) renderThemeWithDict(themeID, page string, data any, dict Dict) ([]byte, error) {
	tmpl, err := r.parseThemeWithDict(themeID, dict)
	if err != nil {
		return nil, err
	}
	t := tmpl.Lookup(page)
	if t == nil {
		return nil, fmt.Errorf("theme %q provides no %s template", themeID, page)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render theme %q page %s: %w", themeID, page, err)
	}
	return buf.Bytes(), nil
}

// themeAssetsPrefix is the stable asset prefix every theme template uses to
// reference its own assets; the server resolves it against the active theme.
const themeAssetsPrefix = "/theme-assets/"

// previewHrefRe matches absolute same-origin hyperlinks in rendered HTML so a
// theme preview can keep the previewed theme across navigation.
var previewHrefRe = regexp.MustCompile(`href="(/[^"]*)"`)

// rewriteThemePreview scopes a rendered page to the theme being previewed.
// Themes hardcode the /theme-assets/ prefix, which the server resolves against
// the *active* theme; subresource requests (CSS, JS, images) carry no ?theme=
// parameter, so without rewriting a preview of a non-active theme would load
// the active theme's assets. Preview pages therefore point at the theme's own
// /themes/<id>/assets/ files and carry ?theme=<id> on their same-origin links
// so navigation inside the preview stays on that theme.
func rewriteThemePreview(out []byte, themeID string) []byte {
	assetPrefix := "/themes/" + url.PathEscape(themeID) + "/assets/"
	if bytes.Contains(out, []byte(themeAssetsPrefix)) {
		out = bytes.ReplaceAll(out, []byte(themeAssetsPrefix), []byte(assetPrefix))
	}
	return previewHrefRe.ReplaceAllFunc(out, func(match []byte) []byte {
		href := string(match[len(`href="`) : len(match)-1])
		rewritten, ok := withThemeParam(href, themeID)
		if !ok {
			return match
		}
		return []byte(`href="` + rewritten + `"`)
	})
}

// withThemeParam appends ?theme=<themeID> to a same-origin page link,
// preserving any existing query and fragment. It reports ok false for links
// that must not be rewritten: relative links, protocol-relative URLs and
// non-page prefixes (API, admin SPA, raw theme files).
func withThemeParam(href, themeID string) (string, bool) {
	if !strings.HasPrefix(href, "/") || strings.HasPrefix(href, "//") {
		return "", false
	}
	for _, prefix := range []string{"/api/", "/admin", "/themes/", themeAssetsPrefix} {
		if strings.HasPrefix(href, prefix) {
			return "", false
		}
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	q := u.Query()
	q.Set("theme", themeID)
	u.RawQuery = q.Encode()
	return u.String(), true
}
