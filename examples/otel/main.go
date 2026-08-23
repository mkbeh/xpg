package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/extra/otelxpg"
)

const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	defaultHTTPAddress = "localhost:9464"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) (runErr error) {
	metrics, err := newMetricsRuntime(ctx)
	if err != nil {
		return fmt.Errorf("initialize metrics: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		runErr = errors.Join(
			runErr,
			metrics.Shutdown(shutdownCtx),
		)
	}()

	config, err := pgxpool.ParseConfig(databaseURL())
	if err != nil {
		return fmt.Errorf("parse PostgreSQL config: %w", err)
	}

	// A small pool makes contention visible when POST /load runs.
	config.MaxConns = 2

	pool, err := xpg.New(
		ctx,
		config,
		xpg.WithName("otel-example"),
		xpg.WithLabel("xpg.pool.role", "primary"),
		xpg.WithMetrics(
			otelxpg.NewMetrics(),
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

	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("POST /load", loadHandler(pool))

	server := &http.Server{
		Addr:              httpAddress(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("OpenTelemetry example listening on http://%s", server.Addr)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func loadHandler(pool *xpg.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()

		if err := runWorkload(request.Context(), pool); err != nil {
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
			_, err := pool.Exec(ctx, "SELECT pg_sleep(1)")
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
