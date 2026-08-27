package mailer

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	// A single connection keeps every goroutine on the same :memory: database.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.SMTPConfig{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewService(Deps{DB: db}), db
}

// seedSMTP writes an enabled SMTP config pointing at a closed local port, so
// real sends fail fast with connection refused.
func seedSMTP(t *testing.T, db *gorm.DB) {
	t.Helper()
	cfg := model.SMTPConfig{
		Enabled:   true,
		Host:      "127.0.0.1",
		Port:      2525,
		FromEmail: "admin@vexgo.example",
		FromName:  "VexGo",
	}
	if err := db.Create(&cfg).Error; err != nil {
		t.Fatalf("failed to seed smtp config: %v", err)
	}
}

// capturedEmail holds the rendered parts of one outgoing email.
type capturedEmail struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// emailCapture is a concurrency-safe recorder for the mail capture hook.
type emailCapture struct {
	mu     sync.Mutex
	emails []capturedEmail
}

// install registers the capture hook and restores it via t.Cleanup.
func (c *emailCapture) install(t *testing.T) {
	t.Helper()
	SetMailCaptureHook(func(to, subject, textBody, htmlBody string) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.emails = append(c.emails, capturedEmail{to, subject, textBody, htmlBody})
	})
	t.Cleanup(func() { SetMailCaptureHook(nil) })
}

func (c *emailCapture) all() []capturedEmail {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]capturedEmail(nil), c.emails...)
}

const testLink = "https://vexgo.example/verify-email?token=abc-123"

// assertRendered checks the parts shared by every rendered email and rejects
// leftover template placeholders.
func assertRendered(t *testing.T, got capturedEmail, wantTo, wantSubject string) {
	t.Helper()
	if got.To != wantTo {
		t.Errorf("To = %q, want %q", got.To, wantTo)
	}
	if got.Subject != wantSubject {
		t.Errorf("Subject = %q, want %q", got.Subject, wantSubject)
	}
	for _, body := range []string{got.TextBody, got.HTMLBody} {
		if strings.Contains(body, "{{") {
			t.Errorf("body contains unrendered placeholder:\n%s", body)
		}
	}
}

func TestSendVerificationEmail_Content(t *testing.T) {
	svc, db := newTestService(t)
	seedSMTP(t, db)
	var cap emailCapture
	cap.install(t)

	err := svc.SendVerificationEmail(context.Background(), "user@example.com",
		&VerificationEmailTemplateData{Name: "alice", Link: testLink})
	if err != nil {
		t.Fatalf("SendVerificationEmail error: %v", err)
	}

	emails := cap.all()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	got := emails[0]
	assertRendered(t, got, "user@example.com", "Please Verify Your Email Address")

	if n := strings.Count(got.TextBody, "alice"); n != 1 {
		t.Errorf("text body: name rendered %d times, want 1", n)
	}
	if n := strings.Count(got.TextBody, testLink); n != 1 {
		t.Errorf("text body: link rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, `href="`+testLink+`"`); n != 1 {
		t.Errorf("html body: link href rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, testLink); n != 2 {
		t.Errorf("html body: link rendered %d times, want 2 (href + plain)", n)
	}
}

func TestSendPasswordResetEmail_Content(t *testing.T) {
	svc, db := newTestService(t)
	seedSMTP(t, db)
	var cap emailCapture
	cap.install(t)

	err := svc.SendPasswordResetEmail(context.Background(), "user@example.com",
		&PasswordResetEmailTemplateData{Name: "bob", Link: testLink})
	if err != nil {
		t.Fatalf("SendPasswordResetEmail error: %v", err)
	}

	emails := cap.all()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	got := emails[0]
	assertRendered(t, got, "user@example.com", "Password Reset Request")

	if n := strings.Count(got.TextBody, "bob"); n != 1 {
		t.Errorf("text body: name rendered %d times, want 1", n)
	}
	if n := strings.Count(got.TextBody, testLink); n != 1 {
		t.Errorf("text body: link rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, `href="`+testLink+`"`); n != 1 {
		t.Errorf("html body: link href rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, testLink); n != 2 {
		t.Errorf("html body: link rendered %d times, want 2 (href + plain)", n)
	}
}

func TestSendEmailChangeEmail_Content(t *testing.T) {
	svc, db := newTestService(t)
	seedSMTP(t, db)
	var cap emailCapture
	cap.install(t)

	err := svc.SendEmailChangeEmail(context.Background(), "old@example.com",
		&EmailChangeEmailTemplateData{Name: "carol", Link: testLink, NewEmail: "new@example.com"})
	if err != nil {
		t.Fatalf("SendEmailChangeEmail error: %v", err)
	}

	emails := cap.all()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	got := emails[0]
	assertRendered(t, got, "old@example.com", "Confirm Email Change")

	for _, body := range []string{got.TextBody, got.HTMLBody} {
		if !strings.Contains(body, "new@example.com") {
			t.Errorf("body does not contain new email:\n%s", body)
		}
	}
	if n := strings.Count(got.TextBody, "carol"); n != 1 {
		t.Errorf("text body: name rendered %d times, want 1", n)
	}
	if n := strings.Count(got.TextBody, testLink); n != 1 {
		t.Errorf("text body: link rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, `href="`+testLink+`"`); n != 1 {
		t.Errorf("html body: link href rendered %d times, want 1", n)
	}
	if n := strings.Count(got.HTMLBody, testLink); n != 2 {
		t.Errorf("html body: link rendered %d times, want 2 (href + plain)", n)
	}
}

func TestSendTestSMTPEmail_Content(t *testing.T) {
	svc, db := newTestService(t)
	seedSMTP(t, db)
	var cap emailCapture
	cap.install(t)

	data := &TestSMTPEmailTemplateData{
		Name:  "dave",
		Host:  "smtp.vexgo.example",
		Port:  587,
		Email: "admin@vexgo.example",
		Time:  "2026-08-26 12:00:00",
	}
	err := svc.SendTestSMTPEmail(context.Background(), "admin@vexgo.example", data)
	if err != nil {
		t.Fatalf("SendTestSMTPEmail error: %v", err)
	}

	emails := cap.all()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}
	got := emails[0]
	assertRendered(t, got, "admin@vexgo.example", "SMTP Configuration Test Email")

	if !strings.Contains(got.TextBody, "- SMTP Server: smtp.vexgo.example:587") {
		t.Errorf("text body missing server line:\n%s", got.TextBody)
	}
	if !strings.Contains(got.TextBody, "Sender: dave <admin@vexgo.example>") {
		t.Errorf("text body missing sender line:\n%s", got.TextBody)
	}
	if !strings.Contains(got.TextBody, "Time: 2026-08-26 12:00:00") {
		t.Errorf("text body missing time:\n%s", got.TextBody)
	}
	for _, part := range []string{"smtp.vexgo.example", "587", "admin@vexgo.example", "2026-08-26 12:00:00"} {
		if !strings.Contains(got.HTMLBody, part) {
			t.Errorf("html body missing %q", part)
		}
	}
	if n := strings.Count(got.HTMLBody, "dave"); n != 2 {
		t.Errorf("html body: name rendered %d times, want 2", n)
	}
}

// --- Race conditions ---

// TestSMTPClient_ConcurrentAccess hammers LoadConfig/Enabled/Send from many
// goroutines; run with -race to prove the client's locking is sound. Send
// targets a closed port so it fails fast after touching all shared state.
func TestSMTPClient_ConcurrentAccess(t *testing.T) {
	c := &SMTPClient{}
	cfg := &model.SMTPConfig{
		Enabled:   true,
		Host:      "127.0.0.1",
		Port:      2525,
		FromEmail: "admin@vexgo.example",
		FromName:  "VexGo",
	}
	msg := Message{To: []string{"x@example.com"}, Subject: "s", TextBody: "body"}

	const goroutines, iterations = 8, 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if err := c.LoadConfig(cfg); err != nil {
					t.Errorf("LoadConfig error: %v", err)
					return
				}
				if !c.Enabled() {
					t.Error("expected Enabled() true after LoadConfig")
					return
				}
				_ = c.Send(context.Background(), msg)
			}
		}()
	}
	wg.Wait()
}

// TestSMTPClient_SendDoesNotHoldLockDuringDial proves Send releases the client
// mutex before touching the network. A stalling loopback listener accepts the
// connection but never answers with an SMTP greeting, parking Send mid-dial;
// Accept is the rendezvous point proving the sender is inside its network
// wait, so at that moment the mutex must already be free for other callers
// (auth flows contend on it via LoadConfig/Enabled). On the previous locking
// scheme the mutex was held for the whole SMTP round-trip and this fails.
func TestSMTPClient_SendDoesNotHoldLockDuringDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tcp := ln.(*net.TCPListener)
	if err := tcp.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set accept deadline: %v", err)
	}

	cfg := &model.SMTPConfig{
		Enabled:   true,
		Host:      "127.0.0.1",
		Port:      tcp.Addr().(*net.TCPAddr).Port,
		FromEmail: "a@b.c",
	}
	c := &SMTPClient{}
	if err := c.LoadConfig(cfg); err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	sendDone := make(chan error, 1)
	go func() {
		sendDone <- c.Send(context.Background(), Message{
			To: []string{"x@example.com"}, Subject: "s", TextBody: "b",
		})
	}()

	// The sender has now parked on the silent server.
	if _, err := tcp.Accept(); err != nil {
		t.Fatalf("sender never dialed the stalling server: %v", err)
	}

	const budget = 500 * time.Millisecond
	deadline := time.Now().Add(budget)
	for {
		if c.mu.TryLock() {
			c.mu.Unlock()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client mutex held during SMTP dial: a slow server serializes unrelated requests")
		}
		time.Sleep(2 * time.Millisecond)
	}

	enabledDone := make(chan bool, 1)
	go func() { enabledDone <- c.Enabled() }()
	select {
	case <-enabledDone:
	case <-time.After(budget):
		t.Error("Enabled() starved while Send was dialing")
	}

	ln.Close()
}

// TestService_ConcurrentSends_CapturedExactlyOnce fires concurrent sends
// through the service and asserts none are lost or duplicated.
func TestService_ConcurrentSends_CapturedExactlyOnce(t *testing.T) {
	svc, db := newTestService(t)
	seedSMTP(t, db)
	var cap emailCapture
	cap.install(t)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func() {
			defer wg.Done()
			to := "user" + string(rune('a'+i)) + "@example.com"
			err := svc.SendVerificationEmail(context.Background(), to,
				&VerificationEmailTemplateData{Name: "alice", Link: testLink})
			if err != nil {
				t.Errorf("send %d error: %v", i, err)
			}
		}()
	}
	wg.Wait()

	emails := cap.all()
	if len(emails) != n {
		t.Fatalf("expected %d emails, got %d", n, len(emails))
	}
	seen := make(map[string]int)
	for _, e := range emails {
		seen[e.To]++
	}
	for to, count := range seen {
		if count != 1 {
			t.Errorf("recipient %q received %d emails, want exactly 1", to, count)
		}
	}
}

// --- Error paths ---

func TestSend_NoSMTPConfig(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	data := &VerificationEmailTemplateData{Name: "a", Link: testLink}
	if err := svc.SendVerificationEmail(ctx, "x@example.com", data); err == nil {
		t.Error("expected error without SMTP config")
	}
	if err := svc.SendPasswordResetEmail(ctx, "x@example.com", &PasswordResetEmailTemplateData{}); err == nil {
		t.Error("expected error without SMTP config")
	}
	if err := svc.SendEmailChangeEmail(ctx, "x@example.com", &EmailChangeEmailTemplateData{}); err == nil {
		t.Error("expected error without SMTP config")
	}
	if err := svc.SendTestSMTPEmail(ctx, "x@example.com", &TestSMTPEmailTemplateData{}); err == nil {
		t.Error("expected error without SMTP config")
	}
	if _, err := svc.Enabled(ctx); err == nil {
		t.Error("expected Enabled() error without SMTP config")
	}
}

func TestSend_SMTPDisabled(t *testing.T) {
	svc, db := newTestService(t)
	if err := db.Create(&model.SMTPConfig{Enabled: false, Host: "127.0.0.1", Port: 2525, FromEmail: "a@b.c"}).Error; err != nil {
		t.Fatalf("seed smtp config: %v", err)
	}
	ctx := context.Background()

	if err := svc.SendVerificationEmail(ctx, "x@example.com", &VerificationEmailTemplateData{}); err == nil || !strings.Contains(err.Error(), "SMTP is not enabled") {
		t.Errorf("expected SMTP-not-enabled error, got %v", err)
	}
	enabled, err := svc.Enabled(ctx)
	if err != nil {
		t.Fatalf("Enabled() error: %v", err)
	}
	if enabled {
		t.Error("expected Enabled() false for disabled config")
	}
}

func TestSend_InvalidConfig(t *testing.T) {
	svc, db := newTestService(t)
	// Enabled but missing host fails validation when loading the client.
	if err := db.Create(&model.SMTPConfig{Enabled: true, Host: "", Port: 2525, FromEmail: "a@b.c"}).Error; err != nil {
		t.Fatalf("seed smtp config: %v", err)
	}
	ctx := context.Background()

	err := svc.SendVerificationEmail(ctx, "x@example.com", &VerificationEmailTemplateData{})
	if err == nil || !strings.Contains(err.Error(), "load SMTP configuration failed") {
		t.Errorf("expected load-config error, got %v", err)
	}
}

func TestSend_UnreachableHost(t *testing.T) {
	svc, db := newTestService(t)
	// Port 1 on loopback is closed: dial fails immediately, offline-safe.
	if err := db.Create(&model.SMTPConfig{Enabled: true, Host: "127.0.0.1", Port: 1, FromEmail: "a@b.c"}).Error; err != nil {
		t.Fatalf("seed smtp config: %v", err)
	}

	err := svc.SendVerificationEmail(context.Background(), "x@example.com",
		&VerificationEmailTemplateData{Name: "a", Link: testLink})
	if err == nil || !strings.Contains(err.Error(), "send verification email failed") {
		t.Errorf("expected send failure, got %v", err)
	}
}

func TestLoadConfig_Validation(t *testing.T) {
	c := &SMTPClient{}
	cases := []struct {
		name string
		cfg  model.SMTPConfig
		want string
	}{
		{"missing host", model.SMTPConfig{Port: 25, FromEmail: "a@b.c"}, "SMTP host is required"},
		{"bad port", model.SMTPConfig{Host: "h", Port: 0, FromEmail: "a@b.c"}, "SMTP port must be greater than zero"},
		{"missing sender", model.SMTPConfig{Host: "h", Port: 25}, "sender address is required"},
	}
	for _, tc := range cases {
		err := c.LoadConfig(&tc.cfg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: expected %q error, got %v", tc.name, tc.want, err)
		}
	}
}

func TestClient_Send_MessageValidation(t *testing.T) {
	c := &SMTPClient{}
	if err := c.LoadConfig(&model.SMTPConfig{Host: "127.0.0.1", Port: 2525, FromEmail: "a@b.c"}); err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	ctx := context.Background()
	full := Message{To: []string{"x@example.com"}, Subject: "s", TextBody: "b"}

	noRecipient := full
	noRecipient.To = nil
	if err := c.Send(ctx, noRecipient); err == nil || !strings.Contains(err.Error(), "at least one recipient") {
		t.Errorf("expected recipient error, got %v", err)
	}

	noSubject := full
	noSubject.Subject = ""
	if err := c.Send(ctx, noSubject); err == nil || !strings.Contains(err.Error(), "subject is required") {
		t.Errorf("expected subject error, got %v", err)
	}

	noBody := full
	noBody.TextBody = ""
	noBody.HTMLBody = ""
	if err := c.Send(ctx, noBody); err == nil || !strings.Contains(err.Error(), "body is required") {
		t.Errorf("expected body error, got %v", err)
	}
}
