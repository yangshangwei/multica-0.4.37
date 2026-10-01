package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"
)

const PasswordIterations = 600000

var ErrPasswordBusy = errors.New("password service busy")
var passwordSlots = make(chan struct{}, max(1, min(4, runtime.GOMAXPROCS(0))))

func NormalizeUsername(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) < 3 || len(s) > 32 {
		return "", errors.New("username must contain 3–32 ASCII characters")
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return "", errors.New("username must contain only letters, numbers or underscores")
		}
	}
	return strings.ToLower(s), nil
}

func ValidatePassword(s string) error {
	n := utf8.RuneCountInString(s)
	if !utf8.ValidString(s) || n < 6 || n > 128 || len(s) > 512 {
		return errors.New("password must contain 6–128 characters and at most 512 bytes")
	}
	return nil
}

func ValidatePasswordName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 80 {
		return "", errors.New("name must contain 1–80 characters")
	}
	return s, nil
}

func derivePassword(ctx context.Context, p string, salt []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case passwordSlots <- struct{}{}:
	default:
		return nil, ErrPasswordBusy
	}
	defer func() { <-passwordSlots }()
	key, err := pbkdf2.Key(sha256.New, p, salt, PasswordIterations, 32)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return key, err
}

func HashPassword(ctx context.Context, p string) (string, error) {
	if err := ValidatePassword(p); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := derivePassword(ctx, p, salt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2-sha256$1$600000$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(ctx context.Context, h, p string) (bool, error) {
	if len(h) > 200 {
		return false, errors.New("invalid password hash")
	}
	fields := strings.Split(h, "$")
	if len(fields) != 5 || fields[0] != "pbkdf2-sha256" || fields[1] != "1" || fields[2] != "600000" {
		return false, errors.New("unsupported password hash")
	}
	salt, err := base64.RawStdEncoding.DecodeString(fields[3])
	if err != nil || len(salt) != 16 {
		return false, errors.New("invalid password salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(fields[4])
	if err != nil || len(expected) != 32 {
		return false, errors.New("invalid password digest")
	}
	key, err := derivePassword(ctx, p, salt)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(key, expected) == 1, nil
}

// DummyPasswordVerification gives unknown usernames the same KDF work as known ones.
func DummyPasswordVerification(ctx context.Context, p string) error {
	_, err := derivePassword(ctx, p, make([]byte, 16))
	return err
}
