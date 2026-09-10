package public

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
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
const (
	IndexTemplate    = "index.html"
	PostTemplate     = "post.html"
	UserTemplate     = "user.html"
	NotFoundTemplate = "404.html"
)

// ThemeTemplateNames lists every template a theme may provide, in the order
// they are parsed into the shared template set.
var ThemeTemplateNames = []string{IndexTemplate, PostTemplate, UserTemplate, NotFoundTemplate}

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
	URL           string
}

// PaginationData drives the prev/next navigation of list pages.
type PaginationData struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
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
	Pagination   *PaginationData
	Query        IndexQueryData
	Categories   []string
	PopularPosts []PostCardData
	PopularTags  []PopularTagData
}

// PostData is the context of the post detail template.
type PostData struct {
	Site *SiteData
	Post struct {
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
		ContentHTML   template.HTML
		URL           string
	}
}

// UserData is the context of the user profile template.
type UserData struct {
	Site *SiteData
	User struct {
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
	Site *SiteData
}

// templateFuncs exposes formatting helpers to theme templates. The Go
// built-ins (printf, len, eq, ...) are available as well.
var templateFuncs = template.FuncMap{
	"date": func(t time.Time, layout string) string {
		return t.Format(layout)
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
// back to safe defaults when no settings row exists.
func (r *Renderer) buildSiteData(ctx context.Context) *SiteData {
	site := &SiteData{
		Name:         "VexGo",
		Description:  "",
		URL:          r.baseURL,
		ItemsPerPage: 20,
	}
	var settings model.GeneralSettings
	if err := r.db.WithContext(ctx).First(&settings).Error; err == nil {
		if settings.SiteName != "" {
			site.Name = settings.SiteName
		}
		site.Description = settings.SiteDescription
		site.Icon = settings.SiteIcon
		if settings.ItemsPerPage > 0 {
			site.ItemsPerPage = settings.ItemsPerPage
		}
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

	cards := make([]PostCardData, 0, len(posts))
	for _, p := range posts {
		card := toPostCard(p)
		if n, ok := commentCounts[p.ID]; ok {
			card.CommentsCount = n
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
	return cards, pagination, total, nil
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

// themeFS resolves a theme id to its file system root: the embedded default
// theme for the built-in id, a directory under data/theme for uploaded ones.
// Untrusted ids are rejected before touching the file system.
func (r *Renderer) themeFS(themeID string) (fs.FS, error) {
	if themeID == DefaultTheme {
		// The embed keeps the defaulttheme/ directory prefix; expose the
		// theme root so all paths are relative to it.
		return fs.Sub(defaultThemeFS, "defaulttheme")
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

// parseThemeTemplates reads every template file the theme provides into one
// parsed set, so {{define}} fragments can be shared across pages.
func (r *Renderer) parseThemeTemplates(themeID string) (*template.Template, error) {
	base, err := r.themeFS(themeID)
	if err != nil {
		return nil, err
	}

	tmpl := template.New("theme").Funcs(templateFuncs)
	for _, name := range ThemeTemplateNames {
		content, err := fs.ReadFile(base, name)
		if err != nil {
			continue
		}
		if _, err := tmpl.New(name).Parse(string(content)); err != nil {
			return nil, fmt.Errorf("parse theme %q template %s: %w", themeID, name, err)
		}
	}

	if tmpl.Lookup(IndexTemplate) == nil &&
		tmpl.Lookup(PostTemplate) == nil &&
		tmpl.Lookup(UserTemplate) == nil &&
		tmpl.Lookup(NotFoundTemplate) == nil {
		return nil, ErrNoThemeTemplates
	}
	return tmpl, nil
}

// templateModSum returns the newest modification time of the theme's template
// files, or the zero time when none exist.
func (r *Renderer) templateModSum(themeID string) (time.Time, error) {
	base, err := r.themeFS(themeID)
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, name := range ThemeTemplateNames {
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
// is one of the ThemeTemplateNames constants.
func (r *Renderer) renderTheme(themeID, page string, data any) ([]byte, error) {
	tmpl, err := r.loadTheme(themeID)
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
