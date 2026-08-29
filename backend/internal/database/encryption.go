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
	// Each target is one secret column: a readable label for error messages,
	// the owning model, and the column holding the secret.
	targets := []struct {
		label  string
		model  any
		column string
	}{
		{label: "smtp password", model: &model.SMTPConfig{}, column: "password"},
		{label: "ai api key", model: &model.AIConfig{}, column: "api_key"},
		{label: "moderation api key", model: &model.CommentModerationConfig{}, column: "api_key"},
	}

	var migrated int
	for _, target := range targets {
		rows, err := plaintextSecrets(db, target.model, target.column)
		if err != nil {
			return migrated, err
		}
		for _, row := range rows {
			encrypted, err := c.Encrypt(row.value)
			if err != nil {
				return migrated, fmt.Errorf("encrypt %s (id=%d): %w", target.label, row.id, err)
			}
			err = db.Model(target.model).
				Where("id = ?", row.id).
				UpdateColumn(target.column, encrypted).
				Error
			if err != nil {
				return migrated, fmt.Errorf("store encrypted %s (id=%d): %w", target.label, row.id, err)
			}
			migrated++
		}
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

	out := []plaintextSecret{}
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plaintext secrets: %w", err)
	}
	return out, nil
}
