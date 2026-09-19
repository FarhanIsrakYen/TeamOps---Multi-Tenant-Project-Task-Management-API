package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Submitter interface {
	Submit(Job) error
}

type Schedule struct {
	Interval time.Duration
	NewJob   func() Job
}

// Scheduler uses one owned goroutine regardless of the number of schedules.
// It submits into the bounded pool and never starts work itself.
type Scheduler struct {
	submitter Submitter
	schedules []Schedule
	log       zerolog.Logger
	cancel    context.CancelFunc
	done      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewScheduler(submitter Submitter, log zerolog.Logger, schedules ...Schedule) *Scheduler {
	return &Scheduler{submitter: submitter, schedules: schedules, log: log, done: make(chan struct{})}
}

func (s *Scheduler) Start(parent context.Context) {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		s.cancel = cancel
		go s.run(ctx)
	})
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		notStarted := false
		s.startOnce.Do(func() {
			notStarted = true
			close(s.done)
		})
		if notStarted {
			return
		}
		s.cancel()
		<-s.done
	})
}

func (s *Scheduler) run(ctx context.Context) {
	defer close(s.done)
	type activeSchedule struct {
		Schedule
		next time.Time
	}
	active := make([]activeSchedule, 0, len(s.schedules))
	now := time.Now()
	for _, schedule := range s.schedules {
		if schedule.Interval > 0 && schedule.NewJob != nil {
			active = append(active, activeSchedule{Schedule: schedule, next: now.Add(schedule.Interval)})
		}
	}
	if len(active) == 0 {
		<-ctx.Done()
		return
	}

	for {
		next := active[0].next
		for i := 1; i < len(active); i++ {
			if active[i].next.Before(next) {
				next = active[i].next
			}
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case now = <-timer.C:
			for i := range active {
				if active[i].next.After(now) {
					continue
				}
				s.submit(active[i].NewJob())
				active[i].next = now.Add(active[i].Interval)
			}
		}
	}
}

func (s *Scheduler) submit(job Job) {
	if err := s.submitter.Submit(job); err != nil {
		level := s.log.Error()
		if errors.Is(err, ErrQueueFull) || errors.Is(err, ErrClosed) {
			level = s.log.Warn()
		}
		level.Err(err).Str("job", job.Name()).Msg("scheduled job was not queued")
	}
}
