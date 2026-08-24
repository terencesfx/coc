package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr                string
	DatabasePath            string
	BackupDir               string
	AssetDir                string
	OfficialOccupationsPath string
	CustomOccupationsPath   string
	CookieSecure            bool
}

func Load() Config {
	return Config{
		HTTPAddr:                envOrDefault("COC_HTTP_ADDR", ":8080"),
		DatabasePath:            envOrDefault("COC_DATABASE_PATH", ".data/coc.db"),
		BackupDir:               envOrDefault("COC_BACKUP_DIR", ".data/backups"),
		AssetDir:                envOrDefault("COC_ASSET_DIR", ".data/assets"),
		OfficialOccupationsPath: envOrDefault("COC_OFFICIAL_OCCUPATIONS_PATH", "data/rules/coc7/occupations.official.json"),
		CustomOccupationsPath:   envOrDefault("COC_CUSTOM_OCCUPATIONS_PATH", ".data/rules/coc7/occupations.custom.json"),
		CookieSecure:            strings.EqualFold(os.Getenv("COC_COOKIE_SECURE"), "true"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
