package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State represents the state of the circuit breaker.
type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit is open and a request is rejected.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            State
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	recoveryTimeout  time.Duration
	lastFailureTime  time.Time
}

// New creates a circuit breaker with the given thresholds and recovery timeout.
func New(failureThreshold, successThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            Closed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow checks whether a request may pass through.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case Closed:
		return nil
	case Open:
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			cb.state = HalfOpen
			cb.successCount = 0
			return nil
		}
		return ErrCircuitOpen
	case HalfOpen:
		return nil
	}

	return nil
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case HalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = Closed
			cb.failureCount = 0
			cb.successCount = 0
		}
	case Closed:
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case Closed:
		if cb.failureCount >= cb.failureThreshold {
			cb.state = Open
		}
	case HalfOpen:
		cb.state = Open
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == Open && time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
		return HalfOpen
	}

	return cb.state
}
