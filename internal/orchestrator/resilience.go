package orchestrator

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"

	"github.com/aristath/orchestrator/internal/backend"
)

// RetryConfig configures exponential backoff retry behavior.
type RetryConfig struct {
	InitialInterval     time.Duration // Initial retry interval (default 100ms)
	MaxInterval         time.Duration // Maximum retry interval (default 10s)
	MaxElapsedTime      time.Duration // Maximum total retry time (default 2min)
	Multiplier          float64       // Backoff multiplier (default 2.0)
	RandomizationFactor float64       // Jitter factor (default 0.5)
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		InitialInterval:     100 * time.Millisecond,
		MaxInterval:         10 * time.Second,
		MaxElapsedTime:      2 * time.Minute,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
	}
}

// CircuitBreakerRegistry manages per-backend-type circuit breakers.
type CircuitBreakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*gobreaker.CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new circuit breaker registry.
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
	}
}

// Get returns the circuit breaker for the given backend type.
// Creates a new one if it doesn't exist.
func (r *CircuitBreakerRegistry) Get(backendType string) *gobreaker.CircuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok := r.breakers[backendType]; ok {
		return cb
	}

	// Create new circuit breaker
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        backendType,
		MaxRequests: 3, // Allow 3 test requests in half-open state
		Interval:    0, // Don't clear counts automatically
		Timeout:     30 * time.Second, // Stay open for 30s before testing recovery
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip circuit after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker %q: %s -> %s", name, from, to)
		},
		IsSuccessful: func(err error) bool {
			// Don't count user cancellation as backend failure
			if err == nil {
				return true
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return true
			}
			return false
		},
	})

	r.breakers[backendType] = cb
	return cb
}

// sendWithRetry sends a message to the backend with exponential backoff retry and circuit breaker protection.
func sendWithRetry(ctx context.Context, b backend.Backend, msg backend.Message, cb *gobreaker.CircuitBreaker, retryCfg RetryConfig) (backend.Response, error) {
	var resp backend.Response

	operation := func() error {
		// Check context first - fail fast if cancelled
		if ctx.Err() != nil {
			return backoff.Permanent(ctx.Err())
		}

		// Execute through circuit breaker
		result, err := cb.Execute(func() (interface{}, error) {
			return b.Send(ctx, msg)
		})

		// Handle circuit breaker errors
		if err != nil {
			// Circuit is open - don't retry
			if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
				return backoff.Permanent(err)
			}

			// Context cancelled - stop retrying
			if ctx.Err() != nil {
				return backoff.Permanent(err)
			}

			// Other errors will be retried
			return err
		}

		// Success - cast result to Response
		resp = result.(backend.Response)
		return nil
	}

	// Create exponential backoff policy
	backoffPolicy := backoff.NewExponentialBackOff()
	backoffPolicy.InitialInterval = retryCfg.InitialInterval
	backoffPolicy.MaxInterval = retryCfg.MaxInterval
	backoffPolicy.MaxElapsedTime = retryCfg.MaxElapsedTime
	backoffPolicy.Multiplier = retryCfg.Multiplier
	backoffPolicy.RandomizationFactor = retryCfg.RandomizationFactor

	// Wrap with context to respect cancellation
	backoffWithContext := backoff.WithContext(backoffPolicy, ctx)

	// Execute with retry
	err := backoff.Retry(operation, backoffWithContext)
	return resp, err
}
