package handler

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type runtimePreparationHTTPFixture struct {
	*configHTTPFixture
	handler   *Handler
	file      service.CollaborationAttachmentView
	claim     protocol.CollaborationRunClaimResponse
	permit    protocol.CollaborationRunPermitResponse
	grant     pgtype.UUID
	daemonCtx context.Context
}

func preparationHTTPRequest(method, runID, fileID, token string, body any) *http.Request {
	r := testutil.JSONRequest(method, "/api/collaboration/runtime/runs/"+runID+"/preparation", body)
	r = withURLParams(r, "runId", runID, "attachmentId", fileID)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
	return r
}

func preparationManagedHTTPRequest(r *http.Request, ctx context.Context) *http.Request {
	source, _ := auth.PasswordSessionFromContext(ctx)
	return r.WithContext(auth.WithPasswordSession(r.Context(), source))
}

func newRuntimePreparationHTTPFixture(t *testing.T) *runtimePreparationHTTPFixture {
	t.Helper()
	base := newConfigHTTPFixture(t)
	h := collaborationFileHandler(t)
	base.svc = h.collaborationAttachmentService()
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	dbfx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": testUserID, "username": "preparation-" + uuid.NewString(),
		"password_hash": "synthetic-unused-hash", "must_change_password": false}, "user_id=$1", testUserID)
	org, err := h.Queries.EnsureInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	dbfx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": org.ID, "workspace_id": testWorkspaceID}, "workspace_id=$1", testWorkspaceID)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	installed := dbfx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "organization_id": org.ID,
		"public_key": []byte(public), "key_fingerprint": installation.Digest(public), "responsible_user_id": testUserID, "os": "linux"})
	daemonID := uuid.NewString()
	dbfx.Exec(t, `UPDATE agent_runtime SET owner_id=$2,daemon_id=$3,provider='codex',status='online' WHERE id=$1`, base.runtime, testUserID, daemonID)
	binding := dbfx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": installed, "workspace_id": testWorkspaceID,
		"daemon_id": daemonID, "principal_user_id": testUserID, "auth_version": 1, "binding_epoch": 1})
	t.Setenv("MULTICA_AUTH_MODE", "password")
	base.ctx = auth.WithPasswordSession(base.ctx, auth.PasswordSession{UserID: testUserID, Version: 1, Kind: "jwt"})
	daemonCtx := auth.WithPasswordSession(t.Context(), auth.PasswordSession{UserID: testUserID, Version: 1, Kind: "daemon_token",
		WorkspaceID: testWorkspaceID, DaemonID: daemonID, BindingID: binding, BindingEpoch: 1})
	f := &runtimePreparationHTTPFixture{configHTTPFixture: base, handler: h, daemonCtx: daemonCtx}
	runtime, err := h.Queries.GetAgentRuntime(t.Context(), parseUUID(base.runtime))
	if err != nil {
		t.Fatal(err)
	}
	caps := protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: 2, DeploymentID: auth.ManagedDeploymentID(), InstallationID: installed,
		RuntimeID: base.runtime, BindingID: binding, BindingEpoch: "1", ExpectedRuntimeConfigEpoch: strconv.FormatInt(runtime.CollaborationConfigEpoch, 10),
		Capabilities: protocol.CollaborationRuntimeCapabilities{Version: 1, PreparationVersion: 1, OS: "linux", Arch: "amd64",
			Containment: protocol.CollaborationContainmentLinuxPIDNamespace, AttachmentFormat: protocol.CollaborationAttachmentFormat,
			Provider: protocol.CollaborationPreparationProvider{Provider: "codex", ProtocolFamily: "codex", AdapterID: "static_json_v1", AdapterVersion: 1,
				BundleDigest: strings.Repeat("a", 64), AccountSource: "runtime_configuration", Resume: "fresh"},
			Limits: protocol.CollaborationPreparationLimits{FileCount: 32, PerFileBytes: 100 << 20, TotalInputBytes: 256 << 20,
				WorkspaceBytes: 512 << 20, GitExpandedBytes: 256 << 20, EntryCount: 4096, TimeoutSeconds: 60}}}
	managed := func(method string, body any) *http.Request {
		return preparationManagedHTTPRequest(preparationHTTPRequest(method, "", "", "mdt_"+strings.Repeat("a", 64), body), daemonCtx)
	}
	testutil.Call(t, h.RegisterCollaborationRuntimeCapabilities, managed(http.MethodPost, caps)).Want(http.StatusOK)
	target := base.register(t, collaboration.TargetAgent, base.agent)
	queueRequest := func() service.CollaborationQueueRequest {
		return service.CollaborationQueueRequest{IssueID: parseUUID(base.issue), RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}
	}
	consent, err := base.svc.ConsentTargetResource(base.ctx, service.CollaborationConsentParams{CollaborationQueueRequest: queueRequest(), TargetID: target.ID,
		AgentID: parseUUID(base.agent), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: service.CollaborationResourceIdentity{Kind: "managed", InstallationID: parseUUID(installed), BindingID: parseUUID(binding), BindingEpoch: 1}})
	if err != nil {
		t.Fatal(err)
	}
	f.grant = consent.GrantID
	for _, table := range []string{"collaboration_grant", "collaboration_batch", "collaboration_command", "collaboration_execution_group", "collaboration_run"} {
		if table == "collaboration_grant" {
			dbfx.Cleanup(t, `DELETE FROM collaboration_grant WHERE issue_id=$1`, base.issue)
		} else {
			dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE session_id=$1`, base.session)
		}
	}
	for _, table := range []string{"collaboration_attempt", "collaboration_termination", "collaboration_business_capability", "collaboration_runtime_reservation", "collaboration_run_event"} {
		dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)`, base.session)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_runtime_scan_cursor WHERE runtime_id=$1`, base.runtime)
	dbfx.Cleanup(t, `DELETE FROM collaboration_group_input WHERE channel_id IN (SELECT id FROM collaboration_channel WHERE session_id=$1)`, base.session)
	dbfx.Cleanup(t, `DELETE FROM collaboration_group_dependency WHERE target_id=$1`, target.ID)
	draft := parseUUID(uuid.NewString())
	f.file, err = base.svc.UploadAttachmentDraft(base.ctx, service.CollaborationAttachmentUpload{DraftID: draft, IssueID: parseUUID(base.issue), RequestID: uuid.NewString(),
		Name: "../../display-name.txt", ContentType: "text/plain", Data: []byte("run-bound private storage bytes")})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, f.file.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, f.file.ID)
	if _, err = base.svc.DeliverAttachments(base.ctx, service.CollaborationAttachmentDelivery{IssueID: parseUUID(base.issue), DraftID: &draft, AttachmentIDs: []pgtype.UUID{f.file.ID}, RequestID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if _, err = base.svc.SubmitCommand(base.ctx, service.CollaborationSubmitCommandParams{CollaborationQueueRequest: queueRequest(), TargetID: target.ID,
		InputEventID: parseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "prepare fixed file"}); err != nil {
		t.Fatal(err)
	}
	gate, err := h.Queries.LockCollaborationGateShared(base.ctx)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=true,epoch=epoch+1 WHERE id=1`)
	dbfx.Cleanup(t, `UPDATE collaboration_execution_gate SET enabled=$1,epoch=$2 WHERE id=1`, gate.Enabled, gate.Epoch)
	target = base.target(t, target.ID)
	session, err := h.Queries.GetCollaborationSession(base.ctx, parseUUID(base.issue))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = base.svc.ResumeQueue(base.ctx, service.CollaborationResumeQueueParams{CollaborationStopQueueParams: service.CollaborationStopQueueParams{
		CollaborationQueueRequest: queueRequest(), Scope: collaboration.ControlIssue, ExpectedControlVersion: session.ControlVersion, ExpectedTargetSetVersion: session.TargetSetVersion,
		Targets: []service.CollaborationTargetControl{{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
			ExpectedHoldVersion: target.HoldVersion, ExpectedControlVersion: target.ControlVersion, ObservedGroupID: target.ActiveGroupID, ObservedGroupGeneration: target.GroupGeneration}}},
		ExpectedExecutionEpoch: gate.Epoch + 1}); err != nil {
		t.Fatal(err)
	}
	list, err := base.svc.ListRuntimeRuns(daemonCtx, parseUUID(base.runtime))
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("HTTP fixture run admission: %v", err)
	}
	claimRequest := protocol.CollaborationRunClaimRequest{ProtocolVersion: 2, DeploymentID: auth.ManagedDeploymentID(), InstallationID: installed, RuntimeID: base.runtime,
		BindingID: binding, BindingEpoch: "1", Containment: protocol.CollaborationContainmentLinuxPIDNamespace, PreparationVersion: 1, BundleDigest: strings.Repeat("a", 64)}
	testutil.Call(t, h.ClaimCollaborationRuntimeRun, preparationManagedHTTPRequest(preparationHTTPRequest(http.MethodPost, list.RunIDs[0], "", "mdt_"+strings.Repeat("a", 64), claimRequest), daemonCtx)).Want(http.StatusOK).JSON(&f.claim)
	permitRequest := protocol.CollaborationRunPermitRequest{Identity: f.claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	testutil.Call(t, h.PermitCollaborationRuntimeRun, preparationManagedHTTPRequest(preparationHTTPRequest(http.MethodPost, list.RunIDs[0], "", "mdt_"+strings.Repeat("a", 64), permitRequest), daemonCtx)).Want(http.StatusOK).JSON(&f.permit)
	return f
}

func TestCollaborationRuntimePreparationHTTPRejectsCredentialFallbacks(t *testing.T) {
	bare := &Handler{}
	for _, handler := range []http.HandlerFunc{bare.GetCollaborationRuntimePreparation, bare.GetCollaborationRuntimeAttachmentContent, bare.RecordCollaborationRuntimePreparation} {
		for _, prefix := range []string{"mct_", "mdt_", "mat_", "mul_", ""} {
			r := preparationHTTPRequest(http.MethodGet, uuid.NewString(), uuid.NewString(), prefix+strings.Repeat("a", 64), nil)
			response := testutil.Call(t, handler, r).Want(http.StatusUnauthorized)
			if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("capability denial allowed caching")
			}
		}
	}
	for _, handler := range []http.HandlerFunc{bare.GetCollaborationRuntimeCapabilities, bare.RegisterCollaborationRuntimeCapabilities} {
		testutil.Call(t, handler, preparationHTTPRequest(http.MethodPost, "", "", "mcb_"+strings.Repeat("a", 64), nil)).Want(http.StatusForbidden)
	}
	for _, raw := range []string{`{"proof_state":"exited_proven"}`, `{"event_id":"one","event_id":"two"}`} {
		testutil.Call(t, bare.RecordCollaborationRuntimePreparation, preparationHTTPRequest(http.MethodPost, uuid.NewString(), "", "mcb_"+strings.Repeat("a", 64), json.RawMessage(raw))).Want(http.StatusBadRequest)
	}
}

func TestCollaborationRuntimePreparationHTTPUsesStorageAndRechecksAfterIO(t *testing.T) {
	f := newRuntimePreparationHTTPFixture(t)
	runID, fileID := f.claim.Identity.RunID, uuidToString(f.file.ID)
	request := func(version string) *http.Request {
		r := preparationHTTPRequest(http.MethodGet, runID, fileID, f.permit.BusinessToken, nil)
		r.URL.RawQuery = "version=" + version
		return r
	}
	var manifest protocol.CollaborationPreparation
	testutil.Call(t, f.handler.GetCollaborationRuntimePreparation, preparationHTTPRequest(http.MethodGet, runID, "", f.permit.BusinessToken, nil)).Want(http.StatusOK).JSON(&manifest)
	if manifest.ManifestDigest != f.claim.Preparation.ManifestDigest {
		t.Fatal("business manifest changed after claim")
	}
	response := testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, request("1")).Want(http.StatusOK)
	if response.Text() != "run-bound private storage bytes" || response.Header().Get("X-Multica-Run-ID") != runID || response.Header().Get("X-Multica-Attachment-ID") != fileID ||
		response.Header().Get("X-Multica-Attachment-Version") != "1" || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Location") != "" {
		t.Fatal("runtime download lost exact identity, storage bytes or privacy headers")
	}
	testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, request("2")).Want(http.StatusNotFound)
	testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, request("01")).Want(http.StatusBadRequest)
	f.handler.Storage = collaborationReadHookStorage{Storage: f.handler.Storage, read: func() {
		_, err := f.svc.RevokeGrant(f.ctx, service.CollaborationRevokeGrantParams{CollaborationQueueRequest: service.CollaborationQueueRequest{IssueID: parseUUID(f.issue), RequestID: uuid.NewString(),
			ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}, GrantID: f.grant, ExpectedGrantEpoch: 1})
		if err != nil {
			t.Fatal(err)
		}
	}}
	denied := testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, request("1")).Want(http.StatusForbidden)
	if strings.Contains(denied.Text(), "run-bound private storage bytes") || denied.Header().Get("Cache-Control") != "no-store" || denied.Header().Get("X-Multica-Attachment-ID") != "" {
		t.Fatal("storage revocation crossed the final byte authorization")
	}
}
