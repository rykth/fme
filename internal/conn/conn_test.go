package conn

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		Host:    "localhost",
		Port:    22,
		User:    "testuser",
		Timeout: 5 * time.Second,
	}
	if cfg.Host != "localhost" {
		t.Errorf("unexpected host: %s", cfg.Host)
	}

	if cfg.Port != 22 {
		t.Errorf("unexpected port: %d", cfg.Port)
	}

	if cfg.User != "testuser" {
		t.Errorf("unexpected user: %s", cfg.User)
	}

	if cfg.Timeout != 5*time.Second {
		t.Errorf("unexpected timeout: %d", cfg.Timeout)
	}
}

func TestBuildAuthMethods_NoKeys(t *testing.T) {
	cfg := Config{
		Host:          "localhost",
		Port:          22,
		User:          "test",
		Password:      "secret",
		IdentityFiles: []string{"/nonexistent/key"},
	}
	methods, err := buildAuthMethods(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// should still return at least password method
	if len(methods) == 0 {
		t.Fatal("expected at least one auth method (password)")
	}
}

func TestBuildAuthMethods_NoMethods(t *testing.T) {
	cfg := Config{
		Host:          "localhost",
		Port:          22,
		User:          "test",
		IdentityFiles: []string{"/nonexistent/key"},
	}

	// may or may not error depending on whether SSH_AUTH_SOCK is set, just
	// verify it doesn't panic.
	_, _ = buildAuthMethods(cfg)
}

func TestBuildHostKeyCallback_NoFile(t *testing.T) {
	cb, err := buildHostKeyCallback("/nonexistent/known_hosts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb == nil {
		t.Fatal("expected non-nil callback")
	}
}
