package mailer

type VerificationEmailTemplateData struct {
	Name string
	Link string
}

type PasswordResetEmailTemplateData struct {
	Name string
	Link string
}

type EmailChangeEmailTemplateData struct {
	Name     string
	Link     string
	NewEmail string
}

type TestSMTPEmailTemplateData struct {
	Name  string
	Host  string
	Port  int
	Email string
	Time  string
}
