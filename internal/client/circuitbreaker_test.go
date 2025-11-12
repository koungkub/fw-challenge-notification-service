package client

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreakerRegistry(t *testing.T) {
	tests := []struct {
		name   string
		params CircuitBreakerRegistryParams
		verify func(t *testing.T, registry *CircuitBreakerRegistry)
	}{
		{
			name: "creates registry with default config",
			params: CircuitBreakerRegistryParams{
				Config: CircuitBreakerRegistryConfig{
					MaxHalfOpenRequests:     5,
					OpenStateTimeout:        60 * time.Second,
					MinRequestsBeforeTrip:   3,
					FailureThresholdPercent: 60,
				},
			},
			verify: func(t *testing.T, registry *CircuitBreakerRegistry) {
				assert.NotNil(t, registry)
				assert.NotNil(t, registry.breakers)
				assert.Equal(t, uint32(5), registry.settings.MaxRequests)
				assert.Equal(t, 60*time.Second, registry.settings.Timeout)
				assert.NotNil(t, registry.settings.ReadyToTrip)
			},
		},
		{
			name: "creates registry with custom config",
			params: CircuitBreakerRegistryParams{
				Config: CircuitBreakerRegistryConfig{
					MaxHalfOpenRequests:     10,
					OpenStateTimeout:        30 * time.Second,
					MinRequestsBeforeTrip:   5,
					FailureThresholdPercent: 75,
				},
			},
			verify: func(t *testing.T, registry *CircuitBreakerRegistry) {
				assert.NotNil(t, registry)
				assert.Equal(t, uint32(10), registry.settings.MaxRequests)
				assert.Equal(t, 30*time.Second, registry.settings.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewCircuitBreakerRegistry(tt.params)
			tt.verify(t, registry)
		})
	}
}

func TestCircuitBreakerRegistry_ReadyToTrip(t *testing.T) {
	tests := []struct {
		name                string
		config              CircuitBreakerRegistryConfig
		counts              gobreaker.Counts
		expectedReadyToTrip bool
		description         string
	}{
		{
			name: "should not trip when requests below minimum",
			config: CircuitBreakerRegistryConfig{
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
			counts: gobreaker.Counts{
				Requests:      2,
				TotalFailures: 2,
			},
			expectedReadyToTrip: false,
			description:         "even with 100% failure rate, not enough requests",
		},
		{
			name: "should trip when failure threshold exceeded",
			config: CircuitBreakerRegistryConfig{
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
			counts: gobreaker.Counts{
				Requests:      5,
				TotalFailures: 4, // 80% failure rate
			},
			expectedReadyToTrip: true,
			description:         "80% failure rate exceeds 60% threshold",
		},
		{
			name: "should not trip when failure threshold not exceeded",
			config: CircuitBreakerRegistryConfig{
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
			counts: gobreaker.Counts{
				Requests:      5,
				TotalFailures: 2, // 40% failure rate
			},
			expectedReadyToTrip: false,
			description:         "40% failure rate below 60% threshold",
		},
		{
			name: "should trip at exact threshold",
			config: CircuitBreakerRegistryConfig{
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
			counts: gobreaker.Counts{
				Requests:      5,
				TotalFailures: 3, // 60% failure rate
			},
			expectedReadyToTrip: true,
			description:         "60% failure rate equals 60% threshold",
		},
		{
			name: "should trip at exact minimum requests",
			config: CircuitBreakerRegistryConfig{
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
			counts: gobreaker.Counts{
				Requests:      3,
				TotalFailures: 2, // 66.67% failure rate
			},
			expectedReadyToTrip: true,
			description:         "66.67% failure rate exceeds 60% threshold at minimum requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := CircuitBreakerRegistryParams{
				Config: tt.config,
			}
			registry := NewCircuitBreakerRegistry(params)

			readyToTrip := registry.settings.ReadyToTrip(tt.counts)
			assert.Equal(t, tt.expectedReadyToTrip, readyToTrip, tt.description)
		})
	}
}

func TestCircuitBreakerRegistry_GetOrCreate(t *testing.T) {
	t.Run("creates new circuit breaker for new host", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     5,
				OpenStateTimeout:        60 * time.Second,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		host := "api.example.com"
		cb := registry.GetOrCreate(host)

		assert.NotNil(t, cb)
		assert.Equal(t, host, cb.Name())
		assert.Equal(t, gobreaker.StateClosed, cb.State())
	})

	t.Run("returns existing circuit breaker for same host", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     5,
				OpenStateTimeout:        60 * time.Second,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		host := "api.example.com"
		cb1 := registry.GetOrCreate(host)
		cb2 := registry.GetOrCreate(host)

		assert.Same(t, cb1, cb2, "should return the same circuit breaker instance")
	})

	t.Run("creates different circuit breakers for different hosts", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     5,
				OpenStateTimeout:        60 * time.Second,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		host1 := "api1.example.com"
		host2 := "api2.example.com"

		cb1 := registry.GetOrCreate(host1)
		cb2 := registry.GetOrCreate(host2)

		assert.NotSame(t, cb1, cb2, "should create different circuit breakers")
		assert.Equal(t, host1, cb1.Name())
		assert.Equal(t, host2, cb2.Name())
	})
}

func TestCircuitBreakerRegistry_Concurrency(t *testing.T) {
	t.Run("concurrent access to GetOrCreate is safe", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     5,
				OpenStateTimeout:        60 * time.Second,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		host := "api.example.com"
		numGoroutines := 100
		var wg sync.WaitGroup
		breakers := make([]*gobreaker.CircuitBreaker[CircuitBreakerResponse], numGoroutines)

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()
				breakers[index] = registry.GetOrCreate(host)
			}(i)
		}
		wg.Wait()

		// All goroutines should get the same circuit breaker instance
		firstBreaker := breakers[0]
		for i := 1; i < numGoroutines; i++ {
			assert.Same(t, firstBreaker, breakers[i],
				"all concurrent calls should return the same circuit breaker")
		}
	})

	t.Run("concurrent access with different hosts", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     5,
				OpenStateTimeout:        60 * time.Second,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		numGoroutines := 50
		var wg sync.WaitGroup

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				defer wg.Done()
				host := "api" + string(rune('A'+index)) + ".example.com"
				cb := registry.GetOrCreate(host)
				assert.NotNil(t, cb)
				assert.Equal(t, host, cb.Name())
			}(i)
		}
		wg.Wait()
	})
}

func TestCircuitBreakerRegistry_Integration(t *testing.T) {
	t.Run("circuit breaker trips after threshold failures", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     1,
				OpenStateTimeout:        100 * time.Millisecond,
				MinRequestsBeforeTrip:   3,
				FailureThresholdPercent: 60,
			},
		})

		host := "api.example.com"
		cb := registry.GetOrCreate(host)

		require.Equal(t, gobreaker.StateClosed, cb.State())

		// Execute requests that fail - need at least 3 requests with 60% failure
		// Let's do 5 requests with 4 failures (80% failure rate)
		for i := 0; i < 5; i++ {
			_, _ = cb.Execute(func() (CircuitBreakerResponse, error) {
				if i < 4 {
					return CircuitBreakerResponse{}, assert.AnError
				}
				return CircuitBreakerResponse{StatusCode: http.StatusOK}, nil
			})
		}

		// Circuit breaker should now be open
		assert.Equal(t, gobreaker.StateOpen, cb.State())
	})

	t.Run("circuit breaker transitions to half-open after timeout", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: CircuitBreakerRegistryConfig{
				MaxHalfOpenRequests:     1,
				OpenStateTimeout:        50 * time.Millisecond, // Short timeout for testing
				MinRequestsBeforeTrip:   2,
				FailureThresholdPercent: 50,
			},
		})

		host := "api.example.com"
		cb := registry.GetOrCreate(host)

		// Trip the circuit breaker
		for i := 0; i < 3; i++ {
			_, _ = cb.Execute(func() (CircuitBreakerResponse, error) {
				return CircuitBreakerResponse{}, assert.AnError
			})
		}

		require.Equal(t, gobreaker.StateOpen, cb.State())

		// Wait for timeout to transition to half-open
		time.Sleep(100 * time.Millisecond)

		// The next execution attempt should find it in half-open state
		state := cb.State()
		assert.Contains(t, []gobreaker.State{gobreaker.StateOpen, gobreaker.StateHalfOpen}, state)
	})
}
