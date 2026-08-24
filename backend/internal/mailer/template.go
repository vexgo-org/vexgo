package mailer

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

// RenderHTMLTemplate renders HTML templates.
func RenderHTMLTemplate(
	templatePath string,
	data any,
) (string, error) {
	tmpl, err := htmltemplate.ParseFiles(templatePath)
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
	tmpl, err := texttemplate.ParseFiles(templatePath)
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
