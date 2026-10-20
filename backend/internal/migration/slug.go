package migration

import (
	"fmt"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

// backfillSlugs generates unique slugs for existing posts that have an empty
// slug column, deriving them from each post's title.
func backfillSlugs(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Post{}) {
		return nil
	}

	var posts []model.Post
	if err := db.Select("id", "title").Where("slug = '' OR slug IS NULL").Find(&posts).Error; err != nil {
		return fmt.Errorf("backfill slugs failed: %w", err)
	}
	if len(posts) == 0 {
		return nil
	}

	for _, post := range posts {
		base := model.SlugFromTitle(post.Title)
		if base == "" {
			base = "post"
		}
		// Fall back to a stable post-id-based slug when the generated slug
		// fails validation (e.g. pure-numeric slugs like "2023").
		if err := model.ValidateSlug(base); err != nil {
			base = fmt.Sprintf("post-%d", post.ID)
		}
		slug := base
		counter := 1
		for {
			// Check for collision with an existing slug in the database.
			var existing model.Post
			err := db.Where("slug = ? AND id != ?", slug, post.ID).First(&existing).Error
			if err != nil {
				break // no collision
			}
			counter++
			slug = fmt.Sprintf("%s-%d", base, counter)
		}
		if err := db.Model(&post).UpdateColumn("slug", slug).Error; err != nil {
			return fmt.Errorf("backfill slugs failed (post_id=%d): %w", post.ID, err)
		}
	}

	return nil
}
