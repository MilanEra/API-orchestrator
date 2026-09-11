package retry

import (
	"context"
	"fmt"
	"time"
)

// Config describes retry behavior.
type Config struct {
	MaxAttempts          int
	InitialDelay         time.Duration
	MaxDelay             time.Duration
	RetryableStatusCodes map[int]bool
}

// Do executes the function with retry logic.
func Do(ctx context.Context, cfg Config, fn func() (int, error)) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}

	delay := cfg.InitialDelay
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		statusCode, err := fn()

		if err == nil {
			return nil
		}

		lastErr = err

		if attempt == cfg.MaxAttempts {
			break
		}

		if statusCode > 0 && !cfg.RetryableStatusCodes[statusCode] {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}
