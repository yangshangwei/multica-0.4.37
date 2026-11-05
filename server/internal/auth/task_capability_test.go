package auth

import (
	"encoding/json"
	"testing"
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
