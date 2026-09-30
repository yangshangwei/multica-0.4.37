package auth

import (
	"context"
	"strings"
	"testing"
)

func TestPasswordHash(t *testing.T) {
	p := "correct horse battery staple"
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
	for _, name := range []string{"ab", "1alice", "姓名", "Kelvin", "a-b", strings.Repeat("a", 33)} {
		if _, err := NormalizeUsername(name); err == nil {
			t.Fatal("accepted", name)
		}
	}
	if err := ValidatePassword(strings.Repeat("密", 12)); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatal("long password accepted")
	}
}

func BenchmarkPasswordHash(b *testing.B) {
	for b.Loop() {
		if _, err := HashPassword(context.Background(), "correct horse battery staple"); err != nil {
			b.Fatal(err)
		}
	}
}
