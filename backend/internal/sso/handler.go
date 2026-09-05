// Package sso handles single-sign-on providers. The provider
// list endpoint returns JSON and is exposed via huma; the
// browser-driven login redirect and HTML callback remain on
// gin since their responses (302 redirects and text/html
// postMessage payloads) are not part of the typed API surface
// the frontend consumes through the generated client.
package sso

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

// Handler exposes the sso domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates an sso HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// ---------- input / output types ----------

type ssoProvidersOutput struct {
	Body ssoProvidersBody
}

// ssoProvidersBody is the body of GET /api/sso/providers. It
// is the public list of enabled providers plus the
// allow-local-login flag, which the frontend uses to decide
// whether to render the local login form.
type ssoProvidersBody struct {
	Providers     []string `json:"providers" doc:"Enabled provider IDs (github, google, ...)"`
	AllowLocalLogin bool   `json:"allow_local_login" doc:"True when the local username/password form is available"`
}

// ---------- huma handler ----------

// GetProviders returns the public list of enabled SSO providers.
func (h *Handler) GetProviders(ctx context.Context, _ *struct{}) (*ssoProvidersOutput, error) {
	enabled, allowLocal := h.svc.Providers()
	return &ssoProvidersOutput{
		Body: ssoProvidersBody{
			Providers:       enabled,
			AllowLocalLogin: allowLocal,
		},
	}, nil
}

// RegisterRoutes registers the sso domain operations on the
// given huma.API plus the gin-registered login redirect and
// HTML callback.
func (h *Handler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-sso-providers",
		Method:      http.MethodGet,
		Path:        "/sso/providers",
		Summary:     "List enabled SSO providers",
		Tags:        []string{"sso"},
	}, h.GetProviders)
}

// RegisterGinRoutes registers the browser-driven gin handlers
// (the login redirect and HTML callback). These are not part
// of the orval surface — the browser handles them via
// top-level navigation and a postMessage popup. They live on
// the gin sub-group because huma would have to wrap them in a
// JSON envelope which the OAuth popup cannot consume.
func (h *Handler) RegisterGinRoutes(api *gin.RouterGroup) {
	ssoGroup := api.Group("/sso")
	ssoGroup.GET("/:provider/login", h.SSOLoginRedirect)
	ssoGroup.GET("/:provider/callback", h.SSOCallback)
}

// ---------- gin handlers (kept verbatim) ----------
//
// The login redirect and HTML callback are browser-driven
// endpoints (302 to the provider, then a small HTML page that
// postMessages the result back to the opener). They are not
// part of the orval surface and stay on gin so the response
// stays byte-identical with the legacy implementation.

// SSOLoginRedirect starts the OAuth2 authorization flow.
//
// GET /api/sso/:provider/login?method=sso_get_token|get_sso_id
//
//   - sso_get_token  → full login, issues a JWT on callback
//   - get_sso_id     → only returns the provider-side ID (used to bind SSO
//     to an existing account from the settings page)
func (h *Handler) SSOLoginRedirect(c *gin.Context) {
	provider := c.Param("provider")
	method := c.DefaultQuery("method", "sso_get_token")

	authURL, status, message := h.svc.LoginRedirect(c, provider, method)
	if message != "" {
		c.JSON(status, gin.H{"error": message})
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// SSOCallback handles the OAuth2 callback for all providers.
// The popup window calls postMessage to pass data back to the opener, then closes.
//
// GET /api/sso/:provider/callback?method=...&code=...&state=...
func (h *Handler) SSOCallback(c *gin.Context) {
	provider := c.Param("provider")
	payload, message := h.svc.Callback(c, provider, c.Query("state"), c.Query("code"))
	if message != "" {
		respondError(c, message)
		return
	}
	respondPostMessage(c, payload)
}

// SSOStorageKey must match the constant in the frontend ssoLogin() helper.
const ssoStorageKey = "sso_callback_result"

// respondPostMessage writes the result to localStorage so the opener window
// can pick it up via the 'storage' event. Using localStorage instead of
// postMessage avoids the window.opener=null issue caused by cross-origin
// redirects during the OAuth2 / OIDC flow.
func respondPostMessage(c *gin.Context, data map[string]string) {
	pairs := make([]string, 0, len(data))
	for k, v := range data {
		pairs = append(pairs, fmt.Sprintf(`%q:%q`, k, v))
	}
	payload := "{" + strings.Join(pairs, ",") + "}"
	html := fmt.Sprintf(`<!DOCTYPE html>
<head></head>
<body>
<script>
try { localStorage.setItem(%q, JSON.stringify(%s)) } catch(e) {}
window.close()
</script>
</body>`, ssoStorageKey, payload)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// respondError writes an error result to localStorage and closes the popup.
func respondError(c *gin.Context, msg string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<head></head>
<body>
<script>
try { localStorage.setItem(%q, JSON.stringify({"error":%q})) } catch(e) {}
window.close()
</script>
</body>`, ssoStorageKey, msg)
	c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(html))
}
