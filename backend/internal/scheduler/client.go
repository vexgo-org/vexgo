package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	c    *cron.Cron
	mu   sync.Mutex
	jobs map[string]cron.EntryID
}

type ScheduledJob struct {
	Cron string
	Job  func()
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		c:    cron.New(cron.WithLocation(time.Local)),
		jobs: make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) Start() {
	s.c.Start()
}

func (s *Scheduler) Stop() context.Context {
	return s.c.Stop()
}

func (s *Scheduler) Register(name string, job *ScheduledJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Empty cron means disabling this job.
	if job.Cron == "" {
		if id, ok := s.jobs[name]; ok {
			s.c.Remove(id)
			delete(s.jobs, name)
		}
		return nil
	}

	id, err := s.c.AddFunc(job.Cron, job.Job)
	if err != nil {
		return fmt.Errorf("register scheduled job %q: %w", name, err)
	}

	if old, ok := s.jobs[name]; ok {
		s.c.Remove(old)
	}

	s.jobs[name] = id

	return nil
}
