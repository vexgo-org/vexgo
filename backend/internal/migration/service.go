package migration

import (
	"github.com/vexgo-org/vexgo/backend/internal/secrets"
	"gorm.io/gorm"
)

type Deps struct {
	DB     *gorm.DB
	Cipher secrets.SecretCipher
}

type Service struct {
	db     *gorm.DB
	cipher secrets.SecretCipher
}

func NewService(deps Deps) *Service {
	return &Service{
		db:     deps.DB,
		cipher: deps.Cipher,
	}
}

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

	return nil
}
