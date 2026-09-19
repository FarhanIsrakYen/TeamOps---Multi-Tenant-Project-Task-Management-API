package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var (
	ErrQueueFull = errors.New("job queue is full")
	ErrClosed    = errors.New("job pool is closed")
)

type Job interface {
	Name() string
	Run(context.Context) error
}

type JobFunc struct {
	JobName string
	Handler func(context.Context) error
}

func (j JobFunc) Name() string { return j.JobName }
func (j JobFunc) Run(ctx context.Context) error {
	if j.Handler == nil {
		return Permanent(errors.New("job handler is nil"))
	}
	return j.Handler(ctx)
}

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent marks an error as non-retryable, for example invalid job input.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

type Config struct {
	Workers        int
	QueueSize      int
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	JobTimeout     time.Duration
}

type Pool struct {
	config Config
	log    zerolog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan Job
	done   chan struct{}

	workers sync.WaitGroup
	mu      sync.RWMutex
	closed  bool
	depth   atomic.Int64
}

var (
	jobsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "teamops_jobs_processed_total", Help: "Jobs completed successfully."}, []string{"job"})
	jobsFailed    = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "teamops_jobs_failed_total", Help: "Jobs that exhausted retries or failed permanently."}, []string{"job"})
	jobsRetried   = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "teamops_jobs_retried_total", Help: "Job retry attempts."}, []string{"job"})
	jobQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{Name: "teamops_jobs_queue_depth", Help: "Number of jobs waiting in the in-process queue."})
	jobDuration   = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "teamops_job_processing_duration_seconds", Help: "End-to-end execution time for a dequeued job, including retries.", Buckets: prometheus.DefBuckets}, []string{"job"})
	metricsOnce   sync.Once
)

func NewPool(config Config, log zerolog.Logger) *Pool {
	config = normalized(config)
	metricsOnce.Do(func() {
		prometheus.MustRegister(jobsProcessed, jobsFailed, jobsRetried, jobQueueDepth, jobDuration)
	})
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{config: config, log: log, ctx: ctx, cancel: cancel, queue: make(chan Job, config.QueueSize), done: make(chan struct{})}
	p.workers.Add(config.Workers)
	for workerID := 1; workerID <= config.Workers; workerID++ {
		go p.runWorker(workerID)
	}
	go func() {
		p.workers.Wait()
		// A deadline-forced shutdown may leave accepted jobs in the closed
		// channel. Account for them as failed before releasing their payloads.
		for job := range p.queue {
			jobsFailed.WithLabelValues(job.Name()).Inc()
		}
		p.depth.Store(0)
		jobQueueDepth.Set(0)
		close(p.done)
	}()
	return p
}

func normalized(config Config) Config {
	if config.Workers < 1 {
		config.Workers = 1
	}
	if config.QueueSize < 1 {
		config.QueueSize = 1
	}
	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	if config.InitialBackoff <= 0 {
		config.InitialBackoff = 100 * time.Millisecond
	}
	if config.MaxBackoff < config.InitialBackoff {
		config.MaxBackoff = config.InitialBackoff
	}
	if config.JobTimeout <= 0 {
		config.JobTimeout = 10 * time.Second
	}
	return config
}

// Submit is deliberately non-blocking. Request handlers cannot accumulate
// waiting goroutines when downstream work is slower than incoming traffic.
func (p *Pool) Submit(job Job) error {
	if job == nil || job.Name() == "" {
		return Permanent(errors.New("job and job name are required"))
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.queue <- job:
		depth := p.depth.Add(1)
		jobQueueDepth.Set(float64(depth))
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown stops accepting work and drains accepted jobs. If the deadline is
// exceeded, active jobs receive cancellation through their contexts.
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()

	select {
	case <-p.done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}

func (p *Pool) QueueDepth() int { return int(p.depth.Load()) }

func (p *Pool) runWorker(workerID int) {
	defer p.workers.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.queue:
			if !ok {
				return
			}
			depth := p.depth.Add(-1)
			jobQueueDepth.Set(float64(depth))
			p.execute(workerID, job)
		}
	}
}

func (p *Pool) execute(workerID int, job Job) {
	started := time.Now()
	defer func() {
		jobDuration.WithLabelValues(job.Name()).Observe(time.Since(started).Seconds())
	}()

	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(p.ctx, p.config.JobTimeout)
		err := runSafely(ctx, job)
		cancel()
		if err == nil {
			jobsProcessed.WithLabelValues(job.Name()).Inc()
			return
		}
		if p.ctx.Err() != nil {
			jobsFailed.WithLabelValues(job.Name()).Inc()
			return
		}
		var permanent permanentError
		if errors.As(err, &permanent) || attempt >= p.config.MaxRetries {
			jobsFailed.WithLabelValues(job.Name()).Inc()
			p.log.Error().Err(err).Str("job", job.Name()).Int("worker", workerID).Int("attempts", attempt+1).Msg("background job failed")
			return
		}

		jobsRetried.WithLabelValues(job.Name()).Inc()
		delay := p.backoff(attempt)
		p.log.Warn().Err(err).Str("job", job.Name()).Int("worker", workerID).Int("retry", attempt+1).Dur("backoff", delay).Msg("background job retry scheduled")
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-p.ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			jobsFailed.WithLabelValues(job.Name()).Inc()
			return
		}
	}
}

func runSafely(ctx context.Context, job Job) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = Permanent(fmt.Errorf("job panicked: %v", recovered))
		}
	}()
	return job.Run(ctx)
}

func (p *Pool) backoff(retry int) time.Duration {
	delay := p.config.InitialBackoff
	for range retry {
		if delay >= p.config.MaxBackoff/2 {
			return p.config.MaxBackoff
		}
		delay *= 2
	}
	if delay > p.config.MaxBackoff {
		return p.config.MaxBackoff
	}
	return delay
}

func (p *Pool) String() string {
	return fmt.Sprintf("workers=%d queue=%d", p.config.Workers, p.config.QueueSize)
}
