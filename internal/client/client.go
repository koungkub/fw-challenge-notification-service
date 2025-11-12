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
}

type HTTPClientConfig struct {
	Timeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"5s"`
}

type HTTPClientParams struct {
	fx.In

	Config                 HTTPClientConfig
	CircuitBreakerRegistry *CircuitBreakerRegistry
	MetricsCollector       *metrics.HTTPClientCollector
}

func NewHTTPClient(params HTTPClientParams) *HTTPClient {
	return &HTTPClient{
		httpclient: &http.Client{
			Timeout: params.Config.Timeout,
		},
		circuitBreakerRegistry: params.CircuitBreakerRegistry,
		metricsCollector:       params.MetricsCollector,
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
		return err
	}

	circuitBreaker := c.circuitBreakerRegistry.GetOrCreate(host)

	cbState := circuitBreaker.State().String()
	c.metricsCollector.RecordCircuitBreakerState(ctx, host, cbState)

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u,
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return err
	}

	resp, err := circuitBreaker.Execute(func() (CircuitBreakerResponse, error) {
		resp, err := c.httpclient.Do(req)
		if err != nil {
			return CircuitBreakerResponse{}, err
		}
		defer resp.Body.Close()

		rawBody, err := io.ReadAll(resp.Body)
		if err != nil {
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
		return err
	}

	statusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		finalErr = errors.New("response status code not equal 200")
		c.metricsCollector.RecordRequest(ctx, http.MethodPost, host, statusCode, duration, finalErr)
		return finalErr
	}

	// Record successful request
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
