package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

type HTTPClientCollector struct {
	requestCount          metric.Int64Counter
	requestDuration       metric.Float64Histogram
	errorCount            metric.Int64Counter
	circuitBreakerState   metric.Int64Gauge
	circuitBreakerChanges metric.Int64Counter
}

func NewHTTPClientCollector(meter metric.Meter) (*HTTPClientCollector, error) {
	// If meter is nil, use noop meter from OpenTelemetry
	// The noop meter never returns errors, so this is safe
	if meter == nil {
		meter = noop.NewMeterProvider().Meter("noop")
	}
	requestCount, err := meter.Int64Counter(
		"http.client.requests",
		metric.WithDescription("Total HTTP client requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http.client.duration",
		metric.WithDescription("HTTP client request duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	errorCount, err := meter.Int64Counter(
		"http.client.errors",
		metric.WithDescription("Total HTTP client errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	circuitBreakerState, err := meter.Int64Gauge(
		"http.client.circuit_breaker.state",
		metric.WithDescription("Circuit breaker state (0=Closed, 1=Open, 2=HalfOpen)"),
		metric.WithUnit("{state}"),
	)
	if err != nil {
		return nil, err
	}

	circuitBreakerChanges, err := meter.Int64Counter(
		"http.client.circuit_breaker.state_changes",
		metric.WithDescription("Circuit breaker state changes"),
		metric.WithUnit("{change}"),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPClientCollector{
		requestCount:          requestCount,
		requestDuration:       requestDuration,
		errorCount:            errorCount,
		circuitBreakerState:   circuitBreakerState,
		circuitBreakerChanges: circuitBreakerChanges,
	}, nil
}

// RecordRequest records HTTP client request metrics
func (c *HTTPClientCollector) RecordRequest(
	ctx context.Context,
	method string,
	host string,
	statusCode int,
	duration time.Duration,
	err error,
) {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.host", host),
		attribute.Int("http.status_code", statusCode),
	}

	c.requestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	c.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		errorAttrs := []attribute.KeyValue{
			attribute.String("http.host", host),
			attribute.String("error.type", getErrorType(err)),
		}
		c.errorCount.Add(ctx, 1, metric.WithAttributes(errorAttrs...))
	}
}

// RecordCircuitBreakerState records the current circuit breaker state
func (c *HTTPClientCollector) RecordCircuitBreakerState(
	ctx context.Context,
	host string,
	state string,
) {
	attrs := []attribute.KeyValue{
		attribute.String("http.host", host),
		attribute.String("circuit_breaker.state", state),
	}

	stateValue := circuitBreakerStateToInt(state)
	c.circuitBreakerState.Record(ctx, stateValue, metric.WithAttributes(attrs...))
}

// RecordCircuitBreakerStateChange records circuit breaker state transitions
func (c *HTTPClientCollector) RecordCircuitBreakerStateChange(
	ctx context.Context,
	host string,
	fromState string,
	toState string,
) {
	attrs := []attribute.KeyValue{
		attribute.String("http.host", host),
		attribute.String("circuit_breaker.from_state", fromState),
		attribute.String("circuit_breaker.to_state", toState),
	}

	c.circuitBreakerChanges.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// circuitBreakerStateToInt converts circuit breaker state to numeric value
func circuitBreakerStateToInt(state string) int64 {
	switch state {
	case "closed":
		return 0
	case "open":
		return 1
	case "half-open":
		return 2
	default:
		return -1
	}
}

// getErrorType extracts error type from error
func getErrorType(err error) string {
	if err == nil {
		return "none"
	}

	// Check for common error patterns
	errMsg := err.Error()
	switch {
	case errMsg == "response status code not equal 200":
		return "invalid_status"
	case errMsg == "circuit breaker is open":
		return "circuit_breaker_open"
	default:
		return "unknown"
	}
}
