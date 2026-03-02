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

	return &c
}

func Load() *Config {
	return &c
}
