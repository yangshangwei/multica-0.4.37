package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/reportsnapshot"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func f2RouterCapture(t *testing.T, f targetedCapabilityFixture) service.ReportSnapshotResponse {
	t.Helper()
	f.fx.Cleanup(t, "DELETE FROM report_snapshot WHERE report_issue_id=$1", f.issue)
	f.fx.Cleanup(t, "DELETE FROM report_snapshot_control WHERE report_issue_id=$1", f.issue)
	token, err := generateTestJWT(testUserID, "f2@example.test", "F2 capture")
	if err != nil {
		t.Fatal(err)
	}
	var revision int64
	f.fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", f.issue).Scan(&revision)
	end := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	req := testutil.JSONRequest("POST", "/api/issues/"+f.issue+"/report-snapshots", map[string]any{"protocol_version": 1, "expected_issue_revision": revision, "timezone": "Asia/Shanghai", "start": end.Add(-24 * time.Hour), "end": end, "period_mode": "rolling_24h", "correction_of": nil})
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	var out service.ReportSnapshotResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(201).JSON(&out)
	return out
}

func TestF2ReportSnapshotPublicProjectionAndHumanReceipt(t *testing.T) {
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	captured := f2RouterCapture(t, f)
	if captured.CaptureReceipt == nil || captured.CaptureReceipt.ReportIssueID != f.issue || captured.CaptureReceipt.SnapshotID != captured.Snapshot.SnapshotID || captured.CaptureReceipt.RequestKey == "" || len(captured.CaptureReceipt.RequestDigest) != 64 {
		t.Fatal("human capture did not retain its exact original request receipt")
	}
	path := "/api/issues/" + f.issue + "/report-snapshots/" + captured.Snapshot.SnapshotID
	response := f2RouterRead(t, f.token, http.MethodGet, path, nil).Want(http.StatusOK)
	var out service.ReportSnapshotResponse
	response.JSON(&out)
	if out.CaptureReceipt != nil || out.Replayed {
		t.Fatal("Reporter GET disclosed human capture metadata")
	}
	if err := reportsnapshot.ValidatePublic(out.Snapshot); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"created_by", "request_key", "read_cut", "origin_id", "assignee_id", "lead_id", "references", "snapshot_digest", "capture_receipt"} {
		if strings.Contains(response.Body.String(), `"`+field+`":`) {
			t.Fatalf("public snapshot contains private field %s", field)
		}
	}
	for _, value := range []string{f.owner, f.originator, f.agent, f.runtime, f.task, captured.CaptureReceipt.RequestKey} {
		if strings.Contains(response.Body.String(), value) {
			t.Fatal("public snapshot leaked internal principal or operation identity")
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public snapshot is cacheable")
	}
	for _, token := range []string{"", "mat_invalid_snapshot_reader"} {
		denied := f2RouterRead(t, token, http.MethodGet, path, nil).Want(http.StatusUnauthorized)
		if denied.Header().Get("Cache-Control") != "no-store" || strings.Contains(denied.Body.String(), captured.Snapshot.ProjectionDigest) {
			t.Fatal("early authentication failure leaked cacheable snapshot data")
		}
	}
}
func f2RouterRead(t *testing.T, token, method, path string, headers map[string]string) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
}

func f2RouterWithdraw(t *testing.T, f targetedCapabilityFixture, current, key string) service.ReportSnapshotWithdrawalResponse {
	t.Helper()
	credential, err := generateTestJWT(testUserID, "f2@example.test", "F2 withdrawal")
	if err != nil {
		t.Fatal(err)
	}
	var revision int64
	f.fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", f.issue).Scan(&revision)
	req := testutil.JSONRequest(http.MethodPost, "/api/issues/"+f.issue+"/report-snapshots/withdrawals", map[string]any{"protocol_version": 1, "scope": "report", "expected_issue_revision": revision, "expected_current_snapshot_id": current, "reason": "Withdraw this report"})
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req.Header.Set("Idempotency-Key", key)
	var out service.ReportSnapshotWithdrawalResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(201).JSON(&out)
	return out
}

func TestF2ReportSnapshotProductionReporterExactGetMatrix(t *testing.T) {
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	captured := f2RouterCapture(t, f)
	path := "/api/issues/" + f.issue + "/report-snapshots/"
	var got service.ReportSnapshotResponse
	f2RouterRead(t, f.token, "GET", path+"current", nil).Want(200).JSON(&got)
	if got.Snapshot.ProjectionDigest != captured.Snapshot.ProjectionDigest || got.Replayed {
		t.Fatal("Reporter read changed sealed result")
	}
	f2RouterRead(t, f.token, "GET", path+captured.Snapshot.SnapshotID, map[string]string{"X-Agent-ID": f.agent, "X-Task-ID": f.task, "X-User-ID": f.owner, "X-Workspace-ID": testWorkspaceID, "X-Actor-Source": "task_token"}).Want(200)
	outside := f.fx.Issue(t, "Other report", testutil.Cols{"project_id": f.project})
	for _, tc := range []struct {
		name, method, path string
		headers            map[string]string
		status             int
	}{
		{"other report", "GET", "/api/issues/" + outside + "/report-snapshots/current", nil, 403},
		{"unbound snapshot", "GET", path + uuid.NewString(), nil, 404},
		{"post capture", "POST", strings.TrimSuffix(path, "/"), nil, 403},
		{"post withdrawal", "POST", path + "withdrawals", nil, 403},
		{"head", "HEAD", path + "current", nil, 403},
		{"delete", "DELETE", path + captured.Snapshot.SnapshotID, nil, 403},
		{"query", "GET", path + "current?project_id=" + f.project, nil, 403},
		{"empty query", "GET", path + "current?", nil, 403},
		{"duplicate query", "GET", path + "current?limit=1&limit=2", nil, 403},
		{"encoded path", "GET", strings.Replace(path+"current", "report-snapshots", "report%2Dsnapshots", 1), nil, 403},
		{"upper uuid", "GET", "/api/issues/" + strings.ToUpper(f.issue) + "/report-snapshots/current", nil, 403},
		{"workspace header", "GET", path + "current", map[string]string{"X-Workspace-ID": uuid.NewString()}, 403},
		{"actor header", "GET", path + "current", map[string]string{"X-User-ID": testUserID}, 403},
		{"source header", "GET", path + "current", map[string]string{"X-Actor-Source": "member"}, 403},
		{"slug header", "GET", path + "current", map[string]string{"X-Workspace-Slug": "outside"}, 403},
		{"credential exchange", "POST", "/api/cli-token", nil, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := f2RouterRead(t, f.token, tc.method, tc.path, tc.headers).Want(tc.status)
			if strings.Contains(res.Body.String(), captured.Snapshot.ProjectionDigest) || strings.Contains(res.Body.String(), "facts_digest") {
				t.Fatal("refusal leaked snapshot body")
			}
			if strings.Contains(tc.path, "report") && res.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("early denial is cacheable")
			}
		})
	}
	if f.fx.Count(t, "SELECT count(*) FROM report_snapshot WHERE report_issue_id=$1", f.issue) != 1 {
		t.Fatal("GET/refusal created snapshots")
	}
	// Existing inert comment capability is not removed or expanded by F2.
	targetedCapabilityRequest(t, f.token, "POST", "/api/issues/"+f.issue+"/comments", map[string]any{"content": "F2 frozen report reviewed"}).Want(201)
	var eligible bool
	f.fx.QueryRow(t, "SELECT dispatch_eligible FROM comment WHERE source_task_id=$1 AND content=$2", f.task, "F2 frozen report reviewed").Scan(&eligible)
	if eligible {
		t.Fatal("Reporter comment became executable")
	}
}

func TestF2ReportSnapshotProductionRevocationAndFreshClaim(t *testing.T) {
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	captured := f2RouterCapture(t, f)
	path := "/api/issues/" + f.issue + "/report-snapshots/" + captured.Snapshot.SnapshotID
	// A newly verified claim may read the same report's old snapshot; old claim cannot.
	f.fx.Exec(t, "UPDATE agent_task_queue SET claim_generation=2 WHERE id=$1", f.task)
	f2RouterRead(t, f.token, "GET", path, nil).Want(403)
	raw, err := json.Marshal(auth.TaskCapability{Version: 1, Profile: auth.TaskCapabilityReporter, IssueID: f.issue, ProjectID: f.project, OriginatorID: f.originator, RuntimeID: f.runtime, ClaimGeneration: 2})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "task_token", testutil.Cols{"token_hash": auth.HashToken(fresh), "task_id": f.task, "agent_id": f.agent, "workspace_id": testWorkspaceID, "user_id": f.owner, "expires_at": time.Now().Add(time.Hour), "api_capability": raw})
	f2RouterRead(t, fresh, "GET", path, nil).Want(200)
	f.fx.Exec(t, "DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2", f.agent, f.originator)
	denied := f2RouterRead(t, fresh, "GET", path, nil).Want(403)
	if strings.Contains(denied.Body.String(), captured.Snapshot.ProjectionDigest) {
		t.Fatal("revoked invoke permission disclosed frozen report")
	}
	f.fx.Exec(t, "DELETE FROM task_token WHERE token_hash=$1", auth.HashToken(fresh))
	f2RouterRead(t, fresh, "GET", path, nil).Want(401)
}

func TestF2ReportSnapshotProductionLegacyReviewerAndNoProject(t *testing.T) {
	for _, tc := range []struct {
		key     string
		version int
		scoped  bool
	}{{"progress-reporter", 1, false}, {"code-reviewer", 2, true}} {
		t.Run(tc.key+string(rune('0'+tc.version)), func(t *testing.T) {
			f := newTargetedCapabilityFixture(t, tc.key, tc.version, tc.scoped)
			f2RouterCapture(t, f)
			f2RouterRead(t, f.token, "GET", "/api/issues/"+f.issue+"/report-snapshots/current", nil).Want(403)
		})
	}
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	f.fx.Exec(t, "UPDATE issue SET project_id=NULL WHERE id=$1", f.issue)
	raw, err := json.Marshal(auth.TaskCapability{Version: 1, Profile: auth.TaskCapabilityReporter, IssueID: f.issue, OriginatorID: f.originator, RuntimeID: f.runtime, ClaimGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "UPDATE task_token SET api_capability=$1 WHERE token_hash=$2", raw, auth.HashToken(f.token))
	captured := f2RouterCapture(t, f)
	var out service.ReportSnapshotResponse
	f2RouterRead(t, f.token, http.MethodGet, "/api/issues/"+f.issue+"/report-snapshots/current", nil).Want(200).JSON(&out)
	if out.Snapshot.ProjectID != nil || out.Snapshot.ProjectionDigest != captured.Snapshot.ProjectionDigest {
		t.Fatal("no-project scope changed")
	}
	f2RouterRead(t, f.token, "GET", "/api/issues?project_id="+f.project, nil).Want(403)
}
