package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/koungkub/fw-challenge-notification-service/internal/metrics"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

//go:generate mockgen -package mockclient -destination ./mock/mockclient.go . HTTPClientProvider
type HTTPClientProvider interface {
	Post(ctx context.Context, u string, reqBody NotificationRequest) error
}

var _ HTTPClientProvider = (*HTTPClient)(nil)

type HTTPClient struct {
	httpclient             *http.Client
	circuitBreakerRegistry *CircuitBreakerRegistry
	metricsCollector       *metrics.HTTPClientCollector
	logger                 *zap.Logger
}

type HTTPClientConfig struct {
	Timeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"5s"`
}

type HTTPClientParams struct {
	fx.In

	Config                 HTTPClientConfig
	CircuitBreakerRegistry *CircuitBreakerRegistry
	MetricsCollector       *metrics.HTTPClientCollector
	Logger                 *zap.Logger
}

func NewHTTPClient(params HTTPClientParams) *HTTPClient {
	return &HTTPClient{
		httpclient: &http.Client{
			Timeout: params.Config.Timeout,
		},
		circuitBreakerRegistry: params.CircuitBreakerRegistry,
		metricsCollector:       params.MetricsCollector,
		logger:                 params.Logger,
	}
}

func NewHTTPClientConfig() HTTPClientConfig {
	var cfg HTTPClientConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}

func (c *HTTPClient) Post(ctx context.Context, u string, reqBody NotificationRequest) error {
	start := time.Now()

	host, err := extractHost(u)
	if err != nil {
		c.logger.Error("failed to extract host from URL",
			zap.String("url", u),
			zap.Error(err),
		)
		return err
	}

	circuitBreaker := c.circuitBreakerRegistry.GetOrCreate(host)

	cbState := circuitBreaker.State().String()
	c.metricsCollector.RecordCircuitBreakerState(ctx, host, cbState)

	c.logger.Debug("circuit breaker state checked",
		zap.String("host", host),
		zap.String("state", cbState),
	)

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		c.logger.Error("failed to marshal request body",
			zap.String("host", host),
			zap.Error(err),
		)
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u,
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		c.logger.Error("failed to create HTTP request",
			zap.String("host", host),
			zap.Error(err),
		)
		return err
	}

	resp, err := circuitBreaker.Execute(func() (CircuitBreakerResponse, error) {
		resp, err := c.httpclient.Do(req)
		if err != nil {
			c.logger.Warn("HTTP request failed",
				zap.String("host", host),
				zap.Error(err),
			)
			return CircuitBreakerResponse{}, err
		}
		defer resp.Body.Close()

		rawBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Error("failed to read response body",
				zap.String("host", host),
				zap.Int("status_code", resp.StatusCode),
				zap.Error(err),
			)
			return CircuitBreakerResponse{}, err
		}

		return CircuitBreakerResponse{
			Body:       rawBody,
			StatusCode: resp.StatusCode,
		}, nil
	})

	duration := time.Since(start)
	statusCode := 0
	var finalErr error

	if err != nil {
		finalErr = err
		c.metricsCollector.RecordRequest(ctx, http.MethodPost, host, statusCode, duration, finalErr)
		c.logger.Error("circuit breaker execution failed",
			zap.String("host", host),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return err
	}

	statusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		finalErr = errors.New("response status code not equal 200")
		c.metricsCollector.RecordRequest(ctx, http.MethodPost, host, statusCode, duration, finalErr)
		c.logger.Warn("received non-200 status code",
			zap.String("host", host),
			zap.Int("status_code", statusCode),
			zap.Duration("duration", duration),
		)
		return finalErr
	}

	c.metricsCollector.RecordRequest(ctx, http.MethodPost, host, statusCode, duration, nil)

	return nil
}

func extractHost(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}
