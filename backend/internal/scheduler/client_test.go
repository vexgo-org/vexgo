package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestScheduler_RegisterAndRun(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var ran atomic.Bool
	_, err := s.Register(ScheduledJob{
		Cron: "@every 1s",
		Job: func() {
			ran.Store(true)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wait for job to run
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for !ran.Load() {
		select {
		case <-ctx.Done():
			t.Fatal("job did not run in time")
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestScheduler_RegisterInvalidCron(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_, err := s.Register(ScheduledJob{
		Cron: "not-a-cron",
		Job:  func() {},
	})
	if err == nil {
		t.Fatal("expected error for invalid cron")
	}
}

func TestScheduler_StopStopsScheduler(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var runCount atomic.Int64
	_, err := s.Register(ScheduledJob{
		Cron: "@every 50ms",
		Job: func() {
			runCount.Add(1)
			time.Sleep(100 * time.Millisecond)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// let a few jobs start
	time.Sleep(200 * time.Millisecond)
	countBeforeStop := runCount.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stopped := s.Stop()
	select {
	case <-stopped.Done():
		// stopped cleanly
	case <-ctx.Done():
		t.Fatal("Stop() timed out")
	}

	// no new jobs should run after Stop()
	time.Sleep(200 * time.Millisecond)
	if countBeforeStop != runCount.Load() {
		t.Fatalf("jobs ran after Stop(): before=%d after=%d", countBeforeStop, runCount.Load())
	}
}

func TestScheduler_RegisterReturnsEntryID(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	id, err := s.Register(ScheduledJob{
		Cron: "@every 1s",
		Job:  func() {},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == cron.EntryID(0) {
		t.Fatal("expected non-zero EntryID")
	}
}

func TestScheduler_MultipleJobs(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var count atomic.Int64
	for range 3 {
		_, err := s.Register(ScheduledJob{
			Cron: "@every 1s",
			Job: func() {
				count.Add(1)
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for count.Load() < 3 {
		select {
		case <-ctx.Done():
			t.Fatal("not all jobs ran")
		case <-time.After(200 * time.Millisecond):
		}
	}
}
