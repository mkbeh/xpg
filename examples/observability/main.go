package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/extra/otelxpg"
	"github.com/mkbeh/xpg/extra/slogxpg"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	defaultHTTPAddress = "localhost:9464"
)

func main() {
	logger := slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error(
			"observability example failed",
			slog.Any("error", err),
		)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) (runErr error) {
	resource, err := newOTelResource(ctx)
	if err != nil {
		return err
	}

	meterProvider, metricsHandler, err := newMeterProvider(resource)
	if err != nil {
		return fmt.Errorf("initialize metrics: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		runErr = errors.Join(
			runErr,
			meterProvider.Shutdown(shutdownCtx),
		)
	}()

	tracerProvider, err := newTracerProvider(resource)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		runErr = errors.Join(
			runErr,
			tracerProvider.Shutdown(shutdownCtx),
		)
	}()

	config, err := pgxpool.ParseConfig(databaseURL())
	if err != nil {
		return fmt.Errorf("parse PostgreSQL config: %w", err)
	}

	// A small pool makes contention visible when POST /load runs.
	config.MaxConns = 2

	pgxLogger := logger.With(
		slog.String("component", "pgx"),
	)

	pool, err := xpg.New(
		ctx,
		config,
		xpg.WithName("observability-example"),
		xpg.WithLabel("xpg.pool.role", "primary"),
		xpg.WithLogger(
			slogxpg.New(pgxLogger),
			tracelog.LogLevelInfo,
		),
		xpg.WithTracer(
			otelpgx.NewTracer(
				otelpgx.WithTracerProvider(tracerProvider),
				otelpgx.WithTrimSQLInSpanName(),
			),
		),
		xpg.WithMetrics(
			otelxpg.NewMetrics(
				otelxpg.WithMeterProvider(meterProvider),
			),
		),
	)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metricsHandler)
	mux.HandleFunc(
		"POST /load",
		loadHandler(
			pool,
			tracerProvider.Tracer(tracingInstrumentationName),
		),
	)

	server := &http.Server{
		Addr:              httpAddress(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info(
		"observability example listening",
		slog.String("address", "http://"+server.Addr),
	)

	if err := serveHTTP(ctx, server); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

func serveHTTP(ctx context.Context, server *http.Server) error {
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func loadHandler(
	pool *xpg.Pool,
	tracer trace.Tracer,
) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		ctx, span := tracer.Start(
			request.Context(),
			"run-load",
		)
		defer span.End()

		startedAt := time.Now()

		if err := runWorkload(ctx, pool); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "workload failed")

			http.Error(
				w,
				fmt.Sprintf("run workload: %v", err),
				http.StatusInternalServerError,
			)

			return
		}

		_, _ = fmt.Fprintf(
			w,
			"workload completed in %s\n",
			time.Since(startedAt),
		)
	}
}

func runWorkload(ctx context.Context, pool *xpg.Pool) error {
	const workerCount = 6

	start := make(chan struct{})
	results := make(chan error, workerCount)

	for range workerCount {
		go func() {
			<-start

			_, err := pool.Exec(
				ctx,
				"SELECT pg_sleep(1)",
			)
			results <- err
		}()
	}

	close(start)

	var workloadErr error

	for range workerCount {
		workloadErr = errors.Join(
			workloadErr,
			<-results,
		)
	}

	return workloadErr
}

func databaseURL() string {
	if value := os.Getenv("XPG_DATABASE_URL"); value != "" {
		return value
	}

	return defaultDatabaseURL
}

func httpAddress() string {
	if value := os.Getenv("HTTP_ADDR"); value != "" {
		return value
	}

	return defaultHTTPAddress
}
