package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"gorm.io/gorm"
)

type AssetConfigRequest struct {
	PruneInterval      string
	PruneRetentionDays int
}

// UpdateAssetConfig updates asset configuration in the database, and
// reschedules the prune job.
func (s *Service) UpdateAssetConfig(ctx context.Context, req AssetConfigRequest) (*model.AssetConfig, error) {
	// Validate cron expression.
	if _, err := cron.ParseStandard(req.PruneInterval); err != nil {
		return nil, fmt.Errorf("%w: %q", ErrInvalidCron, req.PruneInterval)
	}

	config, err := s.repo.GetAssetConfig(ctx)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		config = &model.AssetConfig{
			PruneInterval:      req.PruneInterval,
			PruneRetentionDays: req.PruneRetentionDays,
		}
		if err := s.repo.CreateAssetConfig(ctx, config); err != nil {
			return nil, fmt.Errorf("create asset config: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("get asset config: %w", err)
	default:
		config.PruneInterval = req.PruneInterval
		config.PruneRetentionDays = req.PruneRetentionDays
		if err := s.repo.SaveAssetConfig(ctx, config); err != nil {
			return nil, fmt.Errorf("save asset config: %w", err)
		}
	}

	// Reschedule the prune job after the database write.
	if err := s.assets.ReschedulePruneJob(ctx); err != nil {
		return nil, fmt.Errorf("reschedule prune job: %w", err)
	}

	return config, nil
}

// GetAssetConfig returns the stored asset configuration.
// If the configuration does not exist, it
// returns a default one.
func (s *Service) GetAssetConfig(ctx context.Context) (*model.AssetConfig, error) {
	config, err := s.repo.GetAssetConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.AssetConfig{
			PruneInterval:      "@monthly",
			PruneRetentionDays: 30,
		}, nil
	}
	return config, err
}
