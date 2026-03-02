package config

import "github.com/acoshift/configfile"

type Config struct {
	Port          string
	DatabaseURL   string
	RedisURL      string
	RedisPassword string
	RedisDB       int
}

var r = configfile.NewEnvReader()

var c Config

func Init() *Config {
	configfile.LoadDotEnv()

	c.Port = r.StringDefault("PORT", "8080")
	c.DatabaseURL = r.StringDefault("DATABASE_URL", "postgres://user:password@postgres:5432/recommendations?sslmode=disable")
	c.RedisURL = r.StringDefault("REDIS_URL", "localhost:6379")
	c.RedisPassword = r.StringDefault("REDIS_PASSWORD", "")
	c.RedisDB = r.IntDefault("REDIS_DB", 10)

	return &c
}

func Load() *Config {
	return &c
}
