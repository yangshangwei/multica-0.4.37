package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestMaterialRuntimeObservationEvidence(t *testing.T) {
	fixture := func() (protocol.MaterialExecution, protocol.MaterialObservation) {
		head := strings.Repeat("a", 40)
		content := "Immutable complete approved requirement bytes."
		d := protocol.MaterialExecution{Action: "publish", Provider: "github", InstanceURL: "https://github.com", Repository: "org/repo", SourceBranch: "multica/material-revision", BaseBranch: "main", Files: []protocol.MaterialFile{{Path: "engineering/prd.md", Mode: "100644", Content: content, SHA256: material.Digest(content)}}}
		o := protocol.MaterialObservation{ProtocolVersion: 1, Outcome: "observed", SourceComplete: true, RepositoryIdentity: "github:https://github.com:org/repo", ObservedAt: time.Now(), Review: &protocol.MaterialReview{Number: 1, URL: "https://github.com/org/repo/pull/1", SourceBranch: d.SourceBranch, BaseBranch: d.BaseBranch, HeadSHA: head, State: "open", Changes: []protocol.MaterialChange{{Path: d.Files[0].Path, Mode: "100644", Status: "added"}}}, Files: []protocol.MaterialFile{d.Files[0]}}
		o.Files[0].Commit = head
		return d, o
	}
	d, o := fixture()
	if err := validateMaterialObservation(d, o); err != nil {
		t.Fatalf("valid actual readback: %v", err)
	}
	cases := map[string]func(*protocol.MaterialExecution, *protocol.MaterialObservation){
		"model success without readback": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) { o.SourceComplete = false },
		"wrong repository": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.RepositoryIdentity = "github:https://github.com:other/repo"
		},
		"missing full file": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) { o.Files = nil },
		"wrong bytes with claimed digest": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Files[0].Content = "model claimed content"
		},
		"wrong exact commit": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Files[0].Commit = strings.Repeat("b", 40)
		},
		"foreign change": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.Changes = append(o.Review.Changes, protocol.MaterialChange{Path: "app.go", Mode: "100644", Status: "added"})
		},
		"symlink": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.Changes[0].Mode = "120000"
		},
		"deleted file": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.Changes[0].Status = "removed"
		},
		"wrong target": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.BaseBranch = "production"
		},
		"wrong review host": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.URL = "https://evil.example/org/repo/pull/1"
		},
		"stale evidence": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.ObservedAt = time.Now().Add(-6 * time.Minute)
		},
		"unapproved external merge": func(_ *protocol.MaterialExecution, o *protocol.MaterialObservation) {
			o.Review.State = "merged"
			o.Review.MergeSHA = strings.Repeat("b", 40)
			o.Review.MergeActor = "someone"
		},
		"missing synced decision": func(d *protocol.MaterialExecution, _ *protocol.MaterialObservation) {
			d.Action = "review_sync"
			d.AuditNote = &protocol.MaterialAuditNote{Marker: "decision", Body: "human approved"}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d, o := fixture()
			mutate(&d, &o)
			if validateMaterialObservation(d, o) == nil {
				t.Fatal("unverified evidence accepted")
			}
		})
	}
	t.Run("plan preparation reads fixed baseline", func(t *testing.T) {
		d, o := fixture()
		d.Action = "verify_baseline"
		d.BaselineCommit = o.Files[0].Commit
		o.Review = nil
		if err := validateMaterialObservation(d, o); err != nil {
			t.Fatal(err)
		}
		o.Files[0].Commit = strings.Repeat("c", 40)
		if validateMaterialObservation(d, o) == nil {
			t.Fatal("different baseline commit accepted")
		}
	})
}

func TestMaterialRuntimeLookupPreservesDatabaseErrors(t *testing.T) {
	id := uuid.NewString()
	failure := errors.New("database unavailable")
	_, err := materialRuntimeForUser(context.Background(), db.New(scanErrDBTX{err: failure}), util.MustParseUUID(uuid.NewString()), &id, util.MustParseUUID(uuid.NewString()))
	if !errors.Is(err, failure) {
		t.Fatalf("database uncertainty became permission denial: %v", err)
	}
	_, err = materialRuntimeForUser(context.Background(), db.New(scanErrDBTX{err: pgx.ErrNoRows}), util.MustParseUUID(uuid.NewString()), &id, util.MustParseUUID(uuid.NewString()))
	if materialCode(err) != "material_runtime_unavailable" {
		t.Fatalf("missing runtime: %v", err)
	}
}

func TestMaterialAbsentPublicationRequiresReadOnlyRecovery(t *testing.T) {
	d := protocol.MaterialExecution{Action: "observe", ReconcileAction: "publish", Provider: "github", InstanceURL: "https://github.com", Repository: "org/repo"}
	o := protocol.MaterialObservation{ProtocolVersion: 1, Outcome: "not_found", SourceComplete: true, RepositoryIdentity: "github:https://github.com:org/repo", ObservedAt: time.Now()}
	if err := validateMaterialObservation(d, o); err != nil {
		t.Fatalf("complete read-only absence must allow safe recovery: %v", err)
	}
	for _, action := range []string{"publish", "merge", "review_sync", "verify_baseline"} {
		invalid := d
		invalid.Action = action
		if validateMaterialObservation(invalid, o) == nil {
			t.Fatalf("absence accepted for mutation stage %s", action)
		}
	}
	d.ReconcileAction = "merge"
	if validateMaterialObservation(d, o) == nil {
		t.Fatal("absence cannot prove merged outcome")
	}
	d.ReconcileAction = "publish"
	o.SourceComplete = false
	if validateMaterialObservation(d, o) == nil {
		t.Fatal("incomplete absence accepted")
	}
	o.SourceComplete = true
	o.Review = &protocol.MaterialReview{Number: 1}
	if validateMaterialObservation(d, o) == nil {
		t.Fatal("absence with an existing review accepted")
	}
}

func TestMaterialMergeReceiptUsesProviderTime(t *testing.T) {
	expires := time.Date(2026, 11, 27, 6, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		merged   *time.Time
		observed time.Time
		allowed  bool
	}{
		{"missing", nil, expires, false},
		{"same issued second", materialTime(expires.Add(-time.Minute)), expires.Add(3 * time.Minute), true},
		{"previous second", materialTime(expires.Add(-time.Minute - time.Second)), expires, false},
		{"before permit", materialTime(expires.Add(-2 * time.Minute)), expires, false},
		{"after permit", materialTime(expires.Add(time.Second)), expires.Add(3 * time.Minute), false},
		{"future evidence", materialTime(expires.Add(-10 * time.Second)), expires.Add(-20 * time.Second), false},
		{"delayed read", materialTime(expires.Add(-30 * time.Second)), expires.Add(3 * time.Minute), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := materialMergeWithinPermit(expires, tc.observed, &protocol.MaterialReview{MergedAt: tc.merged}); got != tc.allowed {
				t.Fatalf("allowed=%v, want=%v", got, tc.allowed)
			}
		})
	}
}
func materialTime(value time.Time) *time.Time { return &value }
