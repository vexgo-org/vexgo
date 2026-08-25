package mailer

type verificationEamilTemplateData struct {
	Name string
	Link string
}

type passwordResetEmailTemplateData struct {
	Name string
	Link string
}

type emailChangeEmailTemplateData struct {
	Name     string
	Link     string
	NewEmail string
}
