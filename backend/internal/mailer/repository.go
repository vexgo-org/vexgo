package mailer

import (
	"context"
	"fmt"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type Repository interface {
	GetSMTPSetting(ctx context.Context) (*model.SMTPConfig, error)
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) GetSMTPSetting(ctx context.Context) (*model.SMTPConfig, error) {
	var config model.SMTPConfig
	if err := r.db.WithContext(ctx).First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get SMTP config: %w", err)
	}
	return &config, nil
}
