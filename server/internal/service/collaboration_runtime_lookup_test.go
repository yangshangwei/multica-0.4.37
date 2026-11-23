package service

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationRuntimeLookupsUseProductionCollectorAndTransaction(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	for _, test := range []struct {
		name string
		read func() error
		want float64
	}{
		{"resource options and summary", func() error {
			_, err := f.svc.ResourceOptions(f.ctx, f.issue.Issue.ID, util.MustParseUUID(f.workspace), func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool { return true })
			return err
		}, 2},
		{"resource summary", func() error {
			_, err := f.svc.Resources(f.ctx, f.issue.Issue.ID)
			return err
		}, 1},
		{"owner registration identity", func() error {
			_, err := f.svc.ResourceIdentity(f.ctx, f.issue.Issue.ID, f.targetID, f.grant.AgentID, pgtype.UUID{})
			return err
		}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Router construction wires this exact collector into IssueService;
			// the collaboration wrapper must retain it, not only its source label.
			collector := obsmetrics.NewBusinessMetrics()
			f.svc.Issues.Metrics = collector
			if err := test.read(); err != nil {
				t.Fatal(err)
			}
			if got := lookupCount(t, collector, obsmetrics.RuntimeLookupSourceIssue, obsmetrics.RuntimeLookupResultOK); got != test.want {
				t.Fatalf("collaboration runtime reads were not attributed: got %v, want %v", got, test.want)
			}
		})
	}

	f.submitManaged(t, f.creator, "recording lookup context")
	f.resumeManaged(t)
	candidates, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(candidates.RunIDs) != 1 {
		t.Fatalf("prepare recording run: %+v %v", candidates, err)
	}
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, util.MustParseUUID(candidates.RunIDs[0]))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, `UPDATE agent_runtime SET provider='fixture-transaction-provider' WHERE id=$1`, f.runtime.ID); err != nil {
		t.Fatal(err)
	}
	queries := f.svc.Queries.WithTx(tx)
	collector := obsmetrics.NewBusinessMetrics()
	f.svc.Issues.Metrics = collector
	state := &collaborationRuntimeState{collaborationQueueTx: &collaborationQueueTx{q: queries, access: f.issue}, run: run}
	if err = f.svc.freezeResourceRecording(f.ctx, state); err != nil {
		t.Fatal(err)
	}
	if got := lookupCount(t, collector, obsmetrics.RuntimeLookupSourceIssue, obsmetrics.RuntimeLookupResultOK); got != 1 {
		t.Fatalf("recording read bypassed the configured collector: %v", got)
	}
	event, err := queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: run.ID, ProducerEventID: "resource-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	var saved collaborationResourceRecording
	if err = json.Unmarshal(event.Payload, &saved); err != nil || saved.Provider == nil || *saved.Provider != "fixture-transaction-provider" {
		t.Fatalf("recording lookup escaped the caller's transaction: %+v %v", saved, err)
	}
}
