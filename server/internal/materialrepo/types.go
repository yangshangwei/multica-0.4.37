// Package materialrepo checks repository initialization without performing I/O.
// Snapshots must come from an independently authenticated, complete Git read.
// These checks do not establish human approval, repository identity or authority.
package materialrepo

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/material"
)

const (
	SchemaVersion    = 1
	BundleVersion    = "1.0.0"
	MaxFileBytes     = 1 << 20
	MaxSnapshotBytes = 32 << 20
	MaxEntries       = 10000
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Error struct{ Code, Path string }

func (e *Error) Error() string {
	if e.Path == "" {
		return "material repository: " + e.Code
	}
	return fmt.Sprintf("material repository: %s (%s)", e.Code, e.Path)
}
func invalid(code, p string) error { return &Error{Code: code, Path: p} }

type Binding struct {
	SystemID string `json:"system_id"`
	RootPath string `json:"root_path"`
}
type File struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type Catalog struct {
	Binding       Binding `json:"binding"`
	SchemaVersion int     `json:"schema_version"`
	BundleVersion string  `json:"bundle_version"`
	Files         []File  `json:"files"`
	Digest        string  `json:"digest"`
}

// Entry is a repository-relative tree entry. Directories have no body/digest.
// Every blob needs a SHA256; managed files also need their complete body.
// A non-nil empty string is different from an unread body.
type Entry struct {
	Path    string  `json:"path"`
	Mode    string  `json:"mode"`
	SHA256  string  `json:"sha256,omitempty"`
	Content *string `json:"content,omitempty"`
}
type Snapshot struct {
	Commit   string  `json:"commit"`
	Complete bool    `json:"complete"`
	Entries  []Entry `json:"entries"`
}
type Finding struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}
type Inspection struct {
	Findings   []Finding  `json:"findings"`
	Compatible bool       `json:"compatible"`
	Candidate  *Candidate `json:"candidate,omitempty"`
}

// Candidate is an immutable input record, not an authorization. Persist its
// digest under the authenticated operation and compare it before acting.
// Before includes the complete tree so omission cannot drop preservation checks.
type Candidate struct {
	Binding       Binding  `json:"binding"`
	Before        Snapshot `json:"before"`
	CatalogDigest string   `json:"catalog_digest"`
	Files         []File   `json:"files"`
	Digest        string   `json:"digest"`
}
type Change struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Status       string `json:"status"`
	Mode         string `json:"mode"`
}

// Publication contains already-observed identities; the caller must bind the
// repository, review number, branches and approved HEAD to its operation.
type Publication struct {
	BaseCommit string   `json:"base_commit"`
	HeadCommit string   `json:"head_commit"`
	Complete   bool     `json:"complete"`
	Changes    []Change `json:"changes"`
	Tree       Snapshot `json:"tree"`
}
type Proof struct {
	Binding         Binding `json:"binding"`
	Commit          string  `json:"commit"`
	CatalogDigest   string  `json:"catalog_digest"`
	CandidateDigest string  `json:"candidate_digest,omitempty"`
	Files           []Entry `json:"files"`
}

func validPath(p string, empty bool) bool {
	if p == "" {
		return empty
	}
	if len(p) > 1024 || !utf8.ValidString(p) || path.Clean(p) != p || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\%?#:") {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, part := range strings.Split(p, "/") {
		if part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.TrimSpace(part) != part {
			return false
		}
	}
	return true
}
func validateBinding(b Binding) error {
	if !idPattern.MatchString(b.SystemID) || !validPath(b.RootPath, true) {
		return invalid("invalid_binding", "")
	}
	return nil
}
func textValid(s string) bool {
	return len(s) <= MaxFileBytes && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func fingerprint(p, mode, body string) File {
	return File{Path: p, Mode: mode, Content: body, SHA256: material.Digest(body)}
}

// foldPath canonicalizes every Unicode simple-fold cycle, including sigma and
// long-s variants that strings.ToLower alone does not equate.
func foldPath(s string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < smallest {
				smallest = next
			}
		}
		return smallest
	}, s)
}
