package metrics

import "go.uber.org/fx"

var Module = fx.Module("metric",
	fx.Provide(
		NewMeterProvider,
		NewMetric,
		NewMetricConfig,
	),
	httpCollectorModule,
)

var (
	httpCollectorModule = fx.Provide(
		NewHTTPServerCollector,
	)
)
