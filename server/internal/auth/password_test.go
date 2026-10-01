package auth

import (
	"context"
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	p := "abc123"
	h, err := HashPassword(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashPassword(context.Background(), p)
	if err != nil || h == h2 {
		t.Fatal("salt must be random", err)
	}
	ok, err := VerifyPassword(context.Background(), h, p)
	if err != nil || !ok {
		t.Fatal("valid password rejected", err)
	}
	ok, err = VerifyPassword(context.Background(), h, p+"!")
	if err != nil || ok {
		t.Fatal("wrong password accepted", err)
	}
	for _, bad := range []string{"", "pbkdf2-sha256$1$9999999999$a$b", strings.Repeat("x", 1000)} {
		if ok, _ := VerifyPassword(context.Background(), bad, p); ok {
			t.Fatal("malformed hash accepted")
		}
	}
}

func TestPasswordInput(t *testing.T) {
	if got, err := NormalizeUsername(" Alice_01 "); err != nil || got != "alice_01" {
		t.Fatal(got, err)
	}
	for _, name := range []string{"ab", "12", "姓名", "Kelvin", "a-b", "a b", "a.b", strings.Repeat("a", 33)} {
		if _, err := NormalizeUsername(name); err == nil {
			t.Fatal("accepted", name)
		}
	}
	if err := ValidatePassword(strings.Repeat("密", 6)); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatal("long password accepted")
	}
}

func TestPasswordLength(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{
		{"five characters", "abc12", false},
		{"six characters", "abc123", true},
		{"eleven characters", strings.Repeat("a", 11), true},
		{"existing long password", "correct horse battery staple", true},
		{"maximum length", strings.Repeat("a", 128), true},
		{"over maximum length", strings.Repeat("a", 129), false},
		{"five Unicode characters", strings.Repeat("密", 5), false},
		{"six Unicode characters", strings.Repeat("密", 6), true},
		{"spaces are preserved", " pass ", true},
		{"invalid UTF-8", "abc123\xff", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePassword(tc.password); (err == nil) != tc.valid {
				t.Fatalf("ValidatePassword() error = %v, want valid = %v", err, tc.valid)
			}
		})
	}
}

func TestNormalizeUsernameEmployeeID(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"11052", "11052"},
		{" 0011052 ", "0011052"},
		{"1Alice", "1alice"},
		{"_Alice", "_alice"},
		{"123", "123"},
		{strings.Repeat("1", 32), strings.Repeat("1", 32)},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := NormalizeUsername(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("NormalizeUsername(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
			}
		})
	}
}

func BenchmarkPasswordHash(b *testing.B) {
	for b.Loop() {
		if _, err := HashPassword(context.Background(), "correct horse battery staple"); err != nil {
			b.Fatal(err)
		}
	}
}
