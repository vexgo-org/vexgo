package sso

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeStateStore records state values and can be forced to fail.
type fakeStateStore struct {
	values map[string]string
	err    error
}

func (f *fakeStateStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	if f.err != nil {
		return f.err
	}
	if f.values == nil {
		f.values = make(map[string]string)
	}
	f.values[key] = value
	return nil
}

func (f *fakeStateStore) GetDel(_ context.Context, key string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	value, ok := f.values[key]
	delete(f.values, key)
	return value, ok, nil
}

func TestGenerateAndVerifyState_RoundTrip(t *testing.T) {
	ctx := context.Background()
	svc := NewService(Deps{StateStore: &fakeStateStore{}})

	state, err := svc.generateState(ctx, "github", "10.0.0.1", "sso_get_token")
	if err != nil {
		t.Fatalf("generateState: %v", err)
	}
	method, ok := svc.verifyState(ctx, "github", "10.0.0.1", state)
	if !ok || method != "sso_get_token" {
		t.Fatalf("verifyState = %q, %v; want sso_get_token, true", method, ok)
	}
}

func TestVerifyState_OneTimeUse(t *testing.T) {
	ctx := context.Background()
	svc := NewService(Deps{StateStore: &fakeStateStore{}})

	state, err := svc.generateState(ctx, "github", "10.0.0.1", "sso_get_token")
	if err != nil {
		t.Fatalf("generateState: %v", err)
	}
	if _, ok := svc.verifyState(ctx, "github", "10.0.0.1", state); !ok {
		t.Fatal("first verifyState = false; want true")
	}
	// The state is consumed by the first verification.
	if _, ok := svc.verifyState(ctx, "github", "10.0.0.1", state); ok {
		t.Fatal("second verifyState = true; want false (one-time use)")
	}
}

func TestVerifyState_IPMismatch(t *testing.T) {
	ctx := context.Background()
	svc := NewService(Deps{StateStore: &fakeStateStore{}})

	state, err := svc.generateState(ctx, "github", "10.0.0.1", "sso_get_token")
	if err != nil {
		t.Fatalf("generateState: %v", err)
	}
	if _, ok := svc.verifyState(ctx, "github", "10.0.0.2", state); ok {
		t.Fatal("verifyState from another IP = true; want false")
	}
}

func TestVerifyState_MissingState(t *testing.T) {
	svc := NewService(Deps{StateStore: &fakeStateStore{}})
	if _, ok := svc.verifyState(context.Background(), "github", "10.0.0.1", "unknown"); ok {
		t.Fatal("verifyState for unknown state = true; want false")
	}
}

// TestVerifyState_StoreErrorFailsClosed checks that a store outage rejects
// the callback instead of letting the request through.
func TestVerifyState_StoreErrorFailsClosed(t *testing.T) {
	ctx := context.Background()
	svc := NewService(Deps{StateStore: &fakeStateStore{err: context.DeadlineExceeded}})

	state, err := svc.generateState(ctx, "github", "10.0.0.1", "sso_get_token")
	if err == nil {
		t.Skip("generateState unexpectedly succeeded on an erroring store")
	}
	if _, ok := svc.verifyState(ctx, "github", "10.0.0.1", state); ok {
		t.Fatal("verifyState on erroring store = true; want false (fail closed)")
	}
}

func TestGenerateState_StoreError(t *testing.T) {
	svc := NewService(Deps{StateStore: &fakeStateStore{err: errors.New("boom")}})
	if _, err := svc.generateState(context.Background(), "github", "10.0.0.1", "sso_get_token"); err == nil {
		t.Fatal("generateState on erroring store = nil error; want failure")
	}
}

func TestDefaultStateStoreIsInProcess(t *testing.T) {
	svc := NewService(Deps{})
	if _, ok := svc.states.(*memoryStateStore); !ok {
		t.Fatalf("default states = %T; want *memoryStateStore", svc.states)
	}
}
