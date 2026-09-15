package home

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// capturingHandler records every record routed through the default logger so
// tests can assert that failures are actually logged.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

// attr returns the value of the named attribute on the record.
func attr(t *testing.T, r slog.Record, name string) slog.Value {
	t.Helper()
	var found slog.Value
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == name {
			found = a.Value
			return false
		}
		return true
	})
	return found
}

// captureLogs replaces the default logger with a capturing one and restores
// it on cleanup.
func captureLogs(t *testing.T) *capturingHandler {
	t.Helper()
	handler := &capturingHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return handler
}

// find returns the first record with the given message.
func (h *capturingHandler) find(t *testing.T, msg string) (slog.Record, bool) {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message == msg {
			return r, true
		}
	}
	return slog.Record{}, false
}

// statLogged reports whether a failed-count record was emitted for the named
// counter.
func statLogged(t *testing.T, h *capturingHandler, stat string) bool {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message == "failed to count site stat" && attr(t, r, "stat").String() == stat {
			return true
		}
	}
	return false
}

// A failing counter must not blank the dashboard, but it must not be reported
// as a plain zero either: a permanently zeroed stat would otherwise be
// indistinguishable from a site with no content.
func TestStats_CounterFailuresAreLogged(t *testing.T) {
	logs := captureLogs(t)
	repo := &fakeRepo{countsErr: errors.New("db down")}
	svc := &Service{repo: repo}

	stats := svc.Stats(context.Background(), "user")
	if stats != (Stats{}) {
		t.Errorf("expected every counter to fall back to zero, got %+v", stats)
	}

	for _, stat := range []string{"posts", "users", "categories", "tags", "comments"} {
		if !statLogged(t, logs, stat) {
			t.Errorf("expected the failed %s count to be logged", stat)
		}
	}
}

func TestCachedRepository_SettingsDecodeFailureIsLogged(t *testing.T) {
	ctx := context.Background()
	logs := captureLogs(t)
	repo := &fakeRepo{settings: model.GeneralSettings{AllowGuestViewPosts: true}}
	cache := &fakeCache{values: map[string]string{"home:settings": "{not json"}}
	r := NewCachedRepository(repo, cache)

	config, err := r.GetGeneralSettings(ctx)
	if err != nil || !config.AllowGuestViewPosts {
		t.Fatalf("GetGeneralSettings = %+v, %v; want settings from the database", config, err)
	}

	record, ok := logs.find(t, "home cache decode failed")
	if !ok {
		t.Fatal("expected the decode failure to be logged")
	}
	if _, ok := attr(t, record, "err").Any().(error); !ok {
		t.Fatal("decode failure logged without the underlying error")
	}
}

func TestCachedRepository_CounterFailureIsLogged(t *testing.T) {
	ctx := context.Background()
	logs := captureLogs(t)
	repo := &fakeRepo{countsErr: errors.New("db down")}
	r := NewCachedRepository(repo, &fakeCache{})

	if n, err := r.CountPosts(ctx); err != nil || n != 0 {
		t.Fatalf("CountPosts = %d, %v; want 0, nil", n, err)
	}

	record, ok := logs.find(t, "home counter failed")
	if !ok {
		t.Fatal("expected the counter failure to be logged")
	}
	if _, ok := attr(t, record, "err").Any().(error); !ok {
		t.Fatal("counter failure logged without the underlying error")
	}
}
