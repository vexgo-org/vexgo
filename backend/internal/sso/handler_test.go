package sso

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestWritePopupResult_EscapesScriptBreakout ensures a payload value containing
// a closing script tag cannot break out of the popup page's script element.
func TestWritePopupResult_EscapesScriptBreakout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	writePopupResult(c, http.StatusOK, map[string]string{
		"error": `</script><script>alert(1)</script>`,
	})

	body := w.Body.String()
	if strings.Contains(body, "</script><script>") {
		t.Fatalf("script breakout survived escaping: %s", body)
	}
	if !strings.Contains(body, `\u003c/script\u003e`) {
		t.Errorf("expected escaped angle brackets in payload: %s", body)
	}
	if !strings.Contains(body, ssoStorageKey) {
		t.Errorf("storage key missing from popup page: %s", body)
	}
}
