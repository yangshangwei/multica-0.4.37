package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeNativeObservationReplayKeepsOriginalReceipt(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if err != nil {
		t.Fatal(err)
	}
	f.observe(t, d, a, "started", "")
	first := protocol.TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: a.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: a.AttemptSequence, AttemptNonce: a.AttemptNonce, Kind: "native_session", NativeSessionID: "native-original", Source: "system"}
	if err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, first); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.RequestID = uuid.NewString()
	changed.NativeSessionID = "native-changed"
	changed.Source = "result"
	if err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, changed); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("changed native identity was accepted: %v", err)
	}
	if err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, first); err != nil {
		t.Fatalf("later mismatch rewrote original accepted receipt: %v", err)
	}
	if err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, changed); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatal("replay changed the original refusal")
	}
	f.observe(t, d, a, "terminated", "")
	if _, err = f.tasks.FailTask(f.ctx, f.task.ID, "knowledge_context_changed", "native-original", "", "", "knowledge_context_changed", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	claim, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "native-original")
	if err != nil || prior != "" || claim.Session.Generation == d.Session.Generation {
		t.Fatalf("uncertain native transition was reused: %q %v", prior, err)
	}
}

func (f *taskKnowledgeFixture) recordSession(t *testing.T, native string) *protocol.TaskKnowledgeClaimDescriptor {
	t.Helper()
	d := f.finalize(t, "")
	if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
		t.Fatal(err)
	}
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if err != nil {
		t.Fatal(err)
	}
	f.observe(t, d, a, "started", "")
	f.observe(t, d, a, "native_session", native)
	f.observe(t, d, a, "terminated", "")
	return d
}

func (f *taskKnowledgeFixture) nextTurn(t *testing.T) {
	t.Helper()
	id := f.fx.Task(t, util.UUIDToString(f.task.AgentID), testutil.Cols{"runtime_id": f.task.RuntimeID, "issue_id": f.task.IssueID, "chat_session_id": f.task.ChatSessionID, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1, "originator_user_id": f.creator, "accountable_user_id": f.creator})
	var err error
	f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
}

func (f *taskKnowledgeFixture) withdraw(t *testing.T, id string) {
	t.Helper()
	row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.WithdrawKnowledge(f.ctx, row.ID, protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw retained exposure"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeResumeCancelledTaskRetainsExposure(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.recordSession(t, "cancelled-native")
	if _, err := f.tasks.CancelTaskWithResult(f.ctx, f.task.ID, CancelTaskOptions{UserInitiated: true}); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	f.withdraw(t, f.candidate.CandidateID)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "cancelled-native")
	if err != nil || prior != "" || d == nil || d.Session.NativeSessionID != "" {
		t.Fatalf("cancelled private history was reused: %q %v", prior, err)
	}
}

func TestKnowledgeResumeSameSessionUnionsAllTurns(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "shared-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"done"}`), "shared-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	second := f.finalize(t, "shared-native")
	if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
		t.Fatal(err)
	}
	if second.Session.Generation != first.Session.Generation {
		t.Fatal("same native scope did not inherit its exact generation")
	}
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(second)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"done"}`), "", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.SessionID.Valid {
		t.Fatal("fixture must retain a NULL terminal native ID")
	}
	f.nextTurn(t)
	f.withdraw(t, f.candidate.CandidateID)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "shared-native")
	if err != nil || prior != "" || d.Session.Generation == first.Session.Generation {
		t.Fatalf("NULL result masked cumulative exposure: %q %v", prior, err)
	}
}

func TestKnowledgeReclaimedTaskRetainsPriorDisclosureBindings(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	first, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET claim_generation=2,dispatched_at=now() WHERE id=$1`, f.task.ID)
	f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.finalize(t, "")
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Claims) != 2 || meta.Claims[0].Prepared == nil || meta.Claims[0].Prepared.ManifestSHA256 != first.ManifestSHA256 || meta.Claims[1].Prepared != nil {
		t.Fatal("reclaim replaced previous disclosure or inherited its authority")
	}
}

func TestKnowledgeRuntimeMergeDoesNotRelabelExposureAsLegacy(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.recordSession(t, "old-runtime-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "old-runtime-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	newRuntime := f.fx.Runtime(t, "Merged runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": "new-" + uuid.NewString()})
	f.fx.Exec(t, `UPDATE agent_task_queue SET runtime_id=$2 WHERE id=$1`, f.task.ID, newRuntime)
	f.fx.Exec(t, `UPDATE agent SET runtime_id=$2 WHERE id=$1`, f.task.AgentID, newRuntime)
	f.task.RuntimeID = util.MustParseUUID(newRuntime)
	f.nextTurn(t)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "old-runtime-native")
	if err != nil || prior != "" || d.Session.RuntimeID != newRuntime || d.Session.NativeSessionID != "" {
		t.Fatalf("runtime rewrite relabeled private history: %q %v", prior, err)
	}
}

func TestKnowledgeResumeChangedServerConfigurationStartsFresh(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	old := f.recordSession(t, "configured-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "configured-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='New admitted instructions' WHERE id=$1`, f.task.AgentID)
	f.nextTurn(t)
	d, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "configured-native")
	if err != nil || prior != "" || d == nil || d.Session.Generation == old.Session.Generation {
		t.Fatalf("old configuration namespace was reused: %q %v", prior, err)
	}
}

func TestKnowledgeLegacyNoneDiffersFromCorruptNegotiatedHistory(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET session_id='corrupt-native',knowledge_delivery=jsonb_set(knowledge_delivery,'{claims,0}',(knowledge_delivery->'claims'->0)-'prepared') WHERE id=$1`, f.task.ID)
	_, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "corrupt-native")
	if !collaborationErrorCode(err, "knowledge_context_changed") || prior != "" {
		t.Fatalf("damaged disclosed phase became legacy: %q %v", prior, err)
	}
}

func TestKnowledgeMissingOriginatorDoesNotUseAccountableIdentity(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.fx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=NULL WHERE id=$1`, f.task.ID)
	var err error
	f.task, err = f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	d := f.finalize(t, "")
	out, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Contributions) != 0 {
		t.Fatal("accountable identity was used as source authorization")
	}
}

func TestKnowledgeAuthorizeLostResponseRechecksCurrentEligibility(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	req := f.authorizeRequest(p, d)
	first, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req)
	if err != nil || replay != first {
		t.Fatalf("authorization replay changed identity: %v", err)
	}
	f.withdraw(t, f.candidate.CandidateID)
	_, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req)
	if !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("old permit replay bypassed current eligibility: %v", err)
	}
}

func TestKnowledgeLaunchRequiresPreparedInvocationAndPrompt(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	req := f.authorizeRequest(p, d)
	req.InvocationFingerprint = strings.Repeat("c", 64)
	if _, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("changed invocation was authorized: %v", err)
	}
	req.InvocationFingerprint = p.InvocationFingerprint
	if _, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req); err != nil {
		t.Fatal(err)
	}
	req.PromptSHA256 = strings.Repeat("d", 64)
	if _, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("same attempt accepted different prompt: %v", err)
	}
}

func TestKnowledgeDisabledGatePreservesOriginalObservationCleanup(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
	if err != nil {
		t.Fatal(err)
	}
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d))
	if err != nil {
		t.Fatal(err)
	}
	f.tasks.KnowledgeContextEnabled = false
	f.observe(t, d, a, "started", "")
	f.observe(t, d, a, "native_session", "admitted-native")
	f.observe(t, d, a, "terminated", "")
	if _, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, d)); !collaborationErrorCode(err, "knowledge_invocation_unsupported") {
		t.Fatalf("closed gate authorized more work: %v", err)
	}
}

func TestKnowledgeMetadataDoesNotSerializeThroughOrdinaryTask(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	d := f.finalize(t, "")
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); err != nil {
		t.Fatal(err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), f.candidate.CandidateID) || strings.Contains(string(raw), "knowledge_delivery") || strings.Contains(string(raw), "knowledge_context_version") {
		t.Fatal("private metadata entered a generic task/WS payload")
	}
}

func TestKnowledgeWithdrawnOnlyInOlderTurnForcesFresh(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first := f.recordSession(t, "older-cohort-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "older-cohort-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	later := f.candidate
	later.CandidateID = uuid.NewString()
	later.RequestID = uuid.NewString()
	later.Content = "Later independent contribution"
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, later); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(later.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(later.Content)}); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	d := f.finalize(t, "older-cohort-native")
	if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); err != nil {
		t.Fatal(err)
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	// Model a retained later cohort containing only B. History consumers must
	// not assume a later snapshot subsumes every earlier disclosure.
	p := meta.Claims[0].Prepared
	kept := p.Manifest.Entries[:0]
	for _, entry := range p.Manifest.Entries {
		if entry.CandidateID == later.CandidateID {
			kept = append(kept, entry)
		}
	}
	p.Manifest.Entries = kept
	if len(kept) != 1 {
		t.Fatal("later cohort fixture is missing B")
	}
	p.ManifestSHA256, err = protocol.TaskKnowledgeRequestHash(p.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeTaskKnowledge(meta)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_delivery=$2 WHERE id=$1`, f.task.ID, raw)
	if _, err = f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{}`), "", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	f.nextTurn(t)
	f.withdraw(t, f.candidate.CandidateID)
	claim, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "older-cohort-native")
	if err != nil || prior != "" || claim.Session.Generation == first.Session.Generation {
		t.Fatalf("latest-only proof hid older revoked A: %q %v", prior, err)
	}
}

func TestKnowledgePreparationMultiTargetReplayKeepsExactOrder(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	project := f.fx.Project(t, "Second target")
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=ANY($1::uuid[])`, []string{util.UUIDToString(f.task.IssueID), util.UUIDToString(f.issue.Issue.ID)}, project)
	second := f.candidate
	id := uuid.MustParse(second.CandidateID)
	for i := len(id) - 1; i >= 0; i-- {
		id[i]++
		if id[i] != 0 {
			break
		}
	}
	second.CandidateID = id.String()
	second.RequestID = uuid.NewString()
	second.Target = protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.workspace, ProjectID: project}
	second.Content = "Project contribution"
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(second.Content)}); err != nil {
		t.Fatal(err)
	}
	d := f.finalize(t, "")
	req := f.prepareRequest(d)
	first, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(replay)
	if len(first.Contributions) != 2 || string(a) != string(b) {
		t.Fatal("first disclosure and original-key replay differ across target order")
	}
	for i, item := range first.Contributions {
		if item.CandidateID != first.Manifest.Entries[i].CandidateID {
			t.Fatal("manifest and contribution order differ")
		}
	}
}
