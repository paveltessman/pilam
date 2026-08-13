// Package config is pilam's typed configuration: every environment variable
// the application reads is declared, parsed and validated here, once, at boot.

package config

import (
	"log/slog"
	"os"
	"time"

	// time.LoadLocation reads the host's zoneinfo database, which minimal
	// container images do not ship. Rather than depend on the image,
	// the database travels inside the binary. Costs ~450KB.
	_ "time/tzdata"
)

// Defaults
const (
	defaultHTTPAddr      = ":8080"
	defaultShutdownGrace = "10s"
	defaultDatabaseURL   = "postgres://pilam:pilam@localhost:5433/pilam?sslmode=disable"
	defaultMediaDir      = "tmp/media"
	defaultTimezone      = "Europe/Moscow"
)

type Config struct {
	HTTP     HTTP
	Database Database
	Media    Media
	Timezone *time.Location
}

type HTTP struct {
	// Addr is the listen address, in host:port form. Inside compose this stays
	// at the default.
	Addr string

	// ShutdownGrace bounds how long in-flight requests have to finish after a
	// signal arrives, before the process exits anyway.
	ShutdownGrace time.Duration
}

type Database struct {
	// URL is a libpq/pgx connection URL. Carries the password, so it is never
	// logged directly — see Config.LogValue.
	URL string
}

// Media configures blob storage for colorway photos.
type Media struct {
	// Dir is the path of the directory uploads are written to. Backed
	// by a named volume under compose.
	Dir string
}

// Build the configuration from the process environment
func Load() (Config, error) {
	return load(os.Getenv)
}

/*
load is Load with the environment injected, so tests can describe a whole
environment as a map instead of mutating the process's.
*/
func load(getenv func(string) string) (Config, error) {
	loader := &loader{getenv: getenv}

	http := HTTP{
		Addr:          value(loader, "HTTP_ADDR", defaultHTTPAddr, parseListenAddr),
		ShutdownGrace: value(loader, "HTTP_SHUTDOWN_GRACE", defaultShutdownGrace, parsePositiveTimeDuration),
	}

	db := Database{
		URL: value(loader, "DATABASE_URL", defaultDatabaseURL, parsePostgresURL),
	}

	media := Media{
		Dir: value(loader, "MEDIA_DIR", defaultMediaDir, parseDir),
	}

	cfg := Config{
		HTTP:     http,
		Database: db,
		Media:    media,
		Timezone: value(loader, "BUSINESS_TZ", defaultTimezone, parseTimezone),
	}

	if err := loader.err(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// LogValue renders the configuration for slog with the database password removed.
func (c Config) LogValue() slog.Value {
	v := slog.GroupValue(
		slog.String("http_addr", c.HTTP.Addr),
		slog.Duration("http_shutdown_grace", c.HTTP.ShutdownGrace),
		slog.String("database_url", redactURL(c.Database.URL)),
		slog.String("media_dir", c.Media.Dir),
		slog.String("timezone", c.Timezone.String()),
	)
	return v
}
