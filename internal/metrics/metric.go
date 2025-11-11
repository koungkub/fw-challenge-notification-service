package metrics

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
)

func NewMeterProvider() (*sdkmetric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(provider)
	return provider, nil
}

type MetricParams struct {
	fx.In

	Config        MetricConfig
	MeterProvider *sdkmetric.MeterProvider
}

func NewMetric(lc fx.Lifecycle, params MetricParams) (metric.Meter, error) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return params.MeterProvider.Shutdown(ctx)
		},
	})

	return params.MeterProvider.Meter(params.Config.AppName), nil
}

type MetricConfig struct {
	AppName string `envconfig:"APP_NAME" default:"myapp"`
}

func NewMetricConfig() MetricConfig {
	var cfg MetricConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}
