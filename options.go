package xpg

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"strconv"
	"strings"
)

// Option configures a Pool.
//
// The interface is sealed so options can only be created by this package.
type Option interface {
	apply(*settings) error
}

type optionFunc func(*settings) error

func (option optionFunc) apply(settings *settings) error {
	return option(settings)
}

type settings struct {
	name    string
	labels  map[string]string
	metrics Metrics
}

func (s settings) poolName(host string, port uint16, database string) string {
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

// WithLabels merges labels into the pool metadata.
//
// Labels are defensively copied. When the same key is configured more than
// once, the last value wins.
func WithLabels(labels map[string]string) Option {
	labels = cloneLabels(labels)

	return optionFunc(func(settings *settings) error {
		for key, value := range labels {
			key = strings.TrimSpace(key)
			if key == "" {
				return errors.New("label key must not be blank")
			}

			settings.labels[key] = value
		}

		return nil
	})
}

// WithLabel adds or replaces one pool label.
func WithLabel(key, value string) Option {
	key = strings.TrimSpace(key)

	return optionFunc(func(settings *settings) error {
		if key == "" {
			return errors.New("label key must not be blank")
		}

		settings.labels[key] = value

		return nil
	})
}

// WithMetrics attaches one metrics implementation to the pool.
//
// Metrics are registered during New and unregistered automatically when the
// Pool is closed.
func WithMetrics(metrics Metrics) Option {
	return optionFunc(func(settings *settings) error {
		if metrics == nil {
			return errors.New("pool metrics is nil")
		}

		settings.metrics = metrics

		return nil
	})
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(labels))
	maps.Copy(cloned, labels)

	return cloned
}
