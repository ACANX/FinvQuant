// Package config 加载服务端配置（环境变量驱动，默认值适配 Docker Compose）。
package config

import (
	"os"
)

// Config 服务端运行配置。
type Config struct {
	// Server 监听地址与端口（默认 0.0.0.0:16001）
	Host string
	Port string

	// PostgreSQL 18
	PgHost     string
	PgPort     string
	PgUser     string
	PgPassword string
	PgDatabase string

	// Redis 8
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Gin 运行模式：debug / release
	Mode string
}

// Load 从环境变量加载配置，缺失时使用默认值。
func Load() *Config {
	return &Config{
		Host:         envOr("FINV_SERVER_HOST", "0.0.0.0"),
		Port:         envOr("FINV_SERVER_PORT", "16001"),
		PgHost:       envOr("FINV_PG_HOST", "localhost"),
		PgPort:       envOr("FINV_PG_PORT", "5432"),
		PgUser:       envOr("FINV_PG_USER", "finv"),
		PgPassword:   envOr("FINV_PG_PASSWORD", "finvquant"),
		PgDatabase:   envOr("FINV_PG_DATABASE", "finv"),
		RedisAddr:    envOr("FINV_REDIS_ADDR", "localhost:6379"),
		RedisPassword: envOr("FINV_REDIS_PASSWORD", ""),
		RedisDB:      envIntOr("FINV_REDIS_DB", 0),
		Mode:         envOr("FINV_MODE", "release"),
	}
}

// ListenAddr 返回服务端监听地址。
func (c *Config) ListenAddr() string {
	return c.Host + ":" + c.Port
}

// WebListenAddr 返回前端监听地址（默认 0.0.0.0:16002）。
func (c *Config) WebListenAddr() string {
	return c.Host + ":" + envOr("FINV_WEB_PORT", "16002")
}

// MigrationsDir 返回数据库迁移目录（默认 Deploy/Migrations）。
func (c *Config) MigrationsDir() string {
	return envOr("FINV_MIGRATIONS_DIR", "Deploy/Migrations")
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var parsed int
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		parsed = parsed*10 + int(r-'0')
	}
	return parsed
}
