package ratelimit

import (
	"context"
	"time"
)

// Limiter is a token bucket rate limiter.
// The bucket holds up to `rate` tokens; a token is added every (1s / rate).
// Each request consumes one token. If the bucket is empty,
// Wait blocks on a channel (no busy-waiting) until a token arrives
// or the context is cancelled.
type Limiter struct {
	tokens chan struct{}
	stop   chan struct{}
}

// New creates a token bucket limiter for the given requests per second.
func New(requestsPerSecond int) *Limiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 10
	}

	l := &Limiter{
		tokens: make(chan struct{}, requestsPerSecond),
		stop:   make(chan struct{}),
	}

	// Start with a full bucket.
	for i := 0; i < requestsPerSecond; i++ {
		l.tokens <- struct{}{}
	}

	// Refill goroutine: adds one token per tick, drops it if the bucket is full.
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(requestsPerSecond))
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				select {
				case l.tokens <- struct{}{}:
				default: // bucket full, drop the token
				}
			case <-l.stop:
				return
			}
		}
	}()

	return l
}

// Wait blocks until a token is available or the context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
		return nil
	}
}

// Stop terminates the refill goroutine.
func (l *Limiter) Stop() {
	close(l.stop)
}
