package docker

import (
	"github.com/egon12/pgsnap"
)

type (
	PostgresConfig struct {
		// DockerEndpoint is the endpoint to connect to docker
		DockerEndpoint string

		// MigrationPath is the path to the sql migration files
		MigrationPath string

		// DebugMode will print the logs of the container
		DebugMode bool

		// ExplicitWait is the flag to enable the explicit wait
		ExplicitWait bool

		// PostgresVersion is the version of postgres to use
		PostgresVersion string

		// ContainerNameSuffix is the suffix to add to the container name
		ContainerNameSuffix string

		// KeepContainer will keep the container when the container stop
		KeepContainer bool
	}

	Config struct {
		PostgresConfig
		pgsnap.Config
	}

	Options func(cfg *Config)
)

func WithDebug() Options {
	return func(cfg *Config) {
		cfg.PostgresConfig.DebugMode = true
		cfg.Debug = true
	}
}

func WithMigrationPath(path string) Options {
	return func(cfg *Config) {
		cfg.MigrationPath = path
	}
}
