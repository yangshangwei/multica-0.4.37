//go:build agentintegration && !windows

package main

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

type rk1FailingCapture struct {
	syncErr, closeErr error
	calls             []string
}

func (f *rk1FailingCapture) Sync() error  { f.calls = append(f.calls, "sync"); return f.syncErr }
func (f *rk1FailingCapture) Close() error { f.calls = append(f.calls, "close"); return f.closeErr }

func TestRK1EvidencePublicationFailureRetainsExecutionAndStops(t *testing.T) {
	for _, stage := range []string{"stdout_sync", "stdout_close", "launch_record"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("test-owned evidence I/O fault")
			capture := &rk1FailingCapture{}
			switch stage {
			case "stdout_sync":
				capture.syncErr = failure
			case "stdout_close":
				capture.closeErr = failure
			}
			meta := rk1LaunchMeta{Mode: "real", EvidenceState: "started"}
			var persisted rk1LaunchMeta
			writes := 0
			err := rk1FinalizeLaunchEvidence(capture, &meta, 0, func(next rk1LaunchMeta) error {
				writes++
				// Even a failed final sync may leave complete-looking bytes in
				// the file cache. The wrapper exit must still expose the fault.
				persisted = next
				if stage == "launch_record" {
					return failure
				}
				return nil
			})
			if !errors.Is(err, failure) {
				t.Fatalf("%s persistence error disappeared: %v", stage, err)
			}
			if meta.ExitCode == nil || *meta.ExitCode != 0 || writes != 1 || !slices.Equal(capture.calls, []string{"sync", "close"}) {
				t.Fatalf("evidence fault changed execution or replayed publication: meta=%+v writes=%d calls=%v", meta, writes, capture.calls)
			}
			fault := rk1LaunchEvidenceFault(persisted, rk1EvidenceFailureMarker, false, true)
			if fault == "" || !rk1StopsAfter("incomplete", fault) {
				t.Fatal("successful Provider output with failed evidence could advance the series")
			}
		})
	}
}

func TestRK1EvidencePublicationKeepsProviderFailureSeparate(t *testing.T) {
	for _, executionExit := range []int{0, 7, 43} {
		capture := &rk1FailingCapture{}
		meta := rk1LaunchMeta{EvidenceState: "started"}
		var persisted rk1LaunchMeta
		if err := rk1FinalizeLaunchEvidence(capture, &meta, executionExit, func(next rk1LaunchMeta) error { persisted = next; return nil }); err != nil {
			t.Fatal(err)
		}
		diagnostics := fmt.Sprintf(`level=INFO msg="agent stream protocol summary" provider=claude exit_code=%d`, executionExit)
		if persisted.ExitCode == nil || *persisted.ExitCode != executionExit || rk1LaunchEvidenceFault(persisted, diagnostics, false, true) != "" {
			t.Fatalf("execution exit%d was relabeled as evidence failure: %+v", executionExit, persisted)
		}
		if rk1EvidenceFailureExit(executionExit) == executionExit {
			t.Fatal("evidence failure exit cannot be confused with the execution exit")
		}
	}
}

func TestRK1EvidenceUnfinalizedResultCannotPass(t *testing.T) {
	started := rk1LaunchMeta{EvidenceState: "started"}
	if rk1LaunchEvidenceFault(started, "", false, true) == "" {
		t.Fatal("an unfinalized successful result was accepted")
	}
	if rk1LaunchEvidenceFault(started, "", true, false) != "" {
		t.Fatal("a deliberately killed seed cannot claim completed publication")
	}
	started.EvidenceState, started.EvidenceFailures = "failed", []string{"stdout_close"}
	if rk1LaunchEvidenceFault(started, "", true, false) == "" {
		t.Fatal("Stop hid an explicit evidence failure")
	}
}

func TestRK1EvidenceCachedFinalRecordCannotHideFailedWrapperExit(t *testing.T) {
	executionExit := 0
	meta := rk1LaunchMeta{EvidenceState: "complete", FinishedAt: "2026-11-02T00:00:00Z", ExitCode: &executionExit}
	// The original raw result and cache bytes still say success. Process Wait
	// independently observed the wrapper's evidence-error exit, even if the
	// explicit stderr marker could not be written.
	diagnostics := `time=2026-11-02T00:00:00Z level=INFO msg="agent stream protocol summary" provider=claude model=rk1-fake exit_code=43 event_count=5`
	if observed := rk1HostWrapperExit(diagnostics); observed == nil || *observed != 43 {
		t.Fatal("actual wrapper exit was not retained")
	}
	if rk1LaunchEvidenceFault(meta, diagnostics, false, true) == "" {
		t.Fatal("complete-looking metadata hid a failed durable publication")
	}
	if rk1HostWrapperExit(`level=INFO msg="[claude:stderr] exit_code=43"`) != nil {
		t.Fatal("Provider stderr invented a backend Wait observation")
	}
}

func TestRK1HandoffAnswersRequireTheirExactLineCounts(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	prior := "PRIOR_RUN " + id + " RECORD INCOMPLETE RESULT ABSENT\nRESULT_TEXT NONE RECORDED"
	squad := "MEMBER_RUN " + id + "\nMEMBER_CODE MEMBER-ABCDEF\nWOKEN_BY proof " + id
	if len(rk1PriorAnswerFields(prior)) != 5 || len(rk1SquadAnswer.FindStringSubmatch(squad)) != 5 {
		t.Fatal("the registered two/three-line control was rejected")
	}
	if len(rk1PriorAnswerFields(prior+"\n")) != 5 || len(rk1SquadAnswer.FindStringSubmatch(squad+"\r\n")) != 5 {
		t.Fatal("one terminal line ending must not invent an extra answer line")
	}
	for _, extra := range []struct{ name, prefix, suffix string }{
		{"leading_prose", "An explanation.\n", ""},
		{"trailing_prose", "", "\nAn explanation."},
		{"extra_blank_line", "\n", ""},
	} {
		t.Run(extra.name, func(t *testing.T) {
			if rk1PriorAnswerFields(extra.prefix+prior+extra.suffix) != nil {
				t.Fatal("H1 accepted extra answer lines")
			}
			if rk1SquadAnswer.FindStringSubmatch(extra.prefix+squad+extra.suffix) != nil {
				t.Fatal("H2/H2b accepted extra answer lines")
			}
		})
	}
	if rk1SquadAnswer.FindStringSubmatch("MEMBER_RUN "+id+"\n\nMEMBER_CODE MEMBER-ABCDEF\nWOKEN_BY proof "+id) != nil {
		t.Fatal("cross-line whitespace hid an extra H2 answer line")
	}
	if rk1PriorAnswerFields(prior+"\n\n") != nil || rk1SquadAnswer.FindStringSubmatch(squad+"\n\n") != nil {
		t.Fatal("an extra trailing blank line changed the registered answer format")
	}
}
