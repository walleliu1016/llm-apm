package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all server configuration.
type Config struct {
	Host              string
	Port              int
	DataDir           string
	GreptimeHTTPPort  int
	GreptimeGRPCPort  int
	GreptimeMySQLPort int
	LogLevel          string
	DataTTL           string
	TenantID          string // 预留：多租户扩展
}

// Load reads configuration from environment variables.
func Load() *Config {
	return &Config{
		Host:              getEnv("APM_HOST", "127.0.0.1"),
		Port:              getEnvInt("APM_PORT", 14318),
		DataDir:           expandPath(getEnv("APM_DATA_DIR", "~/.llm-apm")),
		GreptimeHTTPPort:  getEnvInt("APM_GREPTIMEDB_HTTP_PORT", 14000),
		GreptimeGRPCPort:  getEnvInt("APM_GREPTIMEDB_GRPC_PORT", 14001),
		GreptimeMySQLPort: getEnvInt("APM_GREPTIMEDB_MYSQL_PORT", 14002),
		LogLevel:          getEnv("APM_LOG_LEVEL", "info"),
		DataTTL:           getEnv("APM_DATA_TTL", "60d"),
		TenantID:          getEnv("APM_TENANT_ID", ""), // 预留字段
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}