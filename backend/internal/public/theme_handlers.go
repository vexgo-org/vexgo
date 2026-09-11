package public

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const htmlContentType = "text/html; charset=utf-8"

// servePage renders one page of the requested theme and writes it as HTML.
// When rendering fails (theme missing a template, template syntax error) it
// falls back to a plain 404 so public routes never crash.
func (r *Renderer) servePage(c *gin.Context, theme, page string, data any, status int) {
	out, err := r.renderTheme(theme, page, data)
	if err != nil {
		c.Data(http.StatusNotFound, htmlContentType, []byte("Page not found"))
		return
	}
	c.Data(status, htmlContentType, out)
}

// renderNotFound renders the theme's 404 template (when present) with a plain
// fallback otherwise.
func (r *Renderer) renderNotFound(c *gin.Context, theme string, site *SiteData) {
	if out, err := r.renderTheme(theme, NotFoundTemplate, NotFoundData{Site: site, Pages: r.buildNavPages(c.Request.Context())}); err == nil {
		c.Data(http.StatusNotFound, htmlContentType, out)
		return
	}
	c.Data(http.StatusNotFound, htmlContentType, []byte("Page not found"))
}

// isPreviewAdmin reports whether the request carries an admin JWT, granting
// draft-page previews (?preview=1). Without a configured secret it is false.
func (r *Renderer) isPreviewAdmin(c *gin.Context) bool {
	if len(r.jwtSecret) == 0 {
		return false
	}
	header := c.GetHeader("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return false
	}
	token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenUnverifiable
		}
		return r.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}
	role, _ := claims["role"].(string)
	return model.IsAdmin(role)
}

// handleIndex renders the home page: published posts, paginated, with the
// optional ?search= / ?category= / ?page= filters.
func (r *Renderer) handleIndex(c *gin.Context) {
	theme := r.getRequestedTheme(c)
	site := r.buildSiteData(c.Request.Context())
	page := parsePageParam(c)
	search := c.Query("search")
	category := c.Query("category")

	posts, pagination, _, err := r.listPageData(c.Request.Context(), "/", page, site.ItemsPerPage, search, category, 0)
	if err != nil {
		r.renderNotFound(c, theme, site)
		return
	}

	// Distinct categories among published posts, for the sidebar filter.
	var categories []string
	if err := r.db.WithContext(c.Request.Context()).
		Model(&model.Post{}).
		Where("status = ? AND category != ''", model.PostStatusPublished).
		Distinct().
		Pluck("category", &categories).Error; err != nil {
		categories = nil
	}

	r.servePage(c, theme, IndexTemplate, IndexData{
		Site:         site,
		Posts:        posts,
		Pages:        r.buildNavPages(c.Request.Context()),
		Pagination:   pagination,
		Query:        IndexQueryData{Search: search, Category: category},
		Categories:   categories,
		PopularPosts: r.popularPostsData(c.Request.Context(), 5),
		PopularTags:  r.popularTagsData(c.Request.Context(), 10),
	}, http.StatusOK)
}

// handlePost renders a published post by slug. Drafts, pending and rejected
// posts stay hidden — the SSR page must not expose them via a guessable slug
// when the API filters them out.
func (r *Renderer) handlePost(c *gin.Context) {
	theme := r.getRequestedTheme(c)
	site := r.buildSiteData(c.Request.Context())
	slug := c.Param("slug")

	var post model.Post
	if err := r.db.WithContext(c.Request.Context()).
		Preload("Author").
		Preload("Tags").
		Where("slug = ? AND status = ?", slug, model.PostStatusPublished).
		First(&post).Error; err != nil {
		r.renderNotFound(c, theme, site)
		return
	}

	commentCounts := r.countCommentsBatch(c.Request.Context(), []uint{post.ID})
	likeCounts := r.countLikesBatch(c.Request.Context(), []uint{post.ID})

	data := PostData{Site: site, Pages: r.buildNavPages(c.Request.Context())}
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

	r.servePage(c, theme, PostTemplate, data, http.StatusOK)
}

// handleUser renders a user's public profile and their published posts.
func (r *Renderer) handleUser(c *gin.Context) {
	theme := r.getRequestedTheme(c)
	site := r.buildSiteData(c.Request.Context())

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || userID == 0 {
		r.renderNotFound(c, theme, site)
		return
	}

	var user model.User
	if err := r.db.WithContext(c.Request.Context()).First(&user, userID).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("failed to load user for public page", "err", err)
		}
		r.renderNotFound(c, theme, site)
		return
	}

	page := parsePageParam(c)
	posts, pagination, total, err := r.listPageData(
		c.Request.Context(),
		"/user/"+strconv.FormatUint(userID, 10),
		page,
		site.ItemsPerPage,
		"",
		"",
		uint(userID),
	)
	if err != nil {
		r.renderNotFound(c, theme, site)
		return
	}

	data := UserData{Site: site, Pages: r.buildNavPages(c.Request.Context())}
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

	r.servePage(c, theme, UserTemplate, data, http.StatusOK)
}

// handlePage renders a custom page at /:slug. Template resolution is
// <slug>.html first, then the generic page.html, then the 404 template with a
// 404 status (per the locked slug.html chain). Drafts are hidden unless the
// request carries ?preview=1 with an admin JWT.
func (r *Renderer) handlePage(c *gin.Context) {
	theme := r.getRequestedTheme(c)
	site := r.buildSiteData(c.Request.Context())
	slug := c.Param("slug")
	if slug == "" || strings.Contains(slug, "/") {
		r.renderNotFound(c, theme, site)
		return
	}

	preview := c.Query("preview") == "1" && r.isPreviewAdmin(c)

	var page model.Page
	query := r.db.WithContext(c.Request.Context()).Where("slug = ?", slug)
	if !preview {
		query = query.Where("status = ?", model.PageStatusPublished)
	}
	if err := query.First(&page).Error; err != nil {
		r.renderNotFound(c, theme, site)
		return
	}
	if page.Status != model.PageStatusPublished && !preview {
		r.renderNotFound(c, theme, site)
		return
	}

	data := PageData{Site: site, Pages: r.buildNavPages(c.Request.Context())}
	data.Page.ID = page.ID
	data.Page.Title = page.Title
	data.Page.Slug = page.Slug
	data.Page.CreatedAt = page.CreatedAt
	data.Page.UpdatedAt = page.UpdatedAt
	data.Page.ContentHTML = RenderPageContent(page.Content)
	data.Page.URL = "/" + page.Slug

	if dedicated := page.Slug + ".html"; r.themeProvides(theme, dedicated) {
		r.servePage(c, theme, dedicated, data, http.StatusOK)
		return
	}
	if r.themeProvides(theme, PageTemplate) {
		r.servePage(c, theme, PageTemplate, data, http.StatusOK)
		return
	}
	r.renderNotFound(c, theme, site)
}

// parsePageParam reads the 1-based ?page= query parameter, defaulting to 1.
func parsePageParam(c *gin.Context) int {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}
