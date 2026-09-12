package sso

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSSOHTTPClientHasTimeout pins the outbound client to a bounded timeout:
// reverting to http.DefaultClient (which has none) would let a hung identity
// provider pin a login handler open indefinitely.
func TestSSOHTTPClientHasTimeout(t *testing.T) {
	if ssoHTTPClient.Timeout <= 0 {
		t.Fatalf("ssoHTTPClient.Timeout = %v; outbound provider calls must be bounded", ssoHTTPClient.Timeout)
	}
}

// TestAPIGet_SetsAuthHeaderAndReturnsBody covers the happy path and the
// scheme-dependent Authorization header (GitHub uses "token", others "Bearer").
func TestAPIGet_SetsAuthHeaderAndReturnsBody(t *testing.T) {
	cases := []struct {
		scheme string
		want   string
	}{
		{"token", "token secret"},
		{"bearer", "Bearer secret"},
	}
	for _, tc := range cases {
		t.Run(tc.scheme, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != tc.want {
					t.Errorf("Authorization = %q, want %q", got, tc.want)
				}
				_, _ = w.Write([]byte(`{"id":1}`))
			}))
			defer srv.Close()

			body, err := apiGet(context.Background(), srv.URL, "secret", tc.scheme)
			if err != nil {
				t.Fatalf("apiGet: %v", err)
			}
			if string(body) != `{"id":1}` {
				t.Errorf("body = %q, want %q", body, `{"id":1}`)
			}
		})
	}
}

// TestAPIGet_RejectsOversizedResponse guards the memory bound: a provider that
// streams an unbounded body must produce an error, not an unbounded buffer.
func TestAPIGet_RejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxSSOResponseBytes+1))
	}))
	defer srv.Close()

	if _, err := apiGet(context.Background(), srv.URL, "secret", "bearer"); err == nil {
		t.Fatal("expected an oversized provider response to be rejected, got nil error")
	}
}

// TestAPIGet_AcceptsResponseAtCap ensures the cap does not reject a body that is
// exactly the maximum allowed size.
func TestAPIGet_AcceptsResponseAtCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxSSOResponseBytes))
	}))
	defer srv.Close()

	body, err := apiGet(context.Background(), srv.URL, "secret", "bearer")
	if err != nil {
		t.Fatalf("apiGet: %v", err)
	}
	if len(body) != maxSSOResponseBytes {
		t.Errorf("body length = %d, want %d", len(body), maxSSOResponseBytes)
	}
}

// TestAPIGet_HonorsContextCancellation proves the request is bound to the
// caller's context, so an aborted login stops waiting on the provider instead of
// running until the client timeout.
func TestAPIGet_HonorsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := apiGet(ctx, srv.URL, "secret", "bearer"); err == nil {
		t.Fatal("expected a cancelled context to fail the request, got nil error")
	}
}
