package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/public"
	"gorm.io/gorm"
)

// ThemeManager is the seam to the public domain's theme management. The
// settings domain owns the theme catalog (install, activate, delete) and the
// public domain owns the themes themselves, so the settings domain declares
// the operations it needs and the public service satisfies them.
type ThemeManager interface {
	// GetAvailableThemes lists the installed themes, the embedded default one
	// included.
	GetAvailableThemes() []public.ThemeInfo
	// ThemeExists reports whether a theme is installed.
	ThemeExists(themeID string) bool
	// AvailableLanguages lists the i18n language codes a theme ships.
	AvailableLanguages(themeID string) []string
	// ValidateThemeTemplates reports whether every template of a theme parses.
	ValidateThemeTemplates(themeID string) error
	// InvalidateThemeCache drops the cached templates of a theme.
	InvalidateThemeCache(themeID string)
	// ThemePreviewURL mints a signed, short-lived preview link for a theme.
	ThemePreviewURL(themeID string) (string, error)
	// EnsureThemeSeeds creates the theme's seed pages for missing slugs.
	EnsureThemeSeeds(ctx context.Context, themeID string) (int, error)
	// DataDir returns the data directory holding the installed themes.
	DataDir() string
}

// GetThemes returns all available themes.
func (s *Service) GetThemes() []public.ThemeInfo {
	return s.themes.GetAvailableThemes()
}

// ThemeLanguages lists the i18n language codes shipped by one theme.
func (s *Service) ThemeLanguages(themeID string) ([]string, error) {
	if !s.themes.ThemeExists(themeID) {
		return nil, ErrThemeNotFound
	}
	langs := s.themes.AvailableLanguages(themeID)
	if langs == nil {
		langs = []string{}
	}
	return langs, nil
}

// DeleteTheme removes an installed theme from disk. The built-in default
// theme and the currently active theme are protected.
func (s *Service) DeleteTheme(ctx context.Context, themeID string) error {
	if themeID == "" || themeID == public.DefaultTheme {
		return ErrCannotDeleteDefault
	}
	if !s.themes.ThemeExists(themeID) {
		return ErrThemeNotFound
	}
	active, err := s.GetThemeConfig(ctx)
	if err != nil {
		return err
	}
	if active == themeID {
		return ErrThemeIsActive
	}
	themeDir := filepath.Join(s.themes.DataDir(), public.ThemesDir, themeID)
	if err := os.RemoveAll(themeDir); err != nil {
		return fmt.Errorf("failed to delete theme directory: %w", err)
	}
	s.themes.InvalidateThemeCache(themeID)
	return nil
}

// GetThemeConfig returns the currently active theme stored in the database,
// falling back to the default theme.
func (s *Service) GetThemeConfig(ctx context.Context) (string, error) {
	config, err := s.repo.GetThemeConfig(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return public.DefaultTheme, nil
		}
		return "", err
	}
	activeTheme := config.ActiveTheme
	if activeTheme == "" {
		activeTheme = public.DefaultTheme
	}
	return activeTheme, nil
}

// UpdateThemeConfig sets the globally active theme in the database. The
// theme must exist and its templates must parse; a broken theme is rejected
// so activation can never take the public site down.
func (s *Service) UpdateThemeConfig(ctx context.Context, activeTheme string) (string, error) {
	// Validate that the requested theme actually exists
	if !s.themes.ThemeExists(activeTheme) {
		return "", ErrThemeNotFound
	}

	// Strong validation: every template must parse before activation.
	if err := s.themes.ValidateThemeTemplates(activeTheme); err != nil {
		return "", fmt.Errorf("%w: %v", ErrThemeTemplatesInvalid, err)
	}

	config, err := s.repo.GetThemeConfig(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		config = model.ThemeConfig{ActiveTheme: activeTheme}
		if err := s.repo.CreateThemeConfig(ctx, &config); err != nil {
			return "", fmt.Errorf("failed to save theme config: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("failed to get theme config: %w", err)
	} else {
		config.ActiveTheme = activeTheme
		if err := s.repo.SaveThemeConfig(ctx, &config); err != nil {
			return "", fmt.Errorf("failed to update theme config: %w", err)
		}
	}

	// Seed the newly activated theme's default pages (missing slugs only).
	if _, err := s.themes.EnsureThemeSeeds(ctx, config.ActiveTheme); err != nil {
		slog.Warn("failed to ensure theme seed pages", "theme", config.ActiveTheme, "err", err)
	}

	return config.ActiveTheme, nil
}

// ValidatePreviewURL reports whether a theme preview/cover value is allowed.
// Empty means no cover. Otherwise it must be an http(s) URL of at most
// 2048 characters. Local paths are rejected to shrink the attack surface.
func ValidatePreviewURL(preview string) bool {
	if preview == "" {
		return true
	}
	if len(preview) > 2048 {
		return false
	}
	u, err := url.Parse(preview)
	if err != nil || u == nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}
