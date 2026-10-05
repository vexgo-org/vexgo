package post

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

// Tags returns all tags with their per-tag post counts.
func (s *Service) Tags(ctx context.Context, userRole string) ([]model.Tag, error) {
	if userRole == "" && !s.allowGuestView(ctx) {
		return []model.Tag{}, nil
	}
	tags, err := s.repo.FindAllTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("find tags: %w", err)
	}
	s.fillTagUsage(ctx, tags)
	return tags, nil
}

// CreateTag creates a tag. The name is trimmed, matching resolveTags, so
// leading/trailing whitespace cannot create near-duplicates of an existing
// tag; a blank name after trimming is rejected.
func (s *Service) CreateTag(ctx context.Context, role, name string) (*model.Tag, error) {
	if !model.IsContributor(role) {
		return nil, ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrBadRequest
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

// DeleteTag deletes an empty tag (contributor and above). A tag is empty
// when no post references it via the post_tags join table; a tag still in
// use is rejected with an InUseError carrying the count.
func (s *Service) DeleteTag(ctx context.Context, role string, id uint) error {
	if !model.IsContributor(role) {
		return ErrForbidden
	}

	deleted, usage, err := s.repo.DeleteTagIfEmpty(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTagNotFound
		}
		return fmt.Errorf("delete tag: %w", err)
	}
	if !deleted {
		return &InUseError{Count: usage}
	}
	return nil
}

// fillTagUsage batch-loads per-tag post counts to avoid N+1.
func (s *Service) fillTagUsage(ctx context.Context, tags []model.Tag) {
	if len(tags) == 0 {
		return
	}
	tagIDs := make([]uint, len(tags))
	for i := range tags {
		tagIDs[i] = tags[i].ID
	}
	counts, err := s.repo.BatchCountTagUsage(ctx, tagIDs)
	if err != nil {
		slog.Warn("failed to count tag usage", "err", err)
		return
	}
	for i := range tags {
		tags[i].PostCount = counts[tags[i].ID]
	}
}
