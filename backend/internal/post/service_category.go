package post

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Categories returns all categories with their per-category post counts.
func (s *Service) Categories(ctx context.Context, userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Category{}, nil
	}
	categories, err := s.repo.FindAllCategories(ctx)
	if err != nil {
		return nil, err
	}
	s.fillCategoryUsage(ctx, categories)
	return categories, nil
}

// CreateCategory creates a category. The name is trimmed; a blank name after
// trimming is rejected so whitespace-only input cannot bypass the unique
// index or create invisible near-duplicates.
func (s *Service) CreateCategory(ctx context.Context, role, name, description string) (*model.Category, error) {
	if !model.IsContributor(role) {
		return nil, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrBadRequest
	}

	category := &model.Category{Name: name, Description: description}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// DeleteCategory deletes an empty category (contributor and above). A
// category is empty when no post's category string equals its name; a
// category still in use is rejected with an InUseError carrying the count.
func (s *Service) DeleteCategory(ctx context.Context, role string, id uint) error {
	if !model.IsContributor(role) {
		return ErrForbidden
	}

	deleted, usage, err := s.repo.DeleteCategoryIfEmpty(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCategoryNotFound
		}
		return err
	}
	if !deleted {
		return &InUseError{Count: usage}
	}
	return nil
}

// fillCategoryUsage batch-loads per-category post counts to avoid N+1.
func (s *Service) fillCategoryUsage(ctx context.Context, categories []model.Category) {
	if len(categories) == 0 {
		return
	}
	names := make([]string, len(categories))
	for i := range categories {
		names[i] = categories[i].Name
	}
	counts, err := s.repo.BatchCountCategoryUsage(ctx, names)
	if err != nil {
		slog.Warn("failed to count category usage", "err", err)
		return
	}
	for i := range categories {
		categories[i].PostCount = counts[categories[i].Name]
	}
}
