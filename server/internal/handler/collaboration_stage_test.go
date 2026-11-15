package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageHTTP struct {
	h                                *Handler
	fx                               *testutil.Fixture
	svc                              *service.CollaborationService
	actor, recipient, issue, project string
	policyRevision                   int64
}

func stagePrincipal(user string) auth.AuthenticatedPrincipal {
	return auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}
}

func stageHTTPFixture(t *testing.T) *stageHTTP {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	h, fx, actor, recipient := collaborationCreationHTTPFixture(t)
	if err := h.Queries.SeedIssueStatusEntries(t.Context(), parseUUID(fx.WorkspaceID)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, fx.WorkspaceID)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, fx.WorkspaceID)
	for _, table := range []string{"collaboration_stage_child", "collaboration_stage_revocation", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, fx.WorkspaceID)
	}
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id=$1)`, fx.WorkspaceID)
	project := fx.Project(t, "Stage governed project")
	created := testutil.Call(t, h.CreateCollaborationIssue, collaborationCreationHTTPRequest(http.MethodPost, fx.WorkspaceID, uuid.NewString(), actor,
		map[string]any{"source_workspace_id": fx.WorkspaceID, "handler_user_id": recipient, "title": "Stage governed root", "description": "Requirement text.", "project_id": project}, false)).Want(http.StatusCreated).Map()
	issue, ok := created["id"].(string)
	if !ok {
		t.Fatal("creation returned no issue identity")
	}
	return &stageHTTP{h: h, fx: fx, svc: service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService), actor: actor, recipient: recipient, issue: issue, project: project}
}

// request targets one stage route as an authenticated account. Ambient
// workspace headers are deliberately wrong: the account routes ignore them.
func (s *stageHTTP) request(method, suffix, actor, key string, body any, params ...string) *http.Request {
	r := testutil.WithURLParams(testutil.JSONRequest(method, "/api/collaboration/issues/"+s.issue+suffix, body), append([]string{"id", s.issue, "requestId", key}, params...)...)
	testutil.WithHeaders(r, "Idempotency-Key", key, "X-Workspace-ID", "unrelated-ambient-workspace", "X-Workspace-Slug", "unrelated-ambient-workspace")
	if actor != "" {
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), stagePrincipal(actor)))
	}
	return r
}

// observations reads the actor's current write context, as a client would.
func (s *stageHTTP) observations(t *testing.T, actor string) map[string]any {
	t.Helper()
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), stagePrincipal(actor))
	access, err := s.svc.ResolveIssueAccess(ctx, nil, parseUUID(s.issue))
	if err != nil {
		t.Fatal(err)
	}
	text := func(v int64) string { return int64Text(v) }
	return map[string]any{"expected_collaboration_revision": text(access.Revision), "expected_lifecycle_epoch": text(access.LifecycleEpoch),
		"expected_participant_epoch": text(access.ParticipantEpoch.Int64), "expected_account_epoch": text(access.AccountEpoch.Int64)}
}

func int64Text(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for ; v > 0; v /= 10 {
		digits = string(rune('0'+v%10)) + digits
	}
	return digits
}

func (s *stageHTTP) body(t *testing.T, actor string, fields map[string]any) map[string]any {
	t.Helper()
	body := s.observations(t, actor)
	for key, value := range fields {
		body[key] = value
	}
	return body
}

// revise prepares policy rows for issue-action fixtures. Policy HTTP journeys
// use the actual route instead of this setup helper.
func (s *stageHTTP) revise(t *testing.T, enabled bool) {
	t.Helper()
	_, err := s.svc.ReviseStagePolicy(t.Context(), service.StagePolicyRevisionParams{WorkspaceID: parseUUID(s.fx.WorkspaceID), ProjectID: parseUUID(s.project),
		RequestID: uuid.NewString(), ExpectedRevision: s.policyRevision, Enabled: enabled, Reason: "handler fixture policy revision",
		Credential: service.ArtifactCredentialSnapshot{Principal: stagePrincipal(s.actor)}})
	if err != nil {
		t.Fatal(err)
	}
	s.policyRevision++
}

func (s *stageHTTP) policyRequest(method, suffix, actor, key string, body any) *http.Request {
	r := testutil.WithURLParams(testutil.JSONRequest(method, "/api/projects/"+s.project+"/stage-policy"+suffix, body), "id", s.project, "requestId", key)
	testutil.WithHeaders(r, "X-Workspace-ID", s.fx.WorkspaceID, "X-User-ID", actor)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if actor != "" {
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), stagePrincipal(actor)))
	}
	return r
}

func (s *stageHTTP) roles() map[string]any {
	// Three more participants so the two separations can hold.
	return map[string]any{"business_owner_user_id": s.actor, "implementer_user_id": s.recipient, "product_approver_user_id": s.actor}
}

func TestStagePolicyHTTPActivationAndOriginalKey(t *testing.T) {
	s := stageHTTPFixture(t)
	rows := func() int {
		return s.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, s.project)
	}
	read := testutil.Call(t, s.h.GetStagePolicy, s.policyRequest(http.MethodGet, "", s.actor, "", nil)).Want(http.StatusOK).Map()
	if read["policy_revision"] != "0" || read["enabled"] != false || read["contract"] != "collaboration-stage-v1" || read["created_by"] != nil || rows() != 0 {
		t.Fatalf("virtual policy read: %v", read)
	}
	witness, err := readStagePolicyReadinessWitness(t.Context(), s.fx.Pool)
	if err != nil || !witness.healthy() || s.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2 AND role IN ('owner','admin')`, s.fx.WorkspaceID, s.actor) != 1 {
		t.Fatalf("healthy native schema and current owner required: %+v error=%s", witness, stagePolicySQLFailure(err))
	}
	before := s.stagePolicyBusinessSnapshot(t)
	enableKey := uuid.NewString()
	enable := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "switch the stage gates on"}
	enabledCall := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, enableKey, enable))
	enabled := enabledCall.Map()
	// Record the real PUT before asserting 200. The initial false/503 RED must
	// reach this request, rather than fail early on the GET capability flag.
	t.Logf("P2 healthy activation PUT status=%d response=%v policy_rows=%d schema=%+v", enabledCall.Code, enabled, rows(), witness)
	enabledCall.Want(http.StatusOK)
	confirmation, _ := enabled["stage_confirmation"].(map[string]any)
	if enabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != s.project || confirmation["policy_revision"] != "1" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 1 {
		t.Fatalf("enable confirmation: %v rows=%d", enabled, rows())
	}
	if read["activation_available"] != true || s.stagePolicyBusinessSnapshot(t) != before {
		t.Fatalf("enabling did not advertise availability or changed business facts: %v", read)
	}
	for name, tc := range map[string]struct {
		body any
		key  string
		code string
	}{
		"unknown_field":  {`{"expected_policy_revision":"0","enabled":false,"reason":"x","force":true}`, uuid.NewString(), "invalid_request"},
		"duplicate":      {`{"expected_policy_revision":"0","enabled":false,"enabled":true,"reason":"x"}`, uuid.NewString(), "invalid_request"},
		"missing_reason": {map[string]any{"expected_policy_revision": "0", "enabled": false}, uuid.NewString(), "invalid_stage_policy_request"},
		"null_enabled":   {`{"expected_policy_revision":"0","enabled":null,"reason":"x"}`, uuid.NewString(), "invalid_stage_policy_request"},
		"numeric":        {`{"expected_policy_revision":0,"enabled":false,"reason":"x"}`, uuid.NewString(), "invalid_request"},
		"leading_zero":   {map[string]any{"expected_policy_revision": "01", "enabled": false, "reason": "x"}, uuid.NewString(), "invalid_stage_policy_request"},
		"missing_key":    {map[string]any{"expected_policy_revision": "0", "enabled": false, "reason": "x"}, "", "invalid_request_id"},
	} {
		if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, tc.key, tc.body)).Want(http.StatusBadRequest).Map(); got["code"] != tc.code || rows() != 1 {
			t.Fatalf("%s: %v", name, got)
		}
	}
	machine := s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), map[string]any{"expected_policy_revision": "0", "enabled": false, "reason": "x"})
	machine.Header.Set("X-Agent-ID", uuid.NewString())
	if got := testutil.Call(t, s.h.UpdateStagePolicy, machine).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_denied" {
		t.Fatalf("agent actor reached the policy write: %v", got)
	}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", "", uuid.NewString(), enable)).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_denied" || rows() != 1 {
		t.Fatalf("unauthenticated handler policy write: %v", got)
	}
	read = testutil.Call(t, s.h.GetStagePolicy, s.policyRequest(http.MethodGet, "", s.actor, "", nil)).Want(http.StatusOK).Map()
	if read["policy_revision"] != "1" || read["enabled"] != true || read["activation_available"] != true {
		t.Fatalf("enabled policy read: %v", read)
	}
	view := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.actor, "", nil)).Want(http.StatusOK).Map()
	missing, _ := view["missing"].([]any)
	stage, _ := view["stage"].(map[string]any)
	if view["managed"] != true || view["needs_confirmation"] != true || len(missing) != 2 || stage["current"] != "development" || s.stagePolicyBusinessSnapshot(t) != before {
		t.Fatalf("first enable automatically configured or advanced the root: %v", view)
	}
	disableKey := uuid.NewString()
	disable := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "explicit rollback"}
	disabled := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, disableKey, disable)).Want(http.StatusOK).Map()
	confirmation, _ = disabled["stage_confirmation"].(map[string]any)
	if disabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["policy_revision"] != "2" || confirmation["enabled"] != false ||
		confirmation["project_id"] != s.project || len(confirmation) != 4 {
		t.Fatalf("disable confirmation: %v", disabled)
	}
	againKey := uuid.NewString()
	again := map[string]any{"expected_policy_revision": "2", "enabled": true, "reason": "re-enable"}
	reenabled := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, againKey, again)).Want(http.StatusOK).Map()
	confirmation, _ = reenabled["stage_confirmation"].(map[string]any)
	if reenabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != s.project || confirmation["policy_revision"] != "3" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 3 {
		t.Fatalf("re-enable confirmation: %v rows=%d", reenabled, rows())
	}
	for _, original := range []struct {
		key     string
		body    map[string]any
		receipt map[string]any
	}{{enableKey, enable, enabled}, {disableKey, disable, disabled}, {againKey, again, reenabled}} {
		replay := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, original.key, original.body)).Want(http.StatusOK).Map()
		recovered := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+original.key, s.actor, original.key, nil)).Want(http.StatusOK).Map()
		if !reflect.DeepEqual(replay, original.receipt) || !reflect.DeepEqual(recovered, original.receipt) || rows() != 3 || s.stagePolicyBusinessSnapshot(t) != before {
			t.Fatalf("original key adopted a new revision or effect: replay=%v recovered=%v", replay, recovered)
		}
	}
	unknownKey := uuid.NewString()
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+unknownKey, s.actor, unknownKey, nil)).Want(http.StatusNotFound).Map(); got["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("unknown key: %v", got)
	}
	stale := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "stale"}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), stale)).Want(http.StatusConflict).Map(); got["code"] != "stage_policy_revision_conflict" {
		t.Fatalf("stale revision: %v", got)
	}
	for _, changed := range []map[string]any{
		{"expected_policy_revision": "0", "enabled": false, "reason": enable["reason"]},
		{"expected_policy_revision": "0", "enabled": true, "reason": "a changed intent"},
		{"expected_policy_revision": "3", "enabled": true, "reason": enable["reason"]},
	} {
		if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, enableKey, changed)).Want(http.StatusConflict).Map(); got["code"] != "idempotency_conflict" || rows() != 3 {
			t.Fatalf("changed body reused original key: %v", got)
		}
	}
	sameKey := uuid.NewString()
	same := map[string]any{"expected_policy_revision": "3", "enabled": true, "reason": "explicit same-value policy revision"}
	sameReceipt := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, sameKey, same)).Want(http.StatusOK).Map()
	confirmation, _ = sameReceipt["stage_confirmation"].(map[string]any)
	if sameReceipt["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != s.project || confirmation["policy_revision"] != "4" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 4 {
		t.Fatalf("explicit same-value revision: %v rows=%d", sameReceipt, rows())
	}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, sameKey, same)).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, sameReceipt) || rows() != 4 {
		t.Fatalf("same-value replay wrote again: %v", got)
	}
	member := s.fx.User(t, "Policy reader", "stage-policy-"+uuid.NewString()+"@example.invalid")
	s.fx.Member(t, s.fx.WorkspaceID, member, "member")
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", member, uuid.NewString(), map[string]any{"expected_policy_revision": "4", "enabled": false, "reason": "not a manager"})).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_management_denied" || rows() != 4 {
		t.Fatalf("plain member wrote a policy: %v", got)
	}
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+enableKey, member, enableKey, nil)).Want(http.StatusNotFound).Map(); got["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("another actor recovered the original key: %v", got)
	}
	otherProject := s.fx.Project(t, "Other policy project")
	other := *s
	other.project = otherProject
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, other.policyRequest(http.MethodGet, "/operations/"+enableKey, s.actor, enableKey, nil)).Want(http.StatusNotFound).Map(); got["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("another project recovered the original key: %v", got)
	}
	s.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, s.fx.WorkspaceID, s.actor)
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, enableKey, enable)).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, enabled) || rows() != 4 {
		t.Fatalf("former manager lost the historical receipt: %v", got)
	}
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+enableKey, s.actor, enableKey, nil)).Want(http.StatusOK).Map(); !reflect.DeepEqual(got, enabled) {
		t.Fatalf("former manager GET changed the historical receipt: %v", got)
	}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), map[string]any{"expected_policy_revision": "4", "enabled": false, "reason": "new write after demotion"})).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_management_denied" || rows() != 4 {
		t.Fatalf("former manager wrote a fresh revision: %v", got)
	}
	expired := stagePrincipal(s.actor)
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	expiredRequest := s.policyRequest(http.MethodGet, "/operations/"+enableKey, s.actor, enableKey, nil)
	expiredRequest = expiredRequest.WithContext(auth.WithAuthenticatedPrincipal(expiredRequest.Context(), expired))
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, expiredRequest).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_denied" {
		t.Fatalf("expired credential recovered the original key: %v", got)
	}
	s.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, s.fx.WorkspaceID, s.actor)
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+enableKey, s.actor, enableKey, nil)).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_denied" {
		t.Fatalf("removed source member recovered the original key: %v", got)
	}
	if rows() != 4 || s.stagePolicyBusinessSnapshot(t) != before {
		t.Fatal("policy requests changed unrelated business facts")
	}
}

type stagePolicyReadinessWitness struct {
	Tables                bool
	Ledger                bool
	Execution             bool
	Submission            bool
	SubmissionIndexOID    int64
	ExecutionIndexOID     int64
	PolicyRequestIndexOID int64
}

func (w stagePolicyReadinessWitness) healthy() bool {
	return w.Tables && w.Ledger && w.Execution && w.Submission && w.SubmissionIndexOID > 0 && w.ExecutionIndexOID > 0 && w.PolicyRequestIndexOID > 0
}

func readStagePolicyReadinessWitness(ctx context.Context, source db.DBTX) (out stagePolicyReadinessWitness, err error) {
	q := db.New(source)
	if out.Tables, err = q.CollaborationStageSchemaReady(ctx); err != nil {
		return out, err
	}
	execution, err := q.StageExecutionSchemaReady(ctx)
	if err != nil {
		return out, err
	}
	out.Execution = execution.Valid && execution.Bool
	submission, err := q.CollaborationStageSubmissionReady(ctx)
	if err != nil {
		return out, err
	}
	out.Submission = submission.Valid && submission.Bool
	err = source.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version='864_collaboration_stage_submission_ready'),
 COALESCE(to_regclass('collaboration_stage_submission_id_idx')::oid::bigint,0),
 COALESCE(to_regclass('collaboration_stage_execution_approval_idx')::oid::bigint,0),
 COALESCE(to_regclass('project_stage_policy_request_idx')::oid::bigint,0)`).Scan(&out.Ledger, &out.SubmissionIndexOID, &out.ExecutionIndexOID, &out.PolicyRequestIndexOID)
	return out, err
}

func stagePolicySQLFailure(err error) string {
	if err == nil {
		return "none"
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return "SQLSTATE " + pgerr.Code
	}
	return fmt.Sprintf("%T", err)
}

func (s *stageHTTP) stagePolicyBusinessSnapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	s.fx.QueryRow(t, `SELECT jsonb_build_object(
 'stage_tasks',(SELECT count(*) FROM collaboration_stage_task WHERE issue_id=$1),
 'roles',(SELECT count(*) FROM collaboration_stage_role WHERE issue_id=$1),
 'candidates',(SELECT count(*) FROM collaboration_stage_candidate WHERE issue_id=$1),
 'decisions',(SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1),
 'scope_children',(SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1),
 'submissions',(SELECT count(*) FROM collaboration_stage_submission WHERE issue_id=$1),
 'children',(SELECT count(*) FROM issue WHERE parent_issue_id=$1),
 'tasks',(SELECT jsonb_agg(jsonb_build_object('id',id,'status',status) ORDER BY id) FROM agent_task_queue WHERE issue_id=$1 OR issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)),
 'runs',(SELECT count(*) FROM collaboration_run WHERE session_id IN (SELECT id FROM collaboration_session WHERE issue_id=$1)),
 'events',(SELECT count(*) FROM collaboration_event WHERE issue_id=$1),
 'notifications',(SELECT count(*) FROM collaboration_notification WHERE issue_id=$1))::text`, s.issue).Scan(&snapshot)
	return snapshot
}

// The fixture changes real PostgreSQL state before returning the same native
// transaction. It does not substitute any query, row, readiness result or commit.
type stagePolicySchemaMutationStarter struct {
	service.TxStarter
	mutation    string
	calls       int
	affected    int64
	transaction pgx.Tx
	before      stagePolicyReadinessWitness
	after       stagePolicyReadinessWitness
	err         error
}

func (s *stagePolicySchemaMutationStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		s.err = err
		return nil, err
	}
	s.calls++
	s.transaction = tx
	returned := false
	defer func() {
		if !returned {
			_ = tx.Rollback(ctx)
		}
	}()
	if s.before, err = readStagePolicyReadinessWitness(ctx, tx); err != nil {
		s.err = err
		return nil, err
	}
	result, err := tx.Exec(ctx, s.mutation)
	if err != nil {
		s.err = err
		return nil, err
	}
	s.affected = result.RowsAffected()
	if s.after, err = readStagePolicyReadinessWitness(ctx, tx); err != nil {
		s.err = err
		return nil, err
	}
	returned = true
	return tx, nil
}

func TestStagePolicyHTTPReadinessUsesActualSchema(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutation string
	}{
		{"missing_864_ledger", `DELETE FROM schema_migrations WHERE version='864_collaboration_stage_submission_ready';`},
		{"submission_id_index", `DROP INDEX collaboration_stage_submission_id_idx;`},
		{"execution_approval_index", `DROP INDEX collaboration_stage_execution_approval_idx;`},
		{"foundation_policy_index", `DROP INDEX project_stage_policy_request_idx;`},
	} {
		if !t.Run(tc.name, func(t *testing.T) {
			s := stageHTTPFixture(t)
			testutil.Call(t, s.h.GetStagePolicy, s.policyRequest(http.MethodGet, "", s.actor, "", nil)).Want(http.StatusOK)
			before, err := readStagePolicyReadinessWitness(t.Context(), s.fx.Pool)
			if err != nil || !before.healthy() {
				t.Fatalf("actual ready fixture required: %+v error=%s", before, stagePolicySQLFailure(err))
			}
			business := s.stagePolicyBusinessSnapshot(t)
			starter := &stagePolicySchemaMutationStarter{TxStarter: s.h.TxStarter, mutation: tc.mutation}
			mutating := *s.h
			mutating.TxStarter = starter
			key := uuid.NewString()
			body := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "actual schema negative"}
			response := testutil.Call(t, mutating.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, key, body))
			out := response.Map()
			rows := s.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, s.project)
			after, afterErr := readStagePolicyReadinessWitness(t.Context(), s.fx.Pool)
			t.Logf("P2 native schema control=%s status=%d response=%v calls=%d tx=%T affected=%d before=%+v inside=%+v after=%+v error=%s", tc.name, response.Code, out, starter.calls, starter.transaction, starter.affected, starter.before, starter.after, after, stagePolicySQLFailure(starter.err))
			if starter.err != nil || afterErr != nil {
				t.Fatalf("schema fixture/control failure: mutation=%s after=%s", stagePolicySQLFailure(starter.err), stagePolicySQLFailure(afterErr))
			}
			response.Want(http.StatusServiceUnavailable)
			if out["code"] != "stage_schema_unavailable" || out["error"] != "stage_schema_unavailable" || out["stage_confirmation"] != nil || starter.calls != 1 || starter.transaction == nil || starter.before != before {
				t.Fatalf("actual schema control was not consumed: %v calls=%d before=%+v", out, starter.calls, starter.before)
			}
			switch tc.name {
			case "missing_864_ledger":
				want := before
				want.Ledger = false
				if starter.affected != 1 || starter.after != want {
					t.Fatalf("actual missing ledger did not isolate exact864: affected=%d inside=%+v", starter.affected, starter.after)
				}
			case "submission_id_index":
				want := before
				want.Submission, want.SubmissionIndexOID = false, 0
				if starter.after != want {
					t.Fatalf("actual missing submission index witness: %+v", starter.after)
				}
			case "execution_approval_index":
				want := before
				want.Execution, want.ExecutionIndexOID = false, 0
				if starter.after != want {
					t.Fatalf("actual missing execution index witness: %+v", starter.after)
				}
			case "foundation_policy_index":
				want := before
				want.PolicyRequestIndexOID = 0
				if starter.after != want {
					t.Fatalf("actual missing foundation index witness: %+v", starter.after)
				}
			}
			if after != before || rows != 0 || s.stagePolicyBusinessSnapshot(t) != business {
				t.Fatalf("actual refusal did not roll back schema and business effects: after=%+v rows=%d", after, rows)
			}
			recovered := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+key, s.actor, key, nil)).Want(http.StatusNotFound).Map()
			if recovered["code"] != "stage_policy_operation_not_found" {
				t.Fatalf("schema refusal became a successful original-key receipt: %v", recovered)
			}
		}) {
			// An unexpected commit may have changed this dedicated test database.
			// Keep its failed state and do not run another control against it.
			break
		}
	}
}

const stagePolicySubmissionIndexRestoreSQL = `CREATE UNIQUE INDEX CONCURRENTLY collaboration_stage_submission_id_idx ON collaboration_stage_submission (submission_id);`

func restoreStagePolicySubmissionIndex(ctx context.Context, source db.DBTX) (stagePolicyReadinessWitness, error) {
	witness, err := readStagePolicyReadinessWitness(ctx, source)
	if err != nil {
		return witness, err
	}
	if witness.SubmissionIndexOID == 0 {
		if _, err = source.Exec(ctx, stagePolicySubmissionIndexRestoreSQL); err != nil {
			return witness, err
		}
	}
	return readStagePolicyReadinessWitness(ctx, source)
}

func TestStagePolicyHTTPDisableWithIncompleteSubmissionSchema(t *testing.T) {
	s := stageHTTPFixture(t)
	before, err := readStagePolicyReadinessWitness(t.Context(), s.fx.Pool)
	if err != nil || !before.healthy() {
		t.Fatalf("actual ready fixture required: %+v error=%s", before, stagePolicySQLFailure(err))
	}
	business := s.stagePolicyBusinessSnapshot(t)
	rows := func() int {
		return s.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, s.project)
	}
	enableKey := uuid.NewString()
	enable := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "enable before schema rollback control"}
	enableCall := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, enableKey, enable))
	enabled := enableCall.Map()
	t.Logf("P2 rollback setup actual enable status=%d response=%v schema=%+v", enableCall.Code, enabled, before)
	enableCall.Want(http.StatusOK)
	confirmation, _ := enabled["stage_confirmation"].(map[string]any)
	if enabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != s.project || confirmation["policy_revision"] != "1" || confirmation["enabled"] != true || len(confirmation) != 4 || rows() != 1 {
		t.Fatalf("actual enabling confirmation: %v", enabled)
	}
	// This restores only the deliberate test index control, on this exact test
	// database, using the original 862 statement outside an explicit transaction.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		witness, restoreErr := restoreStagePolicySubmissionIndex(ctx, s.fx.Pool)
		if restoreErr != nil || !witness.healthy() {
			t.Errorf("test-only submission index restoration: %+v error=%s", witness, stagePolicySQLFailure(restoreErr))
		}
	})
	starter := &stagePolicySchemaMutationStarter{TxStarter: s.h.TxStarter, mutation: `DROP INDEX collaboration_stage_submission_id_idx;`}
	mutating := *s.h
	mutating.TxStarter = starter
	disableKey := uuid.NewString()
	disable := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "explicit disable while final schema is unavailable"}
	disableCall := testutil.Call(t, mutating.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, disableKey, disable))
	disabled := disableCall.Map()
	t.Logf("P2 actual disable with native missing index status=%d response=%v calls=%d tx=%T inside=%+v error=%s", disableCall.Code, disabled, starter.calls, starter.transaction, starter.after, stagePolicySQLFailure(starter.err))
	disableCall.Want(http.StatusOK)
	confirmation, _ = disabled["stage_confirmation"].(map[string]any)
	inside := before
	inside.Submission, inside.SubmissionIndexOID = false, 0
	if starter.err != nil || starter.calls != 1 || starter.transaction == nil || starter.before != before || starter.after != inside || disabled["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["project_id"] != s.project || confirmation["policy_revision"] != "2" || confirmation["enabled"] != false || len(confirmation) != 4 || rows() != 2 {
		t.Fatalf("actual disable did not commit its own revision/control: %v inside=%+v", disabled, starter.after)
	}
	missing, err := readStagePolicyReadinessWitness(t.Context(), s.fx.Pool)
	if err != nil || missing != inside {
		t.Fatalf("test-only missing index was not actually committed: %+v error=%s", missing, stagePolicySQLFailure(err))
	}
	for _, original := range []struct {
		key     string
		body    map[string]any
		receipt map[string]any
	}{{enableKey, enable, enabled}, {disableKey, disable, disabled}} {
		replay := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, original.key, original.body)).Want(http.StatusOK).Map()
		recovered := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+original.key, s.actor, original.key, nil)).Want(http.StatusOK).Map()
		if !reflect.DeepEqual(replay, original.receipt) || !reflect.DeepEqual(recovered, original.receipt) || rows() != 2 || s.stagePolicyBusinessSnapshot(t) != business {
			t.Fatalf("missing final schema changed historical receipt/effects: replay=%v recovered=%v", replay, recovered)
		}
	}
	newKey := uuid.NewString()
	refused := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, newKey, map[string]any{"expected_policy_revision": "2", "enabled": true, "reason": "new enabling with actual missing index"})).Want(http.StatusServiceUnavailable).Map()
	if refused["code"] != "stage_schema_unavailable" || refused["error"] != "stage_schema_unavailable" || refused["stage_confirmation"] != nil || rows() != 2 || s.stagePolicyBusinessSnapshot(t) != business {
		t.Fatalf("new enabling bypassed actual missing schema: %v", refused)
	}
	if recovered := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+newKey, s.actor, newKey, nil)).Want(http.StatusNotFound).Map(); recovered["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("refused new enabling got an original success: %v", recovered)
	}
	restored, err := restoreStagePolicySubmissionIndex(t.Context(), s.fx.Pool)
	if err != nil || !restored.healthy() || restored.ExecutionIndexOID != before.ExecutionIndexOID || restored.PolicyRequestIndexOID != before.PolicyRequestIndexOID || rows() != 2 || s.stagePolicyBusinessSnapshot(t) != business {
		t.Fatalf("original862 logical schema restoration: %+v error=%s", restored, stagePolicySQLFailure(err))
	}
	t.Logf("P2 actual submission index OID before=%d missing=%d restored=%d logical_submission_ready=%t", before.SubmissionIndexOID, missing.SubmissionIndexOID, restored.SubmissionIndexOID, restored.Submission)
}

func TestStageHTTPStrictBodies(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"roles": (&Handler{}).ConfigureCollaborationStageRoles, "create": (&Handler{}).CreateCollaborationStageProposal,
		"revise": (&Handler{}).ReviseCollaborationStageProposal, "decide": (&Handler{}).DecideCollaborationStageProposal,
		"scope": (&Handler{}).ConfirmCollaborationStageScope,
	} {
		for _, body := range []string{
			`{"unknown":"x"}`,
			`{"expected_policy_revision":"1","expected_policy_revision":"2"}`,
			`{} {}`,
			`null`,
			`{"expected_policy_revision":1}`,
			`{"Expected_Policy_Revision":"1","expected_policy_revision":"1"}`,
		} {
			if out := testutil.Call(t, handler, testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused/stage", body)).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_request" {
				t.Fatalf("%s accepted non-strict JSON %s: %v", name, body, out)
			}
		}
		request := testutil.JSONRequest(http.MethodPost, "/api/collaboration/issues/unused/stage", `{}`)
		request.Header.Set("Content-Type", "text/plain")
		if out := testutil.Call(t, handler, request).Want(http.StatusUnsupportedMediaType).Map(); out["code"] != "json_required" {
			t.Fatalf("%s accepted a non-JSON media type", name)
		}
	}
	nested := `{"expected_collaboration_revision":"1","expected_lifecycle_epoch":"1","expected_participant_epoch":"1","expected_account_epoch":"1",` +
		`"expected_policy_revision":"1","content":{"summary":"s","children":[{"title":"a","description":"","priority":"none","assignee_id":"x"}]},"source":{"kind":"manual"}}`
	if out := testutil.Call(t, (&Handler{}).CreateCollaborationStageProposal, testutil.JSONRequest(http.MethodPost, "/x", nested)).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_request" {
		t.Fatalf("an unknown nested field was accepted: %v", out)
	}
	if out := testutil.Call(t, (&Handler{}).ConfigureCollaborationStageRoles, testutil.JSONRequest(http.MethodPost, "/x", strings.Repeat(" ", (64<<10)+1))).Want(http.StatusRequestEntityTooLarge).Map(); out["code"] != "request_too_large" {
		t.Fatal("role configuration did not bound its request")
	}
}

func TestStageHTTPActionsConfirmationsRefusalsAndRecovery(t *testing.T) {
	s := stageHTTPFixture(t)
	issueID, user := parseUUID(s.issue), func(name string) string {
		return s.fx.User(t, name, strings.ReplaceAll(uuid.NewString(), "-", "")+"@example.test")
	}
	reviewer, qa, release := user("Reviewer"), user("QA"), user("Release")
	s.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{reviewer, qa, release})
	for _, target := range []string{reviewer, qa, release} {
		testutil.Call(t, s.h.InviteCollaborationParticipant, s.request(http.MethodPost, "/participants", s.actor, uuid.NewString(), s.body(t, s.actor, map[string]any{"target_user_id": target}))).Want(http.StatusOK)
	}
	roles := s.roles()
	roles["code_reviewer_user_id"], roles["qa_approver_user_id"], roles["release_approver_user_id"] = reviewer, qa, release
	configure := func(actor string, policy string) map[string]any {
		return s.body(t, actor, map[string]any{"expected_policy_revision": policy, "expected_role_revision": "0", "roles": roles})
	}
	// Without an enabled policy the stage is unmanaged and every write refuses.
	view := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.actor, "", nil)).Want(http.StatusOK).Map()
	if view["managed"] != false || view["needs_confirmation"] != false || view["stage"] != nil || view["protocol_version"] != float64(1) {
		t.Fatalf("unmanaged view: %v", view)
	}
	refusedKey := uuid.NewString()
	refused := testutil.Call(t, s.h.ConfigureCollaborationStageRoles, s.request(http.MethodPost, "/stage/roles", s.actor, refusedKey, configure(s.actor, "1"))).Want(http.StatusConflict).Map()
	if refused["code"] != "stage_policy_disabled" || refused["status"] != float64(http.StatusConflict) || refused["collaboration_revision"] == nil || refused["stage_confirmation"] != nil {
		t.Fatalf("stored refusal body: %v", refused)
	}
	recovered := testutil.Call(t, s.h.GetCollaborationOperation, s.request(http.MethodGet, "/operations/"+refusedKey, s.actor, refusedKey, nil)).Want(http.StatusOK).Map()
	if recovered["code"] != "stage_policy_disabled" || recovered["status"] != float64(http.StatusConflict) || recovered["stage_confirmation"] != nil {
		t.Fatalf("stored refusal recovery: %v", recovered)
	}
	s.revise(t, true)
	for name, fields := range map[string]map[string]any{
		"leading_zero": {"expected_policy_revision": "01", "expected_role_revision": "0", "roles": roles},
		"negative":     {"expected_policy_revision": "1", "expected_role_revision": "-1", "roles": roles},
		"missing":      {"expected_policy_revision": "1", "expected_role_revision": "0"},
		"uppercase_id": {"expected_policy_revision": "1", "expected_role_revision": "0", "roles": map[string]any{"business_owner_user_id": strings.ToUpper(s.actor),
			"implementer_user_id": s.recipient, "product_approver_user_id": s.actor, "code_reviewer_user_id": reviewer, "qa_approver_user_id": qa, "release_approver_user_id": release}},
	} {
		if out := testutil.Call(t, s.h.ConfigureCollaborationStageRoles, s.request(http.MethodPost, "/stage/roles", s.actor, uuid.NewString(), s.body(t, s.actor, fields))).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_stage_request" {
			t.Fatalf("%s: %v", name, out)
		}
	}
	machine := s.request(http.MethodPost, "/stage/roles", "", uuid.NewString(), configure(s.actor, "1"))
	machine = machine.WithContext(auth.WithAuthenticatedPrincipal(machine.Context(), auth.AuthenticatedPrincipal{UserID: s.actor, Kind: auth.CredentialTask}))
	if out := testutil.Call(t, s.h.ConfigureCollaborationStageRoles, machine).Want(http.StatusForbidden).Map(); out["code"] != "forbidden" {
		t.Fatalf("machine credential: %v", out)
	}
	outsider := user("Outsider")
	if out := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", outsider, "", nil)).Want(http.StatusNotFound).Map(); out["code"] != "not_found" {
		t.Fatalf("outsider read: %v", out)
	}
	roleKey := uuid.NewString()
	configured := testutil.Call(t, s.h.ConfigureCollaborationStageRoles, s.request(http.MethodPost, "/stage/roles", s.actor, roleKey, configure(s.actor, "1"))).Want(http.StatusOK).Map()
	confirmation, _ := configured["stage_confirmation"].(map[string]any)
	if configured["code"] != "stage_roles_configured" || confirmation["action"] != "configure_roles" || confirmation["issue_id"] != s.issue || confirmation["role_revision"] != "1" || len(confirmation) != 3 {
		t.Fatalf("role confirmation variant: %v", configured)
	}
	recovered = testutil.Call(t, s.h.GetCollaborationOperation, s.request(http.MethodGet, "/operations/"+roleKey, s.actor, roleKey, nil)).Want(http.StatusOK).Map()
	if c, _ := recovered["stage_confirmation"].(map[string]any); recovered["code"] != "stage_roles_configured" || c["role_revision"] != "1" || recovered["status"] != nil {
		t.Fatalf("success recovery: %v", recovered)
	}
	scope := s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_scope_revision": "0", "children": []any{}, "reason": ""})
	scoped := testutil.Call(t, s.h.ConfirmCollaborationStageScope, s.request(http.MethodPost, "/stage/scope-confirmations", s.actor, uuid.NewString(), scope)).Want(http.StatusOK).Map()
	if c, _ := scoped["stage_confirmation"].(map[string]any); scoped["code"] != "stage_scope_confirmed" || c["action"] != "confirm_scope" || c["scope_revision"] != "1" || c["lifecycle_epoch"] != "1" || len(c) != 5 {
		t.Fatalf("scope confirmation variant: %v", scoped)
	}
	content := map[string]any{"summary": "Split", "children": []any{
		map[string]any{"title": "Implement", "description": "Write it", "priority": "high"}, map[string]any{"title": "Verify", "description": "", "priority": "none"}}}
	proposal := s.body(t, s.recipient, map[string]any{"expected_policy_revision": "1", "content": content, "source": map[string]any{"kind": "manual"}})
	created := testutil.Call(t, s.h.CreateCollaborationStageProposal, s.request(http.MethodPost, "/stage/proposals", s.recipient, uuid.NewString(), proposal)).Want(http.StatusOK).Map()
	candidate, _ := created["stage_confirmation"].(map[string]any)
	if created["code"] != "stage_proposal_created" || candidate["action"] != "create_proposal" || candidate["version"] != "1" || len(candidate) != 6 {
		t.Fatalf("proposal confirmation variant: %v", created)
	}
	proposalID := candidate["proposal_id"].(string)
	ref := map[string]any{"candidate_id": candidate["candidate_id"], "version": "1", "content_digest": candidate["content_digest"]}
	decide := func(actor, decision string) map[string]any {
		return s.body(t, actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "candidate": ref, "decision": decision, "reason": ""})
	}
	if out := testutil.Call(t, s.h.DecideCollaborationStageProposal, s.request(http.MethodPost, "/stage/proposals/"+proposalID+"/decisions", s.recipient, uuid.NewString(), decide(s.recipient, "accept"), "proposalId", proposalID)).
		Want(http.StatusForbidden).Map(); out["code"] != "stage_role_required" || out["status"] != float64(http.StatusForbidden) ||
		s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, issueID) != 0 {
		t.Fatalf("a non-approver accepted over HTTP: %v", out)
	}
	if out := testutil.Call(t, s.h.DecideCollaborationStageProposal, s.request(http.MethodPost, "/stage/proposals/"+proposalID+"/decisions", s.actor, uuid.NewString(), decide(s.actor, "approve"), "proposalId", proposalID)).
		Want(http.StatusBadRequest).Map(); out["code"] != "invalid_stage_request" {
		t.Fatalf("unknown decision value: %v", out)
	}
	acceptKey := uuid.NewString()
	accepted := testutil.Call(t, s.h.DecideCollaborationStageProposal, s.request(http.MethodPost, "/stage/proposals/"+proposalID+"/decisions", s.actor, acceptKey, decide(s.actor, "accept"), "proposalId", proposalID)).Want(http.StatusOK).Map()
	decision, _ := accepted["stage_confirmation"].(map[string]any)
	if accepted["code"] != "stage_proposal_accepted" || decision["action"] != "decide_proposal" || decision["decision"] != "accepted" || decision["candidate_id"] != candidate["candidate_id"] ||
		decision["decision_id"] == nil || decision["role_revision"] != "1" || decision["policy_revision"] != "1" || len(decision) != 10 ||
		s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, issueID) != 2 ||
		s.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)`, issueID) != 0 {
		t.Fatalf("accept over HTTP: %v", accepted)
	}
	if strings.Contains(testutil.Call(t, s.h.GetCollaborationOperation, s.request(http.MethodGet, "/operations/"+acceptKey, s.actor, acceptKey, nil)).Want(http.StatusOK).Text(), "Implement") {
		t.Fatal("an operation receipt carried child content")
	}
	view = testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.recipient, "", nil)).Want(http.StatusOK).Map()
	children, _ := view["children"].(map[string]any)
	items, _ := children["items"].([]any)
	capabilities, _ := view["capabilities"].(map[string]any)
	if view["managed"] != true || view["needs_confirmation"] != false || view["role_revision"] != "1" || view["scope_revision"] != "1" || view["scope_state"] != "current" ||
		len(items) != 2 || capabilities["configure_roles"] != false || capabilities["decide_proposal"] != false || capabilities["create_proposal"] != true {
		t.Fatalf("stage view over HTTP: %v", view)
	}
	first, _ := items[0].(map[string]any)
	if first["origin"] != "materialized" || first["can_open"] != false || first["execution_gated"] != false || first["semantic_version"] != "1" || first["ordinal"] != float64(1) {
		t.Fatalf("child projection for an external participant: %v", first)
	}
	page := testutil.Call(t, s.h.ListCollaborationStageProposals, s.request(http.MethodGet, "/stage/proposals?limit=1", s.actor, "", nil)).Want(http.StatusOK).Map()
	if list, _ := page["items"].([]any); len(list) != 1 || page["next_cursor"] != nil {
		t.Fatalf("proposal page: %v", page)
	}
	for _, query := range []string{"?limit=0", "?limit=51", "?limit=1&limit=2", "?offset=1", "?limit=01", "?cursor=not-base64!"} {
		if out := testutil.Call(t, s.h.ListCollaborationStageProposals, s.request(http.MethodGet, "/stage/proposals"+query, s.actor, "", nil)).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_stage_request" {
			t.Fatalf("query %s: %v", query, out)
		}
	}
	detail := testutil.Call(t, s.h.GetCollaborationStageProposal, s.request(http.MethodGet, "/stage/proposals/"+proposalID, s.actor, "", nil, "proposalId", proposalID)).Want(http.StatusOK).Map()
	if mapped, _ := detail["children"].([]any); detail["state"] != "accepted" || len(mapped) != 2 {
		t.Fatalf("proposal detail: %v", detail)
	}
	missing := uuid.NewString()
	if out := testutil.Call(t, s.h.GetCollaborationStageProposal, s.request(http.MethodGet, "/stage/proposals/"+missing, s.actor, "", nil, "proposalId", missing)).Want(http.StatusNotFound).Map(); out["code"] != "stage_proposal_not_found" {
		t.Fatalf("missing proposal: %v", out)
	}
}

// The ordinary issue handlers are the real machine entry points: POST
// /api/issues (CreateIssue -> IssueService.Create -> CreateInTx) and PUT/batch
// (updateIssueAtomicallyOnce).
func TestStageAdmissionThroughOrdinaryIssueHandlers(t *testing.T) {
	s := stageHTTPFixture(t)
	ws := s.fx.WorkspaceID
	runtime := s.fx.Runtime(t, "Stage handler runtime")
	agent := s.fx.Agent(t, "Stage handler agent", runtime, testutil.Cols{"visibility": "workspace", "permission_mode": "public_to"})
	task := s.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running", "originator_user_id": s.actor, "accountable_user_id": s.actor})
	ordinary := func(method, path string, body any, asAgent bool) *http.Request {
		r := testutil.JSONRequest(method, path, body)
		testutil.WithHeaders(r, "X-User-ID", s.actor, "X-Workspace-ID", ws)
		if asAgent {
			testutil.WithHeaders(r, "X-Agent-ID", agent, "X-Task-ID", task)
		}
		return r
	}
	children := func() int { return s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, s.issue) }
	create := func(title string, asAgent bool) *testutil.Response {
		return testutil.Call(t, s.h.CreateIssue, ordinary(http.MethodPost, "/api/issues?workspace_id="+ws, map[string]any{"title": title, "parent_issue_id": s.issue}, asAgent))
	}
	// No enabled policy: an agent may still create an ordinary child.
	create("Agent child before enablement", true).Want(http.StatusCreated)
	s.revise(t, true)
	before := children()
	out := create("Agent decomposition", true).Want(http.StatusConflict).Map()
	if out["code"] != "stage_proposal_required" || children() != before {
		t.Fatalf("an agent created a child of a managed root: %v", out)
	}
	// Control group: the human keeps manual child creation.
	manual := create("Manual child", false).Want(http.StatusCreated).Map()
	if children() != before+1 || s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, s.issue) != 0 {
		t.Fatal("manual child creation failed or produced an approval mapping")
	}
	// The gate covers the whole subtree: below a child the agent is refused
	// too, and the human keeps manual creation there.
	manualChild := manual["id"].(string)
	below := func(title string, asAgent bool) *testutil.Response {
		return testutil.Call(t, s.h.CreateIssue, ordinary(http.MethodPost, "/api/issues?workspace_id="+ws, map[string]any{"title": title, "parent_issue_id": manualChild}, asAgent))
	}
	if out = below("Agent sub-task", true).Want(http.StatusConflict).Map(); out["code"] != "stage_proposal_required" ||
		s.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, manualChild) != 0 {
		t.Fatalf("an agent decomposed below a child of a managed root: %v", out)
	}
	below("Manual sub-task", false).Want(http.StatusCreated)
	loose := s.fx.Issue(t, "Loose ordinary issue", testutil.Cols{"creator_id": s.actor})
	s.fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT max(number) FROM issue WHERE workspace_id=$1) WHERE id=$1`, ws)
	update := func(id string, body any, asAgent bool) *testutil.Response {
		return testutil.Call(t, s.h.UpdateIssue, testutil.WithURLParams(ordinary(http.MethodPut, "/api/issues/"+id, body, asAgent), "id", id))
	}
	attach := map[string]any{"parent_issue_id": s.issue}
	if out = update(loose, attach, true).Want(http.StatusConflict).Map(); out["code"] != "stage_proposal_required" || children() != before+1 {
		t.Fatalf("an agent attached an issue to a managed root: %v", out)
	}
	batch := testutil.Call(t, s.h.BatchUpdateIssues, ordinary(http.MethodPost, "/api/issues/batch-update?workspace_id="+ws, map[string]any{"issue_ids": []string{loose}, "updates": attach}, true))
	if batch.Code < 400 || children() != before+1 {
		t.Fatalf("an agent batch-attached an issue to a managed root: %d %s", batch.Code, batch.Text())
	}
	deeper := map[string]any{"parent_issue_id": manualChild}
	if out = update(loose, deeper, true).Want(http.StatusConflict).Map(); out["code"] != "stage_proposal_required" {
		t.Fatalf("an agent attached an issue below a child of a managed root: %v", out)
	}
	batch = testutil.Call(t, s.h.BatchUpdateIssues, ordinary(http.MethodPost, "/api/issues/batch-update?workspace_id="+ws, map[string]any{"issue_ids": []string{loose}, "updates": deeper}, true))
	if batch.Code < 400 || s.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND parent_issue_id IS NULL`, loose) != 1 {
		t.Fatalf("an agent batch-attached an issue below a child of a managed root: %d %s", batch.Code, batch.Text())
	}
	update(loose, attach, false).Want(http.StatusOK)
	if children() != before+2 || s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1`, loose) != 0 {
		t.Fatal("a human lost the right to attach a parent, or attaching approved something")
	}
	// A mapped child records content changes on its persistent mapping; a
	// status change is not a content change.
	manualID := manualChild
	s.fx.Exec(t, `INSERT INTO collaboration_stage_child (child_issue_id, root_issue_id, workspace_id, origin, decision_id, lifecycle_epoch) VALUES ($1,$2,$3,'confirmed',$4,1)`,
		manualID, s.issue, ws, uuid.NewString())
	version := func() int {
		return s.fx.Count(t, `SELECT semantic_version::int FROM collaboration_stage_child WHERE child_issue_id=$1`, manualID)
	}
	update(manualID, map[string]any{"status": "in_progress"}, false).Want(http.StatusOK)
	if version() != 1 {
		t.Fatalf("a status change invalidated reviewed content: v=%d", version())
	}
	update(manualID, map[string]any{"title": "Manual child, reworded"}, true).Want(http.StatusOK)
	if version() != 2 {
		t.Fatalf("a content change was not recorded: v=%d", version())
	}
	update(manualID, map[string]any{"parent_issue_id": nil}, false).Want(http.StatusOK)
	if version() != 3 || s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND root_issue_id=$2`, manualID, s.issue) != 1 {
		t.Fatalf("removing the parent erased or ignored the mapping: v=%d", version())
	}
}

// A scope confirmation must state the code version of every adopted child
// explicitly, and only a pull request the server finds linked is accepted.
func TestStageHTTPScopeConfirmationRequiresExplicitVerifiedCode(t *testing.T) {
	s := stageHTTPFixture(t)
	s.revise(t, true)
	reviewer := s.fx.User(t, "Reviewer", strings.ReplaceAll(uuid.NewString(), "-", "")+"@example.test")
	release := s.fx.User(t, "Release", strings.ReplaceAll(uuid.NewString(), "-", "")+"@example.test")
	s.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{reviewer, release})
	for _, target := range []string{reviewer, release} {
		testutil.Call(t, s.h.InviteCollaborationParticipant, s.request(http.MethodPost, "/participants", s.actor, uuid.NewString(), s.body(t, s.actor, map[string]any{"target_user_id": target}))).Want(http.StatusOK)
	}
	roles := s.roles()
	roles["code_reviewer_user_id"], roles["qa_approver_user_id"], roles["release_approver_user_id"] = reviewer, s.actor, release
	testutil.Call(t, s.h.ConfigureCollaborationStageRoles, s.request(http.MethodPost, "/stage/roles", s.actor, uuid.NewString(),
		s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "0", "roles": roles}))).Want(http.StatusOK)
	child := s.fx.Issue(t, "Existing child with code", testutil.Cols{"parent_issue_id": s.issue, "creator_id": s.actor})
	grandchild := s.fx.Issue(t, "Existing sub-task", testutil.Cols{"parent_issue_id": child, "creator_id": s.actor})
	head := strings.Repeat("d", 40)
	now := time.Now()
	pr := s.fx.Insert(t, "github_pull_request", testutil.Cols{"workspace_id": s.fx.WorkspaceID, "installation_id": int64(1), "repo_owner": "stage", "repo_name": uuid.NewString(),
		"pr_number": 7, "title": "Private pull request title", "state": "open", "html_url": "https://example.invalid/private", "pr_created_at": now, "pr_updated_at": now, "head_sha": head})
	s.fx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": child, "pull_request_id": pr}, "issue_id=$1 AND pull_request_id=$2", child, pr)
	read := func() (map[string]any, map[string]map[string]any) {
		view := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.actor, "", nil)).Want(http.StatusOK)
		if strings.Contains(view.Text(), "Private pull request title") || strings.Contains(view.Text(), "example.invalid") {
			t.Fatal("the stage view leaked pull request title or URL")
		}
		out, items := view.Map(), map[string]map[string]any{}
		for _, raw := range out["children"].(map[string]any)["items"].([]any) {
			item := raw.(map[string]any)
			items[item["issue_id"].(string)] = item
		}
		return out, items
	}
	_, items := read()
	external := testutil.Call(t, s.h.GetCollaborationStage, s.request(http.MethodGet, "/stage", s.recipient, "", nil)).Want(http.StatusOK).Map()
	for _, raw := range external["children"].(map[string]any)["items"].([]any) {
		item := raw.(map[string]any)
		if item["can_open"] != false || item["content_digest"] != nil {
			t.Fatalf("external stage projection exposes private digest: %v", item)
		}
	}
	offered, _ := items[child]["pull_requests"].([]any)
	if len(items) != 2 || items[child]["depth"] != float64(1) || items[grandchild]["depth"] != float64(2) || len(offered) != 1 ||
		offered[0].(map[string]any)["pull_request_id"] != pr || offered[0].(map[string]any)["head_sha"] != head || offered[0].(map[string]any)["provider"] != "github" ||
		len(offered[0].(map[string]any)) != 3 || items[child]["code"] != nil || items[child]["code_state"] != "none" {
		t.Fatalf("stage view children: %v", items)
	}
	confirm := func(children any) *testutil.Response {
		body := s.body(t, s.actor, map[string]any{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_scope_revision": "0", "children": children, "reason": ""})
		return testutil.Call(t, s.h.ConfirmCollaborationStageScope, s.request(http.MethodPost, "/stage/scope-confirmations", s.actor, uuid.NewString(), body))
	}
	entry := func(id string, code any, omit bool) map[string]any {
		out := map[string]any{"issue_id": id, "content_digest": items[id]["content_digest"]}
		if !omit {
			out["code"] = code
		}
		return out
	}
	reference := func(id, sha string, extra ...string) map[string]any {
		pull := map[string]any{"provider": "github", "pull_request_id": id, "head_sha": sha}
		for i := 0; i+1 < len(extra); i += 2 {
			pull[extra[i]] = extra[i+1]
		}
		return map[string]any{"pull_request": pull}
	}
	for name, children := range map[string]any{
		"code_absent":       []any{entry(child, nil, true)},
		"code_not_object":   []any{entry(child, "HEAD", false)},
		"empty_reference":   []any{entry(child, map[string]any{}, false)},
		"unknown_subfield":  []any{entry(child, reference(pr, head, "verified", "true"), false)},
		"uppercase_head":    []any{entry(child, reference(pr, strings.ToUpper(head)), false)},
		"abbreviated_head":  []any{entry(child, reference(pr, head[:12]), false)},
		"unknown_top_field": []any{entry(child, map[string]any{"pull_request": map[string]any{"provider": "github", "pull_request_id": pr, "head_sha": head}, "passed": true}, false)},
	} {
		if out := confirm(children).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_stage_request" {
			t.Fatalf("%s: %v", name, out)
		}
	}
	if out := confirm([]any{entry(child, reference(uuid.NewString(), head), false)}).Want(http.StatusConflict).Map(); out["code"] != "stage_scope_code_unavailable" || out["status"] != float64(http.StatusConflict) {
		t.Fatalf("an unverifiable pull request reference was accepted: %v", out)
	}
	if out := confirm([]any{entry(child, reference(pr, strings.Repeat("e", 40)), false)}).Want(http.StatusConflict).Map(); out["code"] != "stage_scope_changed" {
		t.Fatalf("a stale head was accepted: %v", out)
	}
	if s.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1`, s.issue) != 0 {
		t.Fatal("a refused confirmation mapped a child")
	}
	confirm([]any{entry(child, reference(pr, head), false), entry(grandchild, nil, false)}).Want(http.StatusOK)
	view, items := read()
	code, _ := items[child]["code"].(map[string]any)
	pull, _ := code["pull_request"].(map[string]any)
	if view["scope_state"] != "current" || pull["head_sha"] != head || pull["pull_request_id"] != pr || items[child]["code_state"] != "current" ||
		items[grandchild]["code"] != nil || items[grandchild]["code_state"] != "none" || items[grandchild]["origin"] != "confirmed" {
		t.Fatalf("confirmed code versions over HTTP: %v", items)
	}
}
