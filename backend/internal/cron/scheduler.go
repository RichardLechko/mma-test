package cron

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "mma-scheduler/internal/services"

    "github.com/robfig/cron/v3"
)

type JobStatus struct {
    LastRun      time.Time
    LastError    error
    IsRunning    bool
    FailCount    int
    SuccessCount int
}

type Scheduler struct {
    cron     *cron.Cron
    config   *Config
    jobs     map[string]cron.EntryID
    status   map[string]*JobStatus
    jobLocks map[string]*sync.Mutex
    mu       sync.RWMutex
    logger   *log.Logger

    scraperService services.ScraperServiceInterface
    eventService   services.EventServiceInterface
}

func NewScheduler(config *Config, logger *log.Logger, scraperService services.ScraperServiceInterface, eventService services.EventServiceInterface) (*Scheduler, error) {
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    s := &Scheduler{
        cron:          cron.New(cron.WithLocation(time.UTC), cron.WithLogger(cron.VerbosePrintfLogger(logger))),
        config:        config,
        jobs:         make(map[string]cron.EntryID),
        status:       make(map[string]*JobStatus),
        jobLocks:     make(map[string]*sync.Mutex),
        logger:       logger,
        scraperService: scraperService,
        eventService:   eventService,
    }

    s.jobLocks["scraping"] = &sync.Mutex{}
    s.status["scraping"] = &JobStatus{}

    return s, nil
}

func (s *Scheduler) Start() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if err := s.addEnabledJobs(); err != nil {
        return fmt.Errorf("failed to add jobs: %w", err)
    }

    s.cron.Start()
    s.logger.Println("Scheduler started successfully")
    return nil
}

func (s *Scheduler) Stop() {
    ctx := s.cron.Stop()
    <-ctx.Done()
    s.logger.Println("Scheduler stopped successfully")
}

func (s *Scheduler) addEnabledJobs() error {
    if s.config.IsEnabled("scraping") {
        if err := s.addJob("scraping", s.wrapJob("scraping", s.runScraperJob)); err != nil {
            return err
        }
    }
    return nil
}

func (s *Scheduler) runScraperJob(ctx context.Context) error {
    events, err := s.scraperService.ScrapeUpcomingEvents(ctx)
    if err != nil {
        return fmt.Errorf("failed to scrape events: %w", err)
    }

    for _, event := range events {
        if err := s.eventService.CreateEvent(ctx, &event); err != nil {
            s.logger.Printf("Failed to save event %s: %v", event.Name, err)
        }
    }

    return nil
}

func (s *Scheduler) wrapJob(name string, job func(context.Context) error) func() {
    return func() {
        lock := s.jobLocks[name]
        status := s.status[name]

        if !lock.TryLock() {
            s.logger.Printf("Job %s is already running, skipping this execution", name)
            return
        }
        defer lock.Unlock()

        status.IsRunning = true
        status.LastRun = time.Now()
        defer func() { status.IsRunning = false }()

        ctx, cancel := context.WithTimeout(context.Background(), s.config.GetTimeout(name))
        defer cancel()

        var err error
        for attempt := 1; attempt <= s.config.GetRetryAttempts(name); attempt++ {
            err = job(ctx)
            if err == nil {
                status.SuccessCount++
                s.logger.Printf("Job %s completed successfully", name)
                return
            }

            status.FailCount++
            s.logger.Printf("Job %s failed (attempt %d/%d): %v", name, attempt, s.config.GetRetryAttempts(name), err)

            if attempt < s.config.GetRetryAttempts(name) {
                time.Sleep(time.Second * time.Duration(attempt*2))
            }
        }

        status.LastError = err
    }
}

func (s *Scheduler) addJob(name string, job func()) error {
    schedule := s.config.GetSchedule(name)
    if schedule == "" {
        return fmt.Errorf("invalid schedule for job %s", name)
    }

    id, err := s.cron.AddFunc(schedule, job)
    if err != nil {
        return fmt.Errorf("failed to add job %s: %w", name, err)
    }

    s.jobs[name] = id
    return nil
}

func (s *Scheduler) GetJobStatus(name string) (*JobStatus, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    status, exists := s.status[name]
    if !exists {
        return nil, fmt.Errorf("job %s not found", name)
    }

    return status, nil
}

func (s *Scheduler) RunJobManually(name string) error {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if name != "scraping" {
        return fmt.Errorf("job %s is not supported", name)
    }

    if !s.config.IsEnabled(name) {
        return fmt.Errorf("job %s is not enabled", name)
    }

    go s.wrapJob(name, s.runScraperJob)()
    return nil
}