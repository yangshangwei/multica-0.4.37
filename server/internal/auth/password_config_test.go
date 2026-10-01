package auth

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordConfig(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	t.Setenv("MULTICA_CLOUD_URL", "")
	t.Setenv("MULTICA_DEVICE_AUTH_ENABLED", "false")
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", "")
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", "")
	if err := ValidatePasswordConfig(); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct{ key, value string }{{"MULTICA_AUTH_MODE", "unknown"}, {"MULTICA_DEVICE_AUTH_ENABLED", "true"}, {"MULTICA_CLOUD_URL", "https://cloud.example"}, {"MULTICA_PASSWORD_LIMITER_MODE", ""}, {"MULTICA_PASSWORD_MIGRATION_CUTOFF", "2026-09-30T00:00:00Z"}} {
		t.Run(entry.key, func(t *testing.T) {
			t.Setenv(entry.key, entry.value)
			if err := ValidatePasswordConfig(); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}
func TestPasswordLimiterAndProxy(t *testing.T) {
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	l := NewPasswordLimiter(nil)
	limit := PasswordLimit{Key: "account:alice", Count: 2, Window: time.Minute}
	for i := 0; i < 2; i++ {
		if wait, err := l.Allow(context.Background(), limit); err != nil || wait != 0 {
			t.Fatal(wait, err)
		}
	}
	if wait, err := l.Allow(context.Background(), limit); err != nil || wait <= 0 {
		t.Fatal("limit not enforced", wait, err)
	}
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "shared")
	if _, err := l.Allow(context.Background(), limit); err == nil {
		t.Fatal("shared outage allowed")
	}
	r := httptest.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = "192.0.2.1:8000"
	r.Header.Set("X-Forwarded-For", "198.51.100.3")
	t.Setenv("MULTICA_TRUSTED_PROXIES", "")
	if got := PasswordClientIP(r); got != "192.0.2.1" {
		t.Fatal("untrusted forwarding accepted", got)
	}
	t.Setenv("MULTICA_TRUSTED_PROXIES", "192.0.2.0/24")
	if got := PasswordClientIP(r); got != "198.51.100.3" {
		t.Fatal(got)
	}
}
