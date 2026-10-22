// Package public serves the embedded admin SPA, theme assets and the
// server-side-rendered public pages. All state (database, base URL, data
// directory) is injected via NewService.
package public

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/page"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Deps holds the dependencies required by the public domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
	BaseURL   string
	DataDir   string
	// ThemeDir overrides the active theme with a local directory, so a
	// developer can iterate on a theme without rebuilding the embed.
	ThemeDir string
}

// Service contains the business logic of the public domain: the theme engine
// (templates, i18n, seeds) and the view models the public pages render.
type Service struct {
	repo      Repository
	pages     *page.Service
	baseURL   string
	dataDir   string
	themeDir  string
	jwtSecret []byte

	// themeCache and themeSourcesCache hold the parsed template sets and
	// their raw sources. They live on the service so a process holds one
	// cache per renderer instead of one global cache.
	themeCache        themeCacheStore
	themeSourcesCache themeSourcesStore
}

// NewService creates a public service with the given dependencies.
func NewService(deps Deps) *Service {
	s := &Service{
		baseURL:   deps.BaseURL,
		dataDir:   deps.DataDir,
		themeDir:  deps.ThemeDir,
		jwtSecret: deps.JWTSecret,
	}
	if deps.DB != nil {
		s.repo = NewRepository(deps.DB)
		s.pages = page.NewService(page.Deps{DB: deps.DB, JWTSecret: deps.JWTSecret})
	}

	// The themes directory holds uploaded themes. Failing to create it is not
	// fatal here, but left unreported it would surface much later as a
	// baffling "file not found" during a theme install or preview.
	themesDir := filepath.Join(deps.DataDir, ThemesDir)
	if err := os.MkdirAll(themesDir, 0o750); err != nil {
		slog.Warn("failed to create themes directory", "dir", themesDir, "err", err)
	}
	return s
}

// BaseURL returns the configured site base URL used for SSR links.
func (s *Service) BaseURL() string {
	return s.baseURL
}

// DataDir returns the configured data directory used for themes and uploads.
func (s *Service) DataDir() string {
	return s.dataDir
}

// findUser resolves a user id to the account it names. It is the lookup the
// preview-admin check hands to the middleware, so the token's role always
// comes from the stored account and never from the token claim.
func (s *Service) findUser(ctx context.Context, id uint) (model.User, error) {
	if s.repo == nil {
		return model.User{}, gorm.ErrRecordNotFound
	}
	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	return *user, nil
}

// buildSiteData loads the site-wide settings for theme rendering, falling
// back to safe defaults when no settings row exists. lang is the already
// resolved visitor language; empty means "use the site default".
func (s *Service) buildSiteData(ctx context.Context, lang string) *SiteData {
	site := &SiteData{
		Name:            "VexGo",
		Description:     "",
		URL:             s.baseURL,
		ItemsPerPage:    20,
		Language:        DefaultLanguage,
		DefaultLanguage: DefaultLanguage,
	}
	if s.repo != nil {
		settings, err := s.repo.GetGeneralSettings(ctx)
		if err == nil {
			if settings.SiteName != "" {
				site.Name = settings.SiteName
			}
			site.Description = settings.SiteDescription

			if settings.SiteIcon != nil {
				site.Icon = asset.OriginalURL(settings.SiteIcon.PublicID)
			}
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
func (s *Service) countLikesBatch(ctx context.Context, postIDs []uint) map[uint]int64 {
	counts, err := s.repo.CountLikesByPostIDs(ctx, postIDs)
	if err != nil {
		return map[uint]int64{}
	}
	return counts
}

// countCommentsBatch returns the comment count per post id in a single query.
func (s *Service) countCommentsBatch(ctx context.Context, postIDs []uint) map[uint]int64 {
	counts, err := s.repo.CountCommentsByPostIDs(ctx, postIDs)
	if err != nil {
		return map[uint]int64{}
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
func (s *Service) popularPostsData(ctx context.Context, limit int) []PostCardData {
	if limit < 1 {
		limit = 5
	}
	posts, err := s.repo.ListRecentPublishedPosts(ctx, popularPoolLimit)
	if err != nil {
		return nil
	}

	ids := make([]uint, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	likes := s.countLikesBatch(ctx, ids)
	comments := s.countCommentsBatch(ctx, ids)

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
func (s *Service) popularTagsData(ctx context.Context, limit int) []PopularTagData {
	if limit < 1 {
		limit = 10
	}
	rows, err := s.repo.ListPopularTags(ctx, limit)
	if err != nil {
		return nil
	}
	tags := make([]PopularTagData, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, PopularTagData(row))
	}
	return tags
}

// listPageData fetches a published-post page and returns the post cards, the
// pagination links and the total post count.
func (s *Service) listPageData(ctx context.Context, q PostListQuery) ([]PostCardData, *PaginationData, int64, error) {
	page := max(q.Page, 1)
	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	q.Page = page
	q.Limit = limit

	posts, total, err := s.repo.ListPublishedPosts(ctx, q)
	if err != nil {
		return nil, nil, 0, err
	}

	totalPages := max((int(total)+limit-1)/limit, 1)

	postIDs := make([]uint, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	commentCounts := s.countCommentsBatch(ctx, postIDs)
	likes := s.countLikesBatch(ctx, postIDs)

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
		pagination.PrevURL = pageURL(q.Base, page-1, q.Search, q.Category)
	}
	if pagination.HasNext {
		pagination.NextURL = pageURL(q.Base, page+1, q.Search, q.Category)
	}
	pagination.Pages = buildPageLinks(page, totalPages, func(p int) string {
		return pageURL(q.Base, p, q.Search, q.Category)
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
		case isPageLinkVisible(i, total, current):
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

// isPageLinkVisible reports whether page i earns an explicit link: the first
// page, the last page, or a neighbour of the current page. Every other page
// collapses into an ellipsis marker.
func isPageLinkVisible(i, total, current int) bool {
	isEdge := i == 1 || i == total
	isNeighbour := abs(i-current) <= 1
	return isEdge || isNeighbour
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
func (s *Service) buildNavPages(ctx context.Context) []NavPageData {
	if s.repo == nil {
		return nil
	}
	pages, err := s.repo.ListNavPages(ctx)
	if err != nil {
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

// activeTheme returns the currently active theme from the database, falling
// back to the default theme. When a dev theme directory is injected, it wins
// over the database so `vexgo dev --theme-dir` renders the local copy.
func (s *Service) activeTheme(ctx context.Context) string {
	if s.themeDir != "" {
		return DevTheme
	}
	if s.repo == nil {
		return DefaultTheme
	}
	config, err := s.repo.GetThemeConfig(ctx)
	if err != nil {
		return DefaultTheme
	}
	if config.ActiveTheme == "" {
		return DefaultTheme
	}
	return config.ActiveTheme
}

// previewThemeOverride returns the theme selected by the ?theme= preview
// switch, the signature to carry across the preview's own links, and whether
// this request is entitled to use the switch. The switch exists for the admin
// console's theme preview, so it is honored for an authenticated admin, or for
// a request carrying a signed, short-lived preview link — the console opens the
// preview in a new tab, where no Authorization header is available. Every other
// visitor keeps the globally active theme and cannot render a theme that is not
// active. The admin decision is memoized per request, so a page render validates
// the token once rather than for selection and again for rewriting.
func (s *Service) previewThemeOverride(c *gin.Context) (theme, token string, ok bool) {
	theme = c.Query("theme")
	if theme == "" {
		return "", "", false
	}
	token = c.Query(ThemePreviewParam)
	if s.isPreviewAdmin(c) || s.validThemePreviewToken(theme, token) {
		return theme, token, true
	}
	return "", "", false
}

// previewAdminKey caches this request's preview-admin decision. Gin clears the
// context keys between requests, so the cache never leaks to another caller.
const previewAdminKey = "public.previewAdmin"

// isPreviewAdmin reports whether the request carries a still-valid admin JWT,
// granting draft-page previews (?preview=1) and the ?theme= preview switch.
//
// The token is resolved against the database through middleware.TokenUser — the
// same rules the API applies — so the role comes from the stored account, not
// from the token claim. Trusting the claim alone would let a demoted admin (or
// a token invalidated by a password change or a later login) keep previewing
// unpublished pages until it expires.
func (s *Service) isPreviewAdmin(c *gin.Context) bool {
	if cached, ok := c.Get(previewAdminKey); ok {
		admin, _ := cached.(bool)
		return admin
	}
	admin := s.checkPreviewAdmin(c)
	c.Set(previewAdminKey, admin)
	return admin
}

// checkPreviewAdmin performs the uncached preview-admin check.
func (s *Service) checkPreviewAdmin(c *gin.Context) bool {
	token, ok := middleware.BearerToken(c.GetHeader("Authorization"))
	if !ok {
		return false
	}
	user, ok := middleware.TokenUser(c.Request.Context(), s.jwtSecret, token, s.findUser)
	return ok && model.IsAdmin(user.Role)
}

// ActiveThemeID returns the currently active theme id.
func (s *Service) ActiveThemeID(ctx context.Context) string {
	return s.activeTheme(ctx)
}

// IndexView assembles the home page data: the published posts of the
// requested page, the navigation pages, the category filter and the hot
// sidebars.
func (s *Service) IndexView(ctx context.Context, site *SiteData, q PostListQuery) (*IndexData, error) {
	posts, pagination, _, err := s.listPageData(ctx, q)
	if err != nil {
		return nil, err
	}

	// Distinct categories among published posts, for the sidebar filter.
	categories, err := s.repo.ListPublishedCategories(ctx)
	if err != nil {
		categories = nil
	}

	return &IndexData{
		Site:         site,
		Posts:        posts,
		Pages:        s.buildNavPages(ctx),
		Pagination:   pagination,
		Query:        IndexQueryData{Search: q.Search, Category: q.Category},
		Categories:   categories,
		PopularPosts: s.popularPostsData(ctx, 5),
		PopularTags:  s.popularTagsData(ctx, 10),
	}, nil
}

// PostView assembles the post detail data and records the visit. The rendered
// view count includes the current visit.
func (s *Service) PostView(ctx context.Context, site *SiteData, slug string) (*PostData, error) {
	post, err := s.repo.FindPublishedPostBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Record the view (best-effort): unlike the API detail endpoint, the SSR
	// page previously never incremented the counter, so theme reads were
	// invisible in view counts.
	if err := s.repo.IncrementPostViews(ctx, post.ID); err != nil {
		slog.Warn("failed to increment post view count", "postID", post.ID, "err", err)
	} else {
		post.ViewCount++
	}

	commentCounts := s.countCommentsBatch(ctx, []uint{post.ID})
	likeCounts := s.countLikesBatch(ctx, []uint{post.ID})

	data := &PostData{Site: site, Pages: s.buildNavPages(ctx)}
	data.Post.ID = post.ID
	data.Post.Title = post.Title
	data.Post.Slug = post.Slug
	data.Post.Excerpt = post.Excerpt
	data.Post.CoverImage = post.CoverImage
	data.Post.Category = post.Category
	data.Post.CreatedAt = post.CreatedAt
	data.Post.UpdatedAt = post.UpdatedAt
	data.Post.ViewCount = post.ViewCount
	data.Post.CommentsCount = commentCounts[post.ID]
	data.Post.LikesCount = likeCounts[post.ID]
	data.Post.ContentHTML = RenderMarkdown(post.Content)
	data.Post.URL = "/post/" + post.Slug
	if post.Author.ID != 0 {
		data.Post.AuthorName = post.Author.Username
		data.Post.AuthorID = post.Author.ID
		data.Post.AuthorAvatar = post.Author.Avatar
	}
	for _, tag := range post.Tags {
		data.Post.Tags = append(data.Post.Tags, tag.Name)
	}
	return data, nil
}

// UserView assembles the public profile data of one author and their
// published posts.
func (s *Service) UserView(ctx context.Context, site *SiteData, userID uint, q PostListQuery) (*UserData, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("failed to load user for public page", "err", err)
		}
		return nil, err
	}

	posts, pagination, total, err := s.listPageData(ctx, q)
	if err != nil {
		return nil, err
	}

	data := &UserData{Site: site, Pages: s.buildNavPages(ctx)}
	data.User.ID = user.ID
	data.User.Username = user.Username
	data.User.Avatar = user.Avatar
	if !user.HideBio {
		data.User.Bio = user.Bio
	}
	data.User.CreatedAt = user.CreatedAt
	data.User.PostsCount = total
	data.Posts = posts
	data.Pagination = pagination
	return data, nil
}

// PageView assembles the custom page data. A draft is only readable when the
// request is an authorized draft preview.
func (s *Service) PageView(ctx context.Context, site *SiteData, slug string, preview bool) (*PageData, error) {
	page, err := s.repo.FindPageBySlug(ctx, slug, !preview)
	if err != nil {
		return nil, err
	}
	if page.Status != model.PageStatusPublished && !preview {
		return nil, gorm.ErrRecordNotFound
	}

	data := &PageData{Site: site, Pages: s.buildNavPages(ctx)}
	data.Page.ID = page.ID
	data.Page.Title = page.Title
	data.Page.Slug = page.Slug
	data.Page.CreatedAt = page.CreatedAt
	data.Page.UpdatedAt = page.UpdatedAt
	data.Page.ContentHTML = RenderPageContent(page.Content)
	data.Page.URL = "/" + page.Slug
	return data, nil
}
