package materialrepo

import (
	"path"
	"reflect"
	"sort"

	"github.com/multica-ai/multica/server/internal/material"
)

func validateCandidate(c Candidate) error {
	if len(c.Files) == 0 || len(c.Files) > 20 || !digestPattern.MatchString(c.Digest) {
		return invalid("invalid_candidate", "")
	}
	inspected, err := Inspect(c.Binding, c.Before)
	if err != nil {
		return err
	}
	if inspected.Candidate == nil || inspected.Candidate.Digest != c.Digest || inspected.Candidate.CatalogDigest != c.CatalogDigest || !reflect.DeepEqual(inspected.Candidate.Files, c.Files) {
		return invalid("candidate_changed", "")
	}
	return nil
}

// ValidatePublication requires the frozen base, exact observed head, complete
// create-only diff and expected tree. It never authorizes a merge. Callers bind
// review/repository/branch identities and native gates outside this pure check.
func ValidatePublication(c Candidate, expectedHead string, p Publication) error {
	if err := validateCandidate(c); err != nil {
		return err
	}
	if !material.IsSHA(expectedHead) || p.HeadCommit != expectedHead || p.Tree.Commit != expectedHead || p.BaseCommit != c.Before.Commit {
		return invalid("publication_revision_changed", "")
	}
	if !p.Complete || len(p.Changes) != len(c.Files) {
		return invalid("publication_diff_incomplete", "")
	}
	wanted := map[string]File{}
	for _, f := range c.Files {
		wanted[f.Path] = f
	}
	for _, change := range p.Changes {
		f, ok := wanted[change.Path]
		if !ok || change.Status != "added" || change.Mode != f.Mode || change.PreviousPath != "" {
			return invalid("publication_files_changed", change.Path)
		}
		delete(wanted, change.Path)
	}
	return verifyTree(c, p.Tree)
}

// VerifyMerged proves only bytes at the supplied actual merge commit. The
// caller must establish that this is the authorized review's real merge SHA.
// Conservative preservation requires an exact tree, so concurrent unrelated
// target changes require reinspection instead of silently accepting new scope.
func VerifyMerged(c Candidate, mergeCommit string, snapshot Snapshot) (Proof, error) {
	if err := validateCandidate(c); err != nil {
		return Proof{}, err
	}
	if !material.IsSHA(mergeCommit) || snapshot.Commit != mergeCommit {
		return Proof{}, invalid("merge_revision_changed", "")
	}
	if err := verifyTree(c, snapshot); err != nil {
		return Proof{}, err
	}
	return proof(c.Binding, c.CatalogDigest, c.Digest, snapshot)
}

// VerifyExisting returns evidence for an already-compatible repository. There
// is no candidate or synthetic approval for this read-only path.
func VerifyExisting(binding Binding, snapshot Snapshot) (Proof, error) {
	inspection, err := Inspect(binding, snapshot)
	if err != nil {
		return Proof{}, err
	}
	if !inspection.Compatible {
		return Proof{}, invalid("repository_not_compatible", "")
	}
	catalog, err := BuildCatalog(binding)
	if err != nil {
		return Proof{}, err
	}
	return proof(binding, catalog.Digest, "", snapshot)
}
func verifyTree(c Candidate, snapshot Snapshot) error {
	observed, err := validateSnapshot(snapshot)
	if err != nil {
		return err
	}
	projected, err := projectSnapshot(c.Before, c.Files)
	if err != nil {
		return err
	}
	expected, err := validateSnapshot(projected)
	if err != nil {
		return err
	}

	if len(expected) != len(observed) {
		return invalid("merged_tree_changed", "")
	}
	for name, want := range expected {
		got, ok := observed[name]
		if !ok || got.Mode != want.Mode || got.SHA256 != want.SHA256 {
			return invalid("preserved_or_required_file_changed", name)
		}
		// Every content read used to freeze the candidate must still be complete.
		if want.Content != nil && (got.Content == nil || *got.Content != *want.Content) {
			return invalid("required_content_unavailable", name)
		}
	}
	return nil
}
func proof(binding Binding, catalogDigest, candidateDigest string, s Snapshot) (Proof, error) {
	catalog, err := BuildCatalog(binding)
	if err != nil {
		return Proof{}, err
	}
	entries, err := validateSnapshot(s)
	if err != nil {
		return Proof{}, err
	}
	p := Proof{Binding: binding, Commit: s.Commit, CatalogDigest: catalogDigest, CandidateDigest: candidateDigest, Files: []Entry{}}
	for _, f := range catalog.Files {
		e, ok := entries[f.Path]
		if !ok || e.Content == nil {
			return Proof{}, invalid("required_content_unavailable", f.Path)
		}
		e.Content = nil
		p.Files = append(p.Files, e)
	}
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	return p, nil
}

// projectSnapshot is shared by candidate admission and proof verification so a
// returned candidate always fits the same post-state representation limits.
func projectSnapshot(before Snapshot, files []File) (Snapshot, error) {
	expected := map[string]Entry{}
	for _, e := range before.Entries {
		expected[e.Path] = e
	}
	for _, f := range files {
		if _, exists := expected[f.Path]; exists {
			return Snapshot{}, invalid("candidate_would_overwrite", f.Path)
		}
		body := f.Content
		expected[f.Path] = Entry{Path: f.Path, Mode: f.Mode, SHA256: f.SHA256, Content: &body}
		for parent := path.Dir(f.Path); parent != "."; parent = path.Dir(parent) {
			if _, ok := expected[parent]; !ok {
				expected[parent] = Entry{Path: parent, Mode: "040000"}
			}
		}
	}
	result := Snapshot{Commit: before.Commit, Complete: before.Complete, Entries: make([]Entry, 0, len(expected))}
	for _, entry := range expected {
		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}
