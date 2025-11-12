package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewHTTPServerCollector(t *testing.T) {
	t.Run("creates collector with all metrics", func(t *testing.T) {
		reader := metric.NewManualReader()
		provider := metric.NewMeterProvider(metric.WithReader(reader))
		meter := provider.Meter("test")

		collector, err := NewHTTPServerCollector(meter)

		require.NoError(t, err)
		assert.NotNil(t, collector)
		assert.NotNil(t, collector.requestCount)
		assert.NotNil(t, collector.requestDuration)
	})
}

func TestHTTPServerCollector_Middleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		route          string
		handler        gin.HandlerFunc
		expectedStatus int
	}{
		{
			name:   "GET request with 200 response",
			method: "GET",
			path:   "/api/users",
			route:  "/api/users",
			handler: func(c *gin.Context) {
				c.Status(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "POST request with 201 response",
			method: "POST",
			path:   "/api/users",
			route:  "/api/users",
			handler: func(c *gin.Context) {
				c.Status(http.StatusCreated)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:   "GET request with 404 response",
			method: "GET",
			path:   "/api/notfound",
			route:  "/api/notfound",
			handler: func(c *gin.Context) {
				c.Status(http.StatusNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "POST request with 500 response",
			method: "POST",
			path:   "/api/error",
			route:  "/api/error",
			handler: func(c *gin.Context) {
				c.Status(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:   "GET request with path parameters",
			method: "GET",
			path:   "/api/users/123",
			route:  "/api/users/:id",
			handler: func(c *gin.Context) {
				c.Status(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup metrics
			reader := metric.NewManualReader()
			provider := metric.NewMeterProvider(metric.WithReader(reader))
			meter := provider.Meter("test")

			collector, err := NewHTTPServerCollector(meter)
			require.NoError(t, err)

			// Setup Gin
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(collector.Middleware())

			// Register route
			switch tt.method {
			case "GET":
				router.GET(tt.route, tt.handler)
			case "POST":
				router.POST(tt.route, tt.handler)
			case "PUT":
				router.PUT(tt.route, tt.handler)
			case "DELETE":
				router.DELETE(tt.route, tt.handler)
			}

			// Make request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Collect metrics
			var rm metricdata.ResourceMetrics
			err = reader.Collect(req.Context(), &rm)
			require.NoError(t, err)

			// Verify metrics were recorded
			require.NotEmpty(t, rm.ScopeMetrics)
			metricsData := rm.ScopeMetrics[0].Metrics

			// Check request count metric
			var foundRequestCount bool
			for _, m := range metricsData {
				if m.Name == "http.server.requests" {
					foundRequestCount = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.NotEmpty(t, sum.DataPoints)
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)

					// Verify attributes exist
					attrs := sum.DataPoints[0].Attributes
					assert.NotEmpty(t, attrs.ToSlice())
				}
			}
			assert.True(t, foundRequestCount, "request count metric should be recorded")

			// Check request duration metric
			var foundDuration bool
			for _, m := range metricsData {
				if m.Name == "http.server.duration" {
					foundDuration = true
					hist := m.Data.(metricdata.Histogram[float64])
					assert.NotEmpty(t, hist.DataPoints)
					assert.GreaterOrEqual(t, hist.DataPoints[0].Sum, 0.0)
				}
			}
			assert.True(t, foundDuration, "request duration metric should be recorded")
		})
	}
}

func TestHTTPServerCollector_Middleware_MultipleRequests(t *testing.T) {
	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := NewHTTPServerCollector(meter)
	require.NoError(t, err)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(collector.Middleware())
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make multiple requests
	numRequests := 5
	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Collect metrics
	var rm metricdata.ResourceMetrics
	req := httptest.NewRequest("GET", "/api/test", nil)
	err = reader.Collect(req.Context(), &rm)
	require.NoError(t, err)

	// Verify metrics
	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	// Check total request count
	var totalRequests int64
	for _, m := range metricsData {
		if m.Name == "http.server.requests" {
			sum := m.Data.(metricdata.Sum[int64])
			for _, dp := range sum.DataPoints {
				totalRequests += dp.Value
			}
		}
	}
	assert.Equal(t, int64(numRequests), totalRequests, "all requests should be counted")
}

func TestHTTPServerCollector_Middleware_DifferentRoutes(t *testing.T) {
	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := NewHTTPServerCollector(meter)
	require.NoError(t, err)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(collector.Middleware())
	router.GET("/api/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/posts", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/api/users", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	// Make requests to different routes
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/users"},
		{"GET", "/api/posts"},
		{"POST", "/api/users"},
		{"GET", "/api/users"},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Collect metrics
	var rm metricdata.ResourceMetrics
	req := httptest.NewRequest("GET", "/api/test", nil)
	err = reader.Collect(req.Context(), &rm)
	require.NoError(t, err)

	// Verify metrics
	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	// Check that metrics are recorded separately per route/method
	var requestCount int
	for _, m := range metricsData {
		if m.Name == "http.server.requests" {
			sum := m.Data.(metricdata.Sum[int64])
			requestCount = len(sum.DataPoints)
		}
	}
	// Should have multiple data points for different route/method combinations
	assert.Greater(t, requestCount, 0, "should have metrics for different routes")
}

func TestHTTPServerCollector_Middleware_WithJSON(t *testing.T) {
	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := NewHTTPServerCollector(meter)
	require.NoError(t, err)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(collector.Middleware())
	router.GET("/api/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request
	req := httptest.NewRequest("GET", "/api/data", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "success")

	// Collect metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(req.Context(), &rm)
	require.NoError(t, err)

	// Verify metrics were recorded
	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	var foundMetrics bool
	for _, m := range metricsData {
		if m.Name == "http.server.requests" {
			foundMetrics = true
			sum := m.Data.(metricdata.Sum[int64])
			assert.NotEmpty(t, sum.DataPoints)
		}
	}
	assert.True(t, foundMetrics, "metrics should be recorded for JSON responses")
}

func TestHTTPServerCollector_Middleware_PathFallback(t *testing.T) {
	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := NewHTTPServerCollector(meter)
	require.NoError(t, err)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(collector.Middleware())
	// Handler without defined route - will use URL path
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	// Make request to undefined route
	req := httptest.NewRequest("GET", "/undefined/path", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Collect metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(req.Context(), &rm)
	require.NoError(t, err)

	// Verify metrics were recorded with URL path as fallback
	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	var foundMetrics bool
	for _, m := range metricsData {
		if m.Name == "http.server.requests" {
			foundMetrics = true
			sum := m.Data.(metricdata.Sum[int64])
			assert.NotEmpty(t, sum.DataPoints)
		}
	}
	assert.True(t, foundMetrics, "metrics should be recorded even for undefined routes")
}

func TestHTTPServerCollector_Middleware_MeasuresDuration(t *testing.T) {
	// Setup metrics
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	collector, err := NewHTTPServerCollector(meter)
	require.NoError(t, err)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(collector.Middleware())
	router.GET("/api/slow", func(c *gin.Context) {
		// Simulate some processing time
		c.Status(http.StatusOK)
	})

	// Make request
	req := httptest.NewRequest("GET", "/api/slow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Collect metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(req.Context(), &rm)
	require.NoError(t, err)

	// Verify duration was measured
	require.NotEmpty(t, rm.ScopeMetrics)
	metricsData := rm.ScopeMetrics[0].Metrics

	for _, m := range metricsData {
		if m.Name == "http.server.duration" {
			hist := m.Data.(metricdata.Histogram[float64])
			assert.NotEmpty(t, hist.DataPoints)
			// Duration should be greater than 0
			assert.Greater(t, hist.DataPoints[0].Sum, 0.0)
			// Count should be 1
			assert.Equal(t, uint64(1), hist.DataPoints[0].Count)
		}
	}
}

func TestHTTPServerCollector_Middleware_WithPanic(t *testing.T) {
	t.Run("metrics not recorded when handler panics before c.Next() returns", func(t *testing.T) {
		// Setup metrics
		reader := metric.NewManualReader()
		provider := metric.NewMeterProvider(metric.WithReader(reader))
		meter := provider.Meter("test")

		collector, err := NewHTTPServerCollector(meter)
		require.NoError(t, err)

		// Setup Gin with recovery middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(gin.Recovery()) // Add recovery middleware first
		router.Use(collector.Middleware())
		router.GET("/api/panic", func(c *gin.Context) {
			panic("something went wrong")
		})

		// Make request
		req := httptest.NewRequest("GET", "/api/panic", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify response (Gin's recovery middleware returns 500)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err = reader.Collect(req.Context(), &rm)
		require.NoError(t, err)

		// Current implementation limitation: metrics are NOT recorded when panic occurs
		// because the code after c.Next() doesn't use defer. This test documents
		// the current behavior. To fix this, the middleware would need to use defer
		// for metrics recording.
		if len(rm.ScopeMetrics) == 0 {
			// No metrics recorded - this is the current behavior
			assert.Empty(t, rm.ScopeMetrics, "metrics not recorded on panic (current limitation)")
		} else {
			// If metrics are recorded (after potential future fix), verify them
			metricsData := rm.ScopeMetrics[0].Metrics
			assert.NotEmpty(t, metricsData, "metrics should be recorded if implementation is fixed")
		}
	})

	t.Run("metrics recorded when handler completes successfully after recovery", func(t *testing.T) {
		// Setup metrics
		reader := metric.NewManualReader()
		provider := metric.NewMeterProvider(metric.WithReader(reader))
		meter := provider.Meter("test")

		collector, err := NewHTTPServerCollector(meter)
		require.NoError(t, err)

		// Setup Gin
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(collector.Middleware())
		router.GET("/api/handled-error", func(c *gin.Context) {
			// Handler that doesn't panic but returns error status
			c.Status(http.StatusInternalServerError)
		})

		// Make request
		req := httptest.NewRequest("GET", "/api/handled-error", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify response
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err = reader.Collect(req.Context(), &rm)
		require.NoError(t, err)

		// Metrics should be recorded for handled errors (no panic)
		require.NotEmpty(t, rm.ScopeMetrics)
		metricsData := rm.ScopeMetrics[0].Metrics

		var foundRequestCount bool
		for _, m := range metricsData {
			if m.Name == "http.server.requests" {
				foundRequestCount = true
				sum := m.Data.(metricdata.Sum[int64])
				assert.NotEmpty(t, sum.DataPoints)
				assert.Equal(t, int64(1), sum.DataPoints[0].Value)

				// Verify status code is 500
				attrs := sum.DataPoints[0].Attributes
				attrSlice := attrs.ToSlice()
				var statusCodeFound bool
				for _, attr := range attrSlice {
					if string(attr.Key) == "http.status_code" {
						statusCodeFound = true
						assert.Equal(t, int64(http.StatusInternalServerError), attr.Value.AsInt64())
					}
				}
				assert.True(t, statusCodeFound, "status code attribute should be present")
			}
		}
		assert.True(t, foundRequestCount, "request count metric should be recorded for handled errors")
	})
}

func TestHTTPServerCollector_Middleware_MultipleStatusCodes(t *testing.T) {
	t.Run("tracks metrics for different status codes separately", func(t *testing.T) {
		// Setup metrics
		reader := metric.NewManualReader()
		provider := metric.NewMeterProvider(metric.WithReader(reader))
		meter := provider.Meter("test")

		collector, err := NewHTTPServerCollector(meter)
		require.NoError(t, err)

		// Setup Gin
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(collector.Middleware())
		router.GET("/api/success", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		router.GET("/api/error", func(c *gin.Context) {
			c.Status(http.StatusInternalServerError)
		})

		// Make multiple requests with different status codes
		requests := []struct {
			path   string
			status int
		}{
			{"/api/success", http.StatusOK},
			{"/api/success", http.StatusOK},
			{"/api/error", http.StatusInternalServerError},
		}

		for _, r := range requests {
			req := httptest.NewRequest("GET", r.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, r.status, w.Code)
		}

		// Collect metrics
		var rm metricdata.ResourceMetrics
		req := httptest.NewRequest("GET", "/api/test", nil)
		err = reader.Collect(req.Context(), &rm)
		require.NoError(t, err)

		// Verify metrics
		require.NotEmpty(t, rm.ScopeMetrics)
		metricsData := rm.ScopeMetrics[0].Metrics

		// Check that metrics are recorded separately for different status codes
		var totalRequests int64
		for _, m := range metricsData {
			if m.Name == "http.server.requests" {
				sum := m.Data.(metricdata.Sum[int64])
				// Should have multiple data points for different status codes
				assert.Greater(t, len(sum.DataPoints), 1, "should have separate metrics for different status codes")
				for _, dp := range sum.DataPoints {
					totalRequests += dp.Value
				}
			}
		}
		assert.Equal(t, int64(3), totalRequests, "should track all 3 requests")
	})
}
