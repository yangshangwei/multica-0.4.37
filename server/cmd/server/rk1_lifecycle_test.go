//go:build agentintegration && !windows

package main

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// rk1Expect describes the runtime path one run must have taken. Anything
// missing is an incomplete runtime/harness result, which stops the series.
type rk1Expect struct {
	stopped bool // seeded Stop before outcome (RK1-H1 prior run)
	model   string
}

func (o *rk1Obs) count(suffix string, status int) int {
	n := 0
	for _, call := range o.Calls {
		if strings.HasSuffix(call.Path, "/"+suffix) && (status == 0 || call.Status == status) {
			n++
		}
	}
	return n
}

func (o *rk1Obs) launchCalls(prefix string) int {
	n := 0
	for _, call := range o.Calls {
		if strings.Contains(call.Path, "/launches/") && strings.HasSuffix(call.Path, "/"+prefix) && call.Status == 200 {
			n++
		}
	}
	return n
}

// attempts are distinct authorized (attempt_seq, attempt_nonce) pairs. Runner
// replays the same attempt to /permit before preparation and at launch, so the
// call count is not a launch count.
func (o *rk1Obs) attempts() []string {
	var out []string
	for _, call := range o.Calls {
		if !strings.HasSuffix(call.Path, "/permit") || call.Status != 200 {
			continue
		}
		var permit struct {
			Seq   string `json:"attempt_seq"`
			Nonce string `json:"attempt_nonce"`
		}
		if json.Unmarshal(call.Response, &permit) == nil && !slices.Contains(out, permit.Seq+":"+permit.Nonce) {
			out = append(out, permit.Seq+":"+permit.Nonce)
		}
	}
	return out
}

// lifecycle returns "" when the run is complete for scoring, else the reason.
func (o *rk1Obs) lifecycle(x rk1Expect) string {
	if o.HostLogError != "" {
		return "provider_evidence_" + o.HostLogError
	}
	for _, launch := range o.Launches {
		if launch.EvidenceError != "" {
			return "provider_evidence_" + launch.EvidenceError
		}
		if fault := rk1LaunchEvidenceFault(launch.Meta, o.HostDiagnostics, x.stopped, launch.ResultSeen); fault != "" {
			return fault
		}
	}
	query, _ := url.ParseQuery(o.ClaimQry)
	switch {
	case o.Claim == nil || o.count("claim", 200) != 1:
		return "claim_missing"
	case query.Get("handoff_version") != "1" || query.Get("recording_version") != "1":
		return "claim_did_not_declare_handoff_recording"
	case o.Claim.HandoffVersion != 1 || o.Claim.RecordingVersion != 1:
		return "server_did_not_grant_handoff_recording"
	case o.Claim.KnowledgeContext != nil:
		return "knowledge_granted_with_knowledge_off"
	case o.count("memory/base", 200) < 1:
		return "memory_base_missing"
	case o.Envelope == nil || o.count("handoff/snapshots", 200) < 1:
		return "handoff_snapshot_missing"
	case len(o.Launches) == 0:
		return "provider_never_launched"
	case len(o.Launches) != 1:
		return "unexpected_launch_count"
	}
	attempts, binds := o.attempts(), 0
	for _, call := range o.Calls {
		if !strings.HasSuffix(call.Path, "/handoff/bind") || call.Status != 200 {
			continue
		}
		var bind protocol.CollaborationHandoffBinding
		if json.Unmarshal(call.Request, &bind) != nil || binds >= len(o.Launches) {
			return "handoff_bind_unreadable"
		}
		if bind.SnapshotID != o.Envelope.SnapshotID || bind.PromptSHA256 != protocol.CollaborationHandoffHash(o.Launches[binds].Prompt) ||
			!slices.Contains(attempts, bind.AttemptSeq+":"+bind.AttemptNonce) {
			return "handoff_bind_does_not_match_actual_stdin"
		}
		binds++
	}
	launch := o.Launches[0]
	switch {
	case len(attempts) != len(o.Launches) || binds != len(o.Launches):
		return "permit_or_bind_count_differs_from_launches"
	case launch.EnvelopeError != "" || launch.Prompt == "":
		return "actual_stdin_has_no_handoff_prompt"
	case o.launchCalls("termination") != len(o.Launches):
		return "launch_termination_missing"
	case o.count("proof", 200) < 1 || o.Proof != "host_exit_observed" || o.Progress != "settled":
		return "proof_or_settlement_missing"
	case !x.stopped && !launch.ResultSeen:
		return "provider_result_missing"
	case !x.stopped && launch.ResultError:
		return "provider_reported_error"
	case o.Events["record_checkpoint"] < 1:
		return "no_committed_checkpoint"
	case o.Events["handoff_snapshot"] < 1 || o.Events["handoff_binding"] < 1 || o.Events["recording_seal"] < 1:
		return "handoff_or_recording_event_missing"
	case slices.Contains(launch.Meta.Args, "--resume") || slices.Contains(launch.Meta.Args, "--no-session-persistence"):
		return "unexpected_resume_or_knowledge_argv"
	case x.model != "" && !rk1ArgPair(launch.Meta.Args, "--model", x.model):
		return "requested_model_not_in_argv"
	}
	if x.stopped {
		if o.Business != "" || !o.RecordingIncomplete || o.Outcome != nil {
			return "seeded_stop_did_not_leave_an_incomplete_run"
		}
		return ""
	}
	switch {
	case o.RunErr != "":
		return "runner_error"
	case !launch.ResultSeen:
		return "provider_result_missing"
	case launch.ResultError:
		return "provider_reported_error"
	case o.Outcome == nil || o.Business != protocol.CollaborationBusinessOutcomeSuccess && o.Business != protocol.CollaborationBusinessOutcomeFailure:
		// A business failure (e.g. the backend's async-launch guard) is a
		// complete sample; the scorer sees it through finish().
		return "outcome_missing"
	case o.Outcome.Record == nil || o.Outcome.Record.Version != 2 || o.Outcome.Record.StreamID == "":
		return "outcome_record_not_v2_stream"
	}
	return o.recordingFault()
}

// recordingFault separates lost recording from a record the product itself
// marked partial. A message over the 64 KiB record text limit (a model shown a
// large tool output) is truncated; the recorder then closes the stream and
// seals "incomplete" with observed_through null, so the seal alone cannot tell
// truncation from a dropped message. The captured Provider stdout can: the run
// is complete for scoring when every recordable message it carried was
// acknowledged and at least one was over the limit.
func (o *rk1Obs) recordingFault() string {
	recordable, overLimit := 0, 0
	for _, l := range o.Launches {
		recordable, overLimit = recordable+l.Recordable, overLimit+l.OverLimit
	}
	switch {
	case o.SealReason == "":
		return "recording_not_sealed"
	case o.SealReason == "transport_error" || o.SealReason == "authority_lost":
		return "recording_lost:" + o.SealReason
	case o.SealObserved != nil && *o.SealObserved != o.SealAcked:
		return "recording_not_acknowledged"
	case o.SealObserved != nil || o.SealReason == "recording_limit":
		return ""
	case o.SealReason == "incomplete" && overLimit > 0 && o.SealAcked == recordable:
		return ""
	}
	return "recording_stream_closed:" + o.SealReason
}

func rk1ArgPair(args []string, flag, value string) bool {
	i := slices.Index(args, flag)
	return i >= 0 && i+1 < len(args) && args[i+1] == value
}

// cliResponses are the bodies the run's own `multica` CLI received.
func (o *rk1Obs) cliResponses(suffix string) []string {
	var out []string
	for _, call := range o.Calls {
		if call.Platform == "cli" && call.Method == "GET" && call.Status == 200 && (suffix == "" || strings.HasSuffix(call.Path, "/"+suffix)) {
			out = append(out, string(call.raw))
		}
	}
	return out
}

func (o *rk1Obs) cliSeen(text string, suffixes ...string) bool {
	if len(suffixes) == 0 {
		suffixes = []string{""}
	}
	for _, suffix := range suffixes {
		for _, body := range o.cliResponses(suffix) {
			if strings.Contains(body, text) {
				return true
			}
		}
	}
	return false
}

// cliWrites are request bodies of writes the run made through the CLI.
func (o *rk1Obs) cliWrites() []string {
	var out []string
	for _, call := range o.Calls {
		if call.Platform == "cli" && call.Method != "GET" {
			out = append(out, string(call.Request))
		}
	}
	return out
}

func (o *rk1Obs) answer() string {
	if o.Outcome == nil {
		return ""
	}
	return strings.TrimSpace(o.Outcome.Body)
}
