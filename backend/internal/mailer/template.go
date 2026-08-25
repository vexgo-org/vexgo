package mailer

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var templateFS embed.FS

const (
	verificationEmailTemplateText  = "templates/verification.txt"
	verificationEmailTemplateHTML  = "templates/verification.html"
	resetPasswordEmailTemplateText = "templates/password-reset.txt"
	resetPasswordEmailTemplateHTML = "templates/password-reset.html"
)

// RenderHTMLTemplate renders HTML templates.
func RenderHTMLTemplate(
	templatePath string,
	data any,
) (string, error) {
	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read html email template failed: %w", err)
	}

	tmpl, err := htmltemplate.New(templatePath).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf(
			"parse HTML email template %q: %w",
			templatePath,
			err,
		)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf(
			"render HTML email template %q: %w",
			templatePath,
			err,
		)
	}

	return buffer.String(), nil
}

// RenderTextTemplate renders text templates.
func RenderTextTemplate(
	templatePath string,
	data any,
) (string, error) {
	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read text email template failed: %w", err)
	}

	tmpl, err := texttemplate.New(templatePath).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf(
			"parse text email template %q: %w",
			templatePath,
			err,
		)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf(
			"render text email template %q: %w",
			templatePath,
			err,
		)
	}

	return buffer.String(), nil
}
