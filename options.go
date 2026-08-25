package xpg

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/tracelog"
)

// Option configures a Pool.
//
// The interface is sealed so options can only be created by this package.
type Option interface {
	apply(*settings) error
}

// WithName assigns a stable logical name to the pool.
//
// Name is metadata for diagnostics and observability. It does not change the
// PostgreSQL application_name runtime parameter.
func WithName(name string) Option {
	name = strings.TrimSpace(name)

	return optionFunc(func(settings *settings) error {
		if name == "" {
			return errors.New("pool name must not be blank")
		}

		settings.name = name

		return nil
	})
}

// WithLabel adds or replaces one pool label.
func WithLabel(key, value string) Option {
	return optionFunc(func(settings *settings) error {
		if key == "" {
			return errors.New("label key must not be empty")
		}

		settings.labels[key] = value

		return nil
	})
}

// WithLabels merges labels into the pool metadata.
//
// Labels are defensively copied. When the same key is configured more than
// once, the last value wins.
func WithLabels(labels map[string]string) Option {
	labels = cloneLabels(labels)

	return optionFunc(func(settings *settings) error {
		for key, value := range labels {
			if key == "" {
				return errors.New("label key must not be empty")
			}

			settings.labels[key] = value
		}

		return nil
	})
}

// WithLogger attaches a pgx-compatible logger to the pool.
//
// Logging uses pgx tracelog and participates in the same tracing pipeline as
// tracers configured through xpg. pgx tracelog may include SQL text and query
// arguments in log records; applications are responsible for choosing an
// appropriate level and handling sensitive values.
func WithLogger(logger tracelog.Logger, level tracelog.LogLevel) Option {
	return optionFunc(func(settings *settings) error {
		if logger == nil {
			return errors.New("pool logger is nil")
		}

		settings.tracers = append(settings.tracers, &tracelog.TraceLog{
			Logger:   logger,
			LogLevel: level,
		})

		return nil
	})
}

// WithTracer attaches one pgx query tracer to the pool.
//
// The option may be specified multiple times. Configured loggers and tracers
// are combined through pgx multitracer. When xpg logging or tracing options are
// configured, the resulting tracing pipeline replaces any tracer already
// configured on the pgx connection config.
func WithTracer(tracer pgx.QueryTracer) Option {
	return WithTracers(tracer)
}

// WithTracers attaches multiple pgx query tracers to the pool.
//
// Configured loggers and tracers are invoked in configuration order and
// combined through pgx multitracer. When xpg logging or tracing options are
// configured, the resulting tracing pipeline replaces any tracer already
// configured on the pgx connection config.
func WithTracers(tracers ...pgx.QueryTracer) Option {
	return optionFunc(func(settings *settings) error {
		for _, tracer := range tracers {
			if tracer == nil {
				return errors.New("pool tracer is nil")
			}
		}

		settings.tracers = append(settings.tracers, tracers...)

		return nil
	})
}

// WithMetrics attaches one metrics implementation to the pool.
//
// Metrics are registered when the pool is created and unregistered
// automatically when the Pool is closed.
func WithMetrics(metrics Metrics) Option {
	return optionFunc(func(settings *settings) error {
		if metrics == nil {
			return errors.New("pool metrics is nil")
		}

		settings.metrics = metrics

		return nil
	})
}

type optionFunc func(*settings) error

func (option optionFunc) apply(settings *settings) error {
	return option(settings)
}

type settings struct {
	name    string
	labels  map[string]string
	metrics Metrics
	tracers []pgx.QueryTracer
}

func defaultSettings() *settings {
	return &settings{
		labels: make(map[string]string),
	}
}

func applyOptions(settings *settings, opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			return errors.New("xpg: option is nil")
		}

		if err := opt.apply(settings); err != nil {
			return fmt.Errorf("xpg: apply option: %w", err)
		}
	}

	return nil
}

func (s *settings) poolName(host string, port uint16, database string) string {
	if s.name != "" {
		return s.name
	}

	address := net.JoinHostPort(
		host,
		strconv.Itoa(int(port)),
	)
	if database == "" {
		return address
	}

	return address + "/" + database
}

func (s *settings) buildTracer() pgx.QueryTracer {
	if len(s.tracers) == 0 {
		return nil
	}

	return multitracer.New(s.tracers...)
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	return maps.Clone(labels)
}
