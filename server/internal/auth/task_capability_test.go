package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTaskCapabilityProfileAndStrictDecode(t *testing.T) {
	for _, tc := range []struct {
		key     string
		version int32
		want    string
	}{{"progress-reporter", 1, ""}, {"progress-reporter", 2, TaskCapabilityReporter}, {"code-reviewer", 1, ""}, {"code-reviewer", 2, TaskCapabilityReviewer}, {"security-reviewer", 2, TaskCapabilityReviewer}, {"migration-reviewer", 2, ""}, {"migration-reviewer", 3, TaskCapabilityReviewer}, {"custom", 999, ""}} {
		if got := TaskCapabilityProfile(tc.key, tc.version); got != tc.want {
			t.Errorf("%s v%d got %q", tc.key, tc.version, got)
		}
	}
	good := TaskCapability{Version: 1, Profile: TaskCapabilityReporter, IssueID: "00000000-0000-4000-8000-000000000001", OriginatorID: "00000000-0000-4000-8000-000000000002", RuntimeID: "00000000-0000-4000-8000-000000000003", ClaimGeneration: 1}
	raw, _ := json.Marshal(good)
	if c, err := ParseTaskCapability(raw); err != nil || c != good {
		t.Fatalf("valid capability refused: %v", err)
	}
	for _, raw := range [][]byte{nil, []byte("null"), []byte("{}"), []byte(`{"version":999}`), append(append([]byte{}, raw...), []byte("{}")...)} {
		if _, err := ParseTaskCapability(raw); err == nil {
			t.Fatalf("accepted invalid capability %s", raw)
		}
	}
	for _, mutate := range []func(*TaskCapability){func(c *TaskCapability) { c.ClaimGeneration = 0 }, func(c *TaskCapability) { c.Profile = "operator" }, func(c *TaskCapability) { c.ProjectID = "invalid" }, func(c *TaskCapability) { c.OriginatorID = "" }} {
		c := good
		mutate(&c)
		raw, _ := json.Marshal(c)
		if _, err := ParseTaskCapability(raw); err == nil {
			t.Fatal("accepted invalid scope")
		}
	}
}

// Runtime read failures must propagate unchanged, before any further authority
// read. Missing and unavailable runtimes have different HTTP error semantics.
func TestCurrentTaskCapabilityPropagatesRuntimeLookupFailure(t *testing.T) {
	id, err := util.ParseUUID("00000000-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	task := db.AgentTaskQueue{AgentID: id, RuntimeID: id, IssueID: id, OriginatorUserID: id, Status: "dispatched", ClaimGeneration: 1}
	agent := db.Agent{ID: id, RuntimeID: id, WorkspaceID: id, TemplateKey: "progress-reporter", TemplateVersion: 2}
	for _, want := range []error{pgx.ErrNoRows, errors.New("runtime database unavailable")} {
		t.Run(want.Error(), func(t *testing.T) {
			ctx := t.Context()
			calls := 0
			lookup := func(gotCtx context.Context, gotID pgtype.UUID) (db.AgentRuntime, error) {
				calls++
				if gotCtx != ctx || gotID != task.RuntimeID {
					t.Fatal("runtime lookup lost caller context or task identity")
				}
				return db.AgentRuntime{}, want
			}
			_, err := CurrentTaskCapability(ctx, nil, lookup, task, agent, util.UUIDToString(id), util.UUIDToString(id))
			if err != want || calls != 1 {
				t.Fatalf("runtime failure = %v (%d reads), want original error (%v) and one read", err, calls, want)
			}
		})
	}
}
