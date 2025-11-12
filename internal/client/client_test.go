package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/koungkub/fw-challenge-notification-service/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

func TestNewHTTPClient(t *testing.T) {
	metricsCollector, _ := metrics.NewHTTPClientCollector(nil)
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     5,
			OpenStateTimeout:        60 * time.Second,
			MinRequestsBeforeTrip:   3,
			FailureThresholdPercent: 60,
		},
		Logger: zap.NewNop(),
	})

	params := HTTPClientParams{
		Config: HTTPClientConfig{
			Timeout: 10 * time.Second,
		},
		CircuitBreakerRegistry: cbRegistry,
		MetricsCollector:       metricsCollector,
		Logger:                 zap.NewNop(),
	}

	client := NewHTTPClient(params)

	assert.NotNil(t, client)
	assert.NotNil(t, client.httpclient)
	assert.NotNil(t, client.circuitBreakerRegistry)
	assert.NotNil(t, client.metricsCollector)
	assert.Equal(t, 10*time.Second, client.httpclient.Timeout)
}

func TestNewHTTPClientConfig(t *testing.T) {
	config := NewHTTPClientConfig()

	assert.NotZero(t, config.Timeout)
	// Verify timeout is set (actual default value may vary based on env)
	assert.Greater(t, config.Timeout, time.Duration(0))
}

func TestHTTPClient_Post_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req NotificationRequest
		json.Unmarshal(body, &req)
		assert.Equal(t, "test@example.com", req.To)
		assert.Equal(t, "Test Title", req.Title)
		assert.Equal(t, "Test Message", req.Message)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "sent"}`))
	}))
	defer server.Close()

	metricsCollector, _ := metrics.NewHTTPClientCollector(nil)
	client := NewHTTPClient(HTTPClientParams{
		Config: NewHTTPClientConfig(),
		CircuitBreakerRegistry: NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: NewCircuitBreakerRegistryConfig(),
			Logger: zap.NewNop(),
		}),
		MetricsCollector: metricsCollector,
		Logger:           zap.NewNop(),
	})

	ctx := context.Background()
	err := client.Post(ctx, server.URL, NotificationRequest{
		To:      "test@example.com",
		Title:   "Test Title",
		Message: "Test Message",
	})

	assert.NoError(t, err)
}

func TestHTTPClient_Post_NonOKStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"Bad Request", http.StatusBadRequest},
		{"Unauthorized", http.StatusUnauthorized},
		{"Internal Server Error", http.StatusInternalServerError},
		{"Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			metricsCollector, _ := metrics.NewHTTPClientCollector(nil)
			client := NewHTTPClient(HTTPClientParams{
				Config: NewHTTPClientConfig(),
				CircuitBreakerRegistry: NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
					Config: NewCircuitBreakerRegistryConfig(),
					Logger: zap.NewNop(),
				}),
				MetricsCollector: metricsCollector,
				Logger:           zap.NewNop(),
			})

			ctx := context.Background()
			err := client.Post(ctx, server.URL, NotificationRequest{
				To:      "test@example.com",
				Title:   "Test",
				Message: "Test",
			})

			assert.Error(t, err)
			assert.Equal(t, "response status code not equal 200", err.Error())
		})
	}
}

func TestHTTPClient_Post_InvalidURL(t *testing.T) {
	metricsCollector, _ := metrics.NewHTTPClientCollector(nil)
	client := NewHTTPClient(HTTPClientParams{
		Config: NewHTTPClientConfig(),
		CircuitBreakerRegistry: NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: NewCircuitBreakerRegistryConfig(),
			Logger: zap.NewNop(),
		}),
		MetricsCollector: metricsCollector,
		Logger:           zap.NewNop(),
	})

	ctx := context.Background()
	err := client.Post(ctx, "://invalid-url", NotificationRequest{
		To:      "test@example.com",
		Title:   "Test",
		Message: "Test",
	})

	assert.Error(t, err)
}

func TestHTTPClient_Post_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsCollector, _ := metrics.NewHTTPClientCollector(nil)
	client := NewHTTPClient(HTTPClientParams{
		Config: NewHTTPClientConfig(),
		CircuitBreakerRegistry: NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
			Config: NewCircuitBreakerRegistryConfig(),
			Logger: zap.NewNop(),
		}),
		MetricsCollector: metricsCollector,
		Logger:           zap.NewNop(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.Post(ctx, server.URL, NotificationRequest{
		To:      "test@example.com",
		Title:   "Test",
		Message: "Test",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedHost string
		expectError  bool
	}{
		{
			name:         "valid HTTP URL",
			url:          "http://api.example.com/path",
			expectedHost: "api.example.com",
			expectError:  false,
		},
		{
			name:         "valid HTTPS URL",
			url:          "https://api.example.com:8080/path",
			expectedHost: "api.example.com:8080",
			expectError:  false,
		},
		{
			name:         "URL with port",
			url:          "http://localhost:3000/api/test",
			expectedHost: "localhost:3000",
			expectError:  false,
		},
		{
			name:         "invalid URL",
			url:          "://invalid",
			expectedHost: "",
			expectError:  true,
		},
		{
			name:         "URL with subdomain",
			url:          "https://api.staging.example.com/path",
			expectedHost: "api.staging.example.com",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := extractHost(tt.url)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
			}
		})
	}
}

func TestHTTPClient_WithMetrics_SuccessfulRequest(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := metrics.NewHTTPClientCollector(meter)
	require.NoError(t, err)

	// Create HTTP client
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     5,
			OpenStateTimeout:        60 * time.Second,
			MinRequestsBeforeTrip:   3,
			FailureThresholdPercent: 60,
		},
		Logger: zap.NewNop(),
	})

	client := &HTTPClient{
		httpclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		circuitBreakerRegistry: cbRegistry,
		metricsCollector:       collector,
		logger:                 zap.NewNop(),
	}

	// Make request
	ctx := context.Background()
	req := NotificationRequest{
		To:        "user@example.com",
		Title:     "Test",
		Message:   "Test message",
		SecretKey: "secret",
	}

	err = client.Post(ctx, server.URL, req)
	require.NoError(t, err)

	// Verify metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	// Check that request count was recorded
	var foundRequestCount bool
	for _, m := range metricsData {
		if m.Name == "http.client.requests" {
			foundRequestCount = true
			sum := m.Data.(metricdata.Sum[int64])
			assert.NotEmpty(t, sum.DataPoints)
			assert.Equal(t, int64(1), sum.DataPoints[0].Value)
		}
	}
	assert.True(t, foundRequestCount, "request count metric should be recorded")

	// Check that request duration was recorded
	var foundDuration bool
	for _, m := range metricsData {
		if m.Name == "http.client.duration" {
			foundDuration = true
			hist := m.Data.(metricdata.Histogram[float64])
			assert.Len(t, hist.DataPoints, 1)
			assert.Greater(t, hist.DataPoints[0].Sum, 0.0)
		}
	}
	assert.True(t, foundDuration, "request duration metric should be recorded")
}

func TestHTTPClient_WithMetrics_FailedRequest(t *testing.T) {
	// Create test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := metrics.NewHTTPClientCollector(meter)
	require.NoError(t, err)

	// Create HTTP client
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     5,
			OpenStateTimeout:        60 * time.Second,
			MinRequestsBeforeTrip:   3,
			FailureThresholdPercent: 60,
		},
		Logger: zap.NewNop(),
	})

	client := &HTTPClient{
		httpclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		circuitBreakerRegistry: cbRegistry,
		metricsCollector:       collector,
		logger:                 zap.NewNop(),
	}

	// Make request
	ctx := context.Background()
	req := NotificationRequest{
		To:        "user@example.com",
		Title:     "Test",
		Message:   "Test message",
		SecretKey: "secret",
	}

	err = client.Post(ctx, server.URL, req)
	assert.Error(t, err)
	assert.Equal(t, "response status code not equal 200", err.Error())

	// Verify metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	// Check that error count was recorded
	var foundErrorCount bool
	for _, m := range metricsData {
		if m.Name == "http.client.errors" {
			foundErrorCount = true
			sum := m.Data.(metricdata.Sum[int64])
			assert.Len(t, sum.DataPoints, 1)
			assert.Equal(t, int64(1), sum.DataPoints[0].Value)
		}
	}
	assert.True(t, foundErrorCount, "error count metric should be recorded")

	// Check that request was recorded with error status
	var foundRequestCount bool
	for _, m := range metricsData {
		if m.Name == "http.client.requests" {
			foundRequestCount = true
			sum := m.Data.(metricdata.Sum[int64])
			assert.NotEmpty(t, sum.DataPoints)
		}
	}
	assert.True(t, foundRequestCount, "request count metric should be recorded even for failures")
}

func TestHTTPClient_WithMetrics_CircuitBreakerState(t *testing.T) {
	// Create test server that always fails
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := metrics.NewHTTPClientCollector(meter)
	require.NoError(t, err)

	// Create HTTP client with lower thresholds for testing
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     1,
			OpenStateTimeout:        100 * time.Millisecond,
			MinRequestsBeforeTrip:   2,
			FailureThresholdPercent: 50,
		},
		Logger: zap.NewNop(),
	})

	client := &HTTPClient{
		httpclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		circuitBreakerRegistry: cbRegistry,
		metricsCollector:       collector,
		logger:                 zap.NewNop(),
	}

	ctx := context.Background()
	req := NotificationRequest{
		To:        "user@example.com",
		Title:     "Test",
		Message:   "Test message",
		SecretKey: "secret",
	}

	// Make requests to trip the circuit breaker
	for i := 0; i < 5; i++ {
		_ = client.Post(ctx, server.URL, req)
	}

	// Verify circuit breaker state metric was recorded
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	var foundCBState bool
	for _, m := range metricsData {
		if m.Name == "http.client.circuit_breaker.state" {
			foundCBState = true
			gauge := m.Data.(metricdata.Gauge[int64])
			assert.NotEmpty(t, gauge.DataPoints)
		}
	}
	assert.True(t, foundCBState, "circuit breaker state metric should be recorded")
}

func TestHTTPClient_WithNoopMetrics(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	// Create HTTP client with noop metrics (nil meter)
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     5,
			OpenStateTimeout:        60 * time.Second,
			MinRequestsBeforeTrip:   3,
			FailureThresholdPercent: 60,
		},
		Logger: zap.NewNop(),
	})

	metricsCollector, err := metrics.NewHTTPClientCollector(nil)
	require.NoError(t, err)

	client := &HTTPClient{
		httpclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		circuitBreakerRegistry: cbRegistry,
		metricsCollector:       metricsCollector,
		logger:                 zap.NewNop(),
	}

	// Make request - should not panic
	ctx := context.Background()
	req := NotificationRequest{
		To:        "user@example.com",
		Title:     "Test",
		Message:   "Test message",
		SecretKey: "secret",
	}

	err = client.Post(ctx, server.URL, req)
	require.NoError(t, err)
}

func TestHTTPClient_MultipleRequests_Metrics(t *testing.T) {
	// Create test server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount%2 == 0 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := metrics.NewHTTPClientCollector(meter)
	require.NoError(t, err)

	// Create HTTP client
	cbRegistry := NewCircuitBreakerRegistry(CircuitBreakerRegistryParams{
		Config: CircuitBreakerRegistryConfig{
			MaxHalfOpenRequests:     5,
			OpenStateTimeout:        60 * time.Second,
			MinRequestsBeforeTrip:   10, // High threshold to avoid tripping
			FailureThresholdPercent: 90,
		},
		Logger: zap.NewNop(),
	})

	client := &HTTPClient{
		httpclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		circuitBreakerRegistry: cbRegistry,
		metricsCollector:       collector,
		logger:                 zap.NewNop(),
	}

	ctx := context.Background()
	req := NotificationRequest{
		To:        "user@example.com",
		Title:     "Test",
		Message:   "Test message",
		SecretKey: "secret",
	}

	// Make multiple requests
	numRequests := 4
	for i := 0; i < numRequests; i++ {
		_ = client.Post(ctx, server.URL, req)
	}

	// Verify metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(ctx, &rm)
	require.NoError(t, err)

	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	// Check that all requests were counted
	var totalRequests int64
	for _, m := range metricsData {
		if m.Name == "http.client.requests" {
			sum := m.Data.(metricdata.Sum[int64])
			for _, dp := range sum.DataPoints {
				totalRequests += dp.Value
			}
		}
	}
	assert.Equal(t, int64(numRequests), totalRequests, "all requests should be counted")
}
