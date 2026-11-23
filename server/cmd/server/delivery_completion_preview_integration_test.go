package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
)

func completionPreviewRouterPath(f *deliveryPolicyRouterFixture) string {
	return "/api/issues/" + f.request.ResourceID + "/delivery-evidence/completion-preview"
}
func completionPreviewRouterBody(f *deliveryPolicyRouterFixture) map[string]any {
	return map[string]any{"expected_issue_revision": f.body["expected_issue_revision"], "target_status": "done", "approval_id": f.approval.Approval.ID}
}

func TestDeliveryCompletionPreviewRouterAuthorizeCompleteRecover(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			f := newDeliveryPolicyRouterFixture(t, mode, true)
			installCompletionAuthorizationRouterPolicy(t, f, 1)
			before, archive, evaluations := f.snapshot(t), f.archive(t), f.evaluationRows(t)
			var candidates service.DeliveryEvidenceCandidates
			f.call(t, f.token, "GET", "/api/issues/"+f.request.ResourceID+"/delivery-evidence/candidates", nil).Want(200).JSON(&candidates)
			if len(candidates.Items) != 1 || candidates.Items[0].ApprovalID != f.approval.Approval.ID || !candidates.Items[0].Selectable || candidates.HasMore {
				t.Fatalf("candidate discovery lost actual package: %+v", candidates)
			}
			var preview service.DeliveryCompletionPreview
			response := f.call(t, f.token, "POST", completionPreviewRouterPath(f), completionPreviewRouterBody(f)).Want(200).JSON(&preview)
			if response.Header().Get("Cache-Control") != "no-store" || preview.AuthorizationBinding == nil || preview.Observation == nil || preview.Observation.Verdict != "pass" {
				t.Fatalf("invalid preview %+v", preview)
			}
			if f.snapshot(t) != before || !bytes.Equal(f.archive(t), archive) || f.evaluationRows(t) != evaluations || f.fx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.request.ResourceID) != 0 {
				t.Fatal("read path changed business facts")
			}
			b := preview.AuthorizationBinding
			body := service.DeliveryCompletionAuthorizationRequest{ContractVersion: b.ContractVersion, SourceWorkspaceID: b.SourceWorkspaceID, OperationID: uuid.NewString(), Target: b.Target, Project: b.Project, Evidence: b.Evidence, Consumer: b.Consumer, ExpiresAt: preview.MaxExpiresAt.Add(-time.Second), Reason: "Reviewed exact current preview"}
			key := uuid.NewString()
			var grant service.DeliveryCompletionAuthorizationView
			f.call(t, f.token, "POST", completionAuthorizationRouterPath, body, "Idempotency-Key", key).Want(201).JSON(&grant)
			var current struct {
				ID       string                             `json:"id"`
				Revision int64                              `json:"revision"`
				Status   string                             `json:"status"`
				Receipt  *service.DeliveryCompletionReceipt `json:"completion_receipt"`
			}
			f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]any{"status": "done", "completion": service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: body.OperationID, ExpectedIssueRevision: body.Target.ExpectedIssueRevision}}).Want(200).JSON(&current)
			if current.Status != "done" || current.Receipt == nil || current.Receipt.OperationID != body.OperationID {
				t.Fatalf("missing real completion receipt %+v", current)
			}
			completedRevision := current.Receipt.CompletedRevision
			f.call(t, f.token, "PUT", "/api/issues/"+f.request.ResourceID, map[string]any{"status": "todo"}).Want(200)
			f.router = controlledArtifactRouter(t)
			var recovered service.DeliveryCompletionAuthorizationView
			f.call(t, f.token, "GET", completionAuthorizationRouterPath+"/by-request-key/"+key+"?workspace_id="+testWorkspaceID, nil).Want(200).JSON(&recovered)
			if recovered.State != "consumed" || recovered.CompletionReceipt == nil || recovered.CompletionReceipt.CompletedRevision != completedRevision {
				t.Fatal("original read did not preserve receipt", recovered)
			}
			f.call(t, f.token, "GET", "/api/issues/"+f.request.ResourceID, nil).Want(200).JSON(&current)
			if current.Status != "todo" || current.Revision <= completedRevision {
				t.Fatal("history overwrote current reopened task", current)
			}
			if f.fx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 1 {
				t.Fatal("read duplicated terminal")
			}
		})
	}
}

func TestDeliveryCompletionPreviewRouterHumanAndWorkspaceBoundaries(t *testing.T) {
	f := newDeliveryPolicyRouterFixture(t, "legacy", true)
	installCompletionAuthorizationRouterPolicy(t, f, 1)
	for _, path := range []string{completionPreviewRouterPath(f), "/api/issues/" + f.request.ResourceID + "/delivery-evidence/candidates"} {
		method := "POST"
		var body any = completionPreviewRouterBody(f)
		if strings.HasSuffix(path, "candidates") {
			method = "GET"
			body = nil
		}
		f.call(t, f.taskToken, method, path, body).Want(403)
		f.call(t, f.token, method, path, body, "X-Agent-ID", f.request.AgentID).Want(403)
		f.call(t, f.token, method, path, body, "X-Workspace-ID", uuid.NewString()).Want(404)
	}
	raw, _ := json.Marshal(completionPreviewRouterBody(f))
	for _, body := range []string{string(raw) + `{}`, string(raw[:len(raw)-1]) + `,"target_status":"done"}`, strings.Replace(string(raw), `"target_status"`, `"Target_status"`, 1)} {
		f.call(t, f.token, "POST", completionPreviewRouterPath(f), body).Want(400)
	}
}

// This fixture is opt-in and local-only. It never resolves or starts an agent
// executable. The parent coordinator owns the private output path, database,
// actual Web/Desktop checks and stop file; default tests do not create fixtures.
func TestDeliveryCompletionM1BrowserFixture(t *testing.T) {
	file := os.Getenv("MULTICA_M1_BROWSER_FIXTURE_FILE")
	if file == "" {
		t.Skip("opt-in M1 browser fixture")
	}
	if !filepath.IsAbs(file) || filepath.Clean(file) != file {
		t.Fatal("fixture output must be an absolute private path")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Dir(file))
	if err != nil || resolved != filepath.Dir(file) {
		t.Fatal("fixture output must have canonical directory")
	}
	if _, err = os.Lstat(file); !os.IsNotExist(err) {
		t.Fatal("fixture output already exists or is unavailable")
	}
	if _, err = os.Lstat(file + ".stop"); !os.IsNotExist(err) {
		t.Fatal("fixture stop path already exists or is unavailable")
	}
	fixtures := []*deliveryPolicyRouterFixture{}
	for _, name := range []string{"Web", "Desktop"} {
		f := newDeliveryPolicyRouterFixture(t, "legacy", true)
		installCompletionAuthorizationRouterPolicy(t, f, 1)
		f.fx.Exec(t, `UPDATE issue SET title=$1 WHERE id=$2`, "M1 "+name+" completion acceptance", f.request.ResourceID)
		// The artifact, unlike the issue title, is exactly the actual tar output.
		f.archive(t)
		fixtures = append(fixtures, f)
	}
	api := httptest.NewServer(controlledArtifactRouter(t))
	defer api.Close()
	var slug string
	fixtures[0].fx.QueryRow(t, `SELECT slug FROM workspace WHERE id=$1`, testWorkspaceID).Scan(&slug)
	scenarios := make([]map[string]any, 0, len(fixtures))
	for i, f := range fixtures {
		var revision int64
		f.fx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.request.ResourceID).Scan(&revision)
		scenarios = append(scenarios, map[string]any{"name": []string{"web", "desktop"}[i], "user_id": f.user, "token": f.token, "workspace_id": testWorkspaceID, "workspace_slug": slug, "issue_id": f.request.ResourceID, "project_id": f.project, "approval_id": f.approval.Approval.ID, "expected_revision": revision})
	}
	raw, err := json.Marshal(map[string]any{"api_url": api.URL, "scenarios": scenarios, "expires_at": time.Now().Add(15 * time.Minute).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal("cannot create private fixture output")
	}
	if _, err = out.Write(raw); err != nil {
		out.Close()
		t.Fatal("cannot write private fixture output")
	}
	if err = out.Sync(); err != nil {
		out.Close()
		t.Fatal("cannot sync private fixture output")
	}
	if err = out.Close(); err != nil {
		t.Fatal("cannot close private fixture output")
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(15 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ticker.C:
			info, err := os.Lstat(file + ".stop")
			if err == nil {
				if !info.Mode().IsRegular() {
					t.Fatal("fixture stop must be regular file")
				}
				return
			}
			if !os.IsNotExist(err) {
				t.Fatal("fixture stop inspection failed")
			}
		case <-timer.C:
			t.Fatal("M1 browser fixture timed out without coordinator stop")
		}
	}
}
