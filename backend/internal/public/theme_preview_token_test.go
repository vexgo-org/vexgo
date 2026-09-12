package public

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// TestThemePreviewLink_RendersPreviewWithoutAuth covers the flow the admin
// console uses: mint a signed link, then open it in a tab that carries no
// Authorization header.
func TestThemePreviewLink_RendersPreviewWithoutAuth(t *testing.T) {
	r, renderer, _ := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")
	renderer.SetJWTSecret(previewTestSecret)

	link, err := renderer.ThemePreviewURL("alt")
	if err != nil {
		t.Fatalf("ThemePreviewURL: %v", err)
	}

	w := doPublicRequest(t, r, link)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body: %s", link, w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "ALT HOME") {
		t.Errorf("signed preview link did not render the alt theme:\n%s", body)
	}
	if !strings.Contains(body, `href="/themes/alt/assets/style.css"`) {
		t.Errorf("signed preview link did not scope assets to the previewed theme:\n%s", body)
	}
}

// TestThemePreviewLink_RejectsInvalidTokens pins the signature check: forged,
// expired, cross-theme and missing tokens must all fall back to the active
// theme instead of rendering a non-active one.
func TestThemePreviewLink_RejectsInvalidTokens(t *testing.T) {
	r, renderer, _ := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")
	renderer.SetJWTSecret(previewTestSecret)

	validToken, err := renderer.ThemePreviewToken("alt")
	if err != nil {
		t.Fatalf("ThemePreviewToken: %v", err)
	}
	expired, err := renderer.themePreviewTokenAt("alt", time.Now().Add(-2*ThemePreviewTTL))
	if err != nil {
		t.Fatalf("mint expired token: %v", err)
	}

	cases := []struct {
		name  string
		query string
	}{
		{"tampered signature", "/?theme=alt&" + ThemePreviewParam + "=" + validToken + "x"},
		{"garbage token", "/?theme=alt&" + ThemePreviewParam + "=not-a-token"},
		{"expired token", "/?theme=alt&" + ThemePreviewParam + "=" + expired},
		{"token minted for another theme", "/?theme=other&" + ThemePreviewParam + "=" + validToken},
		{"missing token", "/?theme=alt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doPublicRequest(t, r, tc.query)
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d (body %s)", tc.query, w.Code, w.Body.String())
			}
			body := w.Body.String()
			if strings.Contains(body, "/themes/alt/") {
				t.Errorf("invalid preview token activated the alt theme:\n%s", body)
			}
			if !strings.Contains(body, `href="/theme-assets/style.css"`) {
				t.Errorf("invalid preview token did not fall back to the active theme:\n%s", body)
			}
		})
	}
}

// TestThemePreviewLink_KeepsThemeAcrossNavigation verifies the signature travels
// with the preview's own links, so following one stays on the previewed theme
// instead of falling back to the active theme.
func TestThemePreviewLink_KeepsThemeAcrossNavigation(t *testing.T) {
	r, renderer, _ := newPublicRouter(t)
	installPreviewTheme(t, renderer, "alt")
	renderer.SetJWTSecret(previewTestSecret)

	link, err := renderer.ThemePreviewURL("alt")
	if err != nil {
		t.Fatalf("ThemePreviewURL: %v", err)
	}
	if body := doPublicRequest(t, r, link).Body.String(); !strings.Contains(body, ThemePreviewParam+"=") {
		t.Errorf("preview links dropped the signature:\n%s", body)
	}

	token, err := renderer.ThemePreviewToken("alt")
	if err != nil {
		t.Fatalf("ThemePreviewToken: %v", err)
	}
	query := url.Values{}
	query.Set("theme", "alt")
	query.Set(ThemePreviewParam, token)

	w := doPublicRequest(t, r, "/post/hello-world?"+query.Encode())
	if w.Code != http.StatusOK {
		t.Fatalf("GET previewed post status = %d (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ALT POST") {
		t.Errorf("navigated preview fell back to the active theme:\n%s", w.Body.String())
	}
}

// TestThemePreviewLink_DoesNotUnlockDrafts pins capability separation: a theme
// preview token authorizes the ?theme= switch only, never the ?preview=1 draft
// page view, which requires a live admin token.
func TestThemePreviewLink_DoesNotUnlockDrafts(t *testing.T) {
	r, renderer, db := newPublicRouter(t)
	renderer.SetJWTSecret(previewTestSecret)

	if err := db.AutoMigrate(&model.Page{}); err != nil {
		t.Fatalf("migrate pages: %v", err)
	}
	author := model.User{Username: "pager", Email: "pager@example.com", Role: model.RoleAuthor}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	if err := db.Create(&model.Page{
		Slug: "secret", Title: "Secret", Content: "draft",
		Status: model.PageStatusDraft, AuthorID: author.ID,
	}).Error; err != nil {
		t.Fatalf("seed draft page: %v", err)
	}

	token, err := renderer.ThemePreviewToken(DefaultTheme)
	if err != nil {
		t.Fatalf("ThemePreviewToken: %v", err)
	}

	// The token does open a theme preview...
	if w := doPublicRequest(t, r, "/?theme="+DefaultTheme+"&"+ThemePreviewParam+"="+token); w.Code != http.StatusOK {
		t.Fatalf("theme preview status = %d (body %s)", w.Code, w.Body.String())
	}

	// ...but it must not unlock the draft page, even alongside ?preview=1.
	w := doPublicRequest(t, r, "/secret?preview=1&"+ThemePreviewParam+"="+token)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET /secret with a theme preview token status = %d, want 404", w.Code)
	}
}
