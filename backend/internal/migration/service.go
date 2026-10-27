// Package migration runs the startup migrations that bring an existing
// installation's schema and data up to date. It is invoked once from the
// composition root, before any domain service reads the rows it touches, and
// deliberately exposes no handlers or routes.
package migration

import (
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
	"github.com/vexgo-org/vexgo/backend/internal/storage"
	"gorm.io/gorm"
)

type Deps struct {
	DB      *gorm.DB
	Cipher  secrets.SecretCipher
	Storage storage.Storage
}

type Service struct {
	db      *gorm.DB
	cipher  secrets.SecretCipher
	storage storage.Storage
}

func NewService(deps Deps) *Service {
	return &Service{
		db:      deps.DB,
		cipher:  deps.Cipher,
		storage: deps.Storage,
	}
}

// MigrateDB creates or updates the schema and applies the data migrations
// that depend on it. Seeding of default records is not part of this: it runs
// afterwards, from database.Seed.
func (s *Service) MigrateDB() error {
	if err := autoMigrate(s.db); err != nil {
		return err
	}

	// Map the legacy single AI-moderation toggle onto the explicit moderation
	// switches before anything reads the config row.
	if _, err := migrateLegacyModerationConfig(s.db); err != nil {
		return err
	}

	// Backfill slugs for existing posts that don't have one yet.
	if err := backfillSlugs(s.db); err != nil {
		return err
	}

	// Backfill asset and blob from legacy media files.
	if err := s.backfillAssetBlob(); err != nil {
		return err
	}

	// Map the legacy site icon URL onto the asset created above.
	if err := s.backfillSiteIcon(); err != nil {
		return err
	}

	// Map legacy user avatar URLs onto the assets created above.
	if err := s.backfillUserAvatar(); err != nil {
		return err
	}

	return nil
}
