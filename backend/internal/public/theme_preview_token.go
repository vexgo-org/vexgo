package public

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ThemePreviewParam is the query parameter carrying the signature that
// authorizes a ?theme= preview. It is deliberately distinct from the draft
// preview's ?preview=1 flag, which is a different capability.
const ThemePreviewParam = "theme_token"

// ThemePreviewTTL bounds how long a minted theme-preview link stays valid. The
// link is a bearer capability — the admin console opens it in a new tab, which
// cannot carry the Authorization header — so it expires quickly.
const ThemePreviewTTL = 10 * time.Minute

// ErrNoPreviewSecret means the renderer has no JWT secret, so it can neither
// sign nor verify preview links.
var ErrNoPreviewSecret = errors.New("theme preview is unavailable: no signing secret configured")

// Domain-separation labels. The preview key and MAC are derived from the JWT
// secret through these distinct labels, so the JWT signing key is never used
// directly for a second purpose.
const (
	themePreviewKeyLabel = "vexgo:theme-preview:key:v1"
	themePreviewMACLabel = "vexgo:theme-preview:mac:v1"
)

// themePreviewKey derives the preview signing key from the server secret.
func themePreviewKey(secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(themePreviewKeyLabel))
	return mac.Sum(nil)
}

// themePreviewMAC authenticates one theme id and expiry under the derived key.
func themePreviewMAC(secret []byte, themeID string, expires int64) []byte {
	mac := hmac.New(sha256.New, themePreviewKey(secret))
	mac.Write([]byte(themePreviewMACLabel))
	mac.Write([]byte{0})
	mac.Write([]byte(themeID))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatInt(expires, 10)))
	return mac.Sum(nil)
}

// ThemePreviewToken mints a token authorizing the ?theme= preview switch for
// themeID until now+ThemePreviewTTL. It fails when no secret is configured,
// because a preview that cannot be verified must never be handed out.
func (r *Renderer) ThemePreviewToken(themeID string) (string, error) {
	return r.themePreviewTokenAt(themeID, time.Now())
}

// themePreviewTokenAt mints a token as of now, so expiry is testable.
func (r *Renderer) themePreviewTokenAt(themeID string, now time.Time) (string, error) {
	if len(r.jwtSecret) == 0 {
		return "", ErrNoPreviewSecret
	}
	expires := now.Add(ThemePreviewTTL).Unix()
	sig := base64.RawURLEncoding.EncodeToString(themePreviewMAC(r.jwtSecret, themeID, expires))
	return strconv.FormatInt(expires, 10) + "." + sig, nil
}

// ThemePreviewURL mints a same-origin preview URL:
// /?theme=<id>&theme_token=<token>.
func (r *Renderer) ThemePreviewURL(themeID string) (string, error) {
	token, err := r.ThemePreviewToken(themeID)
	if err != nil {
		return "", err
	}
	query := url.Values{}
	query.Set("theme", themeID)
	query.Set(ThemePreviewParam, token)
	return "/?" + query.Encode(), nil
}

// validThemePreviewToken reports whether token authorizes a preview of themeID.
// The signature comparison is constant time, so a caller cannot learn the
// expected signature byte by byte.
func (r *Renderer) validThemePreviewToken(themeID, token string) bool {
	if len(r.jwtSecret) == 0 || token == "" {
		return false
	}
	expiresPart, signaturePart, found := strings.Cut(token, ".")
	if !found {
		return false
	}
	expires, err := strconv.ParseInt(expiresPart, 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(signaturePart)
	if err != nil {
		return false
	}
	return hmac.Equal(signature, themePreviewMAC(r.jwtSecret, themeID, expires))
}
