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
}

type HTTPClientConfig struct {
	Timeout time.Duration `envconfig:"HTTP_CLIENT_TIMEOUT" default:"5s"`
}

type HTTPClientParams struct {
	fx.In

	Config                 HTTPClientConfig
	CircuitBreakerRegistry *CircuitBreakerRegistry
}

func NewHTTPClient(params HTTPClientParams) *HTTPClient {
	return &HTTPClient{
		httpclient: &http.Client{
			Timeout: params.Config.Timeout,
		},
		circuitBreakerRegistry: params.CircuitBreakerRegistry,
	}
}

func NewHTTPClientConfig() HTTPClientConfig {
	var cfg HTTPClientConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}

func (c *HTTPClient) Post(ctx context.Context, u string, reqBody NotificationRequest) error {
	host, err := extractHost(u)
	if err != nil {
		return err
	}

	circuitBreaker := c.circuitBreakerRegistry.GetOrCreate(host)

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {

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
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("response status code not equal 200")
	}

	return nil
}

func extractHost(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}
