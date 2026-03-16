package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr                 string
	DatabaseURL          string
	Timezone             string
	AutoMigrate          bool
	MigrationsDir        string
	ICSImportHorizonDays int
	MaxUploadSizeBytes   int64
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                 env("APP_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://todo:todo@localhost:5432/todo?sslmode=disable"),
		Timezone:             env("APP_TIMEZONE", "Asia/Shanghai"),
		AutoMigrate:          envBool("AUTO_MIGRATE", true),
		MigrationsDir:        env("MIGRATIONS_DIR", "db/migrations"),
		ICSImportHorizonDays: envInt("ICS_IMPORT_HORIZON_DAYS", 180),
		MaxUploadSizeBytes:   envInt64("MAX_UPLOAD_SIZE_BYTES", 4<<20),
	}

	if cfg.ICSImportHorizonDays < 1 {
		return Config{}, fmt.Errorf("ICS_IMPORT_HORIZON_DAYS must be at least 1")
	}
	if cfg.MaxUploadSizeBytes < 1024 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_SIZE_BYTES must be at least 1024")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
