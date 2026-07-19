package lsky

import (
	"strings"
	"testing"
)

func TestLoadFromEnv_FlagOverridesEnv(t *testing.T) {
	t.Setenv("ITB_LSKY_URL", "env-url")
	t.Setenv("ITB_LSKY_TOKEN", "env-token")
	cfg := Config{BaseURL: "flag-url", Token: "flag-token"}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "flag-url" || cfg.Token != "flag-token" {
		t.Fatalf("flag should override env, got %+v", cfg)
	}
}

func TestLoadFromEnv_ReadsNewEnv(t *testing.T) {
	t.Setenv("ITB_LSKY_URL", "env-url")
	t.Setenv("ITB_LSKY_TOKEN", "env-token")
	cfg := Config{}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "env-url" || cfg.Token != "env-token" {
		t.Fatalf("got %+v", cfg)
	}
}

func TestLoadFromEnv_IgnoresOldEnv(t *testing.T) {
	t.Setenv("LSKY_URL", "old-url")
	t.Setenv("LSKY_TOKEN", "old-token")
	t.Setenv("ITB_LSKY_URL", "")
	t.Setenv("ITB_LSKY_TOKEN", "")
	cfg := Config{}
	cfg.LoadFromEnv()
	if cfg.BaseURL != "" || cfg.Token != "" {
		t.Fatalf("old env names must not be read, got %+v", cfg)
	}
}

func TestValidate_MissingURL(t *testing.T) {
	cfg := Config{Token: "t"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ITB_LSKY_URL") {
		t.Fatalf("expected error mentioning ITB_LSKY_URL, got %v", err)
	}
}

func TestValidate_MissingToken(t *testing.T) {
	cfg := Config{BaseURL: "u"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ITB_LSKY_TOKEN") {
		t.Fatalf("expected error mentioning ITB_LSKY_TOKEN, got %v", err)
	}
}

func TestValidate_OK(t *testing.T) {
	cfg := Config{BaseURL: "u", Token: "t"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"根地址", "https://img.example.com", "https://img.example.com/api/v1"},
		{"已含 /api/v1", "https://img.example.com/api/v1", "https://img.example.com/api/v1"},
		{"含 /api", "https://img.example.com/api", "https://img.example.com/api/v1"},
		{"带尾斜杠", "https://img.example.com/", "https://img.example.com/api/v1"},
		{"带空格和尾斜杠", "  https://img.example.com/  ", "https://img.example.com/api/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeBaseURL(tt.raw); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
