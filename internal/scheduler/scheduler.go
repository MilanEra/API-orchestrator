package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"api-orchestrator/internal/config"
	"api-orchestrator/internal/orchestrator"
	"api-orchestrator/internal/publisher"
)

// Scheduler periodically polls sources and publishes rendered payloads
// to the configured publishers. One goroutine per target.
type Scheduler struct {
	targets    []config.TargetConfig
	orch       *orchestrator.Orchestrator
	publishers map[string]publisher.Publisher
	logger     *slog.Logger
	wg         sync.WaitGroup
}

// New creates a Scheduler for the given targets and publishers.
func New(
	targets []config.TargetConfig,
	orch *orchestrator.Orchestrator,
	publishers map[string]publisher.Publisher,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		targets:    targets,
		orch:       orch,
		publishers: publishers,
		logger:     logger,
	}
}

// Start launches one goroutine per valid target and returns immediately.
// Loops stop when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	for _, t := range s.targets {
		if _, ok := s.publishers[t.Publisher]; !ok {
			s.logger.Warn("unknown publisher, target skipped",
				"target", t.Name, "publisher", t.Publisher)
			continue
		}
		if t.IntervalSeconds <= 0 {
			s.logger.Warn("invalid interval, target skipped",
				"target", t.Name, "interval_seconds", t.IntervalSeconds)
			continue
		}
		s.wg.Add(1)
		go s.loop(ctx, t)
	}
	s.logger.Info("scheduler started", "targets", len(s.targets))
}

// Stop waits for all loops to finish, bounded by the given timeout.
// Call it after the global context has been cancelled.
func (s *Scheduler) Stop(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("scheduler stopped gracefully")
	case <-time.After(timeout):
		s.logger.Warn("scheduler stop timeout, exiting anyway",
			"timeout", timeout.String())
	}
}

// RunOnce finds a target by name and executes its pipeline immediately.
// It is the entry point for the on-demand HTTP endpoint.
func (s *Scheduler) RunOnce(ctx context.Context, targetName string) (string, error) {
	for _, t := range s.targets {
		if t.Name == targetName {
			return s.execute(ctx, t)
		}
	}
	return "", fmt.Errorf("target %q not found", targetName)
}

// loop publishes immediately, then on every tick until ctx is done.
func (s *Scheduler) loop(ctx context.Context, t config.TargetConfig) {
	defer s.wg.Done()

	s.runScheduled(ctx, t)

	ticker := time.NewTicker(time.Duration(t.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runScheduled(ctx, t)
		}
	}
}

// runScheduled executes one scheduled cycle and logs failures.
func (s *Scheduler) runScheduled(ctx context.Context, t config.TargetConfig) {
	if _, err := s.execute(ctx, t); err != nil {
		s.logger.Warn("scheduled publish failed",
			"target", t.Name, "error", err)
	}
}

// execute runs the fetch-format-publish pipeline for one target
// and returns the rendered payload. Shared by the ticker loop
// and the on-demand HTTP endpoint.
func (s *Scheduler) execute(ctx context.Context, t config.TargetConfig) (string, error) {
	pub, ok := s.publishers[t.Publisher]
	if !ok {
		return "", fmt.Errorf("unknown publisher %q for target %q", t.Publisher, t.Name)
	}

	results := s.orch.FetchMany(ctx, []string{t.Source}, nil)
	res, ok := results[t.Source]
	if !ok || res.Error != "" {
		errMsg := "unknown error"
		if res.Error != "" {
			errMsg = res.Error
		}
		return "", fmt.Errorf("fetch failed for source %q: %s", t.Source, errMsg)
	}

	data, ok := res.Data.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected payload shape from source %q", t.Source)
	}

	payload, err := publisher.Render(t.Template, data)
	if err != nil {
		return "", fmt.Errorf("render failed for target %q: %w", t.Name, err)
	}

	if err := pub.Publish(ctx, t, data); err != nil {
		return "", fmt.Errorf("publish failed for target %q: %w", t.Name, err)
	}
	return payload, nil
}
