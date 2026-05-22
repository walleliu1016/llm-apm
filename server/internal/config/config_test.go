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

	cfg := Load()

	if cfg.Port != 14318 {
		t.Errorf("expected default Port=14318, got %d", cfg.Port)
	}
	// expandPath converts ~ to absolute path, so check suffix
	if !strings.HasSuffix(cfg.DataDir, "/.llm-apm") {
		t.Errorf("expected DataDir ending with /.llm-apm, got %s", cfg.DataDir)
	}
}