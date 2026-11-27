package materialrepo

import (
	"path"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/material"
)

// Inspect checks a complete tree at one immutable commit and produces an
// add-only candidate. Conflicts preserve the input and produce no candidate.
func Inspect(binding Binding, snapshot Snapshot) (Inspection, error) {
	catalog, err := BuildCatalog(binding)
	if err != nil {
		return Inspection{}, err
	}
	entries, err := validateSnapshot(snapshot)
	if err != nil {
		return Inspection{}, err
	}
	result := Inspection{Findings: []Finding{}}
	missing := []File{}
	conflict := false
	for _, f := range catalog.Files {
		finding := Finding{Path: f.Path, State: "missing"}
		if reason := ancestorConflict(f.Path, entries); reason != "" {
			finding.State = "conflict"
			finding.Reason = reason
		} else if current, ok := entries[f.Path]; ok {
			finding.State = "compatible"
			if current.Mode != "100644" {
				finding.State = "conflict"
				finding.Reason = "unsafe_entry"
			} else if current.Content == nil {
				finding.State = "conflict"
				finding.Reason = "content_unavailable"
			} else if f.Path == path.Join(binding.RootPath, "README.md") {
				finding.State = "preserved"
			} else if f.Path == path.Join(binding.RootPath, "material.yaml") {
				m, e := ParseManifest(*current.Content)
				if e != nil {
					finding.State = "conflict"
					finding.Reason = e.(*Error).Code
				} else if m.SystemID != binding.SystemID {
					finding.State = "conflict"
					finding.Reason = "system_identity_conflict"
				}
			} else if current.SHA256 != f.SHA256 {
				finding.State = "conflict"
				finding.Reason = "managed_content_conflict"
			}
		}
		if finding.State == "missing" {
			missing = append(missing, f)
		}
		if finding.State == "conflict" {
			conflict = true
		}
		result.Findings = append(result.Findings, finding)
	}
	result.Compatible = !conflict && len(missing) == 0
	if !conflict && len(missing) > 0 {
		projected, err := projectSnapshot(snapshot, missing)
		if err != nil {
			return Inspection{}, err
		}
		if _, err := validateSnapshot(projected); err != nil {
			return Inspection{}, err
		}
		c := Candidate{Binding: binding, Before: canonicalSnapshot(snapshot), CatalogDigest: catalog.Digest, Files: missing}
		c.Digest = candidateDigest(c)
		result.Candidate = &c
	}
	return result, nil
}

func validateSnapshot(s Snapshot) (map[string]Entry, error) {
	if !s.Complete || !material.IsSHA(s.Commit) || s.Entries == nil || len(s.Entries) > MaxEntries {
		return nil, invalid("snapshot_incomplete", "")
	}
	entries := make(map[string]Entry, len(s.Entries))
	folded := make(map[string]string, len(s.Entries))
	size := 0
	for _, e := range s.Entries {
		if !validPath(e.Path, false) {
			return nil, invalid("unsafe_path", e.Path)
		}
		key := foldPath(e.Path)
		if _, ok := folded[key]; ok {
			return nil, invalid("path_collision", e.Path)
		}
		folded[key] = e.Path
		size += len(e.Path) + len(e.SHA256) + len(e.Mode)
		switch e.Mode {
		case "040000":
			if e.SHA256 != "" || e.Content != nil {
				return nil, invalid("invalid_tree_entry", e.Path)
			}
		case "100644":
			if !digestPattern.MatchString(e.SHA256) {
				return nil, invalid("missing_file_digest", e.Path)
			}
			if e.Content != nil {
				size += len(*e.Content)
				if !textValid(*e.Content) || material.Digest(*e.Content) != e.SHA256 {
					return nil, invalid("invalid_file_content", e.Path)
				}
			}
		default:
			return nil, invalid("unsafe_entry", e.Path)
		}
		if size > MaxSnapshotBytes {
			return nil, invalid("snapshot_too_large", "")
		}
		entries[e.Path] = e
	}
	for name := range entries {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			e, ok := entries[parent]
			if !ok || e.Mode != "040000" {
				return nil, invalid("incomplete_ancestor", parent)
			}
		}
	}
	return entries, nil
}
func ancestorConflict(name string, entries map[string]Entry) string {
	for part := name; part != "."; part = path.Dir(part) {
		for observed, e := range entries {
			if strings.EqualFold(part, observed) && part != observed {
				return "path_collision"
			}
			if observed == part && part != name && e.Mode != "040000" {
				return "unsafe_ancestor"
			}
		}
	}
	return ""
}
func canonicalSnapshot(s Snapshot) Snapshot {
	entries := make([]Entry, len(s.Entries))
	copy(entries, s.Entries)
	for i := range entries {
		if entries[i].Content != nil {
			body := *entries[i].Content
			entries[i].Content = &body
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	s.Entries = entries
	return s
}
func candidateDigest(c Candidate) string { c.Digest = ""; return material.JSONDigest(c) }
