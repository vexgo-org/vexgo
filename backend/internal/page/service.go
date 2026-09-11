// Package page implements the custom-page domain (standalone pages at /:slug).
package page

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Sentinel errors mapped to HTTP responses by the handler.
var (
	ErrPageNotFound = errors.New("page not found")
	ErrForbidden    = errors.New("forbidden")
	ErrBadRequest   = errors.New("bad request")
)

// ReservedSlugs may never be used as a page slug. Single-segment system
// routes (admin SPA, API, theme assets, legacy redirects) take precedence
// over the /:slug catch-all, so these names are rejected at the service
// layer as well as in the frontend form validation.
var ReservedSlugs = map[string]struct{}{
	"post": {}, "posts": {}, "user": {}, "users": {},
	"admin": {}, "api": {}, "theme-assets": {}, "themes": {}, "uploads": {},
	"assets": {}, "favicon.ico": {},
	"login": {}, "register": {}, "reset-password": {}, "verify-email": {},
	"write": {}, "edit-post": {}, "profile": {}, "my-posts": {},
	"notifications": {}, "settings": {}, "moderation": {},
}

// IsReservedSlug reports whether slug is a system-reserved name.
func IsReservedSlug(slug string) bool {
	_, ok := ReservedSlugs[strings.ToLower(strings.TrimSpace(slug))]
	return ok
}

// Deps holds the dependencies required by the page domain.
type Deps struct {
	DB        *gorm.DB
	JWTSecret []byte
}

// Service contains the business logic of the page domain.
type Service struct {
	repo Repository
}

// NewService creates a page service with the given dependencies.
func NewService(deps Deps) *Service {
	return &Service{repo: NewRepository(deps.DB)}
}

// newServiceWithRepo creates a service around an explicit repository (tests).
func newServiceWithRepo(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListQuery carries pagination and filters for List.
type ListQuery struct {
	Status string
	Search string
	Page   int
	Limit  int
}

// normalizeSlug lowercases and trims the slug before validation and storage.
func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// validatePageSlug checks format plus the reserved-word blocklist.
func validatePageSlug(slug string) error {
	if err := model.ValidateSlug(slug); err != nil {
		// Page slugs are ASCII-only per the locked decision (Q18): reject
		// non-ASCII early with the same invalid-slug error.
		return err
	}
	for _, r := range slug {
		if r > 127 {
			return model.ErrInvalidSlug
		}
	}
	if IsReservedSlug(slug) {
		return fmt.Errorf("%w: %q is reserved", ErrBadRequest, slug)
	}
	return nil
}

// List returns pages ordered by sortOrder. Empty status means all statuses
// (admin console); callers pass "published" for the public read path.
func (s *Service) List(ctx context.Context, q ListQuery) ([]model.Page, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	if len(q.Search) > 200 {
		q.Search = q.Search[:200]
	}
	return s.repo.List(ctx, q.Status, q.Search, q.Page, q.Limit)
}

// GetBySlug returns one page by slug. Published pages are public; drafts are
// only visible to admins (SSR preview and admin console).
func (s *Service) GetBySlug(ctx context.Context, slug, role string) (*model.Page, error) {
	page, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	if page.Status != model.PageStatusPublished && !model.IsAdmin(role) {
		return nil, ErrPageNotFound
	}
	return page, nil
}

// CreateRequest carries the fields accepted when creating a page.
type CreateRequest struct {
	Slug      string
	Title     string
	Content   string
	ShowInNav bool
	SortOrder int
	Status    model.PageStatus
}

// Create creates a page; only admins may create pages.
func (s *Service) Create(ctx context.Context, role string, userID uint, req CreateRequest) (*model.Page, error) {
	if !model.IsAdmin(role) {
		return nil, ErrForbidden
	}
	req.Slug = normalizeSlug(req.Slug)
	if err := validatePageSlug(req.Slug); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return nil, ErrBadRequest
	}
	exists, err := s.repo.SlugExists(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, model.ErrSlugTaken
	}
	status := req.Status
	if status != model.PageStatusPublished {
		status = model.PageStatusDraft
	}
	page := model.Page{
		Slug: req.Slug, Title: req.Title, Content: req.Content,
		ShowInNav: req.ShowInNav, SortOrder: req.SortOrder,
		Status: status, AuthorID: userID,
	}
	if err := s.repo.Create(ctx, &page); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, model.ErrSlugTaken
		}
		return nil, err
	}
	return &page, nil
}

// UpdateRequest carries the fields accepted when updating a page.
type UpdateRequest struct {
	Slug      string
	Title     string
	Content   string
	ShowInNav *bool
	SortOrder *int
	Status    model.PageStatus
}

// Update modifies a page; only admins may update pages.
func (s *Service) Update(ctx context.Context, id, role string, req UpdateRequest) (*model.Page, error) {
	if !model.IsAdmin(role) {
		return nil, ErrForbidden
	}
	page, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	if req.Slug != "" {
		slug := normalizeSlug(req.Slug)
		if slug != page.Slug {
			if err := validatePageSlug(slug); err != nil {
				return nil, err
			}
			exists, err := s.repo.SlugExistsExcludeID(ctx, slug, page.ID)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, model.ErrSlugTaken
			}
			page.Slug = slug
		}
	}
	if req.Title != "" {
		page.Title = req.Title
	}
	if req.Content != "" {
		page.Content = req.Content
	}
	if req.ShowInNav != nil {
		page.ShowInNav = *req.ShowInNav
	}
	if req.SortOrder != nil {
		page.SortOrder = *req.SortOrder
	}
	if req.Status == model.PageStatusPublished || req.Status == model.PageStatusDraft {
		page.Status = req.Status
	}
	if err := s.repo.Save(ctx, page); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, model.ErrSlugTaken
		}
		return nil, err
	}
	return page, nil
}

// Delete removes a page; only admins may delete pages.
func (s *Service) Delete(ctx context.Context, id, role string) error {
	if !model.IsAdmin(role) {
		return ErrForbidden
	}
	page, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPageNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, page)
}
