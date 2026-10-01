package auth

import (
	"errors"
	"os"
	"strings"
	"time"
)

func PasswordMode() bool { return strings.TrimSpace(os.Getenv("MULTICA_AUTH_MODE")) == "password" }

func PasswordMigrationWindow() (time.Time, time.Time, error) {
	c, d := os.Getenv("MULTICA_PASSWORD_MIGRATION_CUTOFF"), os.Getenv("MULTICA_PASSWORD_MIGRATION_DEADLINE")
	if c == "" && d == "" {
		return time.Time{}, time.Time{}, nil
	}
	cutoff, e1 := time.Parse(time.RFC3339, c)
	deadline, e2 := time.Parse(time.RFC3339, d)
	if e1 != nil || e2 != nil || !strings.HasSuffix(c, "Z") || !strings.HasSuffix(d, "Z") || !deadline.After(cutoff) {
		return time.Time{}, time.Time{}, errors.New("password migration requires UTC cutoff and later deadline")
	}
	return cutoff, deadline, nil
}

func ValidatePasswordConfig() error {
	mode := strings.TrimSpace(os.Getenv("MULTICA_AUTH_MODE"))
	if mode != "" && mode != "legacy" && mode != "password" {
		return errors.New("MULTICA_AUTH_MODE must be legacy or password")
	}
	if _, _, err := PasswordMigrationWindow(); err != nil {
		return err
	}
	if !PasswordMode() {
		return nil
	}
	deviceEnabled := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MULTICA_DEVICE_AUTH_ENABLED"))) {
	case "true", "1", "yes", "on":
		deviceEnabled = true
	}
	if deviceEnabled || strings.TrimSpace(os.Getenv("MULTICA_CLOUD_URL")) != "" {
		return errors.New("password mode cannot enable device authentication or Cloud Fleet")
	}
	switch os.Getenv("MULTICA_PASSWORD_LIMITER_MODE") {
	case "single":
	case "shared":
		if strings.TrimSpace(os.Getenv("REDIS_URL")) == "" {
			return errors.New("shared password limiter requires REDIS_URL")
		}
	default:
		return errors.New("password mode requires MULTICA_PASSWORD_LIMITER_MODE=single or shared")
	}
	return nil
}
