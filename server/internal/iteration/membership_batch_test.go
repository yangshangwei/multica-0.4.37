package iteration

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Batch transport must not become a second membership SQL contract. The one
// intentional difference is returning a participation ID to verify one row.
func TestIterationMembershipBatchSQLMatchesCanonical(t *testing.T) {
	files := []string{"../../pkg/db/queries/iteration.sql", "../../pkg/db/queries/iteration_lifecycle.sql"}
	canonical := ""
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		canonical += string(raw) + "\n"
	}
	raw, e := os.ReadFile("../../pkg/db/queries/iteration_membership_batch.sql")
	if e != nil {
		t.Fatal(e)
	}
	body := func(source, name string) string {
		t.Helper()
		re := regexp.MustCompile(`(?s)-- name: ` + name + ` :\w+\n(.*?)(?:\n-- name:|\z)`)
		m := re.FindStringSubmatch(source)
		if len(m) != 2 {
			t.Fatalf("missing query %s", name)
		}
		s := regexp.MustCompile(`(?m)--[^\n]*`).ReplaceAllString(m[1], "")
		s = strings.TrimSpace(s)
		s = strings.ReplaceAll(s, " RETURNING issue_id", "")
		return strings.Join(strings.Fields(strings.TrimSuffix(s, ";")), " ")
	}
	for _, name := range []string{"AppendIterationLifecycleEvent", "LeaveDeletedIssueIterationParticipation", "AdvanceIterationScopeRevision", "JoinIterationParticipation", "SetIssueCurrentIteration"} {
		if body(canonical, name) != body(string(raw), name+"Batch") {
			t.Errorf("batch query drifted: %s", name)
		}
	}
}
