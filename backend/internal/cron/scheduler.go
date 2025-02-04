package cron

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mma-scheduler/internal/cron/jobs"
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

	rankingsJob *jobs.RankingsJob
	scrapingJob *jobs.ScraperJob
	cleanupJob  *jobs.CleanupJob
	archiveJob  *jobs.ArchiveJob
	metricsJob  *jobs.MetricsJob
}

func NewScheduler(config *Config, logger *log.Logger, db *services.Database) (*Scheduler, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	s := &Scheduler{
		cron:     cron.New(cron.WithLocation(time.UTC), cron.WithLogger(cron.VerbosePrintfLogger(logger))),
		config:   config,
		jobs:     make(map[string]cron.EntryID),
		status:   make(map[string]*JobStatus),
		jobLocks: make(map[string]*sync.Mutex),
		logger:   logger,
	}

	sqlDB := db.GetDB()

	fighterService := services.NewFighterService(sqlDB)
	eventService := services.NewEventService(sqlDB)
	fightService := services.NewFightService(sqlDB)

	fighterConcrete := fighterService.(*services.FighterService)
	fightConcrete := fightService.(*services.FightService)

	promotionService := services.NewPromotionService(sqlDB)
	mediaService := services.NewMediaService(sqlDB)
	rankingService := services.NewRankingService(sqlDB)

	processorService := services.NewProcessorService(
		fighterConcrete,
		eventService,
	)

	s.rankingsJob = jobs.NewRankingsJob(
		logger,
		fighterService,
		fightService,
		rankingService,
		promotionService,
	)

	s.scrapingJob = jobs.NewScraperJob(
		logger,
		services.NewScraperService(processorService),
		fighterService,
		eventService,
		fightService,
	)

	s.cleanupJob = jobs.NewCleanupJob(
		logger,
		fighterService,
		fightService,
		eventService,
		mediaService,
	)

	s.archiveJob = jobs.NewArchiveJob(
		logger,
		sqlDB,
		fightConcrete,
		eventService,
		mediaService,
	)

	s.metricsJob = jobs.NewMetricsJob(
		logger,
		fighterConcrete,
		fightConcrete,
		eventService,
		db,
	)

	jobNames := []string{"rankings", "scraping", "cleanup", "archive", "metrics"}
	for _, name := range jobNames {
		s.jobLocks[name] = &sync.Mutex{}
		s.status[name] = &JobStatus{}
	}

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
	if s.config.IsEnabled("rankings") {
		if err := s.addJob("rankings", s.wrapJob("rankings", s.rankingsJob.UpdateRankings)); err != nil {
			return err
		}
	}

	if s.config.IsEnabled("scraping") {
		if err := s.addJob("scraping", s.wrapJob("scraping", s.scrapingJob.RunScraper)); err != nil {
			return err
		}
	}

	if s.config.IsEnabled("cleanup") {
		if err := s.addJob("cleanup", s.wrapJob("cleanup", s.cleanupJob.PerformCleanup)); err != nil {
			return err
		}
	}

	if s.config.IsEnabled("archive") {
		if err := s.addJob("archive", s.wrapJob("archive", s.archiveJob.ArchiveOldFights)); err != nil {
			return err
		}
	}

	if s.config.IsEnabled("metrics") {
		if err := s.addJob("metrics", s.wrapJob("metrics", s.metricsJob.UpdateMetrics)); err != nil {
			return err
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

	if !s.config.IsEnabled(name) {
		return fmt.Errorf("job %s is not enabled", name)
	}

	go func() {
		switch name {
		case "rankings":
			s.wrapJob(name, s.rankingsJob.UpdateRankings)()
		case "scraping":
			s.wrapJob(name, s.scrapingJob.RunScraper)()
		case "cleanup":
			s.wrapJob(name, s.cleanupJob.PerformCleanup)()
		case "archive":
			s.wrapJob(name, s.archiveJob.ArchiveOldFights)()
		case "metrics":
			s.wrapJob(name, s.metricsJob.UpdateMetrics)()
		default:
			s.logger.Printf("Unknown job: %s", name)
		}
	}()

	return nil
}
