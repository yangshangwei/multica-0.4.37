package service

import (
	"encoding/json"
	"fmt"
	"github.com/multica-ai/multica/server/internal/iteration"
	"testing"
)

func TestIterationMembershipEventsFreezeSourceAndTarget(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprintf("issues_%d", count), func(t *testing.T) {
			fx, s := lifecycleFixture(t)
			a, b, c := lifecycleCreate(t, fx, s), lifecycleCreate(t, fx, s), lifecycleCreate(t, fx, s)
			issues := make([]string, count)
			for i := range issues {
				issues[i] = fx.Issue(t, fmt.Sprintf("Transition %d", i), nil)
			}
			move := func(source, target *string) iteration.WriteResult {
				t.Helper()
				reason := "Explicit scope transition"
				draft := iteration.Draft{Operation: "move", ExpectedSettingsRevision: 1, Reason: &reason, Moves: []iteration.Move{}}
				for _, id := range issues {
					var rev int64
					fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", id).Scan(&rev)
					draft.Moves = append(draft.Moves, iteration.Move{IssueID: id, ExpectedIssueRevision: rev, ExpectedSourceID: source, TargetID: target})
				}
				return lifecycleApply(t, fx, s, draft)
			}
			check := func(operation string, source, target *string, beforeCount, afterCount int) {
				t.Helper()
				rows, err := fx.Pool.Query(t.Context(), "SELECT iteration_id::text,before_facts,after_facts FROM iteration_event WHERE operation_id=$1 AND issue_id IS NOT NULL ORDER BY iteration_id,sequence", operation)
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				n := 0
				for rows.Next() {
					var iid string
					var before, after []byte
					if err = rows.Scan(&iid, &before, &after); err != nil {
						t.Fatal(err)
					}
					n++
					raw := after
					wantCount := afterCount
					if source != nil && iid == *source {
						raw = before
						wantCount = beforeCount
						if string(after) != "null" {
							t.Fatal("source leave after facts changed")
						}
					} else if string(before) != "null" {
						t.Fatal("target join before facts changed")
					}
					var facts map[string]json.RawMessage
					if err = json.Unmarshal(raw, &facts); err != nil {
						t.Fatal(err)
					}
					for key, want := range map[string]*string{"source_iteration_id": source, "target_iteration_id": target} {
						value, ok := facts[key]
						if !ok {
							t.Fatalf("new event missing %s: %s", key, raw)
						}
						var got *string
						if err = json.Unmarshal(value, &got); err != nil {
							t.Fatal(err)
						}
						if (got == nil) != (want == nil) || got != nil && *got != *want {
							t.Fatalf("%s=%s want %v", key, value, want)
						}
					}
					var rollover int
					if err = json.Unmarshal(facts["rollover_count"], &rollover); err != nil {
						t.Fatal(err)
					}
					if rollover != wantCount {
						t.Fatalf("rollover=%d want%d", rollover, wantCount)
					}
				}
				if err = rows.Err(); err != nil {
					t.Fatal(err)
				}
				want := count
				if source != nil && target != nil {
					want *= 2
				}
				if n != want {
					t.Fatalf("events=%d want%d", n, want)
				}
			}
			result := move(nil, &a)
			check(result.OperationID, nil, &a, 0, 0)
			result = move(&a, &b)
			check(result.OperationID, &a, &b, 0, 0)
			result = move(&b, nil)
			check(result.OperationID, &b, nil, 0, 0)
			result = move(nil, &b)
			check(result.OperationID, nil, &b, 0, 0)
			joinOp := result.OperationID
			start := lifecycleDraft(t, fx, b, "start")
			start.Start = &iteration.StartDraft{TargetID: b, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}
			lifecycleApply(t, fx, s, start)
			end := lifecycleDraft(t, fx, b, "end")
			reason := "Roll over complete remaining scope"
			end.Reason = &reason
			for _, id := range issues {
				var rev int64
				fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", id).Scan(&rev)
				end.Moves = append(end.Moves, iteration.Move{IssueID: id, ExpectedIssueRevision: rev, ExpectedSourceID: &b, TargetID: &c})
			}
			result = lifecycleApply(t, fx, s, end)
			check(result.OperationID, &b, &c, 0, 1)
			var raw []byte
			fx.QueryRow(t, "SELECT body FROM iteration_snapshot WHERE iteration_id=$1", b).Scan(&raw)
			var snapshot iteration.Snapshot
			if err := json.Unmarshal(raw, &snapshot); err != nil {
				t.Fatal(err)
			}
			found := 0
			for _, event := range snapshot.Events {
				if event.OperationID == joinOp {
					var facts map[string]json.RawMessage
					if err := json.Unmarshal(event.AfterFacts, &facts); err != nil {
						t.Fatal(err)
					}
					if string(facts["source_iteration_id"]) != "null" || string(facts["target_iteration_id"]) != fmt.Sprintf("%q", b) {
						t.Fatalf("frozen transition lost: %s", event.AfterFacts)
					}
					found++
				}
			}
			if found != count {
				t.Fatalf("frozen events=%d want%d", found, count)
			}
			fx.Exec(t, "UPDATE iteration SET name='Renamed live destination' WHERE id=$1", b)
			var after []byte
			fx.QueryRow(t, "SELECT body FROM iteration_snapshot WHERE iteration_id=$1", b).Scan(&after)
			if string(after) != string(raw) {
				t.Fatal("live period edit changed frozen events")
			}
		})
	}
}
