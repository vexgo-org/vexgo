package post

import (
	"context"

	"vexgo/backend/internal/model"
)

// Categories returns all categories.
func (s *Service) Categories(ctx context.Context, userRole string) ([]model.Category, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Category{}, nil
	}
	return s.repo.FindAllCategories(ctx)
}

// CreateCategory creates a category.
func (s *Service) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	category := &model.Category{Name: name, Description: description}
	if err := s.repo.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}
