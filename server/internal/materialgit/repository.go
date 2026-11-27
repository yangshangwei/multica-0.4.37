package materialgit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/materialrepo"
)

// RepositoryObserver is a separate read surface; ordinary material allowlists
// remain unchanged. Snapshot reads every blob to prove preserved-file digests.
type RepositoryObserver interface {
	Observer
	ResolveBranch(context.Context, string) (string, error)
	Snapshot(context.Context, string, []string) (materialrepo.Snapshot, error)
}

func (o *observer) ResolveBranch(ctx context.Context, branch string) (string, error) {
	if !validRef(branch) {
		return "", failure("invalid_ref")
	}
	switch p := o.reader.(type) {
	case *githubProvider:
		return p.ref(ctx, branch)
	case *gitlabProvider:
		c, e := p.branch(ctx, branch)
		return c.ID, e
	}
	return "", failure("unsupported_provider")
}

type snapshotEntry struct{ Path, Mode, Type, SHA string }

func (o *observer) Snapshot(ctx context.Context, commit string, bodies []string) (materialrepo.Snapshot, error) {
	out := materialrepo.Snapshot{Commit: commit, Entries: []materialrepo.Entry{}}
	if !shaPattern.MatchString(commit) {
		return out, failure("invalid_commit")
	}
	wanted := map[string]bool{}
	for _, p := range bodies {
		if !validPath(p) {
			return out, failure("invalid_path")
		}
		wanted[p] = true
	}
	entries := []snapshotEntry{}
	switch p := o.reader.(type) {
	case *githubProvider:
		c, err := p.commit(ctx, commit)
		if err != nil {
			return out, err
		}
		var tree struct {
			SHA       string
			Truncated *bool
			Tree      []ghEntry
		}
		if _, err = p.request(ctx, http.MethodGet, p.repo+"/git/trees/"+c.Tree.SHA+"?recursive=1", nil, &tree); err != nil {
			return out, err
		}
		if tree.SHA != c.Tree.SHA || tree.Truncated == nil || *tree.Truncated || tree.Tree == nil {
			return out, failure("provider_tree_incomplete")
		}
		for _, e := range tree.Tree {
			entries = append(entries, snapshotEntry{e.Path, e.Mode, e.Type, e.SHA})
		}
	case *gitlabProvider:
		for page := 1; ; page++ {
			var rows []glTreeEntry
			h, err := p.request(ctx, http.MethodGet, p.repo+"/repository/tree?recursive=true&per_page=100&ref="+url.QueryEscape(commit)+"&page="+strconv.Itoa(page), nil, &rows)
			if err != nil {
				return out, err
			}
			if rows == nil {
				return out, failure("provider_tree_incomplete")
			}
			for _, e := range rows {
				entries = append(entries, snapshotEntry{e.Path, e.Mode, e.Type, e.ID})
			}
			if len(entries) > materialrepo.MaxEntries {
				return out, failure("snapshot_too_large")
			}
			next, err := nextPage(h, page, len(rows), false)
			if err != nil {
				return out, err
			}
			if !next {
				break
			}
		}
	}
	if len(entries) > materialrepo.MaxEntries {
		return out, failure("snapshot_too_large")
	}
	seen := map[string]bool{}
	total := 0
	for _, e := range entries {
		if !validPath(e.Path) || seen[e.Path] || !shaPattern.MatchString(e.SHA) {
			return out, failure("provider_tree_invalid")
		}
		seen[e.Path] = true
		entry := materialrepo.Entry{Path: e.Path, Mode: e.Mode}
		total += len(e.Path) + len(e.Mode)
		switch {
		case e.Type == "tree" && e.Mode == "040000":
		case e.Type == "blob" && e.Mode == "100644":
			var raw []byte
			if o.c.connection.Provider == "gitlab" {
				// GitLab's JSON blob endpoint may transcode non-UTF8 text. Raw
				// bytes are mandatory for preservation digests, including binary.
				if _, err := o.c.request(ctx, http.MethodGet, o.c.repo+"/repository/blobs/"+e.SHA+"/raw", nil, &raw); err != nil {
					return out, err
				}
				if len(raw) > materialrepo.MaxFileBytes {
					return out, failure("provider_file_invalid")
				}
			} else {
				var b struct {
					SHA               string `json:"sha"`
					Encoding, Content string
					Size              *int
				}
				if _, err := o.c.request(ctx, http.MethodGet, o.c.repo+"/git/blobs/"+e.SHA, nil, &b); err != nil {
					return out, err
				}
				if b.SHA != e.SHA || b.Encoding != "base64" || b.Size == nil || *b.Size < 0 || *b.Size > materialrepo.MaxFileBytes {
					return out, failure("provider_file_invalid")
				}
				var err error
				raw, err = base64.StdEncoding.DecodeString(b.Content)
				if err != nil || len(raw) != *b.Size {
					return out, failure("provider_file_incomplete")
				}
			}

			digest := sha256.Sum256(raw)
			entry.SHA256 = hex.EncodeToString(digest[:])
			total += len(raw) + 64
			if wanted[e.Path] {
				if !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
					return out, failure("provider_file_invalid")
				}
				body := string(raw)
				entry.Content = &body
			}
		default:
			return out, failure("unsafe_entry")
		}
		if total > materialrepo.MaxSnapshotBytes {
			return out, failure("snapshot_too_large")
		}
		out.Entries = append(out.Entries, entry)
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Path < out.Entries[j].Path })
	out.Complete = true
	return out, nil
}
