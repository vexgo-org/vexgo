package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// waitFor blocks until count reaches want, failing the test after 3 seconds.
func waitFor(t *testing.T, count *atomic.Int64, want int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for count.Load() < want {
		select {
		case <-ctx.Done():
			t.Fatalf("job ran %d time(s), want at least %d", count.Load(), want)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestScheduler_RegisterAndRun(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var runs atomic.Int64
	if err := s.Register("test", &ScheduledJob{
		Cron: "@every 50ms",
		Job:  func() { runs.Add(1) },
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitFor(t, &runs, 1)
}

func TestScheduler_RegisterInvalidCronKeepsExistingJob(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var runs atomic.Int64
	if err := s.Register("test", &ScheduledJob{
		Cron: "@every 50ms",
		Job:  func() { runs.Add(1) },
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitFor(t, &runs, 1)

	if err := s.Register("test", &ScheduledJob{
		Cron: "not-a-cron",
		Job:  func() {},
	}); err == nil {
		t.Fatal("expected error for invalid cron")
	}

	// The rejected replacement must leave the previous job registered.
	if entries := s.c.Entries(); len(entries) != 1 {
		t.Fatalf("entries after rejected replacement = %d, want 1", len(entries))
	}
	before := runs.Load()
	waitFor(t, &runs, before+1)
}

func TestScheduler_RegisterReplacesJobWithSameName(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var first, second atomic.Int64
	register := func(job func()) {
		t.Helper()
		if err := s.Register("test", &ScheduledJob{
			Cron: "@every 50ms",
			Job:  job,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	register(func() { first.Add(1) })
	waitFor(t, &first, 1)

	register(func() { second.Add(1) })
	waitFor(t, &second, 1)

	// Only the job registered last may keep running.
	if entries := s.c.Entries(); len(entries) != 1 {
		t.Fatalf("entries after replacement = %d, want 1", len(entries))
	}
	before := first.Load()
	time.Sleep(200 * time.Millisecond)
	if got := first.Load(); got != before {
		t.Errorf("replaced job still ran: before=%d after=%d", before, got)
	}
}

func TestScheduler_RegisterEmptyCronDisablesJob(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var runs atomic.Int64
	if err := s.Register("test", &ScheduledJob{
		Cron: "@every 50ms",
		Job:  func() { runs.Add(1) },
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	waitFor(t, &runs, 1)

	if err := s.Register("test", &ScheduledJob{Cron: ""}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries := s.c.Entries(); len(entries) != 0 {
		t.Fatalf("entries after disabling = %d, want 0", len(entries))
	}

	before := runs.Load()
	time.Sleep(200 * time.Millisecond)
	if got := runs.Load(); got != before {
		t.Errorf("disabled job ran: before=%d after=%d", before, got)
	}
}

func TestScheduler_RegisterEmptyCronUnknownNameIsNoop(t *testing.T) {
	s := NewScheduler()

	if err := s.Register("missing", &ScheduledJob{Cron: ""}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries := s.c.Entries(); len(entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(entries))
	}
}

func TestScheduler_StopStopsScheduler(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var runs atomic.Int64
	if err := s.Register("test", &ScheduledJob{
		Cron: "@every 50ms",
		Job: func() {
			runs.Add(1)
			time.Sleep(100 * time.Millisecond)
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitFor(t, &runs, 1)
	beforeStop := runs.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stopped := s.Stop()
	select {
	case <-stopped.Done():
		// stopped cleanly
	case <-ctx.Done():
		t.Fatal("Stop() timed out")
	}

	// No new runs may start after Stop().
	time.Sleep(200 * time.Millisecond)
	if got := runs.Load(); got != beforeStop {
		t.Errorf("jobs ran after Stop(): before=%d after=%d", beforeStop, got)
	}
}

func TestScheduler_MultipleJobs(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var runs atomic.Int64
	for _, name := range []string{"one", "two", "three"} {
		if err := s.Register(name, &ScheduledJob{
			Cron: "@every 50ms",
			Job:  func() { runs.Add(1) },
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	waitFor(t, &runs, 3)
}
