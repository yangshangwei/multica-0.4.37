package main

import "testing"

func TestPlatformAdminBootstrapRejectsAmbiguousOperatorInput(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	for _, args := range [][]string{nil, {"unknown"}, {"bootstrap"}, {"bootstrap", "--user", "invalid", "--reason", "initial setup"}, {"bootstrap", "--user", "00000000-0000-4000-8000-000000000001"}, {"bootstrap", "--user", "00000000-0000-4000-8000-000000000001", "--reason", "   "}} {
		if err := runPlatformAdmin(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
