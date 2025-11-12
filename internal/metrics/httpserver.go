package metrics

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type HTTPServerCollector struct {
	requestCount    metric.Int64Counter
	requestDuration metric.Float64Histogram
}

func NewHTTPServerCollector(meter metric.Meter) (*HTTPServerCollector, error) {
	requestCount, err := meter.Int64Counter(
		"http.server.requests",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPServerCollector{
		requestCount:    requestCount,
		requestDuration: requestDuration,
	}, nil
}

func (m *HTTPServerCollector) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		attrs := []attribute.KeyValue{
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", path),
			attribute.Int("http.status_code", statusCode),
		}

		ctx := c.Request.Context()

		m.requestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}
