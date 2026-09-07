package sso

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vexgo-org/vexgo/backend/internal/api"
)

// Handler exposes the sso domain over HTTP.
type Handler struct {
	svc *Service
}

// NewHandler creates an sso HTTP handler with the given dependencies.
func NewHandler(deps Deps) *Handler {
	return &Handler{svc: NewService(deps)}
}

// SSOProviders godoc
//
//	@Summary		List enabled SSO providers
//	@Description	Returns the slugs of every enabled SSO provider
//	@Description	(e.g. "github", "google") and whether the
//	@Description	local email/password login is allowed. The login
//	@Description	page uses this to decide which buttons to render.
//	@Description	Public endpoint — no authentication required.
//	@Tags			sso
//	@Produce		json
//	@Success		200	{object}	SSOProvidersResponse
//	@Router			/sso/providers [get]
func (h *Handler) SSOProviders(c *gin.Context) {
	enabled, allowLocalLogin := h.svc.Providers()
	c.JSON(http.StatusOK, SSOProvidersResponse{
		Providers:       enabled,
		AllowLocalLogin: allowLocalLogin,
	})
}

// SSOLoginRedirect godoc
//
//	@Summary		Start an OAuth2 SSO flow
//	@Description	Redirects the browser to the OAuth2 provider's
//	@Description	authorization endpoint. The `method` query
//	@Description	parameter controls the post-callback behaviour:
//	@Description	` sso_get_token` (default) issues a JWT for full
//	@Description	login; `get_sso_id` only returns the
//	@Description	provider-side user id (used to bind SSO to an
//	@Description	existing account).
//	@Tags			sso
//	@Param			provider	path	string	true	"provider slug (github, google, ...)"
//	@Param			method		query	string	false	"flow variant"	Enums(sso_get_token, get_sso_id)
//	@Success		302			"redirect to the provider's authorization URL"
//	@Failure		400			{object}	api.ErrorResponse	"unknown provider or method"
//	@Failure		500			{object}	api.ErrorResponse
//	@Router			/sso/{provider}/login [get]
func (h *Handler) SSOLoginRedirect(c *gin.Context) {
	provider := c.Param("provider")
	method := c.DefaultQuery("method", "sso_get_token")

	authURL, status, message := h.svc.LoginRedirect(c, provider, method)
	if message != "" {
		c.JSON(status, api.ErrorResponse{Error: message})
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// SSOCallback godoc
//
//	@Summary		OAuth2 callback endpoint
//	@Description	Handles the redirect-back from the provider. On
//	@Description	success it returns an HTML page that writes the
//	@Description	result to localStorage and closes the popup; on
//	@Description	failure the same shape is used with an "error"
//	@Description	field. The frontend listens for the storage event
//	@Description	under the key `sso_callback_result` to pick up
//	@Description	the data.
//	@Tags			sso
//	@Produce		html
//	@Param			provider	path	string	true	"provider slug"
//	@Param			state		query	string	true	"state nonce from the original /login redirect"
//	@Param			code		query	string	true	"authorization code from the provider"
//	@Success		200			"HTML — success popup closer"
//	@Failure		400			"HTML — error popup closer"
//	@Router			/sso/{provider}/callback [get]
func (h *Handler) SSOCallback(c *gin.Context) {
	provider := c.Param("provider")
	payload, message := h.svc.Callback(c, provider, c.Query("state"), c.Query("code"))
	if message != "" {
		respondError(c, message)
		return
	}
	respondPostMessage(c, payload)
}

// SSO_STORAGE_KEY must match the constant in the frontend ssoLogin() helper.
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
