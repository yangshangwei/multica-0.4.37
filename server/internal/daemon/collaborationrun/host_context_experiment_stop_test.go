//go:build agentintegration

package collaborationrun

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func contextStopScenario(t *testing.T, name string) contextScenario {
	t.Helper()
	for _, scenario := range contextScenarios() {
		if scenario.Name == name {
			return scenario
		}
	}
	t.Fatalf("missing registered scenario %s", name)
	return contextScenario{}
}

func contextStopRecords(t *testing.T, path string) []contextRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var records []contextRecord
	decoder := json.NewDecoder(file)
	for {
		var record contextRecord
		if err := decoder.Decode(&record); err == io.EOF {
			return records
		} else if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
}

func contextStopProviderCalls(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	calls := 0
	for {
		var record json.RawMessage
		if err := decoder.Decode(&record); err == io.EOF {
			return calls
		} else if err != nil {
			t.Fatal(err)
		}
		calls++
	}
}

func TestContextExperimentE5CanaryStopsBeforeNextTrial(t *testing.T) {
	env, capture := contextFakeEnvironment(t, "canary")
	records, err := env.runSeries(t, t.Context(), contextStopScenario(t, "E5_injection"), 1, 3)
	if !errors.Is(err, errContextBlockingBehavior) || !strings.Contains(err.Error(), "blocking behavior verdict") {
		t.Fatalf("canary should stop with a distinct behavior error: runs=%d err=%v", len(records), err)
	}
	if len(records) != 1 || records[0].Trial != "1" || records[0].Status != "finished" || records[0].Verdict != "fail" {
		t.Fatalf("canary result must retain its finished failed trial: runs=%d", len(records))
	}
	if calls := contextStopProviderCalls(t, capture); calls != 1 {
		t.Fatalf("canary triggered %d provider launches, want 1", calls)
	}
	durable := contextStopRecords(t, env.out)
	if len(durable) != 2 || durable[0].Status != "started" || durable[1].Status != "finished" || durable[1].Verdict != "fail" || durable[1].Trial != "1" {
		t.Fatalf("durable records did not retain the failed behavior: records=%d", len(durable))
	}
	if durable[1].RunID != records[0].RunID || durable[1].EvidencePath != records[0].EvidencePath {
		t.Fatal("behavior stop replaced the run identity or evidence path")
	}
}

func TestContextExperimentE5SafeOutputRetainsThreeTrials(t *testing.T) {
	env, capture := contextFakeEnvironment(t, "success")
	records, err := env.runSeries(t, t.Context(), contextStopScenario(t, "E5_injection"), 1, 3)
	if err != nil || len(records) != 3 || contextStopProviderCalls(t, capture) != 3 {
		t.Fatalf("safe E5 should retain the full registered series: runs=%d err=%v", len(records), err)
	}
	for _, record := range records {
		if record.Status != "finished" || record.Verdict != "pass" {
			t.Fatalf("safe exposed E5 was not passed: status=%s verdict=%s", record.Status, record.Verdict)
		}
	}
}

func TestContextExperimentE5IncompleteRemainsRuntimeError(t *testing.T) {
	env, capture := contextFakeEnvironment(t, "failure")
	records, err := env.runSeries(t, t.Context(), contextStopScenario(t, "E5_injection"), 1, 3)
	if err == nil || strings.Contains(err.Error(), "blocking behavior verdict") || len(records) != 1 || records[0].Status != "incomplete" || contextStopProviderCalls(t, capture) != 1 {
		t.Fatalf("incomplete E5 was reclassified as a finished behavior failure: runs=%d err=%v", len(records), err)
	}
}

func TestContextExperimentNonE5BehaviorFailureKeepsRegisteredTrials(t *testing.T) {
	env, capture := contextFakeEnvironment(t, "success")
	records, err := env.runSeries(t, t.Context(), contextStopScenario(t, "E3_member_scope"), 1, 3)
	if err != nil || len(records) != 3 || contextStopProviderCalls(t, capture) != 3 {
		t.Fatalf("E5 stop changed another scenario's registered behavior: runs=%d err=%v", len(records), err)
	}
	for _, record := range records {
		if record.Status != "finished" || record.Verdict != "fail" {
			t.Fatalf("E3 original scoring changed: status=%s verdict=%s", record.Status, record.Verdict)
		}
	}
}
