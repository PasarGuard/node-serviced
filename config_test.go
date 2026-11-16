package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTLSBackendConfiguration(t *testing.T) {
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "absent.env"))
	t.Setenv("API_KEY", "test-key")
	t.Setenv("SSL_CERT_FILE", "/does/not/exist")
	t.Setenv("SSL_KEY_FILE", "/does/not/exist")
	t.Setenv("TLS_ENABLED", "false")
	t.Setenv("NODE_HOST", "127.0.0.1")
	os.Unsetenv("NODE_HOST")
	cfg, err := loadConfig()
	if err != nil || cfg.NodeHost != "127.0.0.1" || cfg.TLSEnabled {
		t.Fatalf("backend defaults: %+v %v", cfg, err)
	}
	for _, host := range []string{"127.0.0.2", "::1", "10.1.2.3", "172.16.1.1", "192.168.1.1", "fd00::1", "::ffff:192.168.1.1"} {
		t.Setenv("NODE_HOST", host)
		if _, err := loadConfig(); err != nil {
			t.Errorf("rejected %s: %v", host, err)
		}
	}
	for _, host := range []string{"", "localhost", "invalid", "0.0.0.0", "::", "8.8.8.8", "169.254.1.1", "100.64.0.1", "192.0.2.1", "224.0.0.1", "fe80::1", "2001:db8::1", "ff02::1", "::ffff:8.8.8.8", "fd00::1%eth0"} {
		t.Setenv("NODE_HOST", host)
		if _, err := loadConfig(); err == nil {
			t.Errorf("accepted unsafe %s", host)
		}
	}
	t.Setenv("NODE_HOST", "127.0.0.1")
	for _, value := range []string{"", "no", "disabled"} {
		t.Setenv("TLS_ENABLED", value)
		if _, err := loadConfig(); err == nil {
			t.Errorf("accepted invalid TLS_ENABLED=%q", value)
		}
	}
	t.Setenv("TLS_ENABLED", "true")
	if _, err := loadConfig(); err == nil {
		t.Fatal("TLS must still require certificate files")
	}
	os.Unsetenv("TLS_ENABLED")
	if cfg, err := loadConfig(); !cfg.TLSEnabled || err == nil {
		t.Fatal("missing TLS_ENABLED must require TLS")
	}
}
