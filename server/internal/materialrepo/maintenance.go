package materialrepo

import (
	"path"
	"reflect"

	"github.com/multica-ai/multica/server/internal/material"
)

type MaintenanceChange struct {
	Before Entry `json:"before"`
	After  File  `json:"after"`
}
type MaintenanceChangeSet struct {
	Binding       Binding             `json:"binding"`
	Before        Snapshot            `json:"before"`
	PreviousProof Proof               `json:"previous_proof"`
	BundleVersion string              `json:"bundle_version"`
	CatalogDigest string              `json:"catalog_digest"`
	Changes       []MaintenanceChange `json:"changes"`
	Digest        string              `json:"digest"`
}

// VerifyProtected permits ordinary business files to evolve but keeps every
// adopted catalog path fixed. A new repository snapshot is required each time.
func VerifyProtected(p Proof, s Snapshot) error {
	catalog, err := SupportedProofCatalog(p)
	if err != nil {
		return err
	}
	return verifyProtectedCatalog(p, s, catalog)
}
func verifyProtectedCatalog(p Proof, s Snapshot, catalog Catalog) error {
	if p.CatalogDigest != catalog.Digest || len(p.Files) != len(catalog.Files) {
		return invalid("unsupported_proof", "")
	}
	entries, err := validateSnapshot(s)
	if err != nil {
		return err
	}
	expected := map[string]Entry{}
	for _, e := range p.Files {
		if _, exists := expected[e.Path]; exists {
			return invalid("invalid_proof", e.Path)
		}
		expected[e.Path] = e
	}
	for _, f := range catalog.Files {
		want, ok := expected[f.Path]
		if !ok || want.Mode != "100644" || !digestPattern.MatchString(want.SHA256) || (f.Path != path.Join(p.Binding.RootPath, "README.md") && f.Path != path.Join(p.Binding.RootPath, "material.yaml") && want.SHA256 != f.SHA256) {
			return invalid("invalid_proof", f.Path)
		}
		got, ok := entries[f.Path]
		if ok && got.Content == nil {
			return invalid("required_content_unavailable", f.Path)
		}
		if !ok || got.Mode != want.Mode || got.SHA256 != want.SHA256 {
			return invalid("protected_rule_changed", f.Path)
		}
	}
	return nil
}

// PlanMaintenance accepts only a server-adopted proof and known packaged target.
// Custom content without an adopted digest can never become an overwrite grant.
func PlanMaintenance(p Proof, s Snapshot, targetVersion string) (*MaintenanceChangeSet, error) {
	prior, err := SupportedProofCatalog(p)
	if err != nil {
		return nil, err
	}
	catalog, err := BuildCatalogVersion(p.Binding, targetVersion)
	if err != nil {
		return nil, err
	}
	return planMaintenanceCatalogs(p, s, prior, catalog)
}
func planMaintenanceCatalogs(p Proof, s Snapshot, prior, catalog Catalog) (*MaintenanceChangeSet, error) {
	if prior.Binding != p.Binding || catalog.Binding != p.Binding {
		return nil, invalid("catalog_binding_changed", "")
	}
	if err := verifyProtectedCatalog(p, s, prior); err != nil {
		return nil, err
	}
	if prior.Digest == catalog.Digest {
		return nil, nil
	}
	entries, _ := validateSnapshot(s)
	c := MaintenanceChangeSet{Binding: p.Binding, Before: canonicalSnapshot(s), PreviousProof: p, BundleVersion: catalog.BundleVersion, CatalogDigest: catalog.Digest, Changes: []MaintenanceChange{}}
	for _, f := range catalog.Files {
		got := entries[f.Path]
		if f.Path == path.Join(p.Binding.RootPath, "README.md") || got.SHA256 == f.SHA256 {
			continue
		}
		c.Changes = append(c.Changes, MaintenanceChange{Before: got, After: f})
	}
	if len(c.Changes) == 0 {
		return nil, nil
	}
	c.Digest = maintenanceDigest(c)
	return &c, nil
}
func maintenanceDigest(c MaintenanceChangeSet) string { c.Digest = ""; return material.JSONDigest(c) }
func validateMaintenance(c MaintenanceChangeSet) error {
	rebuilt, err := PlanMaintenance(c.PreviousProof, c.Before, c.BundleVersion)
	if err != nil {
		return err
	}
	if rebuilt == nil || !reflect.DeepEqual(*rebuilt, c) {
		return invalid("maintenance_changed", "")
	}
	return nil
}
func ValidateMaintenancePublication(c MaintenanceChangeSet, expectedHead string, p Publication) error {
	if err := validateMaintenance(c); err != nil {
		return err
	}
	if !p.Complete || p.BaseCommit != c.Before.Commit || p.HeadCommit != expectedHead || p.Tree.Commit != expectedHead || !material.IsSHA(expectedHead) || len(p.Changes) != len(c.Changes) {
		return invalid("publication_revision_changed", "")
	}
	wanted := map[string]File{}
	for _, c := range c.Changes {
		wanted[c.After.Path] = c.After
	}
	for _, change := range p.Changes {
		f, ok := wanted[change.Path]
		if !ok || change.Status != "modified" || change.Mode != f.Mode || change.PreviousPath != "" {
			return invalid("publication_files_changed", change.Path)
		}
		delete(wanted, change.Path)
	}
	return verifyMaintenanceTree(c, p.Tree)
}
func VerifyMaintenanceMerged(c MaintenanceChangeSet, commit string, s Snapshot) (Proof, error) {
	if err := validateMaintenance(c); err != nil {
		return Proof{}, err
	}
	if !material.IsSHA(commit) || s.Commit != commit {
		return Proof{}, invalid("merge_revision_changed", "")
	}
	if err := verifyMaintenanceTree(c, s); err != nil {
		return Proof{}, err
	}
	return proof(c.Binding, c.CatalogDigest, c.Digest, s)
}
func verifyMaintenanceTree(c MaintenanceChangeSet, s Snapshot) error {
	observed, err := validateSnapshot(s)
	if err != nil {
		return err
	}
	expected, err := validateSnapshot(c.Before)
	if err != nil {
		return err
	}
	for _, change := range c.Changes {
		body := change.After.Content
		expected[change.After.Path] = Entry{Path: change.After.Path, Mode: change.After.Mode, SHA256: change.After.SHA256, Content: &body}
	}
	if len(expected) != len(observed) {
		return invalid("merged_tree_changed", "")
	}
	for name, want := range expected {
		got, ok := observed[name]
		if !ok || got.Mode != want.Mode || got.SHA256 != want.SHA256 || (want.Content != nil && (got.Content == nil || *got.Content != *want.Content)) {
			return invalid("preserved_or_required_file_changed", name)
		}
	}
	return nil
}
