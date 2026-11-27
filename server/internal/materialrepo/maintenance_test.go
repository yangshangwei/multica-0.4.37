package materialrepo

import (
	"testing"
)

func TestProtectedRulesAllowBusinessChangesButRefuseDrift(t *testing.T) {
	inspected := inspect(t, binding, snapshot(baseSHA, blob("README.md", "our readme"), blob("uat/data", "old")))
	full := applied(*inspected.Candidate, mergeSHA)
	p, err := VerifyMerged(*inspected.Candidate, mergeSHA, full)
	if err != nil {
		t.Fatal(err)
	}
	for i := range full.Entries {
		if full.Entries[i].Path == "uat/data" {
			full.Entries[i] = blob("uat/data", "new business data")
		}
	}
	if err := VerifyProtected(p, full); err != nil {
		t.Fatalf("business change blocked: %v", err)
	}
	for i := range full.Entries {
		if full.Entries[i].Path == "AGENTS.md" {
			full.Entries[i] = blob("AGENTS.md", "unreviewed change")
		}
	}
	if err := VerifyProtected(p, full); err == nil {
		t.Fatal("rule drift accepted")
	}
}
func TestMaintenanceHasSeparateFrozenBeforeAfter(t *testing.T) {
	inspected := inspect(t, binding, snapshot(baseSHA, blob("README.md", "keep readme")))
	full := applied(*inspected.Candidate, mergeSHA)
	p, err := VerifyMerged(*inspected.Candidate, mergeSHA, full)
	if err != nil {
		t.Fatal(err)
	}
	c, err := PlanMaintenance(p, full, BundleVersion)
	if err != nil || c != nil {
		t.Fatalf("same bundle not no-op: %v", err)
	}

	prior, _ := BuildCatalogVersion(binding, "1.0.0")
	entries := []Entry{blob("README.md", "keep readme"), blob("uat/old.bin", "unchanged")}
	for _, f := range prior.Files {
		if f.Path != "README.md" {
			entries = append(entries, blob(f.Path, f.Content))
		}
	}
	full = snapshot(baseSHA, entries...)
	p, err = VerifyExisting(binding, full)
	if err != nil || p.CatalogDigest != prior.Digest {
		t.Fatalf("known prior not adopted: %v", err)
	}
	if err := VerifyProtected(p, full); err != nil {
		t.Fatalf("supported old proof blocked: %v", err)
	}
	c, err = PlanMaintenance(p, full, BundleVersion)
	if err != nil || c == nil || len(c.Changes) != 2 {
		t.Fatalf("maintenance must change AGENTS and manifest: %+v %v", c, err)
	}
	changed := canonicalSnapshot(full)
	changed.Commit = headSHA
	changes := []Change{}
	for _, change := range c.Changes {
		if change.After.Path != "AGENTS.md" && change.After.Path != "material.yaml" {
			t.Fatal("unexpected maintenance path")
		}
		if fullEntry(full, change.After.Path).SHA256 != change.Before.SHA256 {
			t.Fatal("before hash not pinned")
		}
		for i := range changed.Entries {
			if changed.Entries[i].Path == change.After.Path {
				changed.Entries[i] = blob(change.After.Path, change.After.Content)
			}
		}
		changes = append(changes, Change{Path: change.After.Path, Status: "modified", Mode: "100644"})
	}
	publication := Publication{BaseCommit: baseSHA, HeadCommit: headSHA, Complete: true, Tree: changed, Changes: changes}
	if err := ValidateMaintenancePublication(*c, headSHA, publication); err != nil {
		t.Fatal(err)
	}
	upgraded, err := VerifyMaintenanceMerged(*c, headSHA, changed)
	if err != nil {
		t.Fatal(err)
	}
	latest, _ := BuildCatalog(binding)
	if upgraded.CatalogDigest != latest.Digest {
		t.Fatal("new proof kept old catalog")
	}
	if fullEntry(changed, "README.md").SHA256 != fullEntry(full, "README.md").SHA256 || fullEntry(changed, "uat/old.bin").SHA256 != fullEntry(full, "uat/old.bin").SHA256 {
		t.Fatal("preserved content changed")
	}
	if next, err := PlanMaintenance(upgraded, changed, BundleVersion); err != nil || next != nil {
		t.Fatal("same version not no-op")
	}
	publication.Changes[0].Path = "README.md"
	if err := ValidateMaintenancePublication(*c, headSHA, publication); err == nil {
		t.Fatal("foreign modified path accepted")
	}

	c.Changes[0].Before.SHA256 = "invalid"
	if c.Digest == maintenanceDigest(*c) {
		t.Fatal("changed before accepted")
	}
}
func fullEntry(s Snapshot, p string) Entry {
	for _, e := range s.Entries {
		if e.Path == p {
			return e
		}
	}
	return Entry{}
}
