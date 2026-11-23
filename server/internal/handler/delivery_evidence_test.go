package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func deliveryPR(t *testing.T, f *artifactFixture, head string, at time.Time) string {
	t.Helper()
	id := dbfx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": f.ws, "installation_id": 1, "repo_owner": "delivery", "repo_name": uuid.NewString(), "pr_number": 1, "title": "Delivery evidence", "state": "open", "html_url": "https://example.invalid/pr/1", "pr_created_at": at, "pr_updated_at": at, "head_sha": head})
	dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": f.req.ResourceID, "pull_request_id": id}, "issue_id=$1 AND pull_request_id=$2", f.req.ResourceID, id)
	return id
}
func deliveryRequest(t *testing.T, f *artifactFixture, out artifactResponse) deliveryEvidenceRequest {
	t.Helper()
	var revision int64
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.req.ResourceID).Scan(&revision)
	var receipt struct {
		OutputDigest string `json:"output_digest"`
	}
	if err := json.Unmarshal(out.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.OutputDigest == "" {
		receipt.OutputDigest = strings.Repeat("0", 64)
	}
	return deliveryEvidenceRequest{PolicyVersion: deliveryEvidencePolicy, IssueRevision: revision, ApprovalID: out.Approval.ID, SourceRevision: f.req.SourceRevision, SpecDigest: out.SpecDigest, ArtifactDigest: receipt.OutputDigest}
}
func deliveryCall(t *testing.T, f *artifactFixture, body any, status int) deliveryEvidenceResponse {
	t.Helper()
	var out deliveryEvidenceResponse
	req := testutil.WithURLParams(f.request("POST", "/api/issues/"+f.req.ResourceID+"/delivery-evidence/evaluate", body), "id", f.req.ResourceID)
	testutil.Call(t, f.h.EvaluateDeliveryEvidence, req).Want(status).JSON(&out)
	return out
}
func deliverySuccess(t *testing.T) (*artifactFixture, artifactResponse, deliveryEvidenceRequest, string) {
	t.Helper()
	f := newArtifactFixture(t, true)
	out := f.create(t)
	f.approve(t, out)
	out = f.execute(t, out, 200)
	pr := deliveryPR(t, f, f.req.SourceRevision, time.Now().UTC())
	return f, out, deliveryRequest(t, f, out), pr
}
func TestDeliveryEvidenceActualPackageSnapshot(t *testing.T) {
	f, receipt, req, _ := deliverySuccess(t)
	f.h.ArtifactPackager = nil
	for _, state := range []string{"running", "completed"} {
		t.Run(state, func(t *testing.T) {
			dbfx.Exec(t, `UPDATE agent_task_queue SET status=$1 WHERE id=$2`, state, f.req.TaskID)
			out := deliveryCall(t, f, req, 200)
			if out.Verdict != "pass" || out.Reason != "evidence_verified" || out.ReceiptDigest != receipt.ReceiptDigest || out.ExecutionID != receipt.ExecutionID || out.ProducerTaskID != f.req.TaskID || out.ProducerAgentID != f.req.AgentID || out.ValidUntil == nil || !out.ObservedAt.Before(*out.ValidUntil) {
				t.Fatalf("unbound observation: %+v", out)
			}
			var revision int64
			var retained []byte
			dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.req.ResourceID).Scan(&revision)
			dbfx.QueryRow(t, `SELECT receipt_json FROM agent_artifact_execution WHERE approval_id=$1`, req.ApprovalID).Scan(&retained)
			var before, after any
			if err := json.Unmarshal(receipt.Receipt, &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(retained, &after); err != nil {
				t.Fatal(err)
			}
			if revision != req.IssueRevision || !reflect.DeepEqual(before, after) {
				t.Fatal("observation mutated issue or receipt")
			}
		})
	}
}
func TestDeliveryEvidenceNoFalseGreen(t *testing.T) {
	cases := []struct {
		name, verdict, reason string
		status                int
	}{
		{"old_candidate", "hold", "source_revision_mismatch", 200}, {"advanced_head", "hold", "source_revision_mismatch", 200}, {"missing_head", "unknown", "source_head_missing", 200}, {"ambiguous_head", "unknown", "source_head_ambiguous", 200}, {"artifact", "hold", "artifact_digest_mismatch", 200}, {"spec", "hold", "spec_digest_mismatch", 200}, {"receipt_tamper", "hold", "receipt_invalid", 200}, {"row_tamper", "", "", 409}, {"issue_revision", "", "", 409}, {"generation", "", "", 409}, {"autonomy", "", "", 403}, {"originator_revoked", "", "", 403}, {"missing_receipt", "unknown", "evidence_missing", 200}, {"unknown", "unknown", "execution_unknown", 200}, {"failed", "hold", "execution_failed", 200}, {"expired", "hold", "evidence_expired", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, out, req, pr := deliverySuccess(t)
			switch tc.name {
			case "old_candidate":
				req.SourceRevision = strings.Repeat("b", 40)
			case "advanced_head":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), pr)
			case "missing_head":
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.req.ResourceID)
			case "ambiguous_head":
				var at time.Time
				dbfx.QueryRow(t, `SELECT pr_updated_at FROM github_pull_request WHERE id=$1`, pr).Scan(&at)
				deliveryPR(t, f, strings.Repeat("b", 40), at)
			case "artifact":
				req.ArtifactDigest = strings.Repeat("0", 64)
			case "spec":
				req.SpecDigest = strings.Repeat("0", 64)
			case "receipt_tamper":
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET receipt_json=receipt_json || '{"result":"failed"}'::jsonb WHERE approval_id=$1`, req.ApprovalID)
			case "row_tamper":
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET source_revision=$1 WHERE approval_id=$2`, strings.Repeat("b", 40), req.ApprovalID)
			case "issue_revision":
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.req.ResourceID)
			case "generation":
				dbfx.Exec(t, `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`, f.req.TaskID)
			case "autonomy":
				dbfx.Exec(t, `UPDATE agent SET autonomy_level='contributor' WHERE id=$1`, f.req.AgentID)
			case "originator_revoked":
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.actor)
			case "missing_receipt":
				req.ApprovalID = uuid.NewString()
			case "unknown", "failed":
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET result_status=$1,output_artifact_id=NULL,output_artifact_digest='',output_size=NULL WHERE approval_id=$2`, tc.name, req.ApprovalID)
			case "expired":
				var spec artifact.Spec
				if err := json.Unmarshal(out.Spec, &spec); err != nil {
					t.Fatal(err)
				}
				past := time.Now().UTC().Add(-16 * time.Minute).Truncate(time.Microsecond)
				spec.CreatedAt = past.Format("2006-01-02T15:04:05.000000Z")
				spec.ExpiresAt = past.Add(15 * time.Minute).Format("2006-01-02T15:04:05.000000Z")
				raw, digest, err := artifact.CanonicalSpec(spec)
				if err != nil {
					t.Fatal(err)
				}
				req.SpecDigest = digest
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET created_at=$1,expires_at=$2,canonical_spec=$3,spec_digest=$4 WHERE approval_id=$5`, past, past.Add(15*time.Minute), raw, digest, req.ApprovalID)
			}
			got := deliveryCall(t, f, req, tc.status)
			if tc.status == 200 && (got.Verdict != tc.verdict || got.Reason != tc.reason) {
				t.Fatalf("got %+v want %s/%s", got, tc.verdict, tc.reason)
			}
		})
	}
}
func TestDeliveryEvidenceWriterFirst(t *testing.T) {
	for _, target := range []string{"issue", "project", "pr", "link", "agent", "task", "member", "approval", "receipt"} {
		t.Run(target, func(t *testing.T) {
			f, _, req, pr := deliverySuccess(t)
			project := dbfx.Project(t, "Delivery project", testutil.Cols{"workspace_id": f.ws})
			dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.req.ResourceID)
			var rev int64
			dbfx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&rev)
			req.ProjectID = &project
			req.ProjectRevision = &rev
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			stmt, id := "", f.req.ResourceID
			switch target {
			case "issue":
				stmt = `UPDATE issue SET revision=revision+1 WHERE id=$1`
			case "project":
				stmt = `UPDATE project SET revision=revision+1 WHERE id=$1`
				id = project
			case "pr":
				stmt = `UPDATE github_pull_request SET head_sha=head_sha WHERE id=$1`
				id = pr
			case "phantom_github":
				stmt = `INSERT INTO issue_pull_request(issue_id,pull_request_id) SELECT $1,id FROM github_pull_request WHERE id='` + pr + `';`
			case "phantom_vcs":
				stmt = `INSERT INTO issue_vcs_pull_request(issue_id,pull_request_id) VALUES ($1,gen_random_uuid())`
			case "link":
				stmt = `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1`
			case "agent":
				stmt = `UPDATE agent SET autonomy_level='contributor' WHERE id=$1`
				id = f.req.AgentID
			case "task":
				stmt = `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`
				id = f.req.TaskID
			case "member":
				stmt = `DELETE FROM member WHERE workspace_id=$1`
				id = f.ws
			case "approval":
				stmt = `UPDATE agent_approval_request SET decision_note=decision_note WHERE id=$1`
				id = req.ApprovalID
			case "receipt":
				stmt = `UPDATE agent_artifact_execution SET result_code=result_code WHERE approval_id=$1`
				id = req.ApprovalID
			}
			if _, err = tx.Exec(context.Background(), stmt, id); err != nil {
				t.Fatal(err)
			}
			deliveryCall(t, f, req, 409)
		})
	}
}
func TestDeliveryEvidenceObservationFencesWriters(t *testing.T) {
	for _, target := range []string{"issue", "project", "pr", "link", "member", "phantom_github", "phantom_vcs"} {
		t.Run(target, func(t *testing.T) {
			f, _, req, pr := deliverySuccess(t)
			project := dbfx.Project(t, "Delivery project", testutil.Cols{"workspace_id": f.ws})
			dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.req.ResourceID)
			var rev int64
			dbfx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&rev)
			req.ProjectID = &project
			req.ProjectRevision = &rev
			if target == "phantom_github" {
				phantom := deliveryPR(t, f, strings.Repeat("b", 40), time.Now())
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1 AND pull_request_id=$2`, f.req.ResourceID, phantom)
				pr = phantom
			}
			entered, release := make(chan struct{}), make(chan struct{})
			f.h.TxStarter = &artifactBarrierStarter{txStarter: f.h.TxStarter, entered: entered, release: release}
			done := make(chan deliveryEvidenceResponse, 1)
			go func() { done <- deliveryCall(t, f, req, 200) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("evaluation did not reach actual commit")
			}
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				close(release)
				t.Fatal(err)
			}
			if _, err = tx.Exec(context.Background(), `SET LOCAL lock_timeout='100ms'`); err != nil {
				t.Fatal(err)
			}
			stmt, id := "", f.req.ResourceID
			switch target {
			case "issue":
				stmt = `UPDATE issue SET revision=revision+1 WHERE id=$1`
			case "project":
				stmt = `UPDATE project SET revision=revision+1 WHERE id=$1`
				id = project
			case "pr":
				stmt = `UPDATE github_pull_request SET head_sha=head_sha WHERE id=$1`
				id = pr
			case "phantom_github":
				stmt = `INSERT INTO issue_pull_request(issue_id,pull_request_id) SELECT $1,id FROM github_pull_request WHERE id='` + pr + `';`
			case "phantom_vcs":
				stmt = `INSERT INTO issue_vcs_pull_request(issue_id,pull_request_id) VALUES ($1,gen_random_uuid())`
			case "link":
				stmt = `UPDATE issue_pull_request SET reference_only=true WHERE issue_id=$1`
			case "member":
				stmt = `DELETE FROM member WHERE workspace_id=$1`
				id = f.ws
			}
			_, writeErr := tx.Exec(context.Background(), stmt, id)
			tx.Rollback(context.Background())
			close(release)
			var pgerr *pgconn.PgError
			if !errors.As(writeErr, &pgerr) || pgerr.Code != "55P03" {
				t.Fatalf("expected lock refusal 55P03, got %v", writeErr)
			}
			select {
			case got := <-done:
				if got.Verdict != "pass" {
					t.Fatalf("observation failed: %+v", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("evaluation stuck")
			}
		})
	}
}
func TestDeliveryEvidenceProjectBinding(t *testing.T) {
	f, _, req, _ := deliverySuccess(t)
	project := dbfx.Project(t, "Delivery project", testutil.Cols{"workspace_id": f.ws})
	dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.req.ResourceID)
	deliveryCall(t, f, req, 409)
	var rev int64
	dbfx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&rev)
	req.ProjectID = &project
	req.ProjectRevision = &rev
	if got := deliveryCall(t, f, req, 200); got.Verdict != "pass" {
		t.Fatal(got)
	}
	dbfx.Exec(t, `UPDATE project SET revision=revision+1 WHERE id=$1`, project)
	deliveryCall(t, f, req, 409)
}

func TestDeliveryEvidenceDistinctHistoricalPrincipals(t *testing.T) {
	for _, role := range []string{"originator", "runtime_owner", "initiator", "approver", "claimant", "invoke_originator"} {
		t.Run(role, func(t *testing.T) {
			f := newArtifactFixture(t, true)
			people := map[string]string{"runtime_owner": f.actor}
			for _, name := range []string{"observer", "originator", "initiator", "approver", "claimant"} {
				user := dbfx.User(t, name, name+uuid.NewString()+"@example.invalid")
				dbfx.Member(t, f.ws, user, "admin")
				people[name] = user
				dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.req.AgentID, "target_type": "member", "target_id": user})
			}
			dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.req.AgentID)
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, people["originator"], f.req.TaskID)
			f.actor = people["initiator"]
			out := f.create(t)
			f.actor = people["approver"]
			f.approve(t, out)
			f.actor = people["claimant"]
			out = f.execute(t, out, 200)
			deliveryPR(t, f, f.req.SourceRevision, time.Now())
			req := deliveryRequest(t, f, out)
			f.actor = people["observer"]
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, f.req.TaskID)
			if got := deliveryCall(t, f, req, 200); got.Verdict != "pass" {
				t.Fatal(got)
			}
			status := 403
			if role == "invoke_originator" {
				dbfx.Exec(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2`, f.req.AgentID, people["originator"])
			} else {
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, people[role])
				if role == "originator" || role == "runtime_owner" {
					status = 404
				}
			}
			deliveryCall(t, f, req, status)
		})
	}
}
func TestDeliveryEvidenceVCSSelection(t *testing.T) {
	f, _, req, github := deliverySuccess(t)
	now := time.Now().UTC().Add(time.Hour)
	pr := dbfx.Insert(t, "vcs_pull_request", testutil.Cols{"workspace_id": f.ws, "connection_id": uuid.NewString(), "provider": "gitlab", "repo_owner": "delivery", "repo_name": uuid.NewString(), "pr_number": 1, "title": "VCS candidate", "state": "open", "html_url": "https://example.invalid/pr/1", "pr_created_at": now, "pr_updated_at": now, "head_sha": strings.Repeat("b", 40)})
	dbfx.InsertNoID(t, "issue_vcs_pull_request", testutil.Cols{"issue_id": f.req.ResourceID, "pull_request_id": pr}, "issue_id=$1 AND pull_request_id=$2", f.req.ResourceID, pr)
	if got := deliveryCall(t, f, req, 200); got.Verdict != "hold" || got.ObservedHeadSHA != strings.Repeat("b", 40) {
		t.Fatal(got)
	}
	dbfx.Exec(t, `UPDATE issue_vcs_pull_request SET reference_only=true WHERE issue_id=$1`, f.req.ResourceID)
	if got := deliveryCall(t, f, req, 200); got.Verdict != "pass" {
		t.Fatal(got)
	}
	dbfx.Exec(t, `UPDATE issue_vcs_pull_request SET reference_only=false WHERE issue_id=$1`, f.req.ResourceID)
	dbfx.Exec(t, `UPDATE vcs_pull_request SET state='merged' WHERE id=$1`, pr)
	if got := deliveryCall(t, f, req, 200); got.Verdict != "pass" {
		t.Fatal("merged sibling shadowed current open", got)
	}
	dbfx.Exec(t, `UPDATE github_pull_request SET state='closed' WHERE id=$1`, github)
	if got := deliveryCall(t, f, req, 200); got.Verdict != "hold" {
		t.Fatal("latest terminal head ignored", got)
	}
}

func TestDeliveryEvidenceReceiptBindingSurvivesRehash(t *testing.T) {
	for _, field := range []string{"actor_id", "spec_digest", "declared_revision", "output_bytes", "extra"} {
		t.Run(field, func(t *testing.T) {
			f, out, req, _ := deliverySuccess(t)
			var receipt map[string]any
			decoder := json.NewDecoder(bytes.NewReader(out.Receipt))
			decoder.UseNumber()
			if err := decoder.Decode(&receipt); err != nil {
				t.Fatal(err)
			}
			if field == "output_bytes" {
				receipt[field] = json.Number("9007199254740993")
			} else {
				receipt[field] = "different"
			}
			raw, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(raw))
			dbfx.Exec(t, `UPDATE agent_artifact_execution SET receipt_json=$1,receipt_digest=$2 WHERE approval_id=$3`, raw, hash, req.ApprovalID)
			if got := deliveryCall(t, f, req, 200); got.Verdict != "hold" || got.Reason != "receipt_invalid" {
				t.Fatal(got)
			}
		})
	}
}

func TestDeliveryEvidenceCommitFailure(t *testing.T) {
	for _, after := range []bool{false, true} {
		name := "before"
		if after {
			name = "after"
		}
		t.Run(name, func(t *testing.T) {
			f, receipt, req, _ := deliverySuccess(t)
			f.h.ArtifactPackager = nil
			f.h.TxStarter = &artifactFaultStarter{txStarter: f.h.TxStarter, failAt: 1, after: after}
			got := deliveryCall(t, f, req, 503)
			if got.Verdict != "" || !got.ObservedAt.IsZero() {
				t.Fatal("failed commit returned usable observation", got)
			}
			var revision int64
			var digest string
			dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.req.ResourceID).Scan(&revision)
			dbfx.QueryRow(t, `SELECT receipt_digest FROM agent_artifact_execution WHERE approval_id=$1`, req.ApprovalID).Scan(&digest)
			if revision != req.IssueRevision || digest != receipt.ReceiptDigest {
				t.Fatal("observation failure mutated business facts")
			}
		})
	}
}
