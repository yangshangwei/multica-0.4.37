package materialrepo

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/material"
)

var baseSHA = strings.Repeat("a", 40)
var headSHA = strings.Repeat("b", 40)
var mergeSHA = strings.Repeat("c", 40)
var binding = Binding{SystemID: "system-1"}

func body(s string) *string { return &s }
func blob(p, s string) Entry {
	return Entry{Path: p, Mode: "100644", SHA256: material.Digest(s), Content: body(s)}
}
func snapshot(commit string, entries ...Entry) Snapshot {
	all := map[string]Entry{}
	for _, e := range entries {
		all[e.Path] = e
		for parent := path.Dir(e.Path); parent != "."; parent = path.Dir(parent) {
			if _, ok := all[parent]; !ok {
				all[parent] = Entry{Path: parent, Mode: "040000"}
			}
		}
	}
	result := Snapshot{Commit: commit, Complete: true, Entries: []Entry{}}
	for _, e := range all {
		result.Entries = append(result.Entries, e)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Path < result.Entries[j].Path })
	return result
}
func inspect(t *testing.T, b Binding, s Snapshot) Inspection {
	t.Helper()
	r, e := Inspect(b, s)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func applied(c Candidate, commit string) Snapshot {
	entries := append([]Entry{}, c.Before.Entries...)
	for _, f := range c.Files {
		entries = append(entries, blob(f.Path, f.Content))
	}
	return snapshot(commit, entries...)
}
func published(c Candidate) Publication {
	changes := []Change{}
	for _, f := range c.Files {
		changes = append(changes, Change{Path: f.Path, Status: "added", Mode: "100644"})
	}
	return Publication{BaseCommit: c.Before.Commit, HeadCommit: headSHA, Complete: true, Changes: changes, Tree: applied(c, headSHA)}
}
func errorCode(t *testing.T, e error, code string) {
	t.Helper()
	var p *Error
	if !errors.As(e, &p) || p.Code != code {
		t.Fatalf("expected %s, got %v", code, e)
	}
}
func mutateEntry(s Snapshot, name string, f func(*Entry)) Snapshot {
	s = canonicalSnapshot(s)
	for i := range s.Entries {
		if s.Entries[i].Path == name {
			f(&s.Entries[i])
			return s
		}
	}
	panic("entry missing: " + name)
}

func TestInspectAndVerifyPreserveExistingContent(t *testing.T) {
	before := snapshot(baseSHA, blob("README.md", "Existing user documentation\n"), blob("uat/2026-11-27/prd.md", "Preserve this exact test data\n"))
	original := canonicalSnapshot(before)
	r := inspect(t, binding, before)
	if r.Compatible || r.Candidate == nil || len(r.Candidate.Files) != 19 {
		t.Fatal("expected 19 additions while preserving README")
	}
	c := *r.Candidate
	for _, f := range c.Files {
		if f.Path == "README.md" || strings.HasPrefix(f.Path, "uat/") {
			t.Fatal("preserved file was included in writes")
		}
	}
	if !reflect.DeepEqual(original, canonicalSnapshot(before)) {
		t.Fatal("inspection mutated its input")
	}
	if err := ValidatePublication(c, headSHA, published(c)); err != nil {
		t.Fatal(err)
	}
	merged := applied(c, mergeSHA)
	proof, err := VerifyMerged(c, mergeSHA, merged)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.Files) != 20 || proof.Commit != mergeSHA || proof.CandidateDigest != c.Digest {
		t.Fatal("missing proof")
	}
	after := inspect(t, binding, merged)
	if !after.Compatible || after.Candidate != nil {
		t.Fatal("repeated initialization must be a no-op")
	}
	existing, err := VerifyExisting(binding, merged)
	if err != nil || existing.CandidateDigest != "" {
		t.Fatalf("read-only proof should not claim an initialization candidate: %v", err)
	}
	// A change to unrelated retained test data invalidates this frozen proof.
	changed := mutateEntry(merged, "uat/2026-11-27/prd.md", func(e *Entry) { *e = blob(e.Path, "changed") })
	_, err = VerifyMerged(c, mergeSHA, changed)
	errorCode(t, err, "preserved_or_required_file_changed")
}
func TestEmptyRootAndNestedRoot(t *testing.T) {
	for _, root := range []string{"", "docs/materials"} {
		t.Run(root, func(t *testing.T) {
			b := Binding{SystemID: "system-1", RootPath: root}
			r := inspect(t, b, snapshot(baseSHA))
			if r.Candidate == nil || len(r.Candidate.Files) != 20 {
				t.Fatal("empty selected root needs 20 real files")
			}
			for _, f := range r.Candidate.Files {
				rel := strings.TrimPrefix(f.Path, root+"/")
				for _, forbidden := range []string{"requirements/", "issues/", "releases/", "modules/", "uat/"} {
					if strings.HasPrefix(rel, forbidden) {
						t.Fatalf("created business record %s", f.Path)
					}
				}
			}
			if _, err := VerifyMerged(*r.Candidate, mergeSHA, applied(*r.Candidate, mergeSHA)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestManagedConflictsNeverProduceCandidate(t *testing.T) {
	for _, name := range []string{"AGENTS.md", ".material/templates/prd.md", ".material/schemas/issue.schema.json", "material.yaml"} {
		t.Run(name, func(t *testing.T) {
			before := snapshot(baseSHA, blob(name, "custom content remains unchanged\n"))
			r := inspect(t, binding, before)
			if r.Compatible || r.Candidate != nil {
				t.Fatal("conflict must not produce partial runnable plan")
			}
			found := false
			for _, f := range r.Findings {
				if f.Path == name && f.State == "conflict" {
					found = true
				}
			}
			if !found || *before.Entries[len(before.Entries)-1].Content != "custom content remains unchanged\n" {
				t.Fatal("missing conflict or input was modified")
			}
		})
	}
}
func TestSnapshotRefusals(t *testing.T) {
	tests := map[string]Snapshot{
		"unknown tree":           {Commit: baseSHA, Entries: []Entry{}},
		"missing entries":        {Commit: baseSHA, Complete: true},
		"branch not commit":      {Commit: "main", Complete: true, Entries: []Entry{}},
		"symlink":                snapshot(baseSHA, Entry{Path: "AGENTS.md", Mode: "120000", SHA256: strings.Repeat("d", 64)}),
		"submodule":              snapshot(baseSHA, Entry{Path: "module", Mode: "160000"}),
		"executable":             snapshot(baseSHA, Entry{Path: "README.md", Mode: "100755", SHA256: strings.Repeat("d", 64)}),
		"duplicate":              {Commit: baseSHA, Complete: true, Entries: []Entry{blob("README.md", "x"), blob("README.md", "y")}},
		"case collision":         snapshot(baseSHA, blob("README.md", "x"), blob("readme.md", "y")),
		"unicode fold collision": snapshot(baseSHA, blob("Σ.md", "x"), blob("ς.md", "y")),
		"traversal":              {Commit: baseSHA, Complete: true, Entries: []Entry{blob("../README.md", "x")}},
		"missing ancestor":       {Commit: baseSHA, Complete: true, Entries: []Entry{blob(".material/templates/prd.md", "x")}},
		"file parent":            {Commit: baseSHA, Complete: true, Entries: []Entry{blob(".material", "x"), blob(".material/templates/prd.md", "x")}},
		"digest mismatch":        snapshot(baseSHA, Entry{Path: "README.md", Mode: "100644", SHA256: strings.Repeat("d", 64), Content: body("actual")}),
		"oversized":              snapshot(baseSHA, blob("README.md", strings.Repeat("x", MaxFileBytes+1))),
		"invalid UTF8":           snapshot(baseSHA, blob("README.md", string([]byte{255}))),
		"NUL":                    snapshot(baseSHA, blob("README.md", "a\x00b")),
	}
	for name, s := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Inspect(binding, s); err == nil {
				t.Fatal("unsafe or incomplete snapshot accepted")
			}
		})
	}
	for _, s := range []Snapshot{snapshot(baseSHA, blob(".material", "existing file")), snapshot(baseSHA, blob("readme.md", "lowercase collision")), snapshot(baseSHA, Entry{Path: "README.md", Mode: "040000"})} {
		r := inspect(t, binding, s)
		if r.Candidate != nil || r.Compatible {
			t.Fatal("unsafe target must conflict")
		}
	}
	unread := snapshot(baseSHA, Entry{Path: "README.md", Mode: "100644", SHA256: strings.Repeat("d", 64)})
	r := inspect(t, binding, unread)
	if r.Candidate != nil || r.Compatible {
		t.Fatal("unread managed content accepted")
	}
}
func TestPublicationAndMergeRefusals(t *testing.T) {
	c := *inspect(t, binding, snapshot(baseSHA, blob("README.md", "user"), blob("uat/keep.md", "retained"))).Candidate
	for name, change := range map[string]func(*Publication){
		"unknown diff": func(p *Publication) { p.Complete = false },
		"old head":     func(p *Publication) { p.HeadCommit = baseSHA },
		"wrong base":   func(p *Publication) { p.BaseCommit = mergeSHA },
		"missing file": func(p *Publication) { p.Changes = p.Changes[1:] },
		"extra file": func(p *Publication) {
			p.Changes = append(p.Changes, Change{Path: "extra.md", Status: "added", Mode: "100644"})
		},
		"rename":         func(p *Publication) { p.Changes[0].PreviousPath = "old.md" },
		"overwrite":      func(p *Publication) { p.Changes[0].Status = "modified" },
		"duplicate path": func(p *Publication) { p.Changes[0] = p.Changes[1] },
		"forged body": func(p *Publication) {
			p.Tree = mutateEntry(p.Tree, "AGENTS.md", func(e *Entry) { *e = blob(e.Path, "forged") })
		},
		"unrelated added file": func(p *Publication) {
			p.Tree = snapshot(headSHA, append(p.Tree.Entries, blob("unrelated.md", "new"))...)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := published(c)
			change(&p)
			if err := ValidatePublication(c, headSHA, p); err == nil {
				t.Fatal("invalid publication accepted")
			}
		})
	}
	for name, change := range map[string]func(Snapshot) Snapshot{
		"missing content": func(s Snapshot) Snapshot { return mutateEntry(s, "AGENTS.md", func(e *Entry) { e.Content = nil }) },
		"preserved removed": func(s Snapshot) Snapshot {
			entries := []Entry{}
			for _, e := range s.Entries {
				if e.Path != "uat/keep.md" {
					entries = append(entries, e)
				}
			}
			s.Entries = entries
			return s
		},
		"missing required": func(s Snapshot) Snapshot {
			entries := []Entry{}
			for _, e := range s.Entries {
				if e.Path != "material.yaml" {
					entries = append(entries, e)
				}
			}
			s.Entries = entries
			return s
		},
		"different revision": func(s Snapshot) Snapshot { s.Commit = headSHA; return s },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyMerged(c, mergeSHA, change(applied(c, mergeSHA))); err == nil {
				t.Fatal("invalid merged proof accepted")
			}
		})
	}
	forged := c
	forged.Files = append([]File{}, c.Files...)
	forged.Files[0].Content = "changed"
	forged.Files[0].SHA256 = material.Digest("changed")
	forged.Digest = candidateDigest(forged)
	if _, err := VerifyMerged(forged, mergeSHA, applied(forged, mergeSHA)); err == nil {
		t.Fatal("self-rehashed forged plan accepted")
	}
	noop := c
	noop.Files = nil
	if _, err := VerifyMerged(noop, mergeSHA, applied(c, mergeSHA)); err == nil {
		t.Fatal("noop must use read-only verification")
	}
}
func TestCandidateOwnsInputSnapshot(t *testing.T) {
	source := snapshot(baseSHA, blob("README.md", "original"))
	c := inspect(t, binding, source).Candidate
	*source.Entries[0].Content = "mutated by caller"
	if *c.Before.Entries[0].Content != "original" {
		t.Fatal("candidate aliases caller snapshot body")
	}
}

func TestSnapshotLimitsAndOpaqueUnmanagedContent(t *testing.T) {
	opaque := Entry{Path: "uat/opaque.bin", Mode: "100644", SHA256: strings.Repeat("d", 64)}
	before := snapshot(baseSHA, opaque)
	candidate := inspect(t, binding, before).Candidate
	if candidate == nil {
		t.Fatal("opaque unmanaged file blocked catalog")
	}
	if _, err := VerifyMerged(*candidate, mergeSHA, applied(*candidate, mergeSHA)); err != nil {
		t.Fatal(err)
	}
	tooMany := snapshot(baseSHA)
	tooMany.Entries = make([]Entry, MaxEntries+1)
	if _, err := Inspect(binding, tooMany); err == nil {
		t.Fatal("unbounded tree accepted")
	}
	oversized := snapshot(baseSHA)
	for i := 0; i < 33; i++ {
		oversized.Entries = append(oversized.Entries, blob(fmt.Sprintf("file-%d", i), strings.Repeat("x", MaxFileBytes)))
	}
	_, err := Inspect(binding, oversized)
	errorCode(t, err, "snapshot_too_large")
}

func TestInspectRejectsUnrepresentableProjectedTree(t *testing.T) {
	t.Run("entries", func(t *testing.T) {
		empty := inspect(t, binding, snapshot(baseSHA)).Candidate
		addedEntries := len(applied(*empty, mergeSHA).Entries)
		before := snapshot(baseSHA)
		for i := 0; i < MaxEntries-addedEntries; i++ {
			before.Entries = append(before.Entries, Entry{Path: fmt.Sprintf("retained-%05d", i), Mode: "100644", SHA256: strings.Repeat("d", 64)})
		}
		candidate := inspect(t, binding, before).Candidate
		if candidate == nil {
			t.Fatal("exact entry budget should remain usable")
		}
		if _, err := VerifyMerged(*candidate, mergeSHA, applied(*candidate, mergeSHA)); err != nil {
			t.Fatalf("returned candidate cannot be verified: %v", err)
		}
		before.Entries = append(before.Entries, Entry{Path: "retained-over-limit", Mode: "100644", SHA256: strings.Repeat("e", 64)})
		result, err := Inspect(binding, before)
		if err == nil || result.Candidate != nil {
			t.Fatal("projected entry overflow must not return a candidate")
		}
	})
	t.Run("bytes", func(t *testing.T) {
		empty := inspect(t, binding, snapshot(baseSHA)).Candidate
		addedBytes := 0
		for _, e := range applied(*empty, mergeSHA).Entries {
			addedBytes += len(e.Path) + len(e.Mode) + len(e.SHA256)
			if e.Content != nil {
				addedBytes += len(*e.Content)
			}
		}
		before := snapshot(baseSHA)
		remaining := MaxSnapshotBytes - addedBytes
		for i := 0; remaining > 0; i++ {
			name := fmt.Sprintf("retained-%03d", i)
			overhead := len(name) + len("100644") + 64
			size := remaining - overhead
			if size > MaxFileBytes {
				size = MaxFileBytes
			}
			if size < 0 {
				t.Fatal("test fixture budget cannot fit metadata")
			}
			before.Entries = append(before.Entries, blob(name, strings.Repeat("x", size)))
			remaining -= size + overhead
		}
		candidate := inspect(t, binding, before).Candidate
		if candidate == nil {
			t.Fatal("exact byte budget should remain usable")
		}
		if _, err := VerifyMerged(*candidate, mergeSHA, applied(*candidate, mergeSHA)); err != nil {
			t.Fatalf("returned candidate cannot be verified: %v", err)
		}
		last := &before.Entries[len(before.Entries)-1]
		*last = blob(last.Path, *last.Content+"x")
		result, err := Inspect(binding, before)
		if err == nil || result.Candidate != nil {
			t.Fatal("projected byte overflow must not return a candidate")
		}
	})
}
