package client

import "go.uber.org/fx"

var Module = fx.Module("http_client",
	fx.Provide(
		fx.Annotate(
			NewHTTPClient,
			fx.As(new(HTTPClientProvider)),
		),
		NewHTTPClientConfig,
		NewCircuitBreakerRegistry,
		NewCircuitBreakerRegistryConfig,
	),
)
