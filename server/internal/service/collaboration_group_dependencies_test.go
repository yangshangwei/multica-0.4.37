package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimeFreezesEveryInputDependencyAndPrompt(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	one := f.submitManaged(t, f.creator, "creator's original request")
	two := f.submitManaged(t, f.handler, "handler's additional request")
	f.resumeManaged(t)
	runID, request, claim := f.claimManaged(t)
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := f.svc.Queries.ListCollaborationGroupDependencies(f.ctx, run.GroupID)
	if err != nil || len(dependencies) != 2 {
		t.Fatalf("both immutable input identities require frozen dependencies: %v, %v", dependencies, err)
	}
	seen := map[string]bool{}
	for _, dependency := range dependencies {
		seen[util.UUIDToString(dependency.CommandID)] = true
		grants, err := decodeCollaborationRoster[collaborationFrozenGrant](dependency.Grants)
		if err != nil || len(grants) != 1 || grants[0] != freezeCollaborationGrant(f.grant) {
			t.Fatalf("resource, consent and owner generations were not frozen: %+v, %v", grants, err)
		}
	}
	if !seen[util.UUIDToString(one.CommandID)] || !seen[util.UUIDToString(two.CommandID)] {
		t.Fatal("dependency snapshots substituted the first input's identity")
	}
	var saved []protocol.CollaborationRunInput
	if err := json.Unmarshal(run.PromptSnapshot, &saved); err != nil || !reflect.DeepEqual(saved, claim.Inputs) {
		t.Fatalf("claim did not persist its exact prompt: %+v, %v", saved, err)
	}
	f.submitManaged(t, f.handler, "later input belongs to the next execution")
	replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, request)
	if err != nil || !reflect.DeepEqual(replay.Inputs, saved) {
		t.Fatalf("claim replay changed its prompt: %+v, %v", replay.Inputs, err)
	}
	for _, statement := range []string{
		`UPDATE collaboration_group_dependency SET grants='[]'::jsonb WHERE group_id=$1`,
		`DELETE FROM collaboration_group_dependency WHERE group_id=$1`,
	} {
		if _, err := f.pool.Exec(context.Background(), statement, run.GroupID); err == nil {
			t.Fatalf("retained dependency identity was mutable: %s", statement)
		}
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE collaboration_run SET prompt_snapshot='[]'::jsonb WHERE id=$1`, runID); err == nil {
		t.Fatal("claimed prompt was mutable")
	}
}
