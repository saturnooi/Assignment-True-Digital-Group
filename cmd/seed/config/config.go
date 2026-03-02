package config

import "github.com/acoshift/configfile"

type Config struct {
	DatabaseURL string
}

var r = configfile.NewEnvReader()

var c Config

func Init() *Config {
	configfile.LoadDotEnv()

	c.DatabaseURL = r.StringDefault("DATABASE_URL", "postgres://user:password@postgres:5432/recommendations?sslmode=disable")

	return &c
}

func Load() *Config {
	return &c
}
