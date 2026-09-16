package sso

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCallbackURI_OriginTrust pins how the OAuth redirect_uri is built. A
// configured BASE_URL always wins, and X-Forwarded-Proto (a client-supplied
// header) is honored only when the deployment declared itself behind a reverse
// proxy — otherwise a forged header could steer the authorization code to a
// different origin.
func TestCallbackURI_OriginTrust(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		baseURL        string
		behindProxy    bool
		host           string
		forwardedProto string
		tls            bool
		want           string
	}{
		{
			name:           "base url wins over spoofed host and header",
			baseURL:        "https://blog.example",
			behindProxy:    false,
			host:           "evil.example",
			forwardedProto: "https",
			want:           "https://blog.example/api/sso/github/callback",
		},
		{
			name:           "forwarded proto ignored without a reverse proxy",
			host:           "blog.example",
			forwardedProto: "https",
			want:           "http://blog.example/api/sso/github/callback",
		},
		{
			name:           "forwarded proto honored behind a reverse proxy",
			behindProxy:    true,
			host:           "blog.example",
			forwardedProto: "https",
			want:           "https://blog.example/api/sso/github/callback",
		},
		{
			name: "tls wins over the header",
			host: "blog.example",
			tls:  true,
			want: "https://blog.example/api/sso/github/callback",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(Deps{BaseURL: tc.baseURL, BehindReverseProxy: tc.behindProxy})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/api/sso/github/login", nil)
			req.Host = tc.host
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			c.Request = req

			if got := svc.callbackURI(c, "github"); got != tc.want {
				t.Errorf("callbackURI = %q, want %q", got, tc.want)
			}
		})
	}
}
