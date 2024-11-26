package postgres

import (
	"embed"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// An Option lets you add opts to pberrors interceptors using With* funcs.
type Option interface {
	apply(p *Pool)
}

type optionFunc func(p *Pool)

func (f optionFunc) apply(p *Pool) {
	f(p)
}

func WithLogger(l *slog.Logger) Option {
	return optionFunc(func(p *Pool) {
		if l != nil {
			p.logger = l
		}
	})
}

func WithConfig(config *Config) Option {
	return optionFunc(func(p *Pool) {
		if config != nil {
			p.cfg = config
		}
	})
}

func WithClientID(id string) Option {
	return optionFunc(func(p *Pool) {
		if id != "" {
			p.id = id
		}
	})
}

func WithTraceProvider(provider trace.TracerProvider) Option {
	return optionFunc(func(p *Pool) {
		p.traceProvider = provider
	})
}

func WithMigrations(migrations ...embed.FS) Option {
	return optionFunc(func(p *Pool) {
		if len(migrations) > 0 {
			p.migrations = migrations
		}
	})
}
