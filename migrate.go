package postgres

import (
	"context"
	"embed"
	"errors"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // init
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func applyMigrations(fs embed.FS, dsn string, l *slog.Logger) (err error) {
	var src source.Driver
	ctx := context.Background()

	src, err = iofs.New(fs, ".")
	if err != nil {
		err = errors.Join(errors.New("embed.FS init failed"), err)
		return err
	}
	defer func() {
		if ce := src.Close(); ce != nil {
			err = errors.Join(err, ce)
		}
	}()

	var instance *migrate.Migrate
	instance, err = migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		err = errors.Join(errors.New("db instance init failed"), err)
		return err
	}
	defer func() {
		if sErr, ie := instance.Close(); sErr != nil || ie != nil {
			err = errors.Join(err, sErr, ie)
		}
	}()

	if mErr := instance.Up(); mErr != nil && !errors.Is(mErr, migrate.ErrNoChange) {
		err = errors.Join(errors.New("migrate-up failed"), mErr)
	} else {
		ver, dirty, _ := instance.Version()
		l.InfoContext(ctx, "migrate-up done",
			slog.Any("version", ver),
			slog.Any("dirty", dirty))
	}

	return err
}
