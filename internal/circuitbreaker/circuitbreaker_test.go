package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(cb *CircuitBreaker)
		want    State
		wantErr error
	}{
		{
			name:  "fresh breaker in Closed state",
			setup: func(cb *CircuitBreaker) {},
			want:  Closed,
		},
		{
			name: "transitions to Open after N failures",
			setup: func(cb *CircuitBreaker) {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure() // threshold = 3
			},
			want: Open,
		},
		{
			name: "Open state returns ErrCircuitOpen",
			setup: func(cb *CircuitBreaker) {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure()
			},
			want:    Open,
			wantErr: ErrCircuitOpen,
		},
		{
			name: "HalfOpen transitions to Closed after successful checks",
			setup: func(cb *CircuitBreaker) {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure() // -> Open
				// Artificially shift lastFailureTime to the past
				// to bypass recovery timeout and transition to HalfOpen
				cb.lastFailureTime = time.Now().Add(-2 * cb.recoveryTimeout)
				_ = cb.Allow() // triggers transition to HalfOpen
				cb.RecordSuccess()
				cb.RecordSuccess() // threshold = 2
			},
			want: Closed,
		},
		{
			name: "HalfOpen with a single failure returns to Open",
			setup: func(cb *CircuitBreaker) {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordFailure()
				cb.lastFailureTime = time.Now().Add(-2 * cb.recoveryTimeout)
				_ = cb.Allow() // -> HalfOpen
				cb.RecordFailure()
			},
			want: Open,
		},
		{
			name: "successful request in Closed state resets failure count",
			setup: func(cb *CircuitBreaker) {
				cb.RecordFailure()
				cb.RecordFailure()
				cb.RecordSuccess() // counter is reset
				cb.RecordFailure()
			},
			want: Closed, // still Closed: only 1 failure after reset
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cb := New(3, 2, 100*time.Millisecond)
			tc.setup(cb)

			if got := cb.State(); got != tc.want {
				t.Errorf("State() = %v, want %v", got, tc.want)
			}

			if tc.wantErr != nil {
				if err := cb.Allow(); !errors.Is(err, tc.wantErr) {
					t.Errorf("Allow() error = %v, want %v", err, tc.wantErr)
				}
			}
		})
	}
}