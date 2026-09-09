package docs_test

import (
	"os/exec"
	"strings"
	"testing"
)

// docsPackage is the import path whose embed.FS holds the whole docs bundle.
const docsPackage = "github.com/multica-ai/multica/server/internal/docs"

// TestOnlyServerEmbedsDocsBundle pins the one property that keeps this package's
// megabytes off the CLI: cmd/multica must not reach it, transitively or
// otherwise, while cmd/server must.
//
// Nothing else enforces this. embed.FS costs are invisible at compile time, so a
// single innocuous import — a shared helper in cmd/multica that happens to want
// a doc slug — would silently grow every published CLI binary and no existing
// test would notice. Asserting cmd/server DOES import it keeps this test honest:
// without that half, deleting the endpoints entirely would still pass.
func TestOnlyServerEmbedsDocsBundle(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	for _, tt := range []struct {
		binary string
		want   bool
	}{
		{binary: "../../cmd/multica", want: false},
		{binary: "../../cmd/server", want: true},
	} {
		t.Run(tt.binary, func(t *testing.T) {
			out, err := exec.Command("go", "list", "-deps", tt.binary).Output()
			if err != nil {
				t.Fatalf("go list -deps %s: %v", tt.binary, err)
			}
			got := false
			for _, dep := range strings.Fields(string(out)) {
				if dep == docsPackage {
					got = true
					break
				}
			}
			if got != tt.want {
				t.Fatalf("%s imports %s = %v, want %v — the docs bundle is embedded content that only the API server serves",
					tt.binary, docsPackage, got, tt.want)
			}
		})
	}
}
