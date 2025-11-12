package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewHTTPClientCollector(t *testing.T) {
	t.Run("creates collector with all metrics", func(t *testing.T) {
		reader := metric.NewManualReader()
		provider := metric.NewMeterProvider(metric.WithReader(reader))
		meter := provider.Meter("test")

		collector, err := NewHTTPClientCollector(meter)

		require.NoError(t, err)
		assert.NotNil(t, collector)
		assert.NotNil(t, collector.requestCount)
		assert.NotNil(t, collector.requestDuration)
		assert.NotNil(t, collector.errorCount)
		assert.NotNil(t, collector.circuitBreakerState)
		assert.NotNil(t, collector.circuitBreakerChanges)
	})
}

func TestHTTPClientCollector_RecordRequest(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		host         string
		statusCode   int
		duration     time.Duration
		err          error
		expectError  bool
		expectedAttr map[string]any
	}{
		{
			name:        "successful request",
			method:      "POST",
			host:        "api.example.com",
			statusCode:  200,
			duration:    100 * time.Millisecond,
			err:         nil,
			expectError: false,
			expectedAttr: map[string]any{
				"http.method":      "POST",
				"http.host":        "api.example.com",
				"http.status_code": int64(200),
			},
		},
		{
			name:        "failed request with 500 status",
			method:      "POST",
			host:        "api.example.com",
			statusCode:  500,
			duration:    50 * time.Millisecond,
			err:         errors.New("internal server error"),
			expectError: true,
			expectedAttr: map[string]any{
				"http.method":      "POST",
				"http.host":        "api.example.com",
				"http.status_code": int64(500),
			},
		},
		{
			name:        "request with invalid status error",
			method:      "POST",
			host:        "api.example.com",
			statusCode:  400,
			duration:    30 * time.Millisecond,
			err:         errors.New("response status code not equal 200"),
			expectError: true,
			expectedAttr: map[string]any{
				"http.method":      "POST",
				"http.host":        "api.example.com",
				"http.status_code": int64(400),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			meter := provider.Meter("test")

			collector, err := NewHTTPClientCollector(meter)
			require.NoError(t, err)

			ctx := context.Background()
			collector.RecordRequest(ctx, tt.method, tt.host, tt.statusCode, tt.duration, tt.err)

			// Collect metrics
			var rm metricdata.ResourceMetrics
			err = reader.Collect(ctx, &rm)
			require.NoError(t, err)

			// Verify metrics were recorded
			require.NotEmpty(t, rm.ScopeMetrics)
			metrics := rm.ScopeMetrics[0].Metrics

			// Check request count metric
			var foundCount, foundDuration bool
			for _, m := range metrics {
				if m.Name == "http.client.requests" {
					foundCount = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Len(t, sum.DataPoints, 1)
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)
				}
				if m.Name == "http.client.duration" {
					foundDuration = true
					hist := m.Data.(metricdata.Histogram[float64])
					assert.Len(t, hist.DataPoints, 1)
					assert.Greater(t, hist.DataPoints[0].Sum, 0.0)
				}
			}

			assert.True(t, foundCount, "request count metric should be recorded")
			assert.True(t, foundDuration, "request duration metric should be recorded")

			// If error is expected, check error count
			if tt.expectError {
				var foundError bool
				for _, m := range metrics {
					if m.Name == "http.client.errors" {
						foundError = true
						sum := m.Data.(metricdata.Sum[int64])
						assert.Len(t, sum.DataPoints, 1)
						assert.Equal(t, int64(1), sum.DataPoints[0].Value)
					}
				}
				assert.True(t, foundError, "error count metric should be recorded for failed requests")
			}
		})
	}
}

func TestHTTPClientCollector_RecordCircuitBreakerState(t *testing.T) {
	tests := []struct {
		name  string
		host  string
		state string
	}{
		{
			name:  "closed state",
			host:  "api.example.com",
			state: "closed",
		},
		{
			name:  "open state",
			host:  "api.example.com",
			state: "open",
		},
		{
			name:  "half-open state",
			host:  "api.example.com",
			state: "half-open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			meter := provider.Meter("test")

			collector, err := NewHTTPClientCollector(meter)
			require.NoError(t, err)

			ctx := context.Background()
			collector.RecordCircuitBreakerState(ctx, tt.host, tt.state)

			// Collect metrics
			var rm metricdata.ResourceMetrics
			err = reader.Collect(ctx, &rm)
			require.NoError(t, err)

			// Verify circuit breaker state metric
			require.NotEmpty(t, rm.ScopeMetrics)
			metrics := rm.ScopeMetrics[0].Metrics

			var found bool
			for _, m := range metrics {
				if m.Name == "http.client.circuit_breaker.state" {
					found = true
					gauge := m.Data.(metricdata.Gauge[int64])
					assert.Len(t, gauge.DataPoints, 1)
					expectedValue := circuitBreakerStateToInt(tt.state)
					assert.Equal(t, expectedValue, gauge.DataPoints[0].Value)
				}
			}
			assert.True(t, found, "circuit breaker state metric should be recorded")
		})
	}
}

func TestHTTPClientCollector_RecordCircuitBreakerStateChange(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		fromState string
		toState   string
	}{
		{
			name:      "closed to open transition",
			host:      "api.example.com",
			fromState: "closed",
			toState:   "open",
		},
		{
			name:      "open to half-open transition",
			host:      "api.example.com",
			fromState: "open",
			toState:   "half-open",
		},
		{
			name:      "half-open to closed transition",
			host:      "api.example.com",
			fromState: "half-open",
			toState:   "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			meter := provider.Meter("test")

			collector, err := NewHTTPClientCollector(meter)
			require.NoError(t, err)

			ctx := context.Background()
			collector.RecordCircuitBreakerStateChange(ctx, tt.host, tt.fromState, tt.toState)

			// Collect metrics
			var rm metricdata.ResourceMetrics
			err = reader.Collect(ctx, &rm)
			require.NoError(t, err)

			// Verify circuit breaker state change metric
			require.NotEmpty(t, rm.ScopeMetrics)
			metrics := rm.ScopeMetrics[0].Metrics

			var found bool
			for _, m := range metrics {
				if m.Name == "http.client.circuit_breaker.state_changes" {
					found = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Len(t, sum.DataPoints, 1)
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)
				}
			}
			assert.True(t, found, "circuit breaker state change metric should be recorded")
		})
	}
}

func TestCircuitBreakerStateToInt(t *testing.T) {
	tests := []struct {
		state    string
		expected int64
	}{
		{"closed", 0},
		{"open", 1},
		{"half-open", 2},
		{"unknown", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			result := circuitBreakerStateToInt(tt.state)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "none",
		},
		{
			name:     "invalid status error",
			err:      errors.New("response status code not equal 200"),
			expected: "invalid_status",
		},
		{
			name:     "circuit breaker open error",
			err:      errors.New("circuit breaker is open"),
			expected: "circuit_breaker_open",
		},
		{
			name:     "unknown error",
			err:      errors.New("some other error"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getErrorType(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNoopHTTPClientCollector(t *testing.T) {
	t.Run("noop collector does not panic", func(t *testing.T) {
		// Create collector with nil meter, which uses noop meter
		collector, err := NewHTTPClientCollector(nil)
		require.NoError(t, err)

		ctx := context.Background()

		// All methods should be callable without panic
		assert.NotPanics(t, func() {
			collector.RecordRequest(ctx, "POST", "api.example.com", 200, time.Second, nil)
		})

		assert.NotPanics(t, func() {
			collector.RecordCircuitBreakerState(ctx, "api.example.com", "closed")
		})

		assert.NotPanics(t, func() {
			collector.RecordCircuitBreakerStateChange(ctx, "api.example.com", "closed", "open")
		})
	})
}

func TestNewHTTPClientCollectorWithNilMeter(t *testing.T) {
	t.Run("creates noop collector with nil meter", func(t *testing.T) {
		collector, err := NewHTTPClientCollector(nil)
		require.NoError(t, err)
		assert.NotNil(t, collector)

		// Should not panic when used
		ctx := context.Background()
		assert.NotPanics(t, func() {
			collector.RecordRequest(ctx, "POST", "api.example.com", 200, time.Second, nil)
		})
	})
}
