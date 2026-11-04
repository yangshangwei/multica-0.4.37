//go:build agentintegration && !windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	rk1kAlpha = "RK1K_APPROVED_ALPHA: launch only in the reviewed window."
	rk1kBeta  = "RK1K_APPROVED_BETA: retain the original decision key."
)

func rk1kSaveSample(t *testing.T, dir string, sample rk1kSample) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "samples"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := rk1WriteJSON(filepath.Join(dir, "samples", sample.Label+".json"), sample); err != nil {
		t.Fatal(err)
	}
}

func (f *rk1kFixture) claimed(id string) daemon.Task {
	f.t.Helper()
	task, err := f.ordinaryDriver().Claim(f.t.Context())
	if err != nil || task == nil || task.ID != id {
		f.t.Fatalf("actual producer claim: expected %s; task=%v error=%v", id, task != nil, err)
	}
	observed := false
	for _, call := range f.allTrace.snapshot() {
		if call.Status != 200 || !strings.HasSuffix(call.Path, "/tasks/claim") {
			continue
		}
		var response struct {
			Task struct {
				ID        string          `json:"id"`
				Knowledge json.RawMessage `json:"knowledge_context"`
			} `json:"task"`
		}
		if json.Unmarshal(call.Response, &response) != nil || response.Task.ID != id {
			continue
		}
		if task.KnowledgeContext != nil {
			var descriptor protocol.TaskKnowledgeClaimDescriptor
			if json.Unmarshal(response.Task.Knowledge, &descriptor) != nil || descriptor != *task.KnowledgeContext ||
				bytes.Contains(response.Task.Knowledge, []byte(f.source)) || bytes.Contains(response.Task.Knowledge, []byte(rk1kAlpha)) ||
				bytes.Contains(response.Task.Knowledge, []byte(rk1kBeta)) {
				f.t.Fatal("actual claimed knowledge descriptor leaked or changed its source-free binding")
			}
		}
		observed = true
	}
	if !observed {
		f.t.Fatal("actual claim response is missing from persisted trace")
	}
	return *task
}

func (f *rk1kFixture) executeOrdinary(label, surface string, task daemon.Task, native, fault string, expected []string, baseline bool) rk1kSample {
	f.t.Helper()
	sample := rk1kSample{Label: label, Surface: surface, State: "started", Verdict: "unclassified",
		Identity: rk1kIdentity{Kind: "task", TaskID: task.ID, Workspace: task.WorkspaceID, Agent: task.AgentID},
		Criteria: map[string]bool{}, ArtifactHash: map[string]string{}}
	rk1kSaveSample(f.t, f.dir, sample)
	if !baseline && (task.KnowledgeContext == nil || task.KnowledgeContext.TaskID != task.ID) {
		f.t.Fatal("actual ordinary claim did not negotiate knowledge")
	}
	if surface == "quick_create" && task.IssueID != "" {
		f.t.Fatal("quick-create claim was fabricated with a prelinked issue")
	}
	plan := rk1Plan{Mode: "fake", Behavior: surface, Fault: fault, Params: map[string]string{
		"native_id": native, "native_store": f.nativeStore, "cli": f.env.cli, "project": f.project}}
	if err := rk1WriteJSON(filepath.Join(f.plans, sample.Identity.key()+".json"), plan); err != nil {
		f.t.Fatal(err)
	}
	result, runErr := f.driver.Execute(f.t.Context(), task)
	row, err := f.q.GetAgentTask(f.t.Context(), util.MustParseUUID(task.ID))
	if err != nil {
		f.t.Fatal(err)
	}
	for _, call := range f.allTrace.snapshot() {
		if strings.Contains(call.Path, "/tasks/"+task.ID+"/") || strings.HasSuffix(call.Path, "/tasks/claim") ||
			strings.Contains(call.Path, "/projects/") {
			sample.Calls = append(sample.Calls, call)
		}
	}
	launchDirs, err := filepath.Glob(filepath.Join(f.plans, sample.Identity.key(), "launch-*"))
	if err != nil {
		f.t.Fatal(err)
	}
	var prompt, answer string
	inputMatches, resultSeen := false, false
	for _, dir := range launchDirs {
		raw, err := os.ReadFile(filepath.Join(dir, "launch.json"))
		var launch rk1kLaunchMeta
		if err != nil || json.Unmarshal(raw, &launch) != nil {
			sample.Reason = "launch_evidence_missing"
			continue
		}
		sample.Launches = append(sample.Launches, launch)
		if launch.EvidenceState != "complete" || !launch.ResultSeen {
			sample.Reason = "launch_evidence_incomplete"
		}
		frame, err := os.ReadFile(filepath.Join(dir, "stdin-first.jsonl"))
		if err != nil {
			sample.Reason = "input_evidence_missing"
			continue
		}
		prompt, err = rk1kPrompt(frame)
		if err != nil {
			sample.Reason = "invalid_input_frame"
			continue
		}
		items, err := rk1kContributions(prompt)
		var bodies []string
		for _, item := range items {
			bodies = append(bodies, item.Content)
		}
		slices.Sort(bodies)
		want := append([]string{}, expected...)
		slices.Sort(want)
		inputMatches = err == nil && slices.Equal(bodies, want)
		out, err := os.ReadFile(filepath.Join(dir, "stdout.jsonl"))
		if err != nil {
			sample.Reason = "stdout_missing"
			continue
		}
		for _, line := range bytes.Split(out, []byte{'\n'}) {
			var message struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal(line, &message) == nil && message.Type == "result" {
				resultSeen, answer = true, message.Result
			}
		}
		for _, file := range []string{"launch.json", "stdin-first.jsonl", "stdout.jsonl"} {
			path := filepath.Join(dir, file)
			body, err := os.ReadFile(path)
			if err != nil {
				sample.Reason = "artifact_missing"
				continue
			}
			sample.ArtifactHash[path] = protocol.CollaborationKnowledgeHash(string(body))
		}
	}
	var auths, nativeACK, terminated, completeFence, cliRead bool
	for _, call := range sample.Calls {
		if strings.HasSuffix(call.Path, "/knowledge-context/authorize") && call.Status == 200 {
			var request protocol.TaskKnowledgeAuthorizeRequest
			if json.Unmarshal(call.Request, &request) == nil && request.PromptSHA256 == protocol.CollaborationKnowledgeHash(prompt) {
				auths = true
			}
		}
		if strings.HasSuffix(call.Path, "/knowledge-context/observations") && call.Status == 200 {
			var observation protocol.TaskKnowledgeObservation
			var ack protocol.TaskKnowledgeObservationAck
			if json.Unmarshal(call.Request, &observation) == nil && json.Unmarshal(call.Response, &ack) == nil && ack.Accepted {
				nativeACK = nativeACK || observation.Kind == "native_session" && observation.NativeSessionID == native
				terminated = terminated || observation.Kind == "terminated" && observation.WaitObserved
			}
		}
		if strings.HasSuffix(call.Path, "/complete") && call.Status == 200 {
			if baseline {
				completeFence = true
			} else {
				var request struct {
					Fence *protocol.TaskKnowledgeFence `json:"knowledge_fence"`
				}
				if json.Unmarshal(call.Request, &request) == nil && request.Fence != nil && task.KnowledgeContext != nil {
					completeFence = request.Fence.ClaimGeneration == task.KnowledgeContext.ClaimGeneration &&
						request.Fence.DispatchedAt == task.KnowledgeContext.DispatchedAt
				}
			}
		}
		cliRead = cliRead || call.Path == "/api/projects/"+f.project && call.Platform == "cli" && call.Credential == "mat" && call.Status == 200
	}
	sample.Criteria["actual_input_matches_current_manifest"] = inputMatches
	sample.Criteria["actual_task_scoped_project_read"] = cliRead
	sample.Criteria["exact_complete_fence"] = completeFence
	sample.Criteria["native_observation_positive_ACK"] = nativeACK
	sample.Criteria["termination_observed"] = terminated
	sample.Criteria["actual_authorization_prompt_hash"] = auths
	sample.Criteria["persisted_terminal"] = row.Status == "completed" && result.Status == "completed"
	if surface == "quick_create" {
		sample.Criteria["quick_create_link_after_completion"] = row.IssueID.Valid && task.IssueID == ""
		var count int
		if err := f.base.managed.pool.QueryRow(f.t.Context(), "SELECT count(*) FROM issue WHERE origin_type='quick_create' AND origin_id=$1", task.ID).Scan(&count); err != nil {
			f.t.Fatal(err)
		}
		sample.Criteria["quick_create_unique_origin"] = count == 1
	}
	wantAnswer := "NONE"
	if len(expected) > 0 {
		sample.Criteria["semantic_material_present"] = true
		for _, body := range expected {
			sample.Criteria["semantic_material_present"] = sample.Criteria["semantic_material_present"] && strings.Contains(answer, body)
		}
	} else {
		sample.Criteria["semantic_material_present"] = answer == wantAnswer
	}
	sample.State, sample.Verdict = "finished", "pass"
	if runErr != nil || sample.Reason != "" || !resultSeen || len(sample.Launches) != 1 {
		sample.State, sample.Verdict = "incomplete", "unclassified"
		if sample.Reason == "" {
			sample.Reason = "missing_or_failed_provider_result"
		}
	} else {
		for key, yes := range sample.Criteria {
			if baseline && (key == "native_observation_positive_ACK" || key == "termination_observed" || key == "actual_authorization_prompt_hash") {
				continue
			}
			if !yes {
				if key == "actual_input_matches_current_manifest" || key == "semantic_material_present" {
					sample.Verdict = "fail"
				} else {
					sample.State, sample.Verdict, sample.Reason = "incomplete", "unclassified", "ordinary_lifecycle_"+key
					break
				}
			}
		}
	}
	rk1kSaveSample(f.t, f.dir, sample)
	return sample
}

func (f *rk1kFixture) hostSample(h *rk1Fixture, label, role, run string, plan rk1Plan, expected []string) rk1kSample {
	f.t.Helper()
	runErr, duringErr := h.execute(run, plan, nil)
	o := h.observe(run, runErr)
	sample := rk1kSample{Label: label, Surface: role, State: "finished", Verdict: "pass",
		Identity: rk1kIdentity{Kind: "run", RunID: run}, Calls: o.Calls,
		Criteria: map[string]bool{}, ArtifactHash: map[string]string{}}
	if o.Claim != nil {
		sample.Identity.Host = &o.Claim.Identity
	}
	var allBodies []string
	var answer string
	var authHash bool
	for index, launch := range o.Launches {
		dir := filepath.Join(f.plans, run, "launch-"+strconv.Itoa(index+1))
		raw, err := os.ReadFile(filepath.Join(dir, "launch.json"))
		var meta rk1kLaunchMeta
		if err == nil && json.Unmarshal(raw, &meta) == nil {
			sample.Launches = append(sample.Launches, meta)
			if meta.EvidenceState != "complete" || !meta.ResultSeen || !meta.Identity.valid() || meta.Identity.RunID != run {
				sample.Reason = "host_launch_evidence_incomplete"
			}
		} else {
			sample.Reason = "host_launch_evidence_missing"
		}
		stdout, err := os.ReadFile(filepath.Join(dir, "stdout.jsonl"))
		if err != nil {
			sample.Reason = "host_stdout_missing"
		} else {
			for _, line := range bytes.Split(stdout, []byte{'\n'}) {
				var result struct {
					Type   string `json:"type"`
					Result string `json:"result"`
				}
				if json.Unmarshal(line, &result) == nil && result.Type == "result" {
					answer = result.Result
				}
			}
		}
		for _, file := range []string{"launch.json", "stdin-first.jsonl", "stdout.jsonl"} {
			path := filepath.Join(dir, file)
			raw, err := os.ReadFile(path)
			if err != nil {
				sample.Reason = "host_artifact_missing"
				continue
			}
			sample.ArtifactHash[path] = protocol.CollaborationKnowledgeHash(string(raw))
		}
		items, err := rk1kContributions(launch.Prompt)
		if err != nil {
			sample.Reason = "knowledge_array_invalid"
		}
		for _, item := range items {
			allBodies = append(allBodies, item.Content)
		}
		for _, call := range o.Calls {
			if strings.HasSuffix(call.Path, "/knowledge-context/authorize") && call.Status == 200 {
				var request protocol.CollaborationRunKnowledgeAuthorization
				if json.Unmarshal(call.Request, &request) == nil && request.PromptSHA256 == protocol.CollaborationHandoffHash(launch.Prompt) {
					authHash = true
				}
			}
		}
	}
	slices.Sort(allBodies)
	want := append([]string{}, expected...)
	slices.Sort(want)
	seed := plan.Params["seed"] == "true"
	sample.Criteria["actual_input_matches_current_manifest"] = slices.Equal(allBodies, want)
	sample.Criteria["semantic_material_present"] = answer == "NONE"
	if len(expected) > 0 {
		sample.Criteria["semantic_material_present"] = true
		for _, body := range expected {
			sample.Criteria["semantic_material_present"] = sample.Criteria["semantic_material_present"] && strings.Contains(answer, body)
		}
	}
	sample.Criteria["fresh_host_namespace"] = len(o.Launches) == 1 && !slices.Contains(o.Launches[0].Meta.Args, "--resume")
	sample.Criteria["host_persistence_policy"] = seed || len(o.Launches) == 1 && slices.Contains(o.Launches[0].Meta.Args, "--no-session-persistence")
	sample.Criteria["actual_role"] = o.Envelope != nil && o.Envelope.Assignment.Role == role
	sample.Criteria["actual_authorization_prompt_hash"] = seed || authHash
	sample.Criteria["actual_host_exit_proof"] = o.Proof == "host_exit_observed" && o.Progress == "settled"
	sample.Criteria["recording_complete"] = o.recordingFault() == ""
	sample.Criteria["native_pointer_not_reused"] = o.Claim != nil && (seed || o.Claim.KnowledgeContext != nil && o.Claim.KnowledgeContext.Namespace == run)
	if role == "member" {
		sample.Criteria["admitted_parent"] = o.Envelope != nil && o.Envelope.Assignment.ParentRunID != ""
	}
	if role == "continuation" {
		sample.Criteria["actual_wakeup"] = o.Envelope != nil && len(o.Envelope.WakeupCauses.Items) > 0
	}
	if runErr != nil || duringErr != nil || sample.Reason != "" || len(o.Launches) != 1 || !o.Launches[0].ResultSeen {
		sample.State, sample.Verdict, sample.Reason = "incomplete", "unclassified", "host_runtime_or_evidence_incomplete"
	} else {
		for key, yes := range sample.Criteria {
			if !yes {
				if key == "actual_input_matches_current_manifest" || key == "semantic_material_present" {
					sample.Verdict = "fail"
				} else {
					sample.State, sample.Verdict, sample.Reason = "incomplete", "unclassified", "host_lifecycle_"+key
					break
				}
			}
		}
	}
	rk1kSaveSample(f.t, f.dir, sample)
	return sample
}

func rk1kRequireSample(t *testing.T, sample rk1kSample) {
	t.Helper()
	if sample.State != "finished" || sample.Verdict != "pass" {
		t.Fatalf("sample %s %s/%s: %s criteria=%v", sample.Label, sample.State, sample.Verdict, sample.Reason, sample.Criteria)
	}
}

func TestRK1KnowledgeFakeProof(t *testing.T) {
	rk1kRequireDatabase(t)
	out := os.Getenv("RK1K_FAKE_PROOF_OUT")
	if out == "" {
		out = t.TempDir()
	}
	var samples []rk1kSample
	if !t.Run("E0b", func(t *testing.T) {
		f := newRK1KnowledgeFixture(t, filepath.Join(out, "E0b"), false)
		f.h.TaskService.KnowledgeContextEnabled, f.svc.KnowledgeContextEnabled = false, false
		chat := f.chat(f.base.ownerToken)
		id := f.chatMessage(f.base.ownerToken, chat)
		sample := f.executeOrdinary("E0b", "chat", f.claimed(id), uuid.NewString(), "", nil, true)
		rk1kRequireSample(t, sample)
		samples = append(samples, sample)
	}) {
		t.FailNow()
	}
	if !t.Run("K1", func(t *testing.T) {
		f := newRK1KnowledgeFixture(t, filepath.Join(out, "K1"), false)
		f.updateMemory("Source evidence for policy alpha.")
		alpha, pending := f.propose(rk1kAlpha, nil)
		var approved, corrected protocol.CollaborationKnowledgeReceipt
		var beta protocol.CollaborationKnowledgeProposal
		for _, state := range []string{"pending", "approved", "corrected", "withdrawn"} {
			expected := []string{}
			switch state {
			case "approved":
				_, approved = f.approve(alpha, pending)
				expected = []string{rk1kAlpha}
			case "corrected":
				f.updateMemory("Corrected source evidence for policy beta.")
				var before struct {
					Revision string `json:"knowledge_revision"`
				}
				beforeResponse := f.human(f.base.ownerToken, "GET", "/api/projects/"+f.project+"/knowledge", nil, nil).Want(200).JSON(&before)
				beta, pending = f.propose(rk1kBeta, &alpha.CandidateID)
				var retained struct {
					Target   protocol.CollaborationKnowledgeTarget `json:"target"`
					Revision string                                `json:"knowledge_revision"`
					Items    []struct {
						ID      string `json:"candidate_id"`
						State   string `json:"state"`
						Version string `json:"version"`
						Hash    string `json:"content_sha256"`
					} `json:"items"`
				}
				pendingResponse := f.human(f.base.ownerToken, "GET", "/api/projects/"+f.project+"/knowledge", nil, nil).Want(200).JSON(&retained)
				var sourcePage struct {
					Items []struct {
						ID      string `json:"candidate_id"`
						State   string `json:"state"`
						Version string `json:"version"`
					} `json:"items"`
				}
				sourceResponse := f.human(f.sourceToken, "GET", "/api/collaboration/issues/"+f.source+"/knowledge-candidates", nil, nil).Want(200).JSON(&sourcePage)
				if before.Revision == "" || retained.Revision != before.Revision || retained.Target != alpha.Target {
					t.Fatal("pending correction changed the approved target revision or identity")
				}
				approvedCount := 0
				for _, item := range retained.Items {
					if item.State == "approved" {
						approvedCount++
						if item.ID != alpha.CandidateID || item.Version != approved.Version || item.Hash != protocol.CollaborationKnowledgeHash(alpha.Content) {
							t.Fatal("pending correction replaced the exact approved v1 contribution")
						}
					}
				}
				pendingSeen := false
				for _, item := range sourcePage.Items {
					if item.ID == beta.CandidateID && item.State == "pending" && item.Version == pending.Version {
						pendingSeen = true
						break
					}
				}
				if approvedCount != 1 || !pendingSeen {
					t.Fatal("pending correction did not retain the approved target contribution and source-side pending version")
				}
				if err := rk1WriteJSON(filepath.Join(f.dir, "pending-v2-target.json"), map[string]json.RawMessage{
					"before": beforeResponse.Body.Bytes(), "pending": pendingResponse.Body.Bytes(), "source": sourceResponse.Body.Bytes(),
				}); err != nil {
					t.Fatal(err)
				}
				_, corrected = f.approve(beta, pending)
				expected = []string{rk1kBeta}
			case "withdrawn":
				f.withdraw(beta, corrected)
			}
			h, target, agent, _ := f.newConsumer(state, false)
			f.submitConsumer(h, target, "agent", agent)
			run := h.nextRun()
			plan := rk1Plan{Mode: "fake", Behavior: "knowledge", Params: map[string]string{"cli": f.env.cli}}
			sample := f.hostSample(h, "K1-"+state, "agent", run, plan, expected)
			rk1kRequireSample(t, sample)
			samples = append(samples, sample)
		}
	}) {
		t.FailNow()
	}
	if !t.Run("K2", func(t *testing.T) {
		f := newRK1KnowledgeFixture(t, filepath.Join(out, "K2"), true)
		f.updateMemory("Independent source alpha for native history.")
		alpha, pending := f.propose(rk1kAlpha, nil)
		approval, approved := f.approveLostResponse(alpha, pending)
		chat, native := f.chat(f.base.ownerToken), uuid.NewString()
		firstID := f.chatMessage(f.base.ownerToken, chat)
		first := f.executeOrdinary("K2-fresh", "chat", f.claimed(firstID), native, "", []string{rk1kAlpha}, false)
		rk1kRequireSample(t, first)
		samples = append(samples, first)
		beta, pending := f.propose(rk1kBeta, nil)
		f.approve(beta, pending)
		secondID := f.chatMessage(f.base.ownerToken, chat)
		secondTask := f.claimed(secondID)
		if secondTask.KnowledgeContext == nil || secondTask.KnowledgeContext.Session.NativeSessionID != native {
			t.Fatal("supported second claim did not select actual S1")
		}
		f.transport.mu.Lock()
		f.transport.dropPath, f.transport.dropped = "/api/daemon/tasks/"+secondID+"/knowledge-context/prepare", false
		f.transport.mu.Unlock()
		second := f.executeOrdinary("K2-resume", "chat", secondTask, native, "", []string{rk1kAlpha, rk1kBeta}, false)
		rk1kRequireSample(t, second)
		samples = append(samples, second)
		var repeated [][]byte
		for _, call := range second.Calls {
			if call.Path == "/api/daemon/tasks/"+secondID+"/knowledge-context/prepare" {
				repeated = append(repeated, call.Request)
			}
		}
		if len(repeated) != 2 || !bytes.Equal(repeated[0], repeated[1]) {
			t.Fatal("committed preparation loss changed original request bytes or retry count")
		}
		thirdID := f.chatMessage(f.base.ownerToken, chat)
		thirdTask := f.claimed(thirdID)
		if thirdTask.KnowledgeContext == nil || thirdTask.KnowledgeContext.Session.NativeSessionID != native {
			t.Fatal("third claim was not validly frozen before source revocation")
		}
		ready, release := make(chan []byte, 1), make(chan struct{})
		var paused atomic.Bool
		f.transport.hooks(func(request *http.Request, body []byte) {
			if request.URL.Path == "/api/daemon/tasks/"+thirdID+"/knowledge-context/authorize" && paused.CompareAndSwap(false, true) {
				ready <- append([]byte(nil), body...)
				select {
				case <-release:
				case <-request.Context().Done():
				}
			}
		}, nil)
		done := make(chan daemon.TaskResult, 1)
		failures := make(chan error, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		go func() {
			result, err := f.driver.Execute(ctx, thirdTask)
			done <- result
			failures <- err
		}()
		var frozen []byte
		select {
		case frozen = <-ready:
		case <-ctx.Done():
			t.Fatal("third preparation did not reach authorization barrier")
		}
		var authorization protocol.TaskKnowledgeAuthorizeRequest
		if json.Unmarshal(frozen, &authorization) != nil || authorization.Session.NativeSessionID != native ||
			authorization.Session != thirdTask.KnowledgeContext.Session {
			t.Fatal("third authorization replaced the valid native binding")
		}
		f.human(f.sourceToken, "POST", "/api/collaboration/issues/"+f.source+"/participants/"+f.base.owner+"/remove",
			f.sourceMutation(), nil).Want(200)
		close(release)
		result, err := <-done, <-failures
		f.transport.hooks(nil, nil)
		if err != nil || result.FailureReason != "knowledge_context_changed" {
			t.Fatalf("frozen launch refusal: %s error=%v", result.FailureReason, err)
		}
		dirs, _ := filepath.Glob(filepath.Join(f.plans, "task-"+thirdID, "launch-*"))
		if len(dirs) != 0 {
			t.Fatal("revoked third authorization started another Provider")
		}
		thirdRow, err := f.q.GetAgentTask(t.Context(), util.MustParseUUID(thirdID))
		if err != nil || thirdRow.Status != "failed" {
			t.Fatal("frozen third refusal did not persist a failed terminal")
		}
		var originalPrepare json.RawMessage
		for _, call := range f.allTrace.snapshot() {
			if call.Path == "/api/daemon/tasks/"+thirdID+"/knowledge-context/prepare" && call.Status == 200 {
				originalPrepare = append(json.RawMessage(nil), call.Request...)
			}
		}
		if len(originalPrepare) == 0 {
			t.Fatal("third preparation was not actually committed before source revocation")
		}
		replay := f.base.managed.request(t, f.ordinaryToken, false, "POST",
			"/api/daemon/tasks/"+thirdID+"/knowledge-context/prepare", originalPrepare, nil)
		replay.Want(409)
		if bytes.Contains(replay.Body.Bytes(), []byte(rk1kAlpha)) || bytes.Contains(replay.Body.Bytes(), []byte(rk1kBeta)) {
			t.Fatal("original preparation replay disclosed revoked source material")
		}
		var receipt protocol.CollaborationKnowledgeReceipt
		f.human(f.base.ownerToken, "GET", "/api/collaboration/knowledge-candidates/"+alpha.CandidateID+"/operations/"+approval.RequestID,
			nil, nil).Want(200).JSON(&receipt)
		if receipt.RequestID != approval.RequestID || receipt.StateRevision != approved.StateRevision {
			t.Fatal("original approval key recovery lost its receipt")
		}
		freshID := f.chatMessage(f.base.ownerToken, chat)
		fresh := f.claimed(freshID)
		if fresh.KnowledgeContext == nil || fresh.KnowledgeContext.Session.NativeSessionID != "" ||
			fresh.KnowledgeContext.Session.Generation == thirdTask.KnowledgeContext.Session.Generation {
			t.Fatal("a supported claim after prior revocation did not choose a separate fresh generation")
		}
		rk1kSaveSample(t, f.dir, rk1kSample{Label: "K2-frozen-refusal", Surface: "chat",
			Identity: rk1kIdentity{Kind: "task", TaskID: thirdID, Workspace: thirdTask.WorkspaceID, Agent: thirdTask.AgentID},
			State:    "finished", Verdict: "pass", Reason: "expected knowledge_context_changed; zero third process",
			Calls: f.allTrace.snapshot(), Criteria: map[string]bool{"frozen_valid_before_revocation": true,
				"original_receipt_recovered": true, "original_prepare_replay_refused": true,
				"supported_new_claim_selects_fresh_without_launch": true, "third_physical_starts_zero": len(dirs) == 0}})
	}) {
		t.FailNow()
	}
	if !t.Run("K3", func(t *testing.T) {
		f := newRK1KnowledgeFixture(t, filepath.Join(out, "K3"), false)
		f.updateMemory("Shared approved policy source for the producer matrix.")
		policy, pending := f.propose(rk1kAlpha, nil)
		f.approve(policy, pending)
		for _, surface := range []string{"issue", "comment", "chat", "quick_create", "autopilot"} {
			ids := f.producer(surface)
			if len(ids) != 1 || !rk1kUUID(ids[0]) {
				t.Fatalf("%s producer did not enqueue one real task: %v", surface, ids)
			}
			sample := f.executeOrdinary("K3-"+surface, surface, f.claimed(ids[0]), uuid.NewString(), "", []string{rk1kAlpha}, false)
			rk1kRequireSample(t, sample)
			samples = append(samples, sample)
		}
		for _, role := range []string{"leader", "member", "continuation"} {
			f := newRK1KnowledgeFixture(t, filepath.Join(f.dir, "role-"+role), false)
			f.updateMemory("Shared approved policy source for the producer matrix.")
			policy, pending := f.propose(rk1kAlpha, nil)
			f.approve(policy, pending)
			h, target, leader, member := f.newConsumer(role, true)
			targetRow := f.consumerTarget(h, target)
			f.submitConsumer(h, target, "squad", util.UUIDToString(targetRow.ResourceID))
			if role != "leader" {
				f.h.TaskService.KnowledgeContextEnabled = false
				seedRun := h.nextRunForIssue()
				seed := f.hostSample(h, "K3-"+role+"-seed-root", "leader", seedRun,
					rk1Plan{Mode: "fake", Behavior: "delegate", Params: map[string]string{"cli": f.env.cli,
						"seed": "true", "comment": "[@Member](mention://agent/" + member + ") Perform the admitted verification."}}, nil)
				rk1kRequireSample(t, seed)
				samples = append(samples, seed)
				if role == "continuation" {
					seedMember := f.hostSample(h, "K3-continuation-seed-member", "member", h.nextRunForIssue(),
						rk1Plan{Mode: "fake", Behavior: "knowledge", Params: map[string]string{"cli": f.env.cli, "seed": "true"}}, nil)
					rk1kRequireSample(t, seedMember)
					samples = append(samples, seedMember)
				}
				f.h.TaskService.KnowledgeContextEnabled = true
			}
			label := role
			if role == "leader" {
				label = "root"
			}
			sample := f.hostSample(h, "K3-"+label, role, h.nextRunForIssue(),
				rk1Plan{Mode: "fake", Behavior: "knowledge", Params: map[string]string{"cli": f.env.cli}}, []string{rk1kAlpha})
			rk1kRequireSample(t, sample)
			samples = append(samples, sample)
			_ = leader
		}
	}) {
		t.FailNow()
	}
	measured, seed, paid := rk1kCountStarts(samples)
	if measured != 15 || seed != 3 || paid != 0 {
		t.Fatalf("finite E0b/K1/K2/K3 counters: measured=%d seed=%d paid=%d", measured, seed, paid)
	}
	if err := rk1WriteJSON(filepath.Join(out, "summary.json"), map[string]any{
		"schema_version": 1, "kind": "fake_framework_proof", "measured_fake_starts": measured, "fake_seed_starts": seed,
		"paid_starts": paid, "knowledge_allocated": 0, "real_runnable": false, "samples": samples}); err != nil {
		t.Fatal(err)
	}
}

func TestRK1KnowledgeEvidenceFaults(t *testing.T) {
	rk1kRequireDatabase(t)
	for _, fault := range []string{"wrong", "missing_result", "missing_native", "evidence_stdout_write", "evidence_stdout_sync", "evidence_stdout_close", "evidence_launch_record"} {
		t.Run(fault, func(t *testing.T) {
			f := newRK1KnowledgeFixture(t, filepath.Join(t.TempDir(), fault), false)
			f.updateMemory("Source evidence for an independent fault control.")
			proposal, pending := f.propose(rk1kAlpha, nil)
			f.approve(proposal, pending)
			id := f.chatMessage(f.base.ownerToken, f.chat(f.base.ownerToken))
			sample := f.executeOrdinary("fault-"+fault, "chat", f.claimed(id), uuid.NewString(), fault, []string{rk1kAlpha}, false)
			if fault == "wrong" {
				if sample.State != "finished" || sample.Verdict != "fail" {
					t.Fatalf("complete wrong answer was mislabeled: %s/%s", sample.State, sample.Verdict)
				}
			} else if sample.State != "incomplete" {
				t.Fatalf("missing evidence was scored as complete: %s/%s", sample.State, sample.Verdict)
			}
		})
	}
}
