package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestLoadRejectsWildcardOrigin(t *testing.T) {
	t.Setenv("KG_DATABASE_URL", "postgres://primary")
	t.Setenv("KG_ALLOWED_ORIGINS", "*")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected wildcard origin error")
	}
}

func TestLoadRejectsTrailingSlashOrigin(t *testing.T) {
	t.Setenv("KG_DATABASE_URL", "postgres://primary")
	t.Setenv("KG_ALLOWED_ORIGINS", "https://www.example.com/")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected trailing slash origin error")
	}
}

func TestLoadRejectsPublicURLPath(t *testing.T) {
	t.Setenv("KG_DATABASE_URL", "postgres://primary")
	t.Setenv("KG_PUBLIC_API_URL", "https://api.example.com/api")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected public API URL path error")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("KG_DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected missing database URL error")
	}
}

func TestLoadUsesDirectDatabaseFallback(t *testing.T) {
	t.Setenv("KG_ALLOWED_ORIGINS", "https://www.example.com")
	t.Setenv("KG_DATABASE_URL", "postgres://primary")
	t.Setenv("KG_DATABASE_DIRECT_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDirectURL != cfg.DatabaseURL {
		t.Fatalf("DatabaseDirectURL = %q, want %q", cfg.DatabaseDirectURL, cfg.DatabaseURL)
	}
}

func TestLoadValidatesTOTPEncryptionKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "empty is allowed outside production"},
		{name: "32 byte standard base64", key: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))},
		{name: "32 byte raw url base64", key: base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("q", 32)))},
		{name: "not base64", key: "not-base64", wantErr: true},
		{name: "wrong decoded length", key: base64.StdEncoding.EncodeToString([]byte("short")), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KG_DATABASE_URL", "postgres://primary")
			t.Setenv("KG_TOTP_ENCRYPTION_KEY", tt.key)
			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadDefaultsMFAWindows(t *testing.T) {
	t.Setenv("KG_DATABASE_URL", "postgres://primary")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MFAChallengeTTL != 5*time.Minute {
		t.Fatalf("MFAChallengeTTL = %s, want 5m", cfg.MFAChallengeTTL)
	}
	if cfg.MFARecentTTL != 5*time.Minute {
		t.Fatalf("MFARecentTTL = %s, want 5m", cfg.MFARecentTTL)
	}
}

func TestLoadValidatesTrustedProxies(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty trusts no proxy"},
		{name: "addresses and ranges", value: "172.30.27.10, 192.0.2.0/24, ::1"},
		{name: "hostname rejected", value: "caddy", wantErr: true},
		{name: "wildcard rejected", value: "*", wantErr: true},
		{name: "invalid range rejected", value: "172.30.27.0/99", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("KG_DATABASE_URL", "postgres://primary")
			t.Setenv("KG_TRUSTED_PROXIES", test.value)
			_, err := Load()
			if (err != nil) != test.wantErr {
				t.Fatalf("Load() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
