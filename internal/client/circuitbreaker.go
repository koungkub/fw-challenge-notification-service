package client

import (
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/fx"
)

type CircuitBreakerRegistry struct {
	breakers *sync.Map
	settings gobreaker.Settings
}

type CircuitBreakerResponse struct {
	Body       []byte
	StatusCode int
}

type CircuitBreakerRegistryParams struct {
	fx.In

	Config CircuitBreakerRegistryConfig
}

func NewCircuitBreakerRegistry(params CircuitBreakerRegistryParams) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: &sync.Map{},
		settings: gobreaker.Settings{
			MaxRequests: params.Config.MaxHalfOpenRequests,
			Timeout:     params.Config.OpenStateTimeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)

				return counts.Requests >= params.Config.MinRequestsBeforeTrip &&
					failureRatio >= (params.Config.FailureThresholdPercent/100)
			},
		},
	}
}

type CircuitBreakerRegistryConfig struct {
	MaxHalfOpenRequests     uint32        `envconfig:"CIRCUIT_BREAKER_MAX_HALF_OPEN_REQUESTS" default:"5"`
	OpenStateTimeout        time.Duration `envconfig:"CIRCUIT_BREAKER_OPEN_STATE_TIMEOUT" default:"60s"`
	MinRequestsBeforeTrip   uint32        `envconfig:"CIRCUIT_BREAKER_MIN_REQUESTS_BEFORE_TRIP" default:"3"`
	FailureThresholdPercent float64       `envconfig:"CIRCUIT_BREAKER_FAILURE_THRESHOLD_PERCENT" default:"60"`
}

func NewCircuitBreakerRegistryConfig() CircuitBreakerRegistryConfig {
	var cfg CircuitBreakerRegistryConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}

func (r *CircuitBreakerRegistry) GetOrCreate(host string) *gobreaker.CircuitBreaker[CircuitBreakerResponse] {
	if cb, ok := r.breakers.Load(host); ok {
		return cb.(*gobreaker.CircuitBreaker[CircuitBreakerResponse])
	}

	settings := r.settings
	settings.Name = host

	cb := gobreaker.NewCircuitBreaker[CircuitBreakerResponse](settings)

	actual, _ := r.breakers.LoadOrStore(host, cb)
	return actual.(*gobreaker.CircuitBreaker[CircuitBreakerResponse])
}
