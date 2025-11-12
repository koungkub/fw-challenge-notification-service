package metrics

import "go.uber.org/fx"

var Module = fx.Module("metric",
	fx.Provide(
		NewMeterProvider,
		NewMetric,
		NewMetricConfig,
	),
	httpCollectorModule,
	httpclientCollectorModule,
)

var httpCollectorModule = fx.Provide(
	NewHTTPServerCollector,
)

var httpclientCollectorModule = fx.Provide(
	NewHTTPClientCollector,
)
