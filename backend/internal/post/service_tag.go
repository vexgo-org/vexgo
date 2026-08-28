package post

import (
	"context"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// Tags returns all tags.
func (s *Service) Tags(ctx context.Context, userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Tag{}, nil
	}
	return s.repo.FindAllTags(ctx)
}

// CreateTag creates a tag.
func (s *Service) CreateTag(ctx context.Context, role, name string) (*model.Tag, error) {
	if !model.IsContributor(role) {
		return nil, ErrForbidden
	}
	return s.repo.FindOrCreateTag(ctx, name)
}

// resolveTags takes names and returns Tag models (creating missing ones).
func (s *Service) resolveTags(ctx context.Context, names []string) ([]model.Tag, error) {
	var tags []model.Tag
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tag, err := s.repo.FindOrCreateTag(ctx, name)
		if err != nil {
			return nil, err
		}
		tags = append(tags, *tag)
	}
	return tags, nil
}
