package page

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
	"github.com/vexgo-org/vexgo/backend/internal/middleware"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Handler exposes the page domain over HTTP.
type Handler struct {
	svc *Service
	mw  *middleware.Auth
}

// NewHandler creates a page HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps), mw: middleware.NewAuth(deps.DB, deps.JWTSecret)}
}

// GetPages godoc
//
//	@Summary		List pages
//	@Description	Public callers see published pages; admins may filter by status.
//	@Tags			pages
//	@Produce		json
//	@Param			page	query		int		false	"page number (1-based)"	default(1)
//	@Param			limit	query		int		false	"page size"				default(20)
//	@Param			status	query		string	false	"status filter"			Enums(draft,published)
//	@Param			search	query		string	false	"free-text filter"
//	@Success		200		{object}	PageListResponse
//	@Failure		500		{object}	api.ErrorResponse
//	@Router			/pages [get]
func (h *Handler) GetPages(c *gin.Context) {
	page, limit := middleware.ParsePagination(c, 20)
	status := c.DefaultQuery("status", "")
	search := c.DefaultQuery("search", "")

	u, _ := middleware.CurrentUser(c)
	// Non-admins may only list published pages; a status filter for drafts
	// is silently narrowed to published.
	if !model.IsAdmin(u.Role) {
		if status != "" && status != string(model.PageStatusPublished) {
			status = string(model.PageStatusPublished)
		}
		if status == "" {
			status = string(model.PageStatusPublished)
		}
	}

	pages, total, err := h.svc.List(c.Request.Context(), ListQuery{
		Status: status, Search: search, Page: page, Limit: limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to fetch pages"})
		return
	}
	totalPages := (int(total) + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	c.JSON(http.StatusOK, PageListResponse{
		Pages: pages,
		Pagination: Pagination{
			Total: total, Page: page, Limit: limit, TotalPages: totalPages,
		},
	})
}

// GetPage godoc
//
//	@Summary		Look up a page by slug
//	@Description	Published pages are public; drafts require an admin session.
//	@Tags			pages
//	@Produce		json
//	@Param			slug	path		string	true	"page slug"
//	@Success		200		{object}	PageSingleResponse
//	@Failure		404		{object}	api.ErrorResponse	"page not found"
//	@Router			/pages/{slug} [get]
func (h *Handler) GetPage(c *gin.Context) {
	slug := c.Param("slug")
	u, _ := middleware.CurrentUser(c)
	page, err := h.svc.GetBySlug(c.Request.Context(), slug, u.Role)
	if err != nil {
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Page does not exist"})
		return
	}
	c.JSON(http.StatusOK, PageSingleResponse{Page: page})
}

// CreatePage godoc
//
//	@Summary		Create a page
//	@Description	Admins only. Slug must be lowercase a-z0-9- and not reserved.
//	@Tags			pages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		CreatePageRequest	true	"page payload"
//	@Success		201		{object}	PageMessageResponse
//	@Failure		400		{object}	api.ErrorResponse		"validation error"
//	@Failure		403		{object}	api.ErrorResponse		"admin only"
//	@Failure		409		{object}	api.CodeErrorResponse	"slug already taken"
//	@Router			/pages [post]
func (h *Handler) CreatePage(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Please log in first"})
		return
	}
	var req CreatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}
	page, err := h.svc.Create(c.Request.Context(), u.Role, u.ID, CreateRequest{
		Slug: req.Slug, Title: req.Title, Content: req.Content,
		ShowInNav: req.ShowInNav, SortOrder: req.SortOrder,
		Status: model.PageStatus(req.Status),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Admins only"})
		case errors.Is(err, model.ErrSlugTaken):
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "Slug is already taken by another page", Code: "slug_taken"})
		case errors.Is(err, ErrBadRequest) || errors.Is(err, model.ErrEmptySlug) || errors.Is(err, model.ErrInvalidSlug) || errors.Is(err, model.ErrSlugTooLong):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to create page"})
		}
		return
	}
	c.JSON(http.StatusCreated, PageMessageResponse{Message: "Page created successfully", Page: page})
}

// UpdatePage godoc
//
//	@Summary		Update a page
//	@Description	Admins only. Only the supplied fields are updated.
//	@Tags			pages
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"page id"
//	@Param			request	body		UpdatePageRequest	true	"updated page fields"
//	@Success		200		{object}	PageMessageResponse
//	@Failure		400		{object}	api.ErrorResponse		"invalid slug"
//	@Failure		403		{object}	api.ErrorResponse		"admin only"
//	@Failure		404		{object}	api.ErrorResponse		"page not found"
//	@Failure		409		{object}	api.CodeErrorResponse	"slug already taken"
//	@Router			/pages/{id} [put]
func (h *Handler) UpdatePage(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Please log in first"})
		return
	}
	var req UpdatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: "Invalid request payload"})
		return
	}
	page, err := h.svc.Update(c.Request.Context(), c.Param("id"), u.Role, UpdateRequest{
		Slug: req.Slug, Title: req.Title, Content: req.Content,
		ShowInNav: req.ShowInNav, SortOrder: req.SortOrder,
		Status: model.PageStatus(req.Status),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPageNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Page does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Admins only"})
		case errors.Is(err, model.ErrSlugTaken):
			c.JSON(http.StatusConflict, api.CodeErrorResponse{Error: "Slug is already taken by another page", Code: "slug_taken"})
		case errors.Is(err, ErrBadRequest) || errors.Is(err, model.ErrEmptySlug) || errors.Is(err, model.ErrInvalidSlug) || errors.Is(err, model.ErrSlugTooLong):
			c.JSON(http.StatusBadRequest, api.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to update page"})
		}
		return
	}
	c.JSON(http.StatusOK, PageMessageResponse{Message: "Page updated successfully", Page: page})
}

// DeletePage godoc
//
//	@Summary		Delete a page
//	@Description	Admins only.
//	@Tags			pages
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"page id"
//	@Success		200	{object}	PageDeleteResponse
//	@Failure		403	{object}	api.ErrorResponse	"admin only"
//	@Failure		404	{object}	api.ErrorResponse	"page not found"
//	@Router			/pages/{id} [delete]
func (h *Handler) DeletePage(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Please log in first"})
		return
	}
	if _, err := strconv.Atoi(c.Param("id")); err != nil {
		// Keep numeric-id validation consistent: non-numeric ids are 404.
		c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Page does not exist"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), c.Param("id"), u.Role); err != nil {
		switch {
		case errors.Is(err, ErrPageNotFound):
			c.JSON(http.StatusNotFound, api.ErrorResponse{Error: "Page does not exist"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, api.ErrorResponse{Error: "Admins only"})
		default:
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{Error: "Failed to delete page"})
		}
		return
	}
	c.JSON(http.StatusOK, PageDeleteResponse{Message: "Page deleted successfully"})
}
