package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/integrations/lark"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Production router.go registers these routes outside ordinary Auth. The
// handlers themselves authenticate the purpose-bound capability; a task owner,
// plugin invocation or IM binding must not substitute its user identity.
func TestCollaborationRuntimeCredentialMatrix(t *testing.T) {
	f := runtimeCredentialHostFixture(t)
	h := f.handler
	runID := f.claim.Identity.RunID
	const sentinel = "PRIVATE_RUNTIME_CREDENTIAL_SENTINEL"
	commentID := dbfx.Comment(t, f.issue, sentinel)
	handoffID := dbfx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue, "actor_user_id": testUserID,
		"from_user_id": testUserID, "to_user_id": testUserID, "purpose": "transfer", "previous_status": "todo", "note": sentinel})
	dbfx.Cleanup(t, `DELETE FROM issue_handoff WHERE id=$1`, handoffID)
	sourceQuery := url.Values{"kind": {"handoff"}, "sha256": {protocol.CollaborationHandoffHash(sentinel)}}.Encode()
	streamID := uuid.NewString()
	checkpoint := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: f.claim.Identity, AttemptSeq: f.permit.AttemptSeq,
		AttemptNonce: f.permit.AttemptNonce, StreamID: streamID, Sequence: 1, Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: sentinel}}}
	observed := 1
	seal := protocol.CollaborationRecordingSeal{Version: 1, Identity: f.claim.Identity, AttemptSeq: f.permit.AttemptSeq,
		AttemptNonce: f.permit.AttemptNonce, StreamID: streamID, Sequence: 1, ObservedThrough: &observed, AcknowledgedThrough: 1, Reason: "stopped"}
	snapshotRequest := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()}
	snapshot, err := f.svc.RuntimeHandoffSnapshot(context.Background(), parseUUID(runID), f.permit.BusinessToken, snapshotRequest)
	if err != nil {
		t.Fatal(err)
	}
	binding := protocol.CollaborationHandoffBinding{SnapshotID: snapshot.SnapshotID, PromptSHA256: protocol.CollaborationHandoffHash("test application input"),
		AttemptSeq: f.permit.AttemptSeq, AttemptNonce: f.permit.AttemptNonce}

	_, taskToken := autopilotPrincipalTaskTokenCaller(t, dbfx, f.runtime, testUserID)
	if _, err := h.Queries.GetTaskTokenByHash(t.Context(), auth.HashToken(taskToken)); err != nil {
		t.Fatal("task credential was not persisted")
	}
	withPluginsV1Flag(t, testHandler, true)
	cleanupPluginInstallations(t)
	// The installation helper exercises the ordinary human path in legacy mode;
	// the issued credentials below are verified again in the run's password mode.
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	pluginID := installHookPlugin(t)
	t.Setenv("MULTICA_AUTH_MODE", "password")
	pluginToken, err := testHandler.PluginService.IssueInstallToken(f.ctx, parseUUID(pluginID))
	if err != nil {
		t.Fatal(err)
	}
	plugin, err := testHandler.PluginService.AuthenticateInstallToken(f.ctx, pluginToken)
	if err != nil {
		t.Fatal("plugin installation credential was not valid")
	}
	callbacks := service.NewCallbackTokens()
	callbackToken, err := callbacks.Issue(f.ctx, service.HookInvocation{Installation: plugin, Hook: plugincontract.Hook{Key: "summarize"},
		Trigger: plugincontract.TriggerManual, Actor: service.HookActor{Type: "member", ID: parseUUID(testUserID)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := callbacks.Resolve(callbackToken); err != nil {
		t.Fatal("plugin callback credential was not valid")
	}
	installationID := dbfx.Insert(t, "channel_installation", testutil.Cols{"workspace_id": testWorkspaceID, "agent_id": f.agent,
		"channel_type": "feishu", "config": json.RawMessage(`{}`), "installer_user_id": testUserID, "status": "active"})
	dbfx.Cleanup(t, `DELETE FROM channel_binding_token WHERE installation_id=$1`, installationID)
	imToken, err := lark.NewBindingTokenService(h.Queries, testPool).Mint(t.Context(), parseUUID(testWorkspaceID), parseUUID(installationID), lark.OpenID("ou_runtime_boundary"))
	if err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM channel_binding_token WHERE installation_id=$1 AND consumed_at IS NULL AND expires_at>now()`, installationID) != 1 {
		t.Fatal("IM binding credential was not live")
	}
	credentials := []struct{ name, token string }{
		{"task", taskToken}, {"plugin_installation", pluginToken}, {"plugin_callback", callbackToken}, {"im_binding", imToken.Raw},
	}
	type endpoint struct {
		name, method, pattern, suffix string
		handler                       http.HandlerFunc
		body                          any
		termination                   bool
	}
	endpoints := []endpoint{
		{"handoff", http.MethodGet, "/handoff", "/handoff", h.GetCollaborationRuntimeHandoff, nil, false},
		{"snapshot", http.MethodPost, "/handoff/snapshots", "/handoff/snapshots", h.CreateCollaborationRuntimeHandoffSnapshot, protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()}, false},
		{"bind", http.MethodPost, "/handoff/bind", "/handoff/bind", h.BindCollaborationRuntimeHandoff, binding, false},
		{"source", http.MethodGet, "/handoff/sources/{sourceId}", "/handoff/sources/" + handoffID + "?" + sourceQuery, h.GetCollaborationRuntimeHandoffSource, nil, false},
		{"checkpoint", http.MethodPost, "/checkpoints", "/checkpoints", h.RecordCollaborationRuntimeCheckpoint, checkpoint, false},
		{"recall", http.MethodGet, "/comments", "/comments", h.ListCollaborationRuntimeComments, nil, false},
		{"content", http.MethodGet, "/comments/{commentId}/content", "/comments/" + commentID + "/content?revision=1", h.GetCollaborationRuntimeCommentContent, nil, false},
		{"seal", http.MethodPost, "/recording/seal", "/recording/seal", h.SealCollaborationRuntimeRecording, seal, true},
	}
	router := chi.NewRouter()
	for _, endpoint := range endpoints {
		router.Method(endpoint.method, "/api/collaboration/runtime/runs/{runId}"+endpoint.pattern, endpoint.handler)
	}
	router.Get("/api/collaboration/runtime/runs/{runId}/stop-intent", h.GetCollaborationRuntimeStopIntent)
	call := func(t *testing.T, method, suffix, token string, body any) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, "/api/collaboration/runtime/runs/"+runID+suffix, body)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-User-ID", testUserID)
		r.Header.Set("X-Workspace-ID", testWorkspaceID)
		r.AddCookie(&http.Cookie{Name: "multica_token", Value: "unrelated-human-cookie"})
		// Even a valid ambient account principal cannot turn these credentials
		// into a run capability. Production routes do not call ordinary Auth.
		r = r.WithContext(f.ctx)
		return testutil.Call(t, router.ServeHTTP, r)
	}
	state := func(t *testing.T) string {
		t.Helper()
		var value string
		dbfx.QueryRow(t, `SELECT jsonb_build_object(
			'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$2),
			'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY e.seq) FROM collaboration_run_event e WHERE e.run_id=$3),
			'comments',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM comment c WHERE c.issue_id=$1),
			'memory',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.version) FROM collaboration_memory_version m WHERE m.issue_id=$1),
			'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.attempt_seq) FROM collaboration_attempt a WHERE a.run_id=$3),
			'reservations',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.run_id) FROM collaboration_runtime_reservation r WHERE r.run_id=$3),
			'tasks',(SELECT jsonb_agg(to_jsonb(q) ORDER BY q.id) FROM agent_task_queue q WHERE q.issue_id=$1),
			'issue_events',(SELECT jsonb_agg(to_jsonb(e)-'outbox_state' ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=$1)
		)::text`, f.issue, f.session, runID).Scan(&value)
		return value
	}
	before := state(t)
	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			cases := append([]struct{ name, token string }{}, credentials...)
			wrongPurpose := f.claim.TerminationToken
			if endpoint.termination {
				wrongPurpose = f.permit.BusinessToken
			}
			cases = append(cases, struct{ name, token string }{"wrong_purpose", wrongPurpose})
			for _, credential := range cases {
				t.Run(credential.name, func(t *testing.T) {
					response := call(t, endpoint.method, endpoint.suffix, credential.token, endpoint.body).Want(http.StatusUnauthorized)
					if response.Map()["code"] != "invalid_collaboration_capability" || response.Header().Get("Cache-Control") != "no-store" ||
						strings.Contains(response.Text(), sentinel) || strings.Contains(response.Text(), credential.token) || state(t) != before {
						t.Fatal("credential substitution exposed private material or changed runtime state")
					}
				})
			}
		})
	}

	// Positive controls use the same live run and router/payloads as the matrix.
	for _, endpoint := range endpoints {
		if endpoint.termination {
			continue
		}
		t.Run("business_control_"+endpoint.name, func(t *testing.T) {
			response := call(t, endpoint.method, endpoint.suffix, f.permit.BusinessToken, endpoint.body).Want(http.StatusOK)
			if (endpoint.name == "source" || endpoint.name == "recall" || endpoint.name == "content") && !strings.Contains(response.Text(), sentinel) {
				t.Fatal("authorized control did not reach the seeded private material")
			}
		})
	}
	t.Run("termination_metadata_after_revocation", func(t *testing.T) {
		grant, err := f.handler.Queries.GetCollaborationGrant(f.ctx, f.grant)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.RevokeGrant(f.ctx, service.CollaborationRevokeGrantParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f), GrantID: f.grant, ExpectedGrantEpoch: grant.Epoch}); err != nil {
			t.Fatal(err)
		}
		response := call(t, http.MethodGet, "/stop-intent", f.claim.TerminationToken, nil).Want(http.StatusOK)
		if response.Map()["stop_requested"] != true || strings.Contains(response.Text(), sentinel) {
			t.Fatal("original termination capability lost content-free Stop metadata")
		}
		beforeSeal := state(t)
		for _, field := range []string{"body", "messages", "content"} {
			raw, err := json.Marshal(seal)
			if err != nil {
				t.Fatal(err)
			}
			var expanded map[string]any
			if err := json.Unmarshal(raw, &expanded); err != nil {
				t.Fatal(err)
			}
			expanded[field] = sentinel
			response := call(t, http.MethodPost, "/recording/seal", f.claim.TerminationToken, expanded).Want(http.StatusBadRequest)
			if strings.Contains(response.Text(), sentinel) || state(t) != beforeSeal {
				t.Fatal("termination seal accepted or echoed a business-content field")
			}
		}
		var retained protocol.CollaborationRecordingRetention
		response = call(t, http.MethodPost, "/recording/seal", f.claim.TerminationToken, seal).Want(http.StatusOK).JSON(&retained)
		if !retained.Sealed || retained.RetainedThrough != 1 || retained.ObservedThrough == nil || *retained.ObservedThrough != 1 || strings.Contains(response.Text(), sentinel) {
			t.Fatal("metadata-only seal did not preserve the accepted prefix")
		}
		if dbfx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='record_checkpoint'`, runID) != 1 ||
			dbfx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='recording_seal' AND payload::text LIKE $2`, runID, "%"+sentinel+"%") != 0 ||
			dbfx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
			t.Fatal("recording metadata wrote private content or released the unproven run")
		}
	})
}

func runtimeCredentialQueueRequest(f *runtimePreparationHTTPFixture) service.CollaborationQueueRequest {
	return service.CollaborationQueueRequest{IssueID: parseUUID(f.issue), RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1,
		ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1, ExecutionViewVersion: 6}
}

// Reuse the established HTTP managed fixture and perform an actual mode change
// after settling its legacy run. The host run is then newly consented, submitted,
// claimed and permitted; no existing run is relabeled and no process is started.
func runtimeCredentialHostFixture(t *testing.T) *runtimePreparationHTTPFixture {
	t.Helper()
	f := newRuntimePreparationHTTPFixture(t)
	zero := 0
	if _, err := f.svc.AcceptRuntimeProof(t.Context(), parseUUID(f.claim.Identity.RunID), f.claim.TerminationToken, protocol.CollaborationRunProof{
		ReceiptID: uuid.NewString(), Identity: f.claim.Identity, LastAuthorizedAttemptSeq: "1", SealedAttemptSeq: "1",
		AttemptNonce: f.permit.AttemptNonce, State: "exited_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace, ExitCode: &zero}); err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE agent_runtime SET runtime_mode='local' WHERE id=$1`, f.runtime)
	runtime, err := f.handler.Queries.GetAgentRuntime(f.ctx, parseUUID(f.runtime))
	if err != nil {
		t.Fatal(err)
	}
	source, ok := auth.PasswordSessionFromContext(f.daemonCtx)
	if !ok {
		t.Fatal("managed fixture omitted its actual daemon principal")
	}
	caps := protocol.CollaborationRuntimeCapabilities{Version: 2, PreparationVersion: 2, OS: "linux", Arch: "amd64",
		Containment: protocol.CollaborationContainmentHostProcessSupervision, AttachmentFormat: protocol.CollaborationAttachmentFormat,
		Provider: f.claim.Preparation.Provider, Limits: f.claim.Preparation.Limits,
		Host: &protocol.CollaborationHostCapability{ProviderVersion: "test-cli-1", ProviderMode: "local", ConfigurationDigest: strings.Repeat("d", 64), SupervisionScopes: []string{"direct_process", "process_group"}}}
	caps.Provider.AdapterID, caps.Provider.BundleDigest, caps.Provider.Resume, caps.Provider.Fallback = "ordinary_backend_v1", "", "isolated", true
	registration := protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 3, DeploymentID: f.claim.Identity.DeploymentID,
		InstallationID: f.claim.Identity.InstallationID, RuntimeID: f.runtime, BindingID: source.BindingID, BindingEpoch: strconv.FormatInt(source.BindingEpoch, 10),
		ExpectedRuntimeConfigEpoch: strconv.FormatInt(runtime.CollaborationConfigEpoch, 10), Capabilities: caps}
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	target := f.target(t, parseUUID(f.claim.Identity.TargetID))
	var roster []collaboration.TargetResource
	if err := json.Unmarshal(target.Roster, &roster); err != nil {
		t.Fatal(err)
	}
	control := func() service.CollaborationTargetControl {
		return service.CollaborationTargetControl{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
			ExpectedHoldVersion: target.HoldVersion, ExpectedControlVersion: target.ControlVersion, ObservedGroupID: target.ActiveGroupID, ObservedGroupGeneration: target.GroupGeneration}
	}
	if _, err := f.svc.StopQueue(f.ctx, service.CollaborationStopQueueParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f),
		Scope: collaboration.ControlTarget, Targets: []service.CollaborationTargetControl{control()}}); err != nil {
		t.Fatalf("pause settled legacy target before host Switch: %v", err)
	}
	target = f.target(t, target.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_switch_proposal WHERE issue_id=$1`, f.issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_switch_consent WHERE proposal_id IN (SELECT id FROM collaboration_switch_proposal WHERE issue_id=$1)`, f.issue)
	proposal, err := f.svc.ProposeTargetSwitch(f.ctx, service.CollaborationProposeSwitchParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f),
		TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Roster: roster})
	if err != nil {
		t.Fatalf("propose host configuration: %v", err)
	}
	_, err = f.svc.ConsentTargetSwitchResource(f.ctx, service.CollaborationSwitchConsentParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f),
		TargetID: target.ID, ProposalID: proposal.ProposalID, AgentID: parseUUID(f.agent),
		Identity: service.CollaborationResourceIdentity{Kind: "managed", InstallationID: parseUUID(registration.InstallationID), BindingID: parseUUID(source.BindingID), BindingEpoch: source.BindingEpoch}})
	if err != nil {
		t.Fatalf("consent host configuration: %v", err)
	}
	if _, err := f.svc.CommitTargetSwitch(f.ctx, service.CollaborationCommitSwitchParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f), ProposalID: proposal.ProposalID, Control: control()}); err != nil {
		t.Fatalf("commit host configuration: %v", err)
	}
	target = f.target(t, target.ID)
	grants, err := f.handler.Queries.ListCollaborationTargetGrants(f.ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.grant.Valid = false
	for _, grant := range grants {
		if !grant.RevokedAt.Valid {
			f.grant = grant.ID
		}
	}
	if !f.grant.Valid {
		t.Fatal("production Switch did not create current owner consent")
	}
	gate, err := f.handler.Queries.GetCollaborationGate(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	dbfx.Cleanup(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=$1,epoch=$2 WHERE id=1`, gate.HostExecutionEnabled, gate.Epoch)
	if _, err := f.svc.SubmitCommand(f.ctx, service.CollaborationSubmitCommandParams{CollaborationQueueRequest: runtimeCredentialQueueRequest(f), TargetID: target.ID,
		InputEventID: parseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "Host credential acceptance fixture"}); err != nil {
		t.Fatal(err)
	}
	session, err := f.handler.Queries.GetCollaborationSession(f.ctx, parseUUID(f.issue))
	if err != nil {
		t.Fatal(err)
	}
	target = f.target(t, target.ID)
	if _, err := f.svc.ResumeQueue(f.ctx, service.CollaborationResumeQueueParams{CollaborationStopQueueParams: service.CollaborationStopQueueParams{
		CollaborationQueueRequest: runtimeCredentialQueueRequest(f), Scope: collaboration.ControlIssue, ExpectedControlVersion: session.ControlVersion,
		ExpectedTargetSetVersion: session.TargetSetVersion, Targets: []service.CollaborationTargetControl{control()}}, ExpectedExecutionEpoch: gate.Epoch + 1}); err != nil {
		t.Fatal(err)
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, parseUUID(f.runtime))
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("new host admission: candidates=%d err=%v", len(listed.RunIDs), err)
	}
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	claim := protocol.CollaborationRunClaimRequest{ProtocolVersion: 3, DeploymentID: registration.DeploymentID, InstallationID: registration.InstallationID,
		RuntimeID: f.runtime, BindingID: source.BindingID, BindingEpoch: registration.BindingEpoch, Containment: caps.Containment, PreparationVersion: 2, CapabilityDigest: digest}
	f.claim, err = f.svc.ClaimRuntimeRun(f.daemonCtx, parseUUID(listed.RunIDs[0]), claim)
	if err != nil {
		t.Fatal(err)
	}
	f.permit, err = f.svc.PermitRuntimeRun(f.daemonCtx, parseUUID(f.claim.Identity.RunID), protocol.CollaborationRunPermitRequest{ProtocolVersion: 3,
		Identity: f.claim.Identity, Containment: caps.Containment, AttemptSeq: "1", AttemptNonce: uuid.NewString(),
		LaunchDescriptor: &protocol.CollaborationLaunchDescriptor{Kind: "provider", CapabilityDigest: digest, ConfigurationDigest: caps.Host.ConfigurationDigest}})
	if err != nil {
		t.Fatal(err)
	}
	return f
}
