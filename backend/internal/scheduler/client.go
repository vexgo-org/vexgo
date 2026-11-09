package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	c *cron.Cron
}

type ScheduledJob struct {
	Cron string
	Job  func()
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		c: cron.New(cron.WithLocation(time.Local)),
	}
}

func (s *Scheduler) Start() {
	s.c.Start()
}

func (s *Scheduler) Stop() context.Context {
	return s.c.Stop()
}

func (s *Scheduler) Register(scheduledJob ScheduledJob) (cron.EntryID, error) {
	id, err := s.c.AddFunc(scheduledJob.Cron, scheduledJob.Job)
	if err != nil {
		return 0, fmt.Errorf("register schedule: %w", err)
	}
	return id, nil
}
