package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func newManagedRouterFixture(t *testing.T) *platformAdminRouterFixture {
	t.Helper()
	t.Setenv("MULTICA_DEPLOYMENT_ID", "00000000-0000-4000-8000-000000000009")
	t.Setenv("MULTICA_MANAGED_INSTALLATIONS_ENABLED", "true")
	f := newPlatformAdminRouterFixture(t)
	for _, table := range []string{"managed_installation", "installation_user", "installation_daemon_binding", "installation_challenge", "installation_report_cursor"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	return f
}

func TestManagedInstallationRouterEnrollmentBindingRenewalAndRecovery(t *testing.T) {
	f := newManagedRouterFixture(t)
	user := f.user(t, "")
	human := platformRouterJWT(t, user, nil)
	pat := "mul_" + uuid.NewString()
	f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": user, "name": "Managed daemon", "token_hash": auth.HashToken(pat), "token_prefix": "mul_", "auth_version": 1})
	ws := f.fx.Workspace(t, "Managed namespace", "managed-router")
	otherWS := f.fx.Workspace(t, "Other namespace", "other-router")
	f.fx.Member(t, ws, user, "owner")
	f.fx.Member(t, otherWS, user, "owner")
	var org string
	f.fx.QueryRow(t, "SELECT id FROM organization").Scan(&org)
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": ws, "organization_id": org}, "workspace_id=$1", ws)
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"workspace_id": otherWS, "organization_id": org}, "workspace_id=$1", otherWS)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(public)
	challenge := func(token, path string, body any) installation.Proof {
		t.Helper()
		var result installation.Challenge
		response := f.request(t, token, false, "POST", path, body, nil).Want(200)
		platformAssertNoStore(t, response)
		response.JSON(&result)
		raw, err := base64.RawURLEncoding.DecodeString(result.SignaturePayload)
		if err != nil {
			t.Fatal(err)
		}
		return installation.Proof{ChallengeID: result.ChallengeID, SignaturePayload: result.SignaturePayload, BodyPayload: result.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, raw))}
	}
	enrollBody := map[string]any{"purpose": "enroll", "public_key": key, "desktop_version": "0.4.40", "os": "macos"}
	f.request(t, pat, false, "POST", "/api/installations/challenges", enrollBody, nil).Want(403)
	enrollmentProof := challenge(human, "/api/installations/challenges", enrollBody)
	var enrolled service.InstallationEnrollment
	f.request(t, human, false, "POST", "/api/installations/enroll", enrollmentProof, nil).Want(200).JSON(&enrolled)
	if enrolled.InstallationID == "" || enrolled.MetadataProof == "" {
		t.Fatal("enrollment omitted identity or restricted attribution proof")
	}
	f.request(t, human, false, "GET", "/api/me", nil, map[string]string{"X-Installation-Proof": enrolled.MetadataProof}).Want(200)
	f.request(t, enrolled.MetadataProof, false, "GET", "/api/me", nil, nil).Want(401)
	other := f.user(t, "")
	f.request(t, platformRouterJWT(t, other, nil), false, "GET", "/api/me", nil, map[string]string{"X-Installation-Proof": enrolled.MetadataProof}).Want(403)
	daemonID := uuid.NewString()
	bindBody := map[string]any{"purpose": "bind", "public_key": key, "installation_id": enrolled.InstallationID, "workspace_id": ws, "daemon_id": daemonID, "expected_binding_epoch": nil}
	bindProof := challenge(human, "/api/installations/challenges", bindBody)
	var bound service.InstallationBindingResult
	f.request(t, pat, false, "POST", "/api/daemon/installation-bindings", bindProof, nil).Want(200).JSON(&bound)
	if bound.BindingID == "" || bound.BindingEpoch != "1" || bound.PrincipalUserID != user {
		t.Fatal("binding response omitted the verified authority")
	}
	var workspaces []struct {
		ID string `json:"id"`
	}
	f.request(t, bound.DaemonToken, false, "GET", "/api/daemon/workspaces", nil, nil).Want(200).JSON(&workspaces)
	if len(workspaces) != 1 || workspaces[0].ID != ws {
		t.Fatal("scoped daemon credential discovered another workspace")
	}
	f.fx.Exec(t, "UPDATE installation_challenge SET expires_at=now()-interval '1 minute' WHERE id=$1", bindProof.ChallengeID)
	var recovered service.InstallationBindingResult
	f.request(t, pat, false, "POST", "/api/daemon/installation-bindings", bindProof, nil).Want(200).JSON(&recovered)
	if recovered.DaemonToken != bound.DaemonToken || f.fx.Count(t, "SELECT count(*) FROM daemon_token") != 1 {
		t.Fatal("lost response recovery reapplied binding or changed token")
	}
	var hints service.InstallationEnrollment
	f.request(t, human, false, "POST", "/api/installations/enroll", enrollmentProof, nil).Want(200).JSON(&hints)
	if len(hints.Bindings) != 1 || hints.Bindings[0].BindingEpoch != bound.BindingEpoch {
		t.Fatal("new profile cannot discover its existing binding epoch")
	}
	lookupPath := "/api/installations/" + enrolled.InstallationID + "/binding?workspace_id=" + ws + "&daemon_id=" + daemonID
	metadataHeaders := map[string]string{"X-Installation-Proof": enrolled.MetadataProof}
	var lookup struct {
		Binding *service.InstallationBindingHint `json:"binding"`
	}
	f.request(t, human, false, "GET", lookupPath, nil, metadataHeaders).Want(200).JSON(&lookup)
	if lookup.Binding == nil || lookup.Binding.BindingID != bound.BindingID {
		t.Fatal("targeted recovery lookup omitted the current binding")
	}
	f.request(t, human, false, "GET", lookupPath, nil, nil).Want(403)
	f.request(t, pat, false, "GET", lookupPath, nil, metadataHeaders).Want(403)
	f.request(t, human, false, "GET", "/api/installations/"+uuid.NewString()+"/binding?workspace_id="+ws+"&daemon_id="+daemonID, nil, metadataHeaders).Want(403)
	for i := 0; i < 101; i++ {
		f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": enrolled.InstallationID, "workspace_id": otherWS, "daemon_id": fmt.Sprintf("00000000-0000-4000-8000-%012x", i), "principal_user_id": user, "auth_version": 1, "binding_epoch": 1})
	}
	omittedDaemon := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	omittedBinding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": enrolled.InstallationID, "workspace_id": otherWS, "daemon_id": omittedDaemon, "principal_user_id": user, "auth_version": 1, "binding_epoch": 3, "state": "revoked", "revoked_at": time.Now()})
	f.request(t, human, false, "POST", "/api/installations/enroll", enrollmentProof, nil).Want(200).JSON(&hints)
	if !hints.BindingsTruncated || len(hints.Bindings) != 100 {
		t.Fatal("enrollment did not declare bounded binding hints")
	}
	for _, hint := range hints.Bindings {
		if hint.BindingID == omittedBinding {
			t.Fatal("targeted lookup fixture was not beyond the enrollment page")
		}
	}
	f.request(t, human, false, "GET", "/api/installations/"+enrolled.InstallationID+"/binding?workspace_id="+otherWS+"&daemon_id="+omittedDaemon, nil, metadataHeaders).Want(200).JSON(&lookup)
	if lookup.Binding == nil || lookup.Binding.BindingID != omittedBinding || lookup.Binding.BindingEpoch != "3" {
		t.Fatal("targeted lookup lost omitted revoked namespace history")
	}
	f.request(t, human, false, "GET", "/api/installations/"+enrolled.InstallationID+"/binding?workspace_id="+uuid.NewString()+"&daemon_id="+omittedDaemon, nil, metadataHeaders).Want(200).JSON(&lookup)
	if lookup.Binding != nil {
		t.Fatal("targeted lookup returned another workspace's binding")
	}
	// A second profile must not replace the first profile's identity or work.
	runtime := f.fx.Runtime(t, "Active managed runtime", testutil.Cols{"workspace_id": ws, "owner_id": user, "daemon_id": daemonID, "metadata": []byte(`{"cli_version":"99.0.0"}`)})
	agent := f.fx.Agent(t, "Managed agent", runtime, testutil.Cols{"workspace_id": ws, "owner_id": user})
	for _, tc := range []struct {
		name, proof, declared string
		attributed            bool
	}{{"desktop", enrolled.MetadataProof, "", true}, {"web", "", "", false}, {"forged_declared_id", "", enrolled.InstallationID, false}} {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{"X-Workspace-ID": ws, "X-Installation-ID": tc.declared}
			if tc.proof != "" {
				headers["X-Installation-Proof"] = tc.proof
			}
			var submitted struct {
				TaskID string `json:"task_id"`
			}
			f.request(t, human, false, "POST", "/api/issues/quick-create", map[string]any{"agent_id": agent, "prompt": "Router attribution verification", "submitted_installation_id": enrolled.InstallationID}, headers).Want(202).JSON(&submitted)
			var origin, execution pgtype.UUID
			f.fx.QueryRow(t, "SELECT submitted_installation_id,execution_installation_id FROM agent_task_queue WHERE id=$1", submitted.TaskID).Scan(&origin, &execution)
			if origin.Valid != tc.attributed || execution.Valid || origin.Valid && uuid.UUID(origin.Bytes).String() != enrolled.InstallationID {
				t.Fatalf("HTTP submission attribution incorrect: origin=%v execution=%v", origin, execution)
			}
		})
	}
	task := f.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running", "execution_binding_id": bound.BindingID, "execution_binding_epoch": 1, "execution_installation_id": enrolled.InstallationID})
	bindBody["expected_binding_epoch"] = bound.BindingEpoch
	secondProof := challenge(human, "/api/installations/challenges", bindBody)
	var second service.InstallationBindingResult
	f.request(t, pat, false, "POST", "/api/daemon/installation-bindings", secondProof, nil).Want(200).JSON(&second)
	if second.BindingID != bound.BindingID || second.BindingEpoch != bound.BindingEpoch || second.DaemonToken == bound.DaemonToken {
		t.Fatal("new profile replaced current authority")
	}
	f.request(t, bound.DaemonToken, false, "GET", "/api/daemon/workspaces", nil, nil).Want(200)
	if f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='running'", task) != 1 {
		t.Fatal("new profile disturbed active work")
	}
	bindBody["purpose"] = "renew"
	renewProof := challenge(bound.DaemonToken, "/api/daemon/installations/challenges", bindBody)
	var renewed service.InstallationBindingResult
	f.request(t, bound.DaemonToken, false, "POST", "/api/daemon/installation-bindings/renew", renewProof, nil).Want(200).JSON(&renewed)
	if renewed.BindingID != bound.BindingID || renewed.BindingEpoch != bound.BindingEpoch {
		t.Fatal("routine renewal changed binding epoch")
	}
	mat := "mat_" + uuid.NewString()
	f.fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(mat), "task_id": task, "agent_id": agent, "workspace_id": ws, "user_id": user, "auth_version": 1, "expires_at": time.Now().Add(time.Hour), "installation_binding_id": bound.BindingID, "installation_binding_epoch": 1})
	f.request(t, mat, false, "GET", "/api/me", nil, nil).Want(200)
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked',revoked_at=now() WHERE id=$1", bound.BindingID)
	f.request(t, renewed.DaemonToken, false, "GET", "/api/daemon/workspaces", nil, nil).Want(401)
	f.request(t, mat, false, "GET", "/api/me", nil, nil).Want(401)
}
