package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set test env vars
	os.Setenv("APM_PORT", "18080")
	os.Setenv("APM_DATA_DIR", "/tmp/test-apm")
	defer os.Unsetenv("APM_PORT")
	defer os.Unsetenv("APM_DATA_DIR")

	cfg := Load()

	if cfg.Port != 18080 {
		t.Errorf("expected Port=18080, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/test-apm" {
		t.Errorf("expected DataDir=/tmp/test-apm, got %s", cfg.DataDir)
	}
}

func TestDefaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("APM_PORT")
	os.Unsetenv("APM_DATA_DIR")
	os.Unsetenv("APM_GREPTIMEDB_HOST")
	os.Unsetenv("APM_GREPTIMEDB_EMBEDDED")

	cfg := Load()

	if cfg.Port != 14318 {
		t.Errorf("expected default Port=14318, got %d", cfg.Port)
	}
	// expandPath converts ~ to absolute path, so check suffix
	if !strings.HasSuffix(cfg.DataDir, "/.llm-apm") {
		t.Errorf("expected DataDir ending with /.llm-apm, got %s", cfg.DataDir)
	}
	if cfg.GreptimeDBHost != "127.0.0.1" {
		t.Errorf("expected default GreptimeDBHost=127.0.0.1, got %s", cfg.GreptimeDBHost)
	}
	if cfg.GreptimeEmbedded != true {
		t.Errorf("expected default GreptimeEmbedded=true, got %v", cfg.GreptimeEmbedded)
	}
}

func TestRemoteGreptimeDB(t *testing.T) {
	// Set remote GreptimeDB config
	os.Setenv("APM_GREPTIMEDB_HOST", "192.168.1.100")
	os.Setenv("APM_GREPTIMEDB_EMBEDDED", "false")
	defer os.Unsetenv("APM_GREPTIMEDB_HOST")
	defer os.Unsetenv("APM_GREPTIMEDB_EMBEDDED")

	cfg := Load()

	if cfg.GreptimeDBHost != "192.168.1.100" {
		t.Errorf("expected GreptimeDBHost=192.168.1.100, got %s", cfg.GreptimeDBHost)
	}
	if cfg.GreptimeEmbedded != false {
		t.Errorf("expected GreptimeEmbedded=false, got %v", cfg.GreptimeEmbedded)
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		envValue string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", false}, // default is false when key not set
	}

	for _, tt := range tests {
		if tt.envValue != "" {
			os.Setenv("TEST_BOOL", tt.envValue)
		} else {
			os.Unsetenv("TEST_BOOL")
		}
		result := getEnvBool("TEST_BOOL", false)
		if result != tt.expected {
			t.Errorf("getEnvBool(%s) expected %v, got %v", tt.envValue, tt.expected, result)
		}
	}
	os.Unsetenv("TEST_BOOL")
}