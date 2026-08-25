# OpenTelemetry Metrics for xpg

`otelxpg` provides optional OpenTelemetry metrics integration for `xpg`.

The package is exporter-agnostic: applications own the OpenTelemetry SDK lifecycle and exporter configuration, while
`otelxpg` uses the configured `MeterProvider` to expose connection pool metrics.

## Installation

```bash
go get github.com/mkbeh/xpg/extra/otelxpg
```

## Usage

<!-- @formatter:off -->
```go
import (
	"context"

	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/extra/otelxpg"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

meterProvider := sdkmetric.NewMeterProvider()
defer meterProvider.Shutdown(context.Background())

// Create a reusable OpenTelemetry metrics integration.
metrics := otelxpg.NewMetrics(
	otelxpg.WithMeterProvider(meterProvider),
)

// Attach metrics when creating the pool.
pool, _ := xpg.Open(
	context.Background(),
	databaseURL,
	xpg.WithName("example-pool"),
	xpg.WithMetrics(metrics),
)
defer pool.Close()
```
<!-- @formatter:on -->

For a complete runnable setup using OpenTelemetry and Prometheus, see the
[observability example](../../examples/observability).