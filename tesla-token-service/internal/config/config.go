package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Debug    bool           `json:"debug"`
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
	Tesla    TeslaConfig    `json:"tesla"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type JWTConfig struct {
	Secret string `json:"secret"`
	TTL    int    `json:"ttl"` // seconds
}

type TeslaConfig struct {
	BaseURL string `json:"base_url"`
	Timeout int    `json:"timeout"` // seconds
}

func Load() (*Config, error) {
	cfg := &Config{
		Debug: getEnvBool("DEBUG", false),
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
			TTL:    getEnvInt("JWT_TTL", 86400), // 24 hours
		},
		Tesla: TeslaConfig{
			BaseURL: getEnv("TESLA_BASE_URL", "https://owner-api.teslamotors.com"),
			Timeout: getEnvInt("TESLA_TIMEOUT", 30),
		},
	}

	// 验证必需的配置
	if cfg.Database.User == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	if cfg.Database.Name == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}