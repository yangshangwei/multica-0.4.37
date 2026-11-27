package materialrepo

import (
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/material"
	"os"
	"testing"
)

func TestShippedBundle100BytesRemainImmutable(t *testing.T) {
	raw, err := os.ReadFile("bundle-1.0.0-sha256.json")
	if err != nil {
		t.Fatal(err)
	}
	var pinned []struct{ Path, SHA256 string }
	if json.Unmarshal(raw, &pinned) != nil || len(pinned) != 20 {
		t.Fatal("invalid baseline inventory")
	}
	catalog, err := BuildCatalogVersion(Binding{SystemID: "system-id"}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Files) != 20 {
		t.Fatal("old catalog changed file count")
	}
	for _, entry := range pinned {
		raw, err := assets.ReadFile("assets/" + entry.Path)
		if err != nil || material.Digest(string(raw)) != entry.SHA256 {
			t.Fatalf("shipped file changed %s", entry.Path)
		}
	}
}
func TestKnownBundleCompatibilityAndUpgradeBoundaries(t *testing.T) {
	for _, version := range AvailableBundleVersions() {
		catalog, err := BuildCatalogVersion(binding, version)
		if err != nil {
			t.Fatal(err)
		}
		entries := []Entry{}
		for _, f := range catalog.Files {
			entries = append(entries, blob(f.Path, f.Content))
		}
		full := snapshot(baseSHA, entries...)
		inspected, err := Inspect(binding, full)
		if err != nil || !inspected.Compatible || inspected.Candidate != nil {
			t.Fatalf("supported %s conflicts: %v", version, err)
		}
		p, err := VerifyExisting(binding, full)
		if err != nil || p.CatalogDigest != catalog.Digest {
			t.Fatalf("wrong %s proof: %v", version, err)
		}
		if _, err := SupportedProofCatalog(p); err != nil {
			t.Fatal(err)
		}
		if c, err := PlanMaintenance(p, full, version); err != nil || c != nil {
			t.Fatal("same version rewrites bytes")
		}
	}
	if _, err := BuildCatalogVersion(binding, "1.99.0"); err == nil {
		t.Fatal("unknown version accepted")
	}
	// Candidate frozen under 1.0.0 must stay valid after current default changes.
	old, err := InspectVersion(binding, snapshot(baseSHA), "1.0.0")
	if err != nil || old.Candidate == nil {
		t.Fatal(err)
	}
	if err := ValidatePublication(*old.Candidate, headSHA, published(*old.Candidate)); err != nil {
		t.Fatal("old pending candidate changed", err)
	}
}
