package settings

import "errors"

// Sentinel errors mapped to HTTP responses by the handler. Each error carries
// the exact message of the original handler response it replaces.
var (
	// SMTP config errors.
	ErrSMTPNotConfigured = errors.New("please configure SMTP settings first")
	ErrSMTPDisabled      = errors.New("SMTP is not enabled, please enable and save configuration first")
	ErrSMTPIncomplete    = errors.New("please fill in all SMTP configuration fields")
	ErrSMTPNoRecipient   = errors.New("please fill in test email address first")

	// AI config errors.
	ErrAINotConfigured    = errors.New("please configure AI settings first")
	ErrAIDisabled         = errors.New("AI is not enabled, please enable and save configuration first")
	ErrAIIncomplete       = errors.New("please fill in all AI configuration fields (endpoint, API key, model name)")
	ErrAIIncompleteModels = errors.New("please fill in all AI configuration fields (endpoint, API key)")

	// Theme errors.
	ErrThemeNotFound         = errors.New("theme not found")
	ErrInvalidThemeMeta      = errors.New("invalid theme metadata")
	ErrInvalidPreviewURL     = errors.New("invalid preview URL")
	ErrThemeTemplatesInvalid = errors.New("theme templates are invalid")
	ErrThemeIsActive         = errors.New("active theme cannot be deleted")
	ErrCannotDeleteDefault   = errors.New("default theme cannot be deleted")

	// General settings errors.
	ErrInvalidSiteIcon = errors.New("invalid site icon")
)
