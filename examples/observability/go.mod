module observability

go 1.27

require (
	github.com/exaring/otelpgx v0.11.1
	github.com/jackc/pgx/v5 v5.10.0
	github.com/mkbeh/xpg v0.2.0
	github.com/mkbeh/xpg/extra/otelxpg v0.1.0
	github.com/mkbeh/xpg/extra/slogxpg v0.1.0
	github.com/prometheus/client_golang v1.24.1
	go.opentelemetry.io/otel v1.45.0
	go.opentelemetry.io/otel/exporters/prometheus v0.68.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.46.0
	go.opentelemetry.io/otel/sdk v1.46.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.45.0
)
