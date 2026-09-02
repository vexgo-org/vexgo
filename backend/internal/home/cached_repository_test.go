package home

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// fakeRepo is a Repository fake recording calls to the cached methods.
type fakeRepo struct {
	Repository // nil; only the overridden methods may be called

	countPostsCalls int
	settingsCalls   int
	countsErr       error
	settings        model.GeneralSettings
	settingsErr     error
}

func (f *fakeRepo) CountPosts(_ context.Context) (int64, error) {
	f.countPostsCalls++
	if f.countsErr != nil {
		return 0, f.countsErr
	}
	return 42, nil
}

func (f *fakeRepo) GetGeneralSettings(_ context.Context) (model.GeneralSettings, error) {
	f.settingsCalls++
	if f.settingsErr != nil {
		return model.GeneralSettings{}, f.settingsErr
	}
	return f.settings, nil
}

// fakeCache is an in-memory ReadCache fake with error injection.
type fakeCache struct {
	values map[string]string
	err    error
}

func (c *fakeCache) Get(_ context.Context, key string) (string, bool, error) {
	if c.err != nil {
		return "", false, c.err
	}
	value, ok := c.values[key]
	return value, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[key] = value
	return nil
}

func TestCachedRepository_CountsAreCached(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	r := NewCachedRepository(repo, &fakeCache{})

	for range 2 {
		n, err := r.CountPosts(ctx)
		if err != nil || n != 42 {
			t.Fatalf("CountPosts = %d, %v; want 42, nil", n, err)
		}
	}
	if repo.countPostsCalls != 1 {
		t.Fatalf("expected 1 underlying CountPosts call, got %d", repo.countPostsCalls)
	}
}

func TestCachedRepository_CountErrorIsNotCached(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{countsErr: errors.New("db down")}
	r := NewCachedRepository(repo, &fakeCache{})

	// Counter failures read as zero and are never cached, so recovery is
	// immediate.
	for range 2 {
		n, err := r.CountPosts(ctx)
		if err != nil || n != 0 {
			t.Fatalf("CountPosts = %d, %v; want 0, nil", n, err)
		}
	}
	if repo.countPostsCalls != 2 {
		t.Fatalf("expected 2 underlying calls, got %d", repo.countPostsCalls)
	}

	// After recovery the counter is served and cached.
	repo.countsErr = nil
	if n, _ := r.CountPosts(ctx); n != 42 {
		t.Fatalf("CountPosts after recovery = %d; want 42", n)
	}
	if repo.countPostsCalls != 3 {
		t.Fatalf("expected 3 underlying calls, got %d", repo.countPostsCalls)
	}
}

func TestCachedRepository_SettingsAreCached(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{settings: model.GeneralSettings{AllowGuestViewPosts: true}}
	r := NewCachedRepository(repo, &fakeCache{})

	for range 2 {
		config, err := r.GetGeneralSettings(ctx)
		if err != nil || !config.AllowGuestViewPosts {
			t.Fatalf("GetGeneralSettings = %+v, %v", config, err)
		}
	}
	if repo.settingsCalls != 1 {
		t.Fatalf("expected 1 underlying GetGeneralSettings call, got %d", repo.settingsCalls)
	}
}

func TestCachedRepository_CacheErrorFallsThroughToDB(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{settings: model.GeneralSettings{AllowGuestViewPosts: true}}
	r := NewCachedRepository(repo, &fakeCache{err: context.DeadlineExceeded})

	for range 2 {
		config, err := r.GetGeneralSettings(ctx)
		if err != nil || !config.AllowGuestViewPosts {
			t.Fatalf("GetGeneralSettings = %+v, %v", config, err)
		}
	}
	if repo.settingsCalls != 2 {
		t.Fatalf("expected every read to hit the database on cache errors, got %d", repo.settingsCalls)
	}
}
