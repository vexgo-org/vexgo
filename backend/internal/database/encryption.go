package database

import (
	"fmt"
	"log/slog"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/secrets"

	"gorm.io/gorm"
)

// Encrypter is the seam the migration needs from the cipher: sealing a
// plaintext secret. It is satisfied by secrets.Cipher.
type Encrypter interface {
	Encrypt(plaintext string) (string, error)
}

// MigrateSecretsAtRest encrypts the plaintext secrets at rest in place: the
// SMTP password and the AI / comment-moderation API keys. It runs once at
// startup when a settings_encryption_key is configured, before any service
// reads those rows.
//
// The migration is idempotent: values already carrying the encrypted marker
// (from a previous run) are left untouched, so repeated startups and re-runs
// are no-ops.
func MigrateSecretsAtRest(db *gorm.DB, c Encrypter) (int, error) {
	migrated := 0

	// SMTP password.
	rows, err := plaintextSecrets(db, &model.SMTPConfig{}, "password")
	if err != nil {
		return migrated, err
	}
	for _, row := range rows {
		encrypted, err := c.Encrypt(row.value)
		if err != nil {
			return migrated, fmt.Errorf("encrypt smtp password (id=%d): %w", row.id, err)
		}
		if err := db.Model(&model.SMTPConfig{}).Where("id = ?", row.id).UpdateColumn("password", encrypted).Error; err != nil {
			return migrated, fmt.Errorf("store encrypted smtp password (id=%d): %w", row.id, err)
		}
		migrated++
	}

	// AI API key.
	rows, err = plaintextSecrets(db, &model.AIConfig{}, "api_key")
	if err != nil {
		return migrated, err
	}
	for _, row := range rows {
		encrypted, err := c.Encrypt(row.value)
		if err != nil {
			return migrated, fmt.Errorf("encrypt ai api key (id=%d): %w", row.id, err)
		}
		if err := db.Model(&model.AIConfig{}).Where("id = ?", row.id).UpdateColumn("api_key", encrypted).Error; err != nil {
			return migrated, fmt.Errorf("store encrypted ai api key (id=%d): %w", row.id, err)
		}
		migrated++
	}

	// Comment-moderation API key.
	rows, err = plaintextSecrets(db, &model.CommentModerationConfig{}, "api_key")
	if err != nil {
		return migrated, err
	}
	for _, row := range rows {
		encrypted, err := c.Encrypt(row.value)
		if err != nil {
			return migrated, fmt.Errorf("encrypt moderation api key (id=%d): %w", row.id, err)
		}
		if err := db.Model(&model.CommentModerationConfig{}).Where("id = ?", row.id).UpdateColumn("api_key", encrypted).Error; err != nil {
			return migrated, fmt.Errorf("store encrypted moderation api key (id=%d): %w", row.id, err)
		}
		migrated++
	}

	return migrated, nil
}

// plaintextSecret holds one row whose secret column is still plaintext.
type plaintextSecret struct {
	id    uint
	value string
}

// plaintextSecrets selects (id, column) pairs from the given table where the
// column is non-empty and does not yet carry the encrypted marker.
func plaintextSecrets(db *gorm.DB, dest any, column string) ([]plaintextSecret, error) {
	rows, err := db.Model(dest).Where(column+" != ''").Select("id", column).Rows()
	if err != nil {
		return nil, fmt.Errorf("list plaintext secrets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("close plaintext secrets rows failed", "err", closeErr)
		}
	}()

	var out []plaintextSecret
	for rows.Next() {
		var id uint
		var value string
		if err := rows.Scan(&id, &value); err != nil {
			return nil, fmt.Errorf("scan plaintext secrets: %w", err)
		}
		if secrets.IsEncrypted(value) {
			continue
		}
		out = append(out, plaintextSecret{id: id, value: value})
	}
	return out, rows.Err()
}
