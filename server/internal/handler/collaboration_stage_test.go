package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
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

// revise switches the policy through the service: no HTTP route can enable it.
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

func TestStagePolicyHTTPEnablingIsUnavailableUntilEveryGateExists(t *testing.T) {
	s := stageHTTPFixture(t)
	rows := func() int {
		return s.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, s.project)
	}
	enable := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "switch the stage gates on"}
	out := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), enable)).Want(http.StatusServiceUnavailable).Map()
	if out["code"] != "stage_policy_unavailable" || rows() != 0 {
		t.Fatalf("an owner enabled the stage policy over HTTP: %v rows=%d", out, rows())
	}
	read := testutil.Call(t, s.h.GetStagePolicy, s.policyRequest(http.MethodGet, "", s.actor, "", nil)).Want(http.StatusOK).Map()
	if read["policy_revision"] != "0" || read["enabled"] != false || read["activation_available"] != false || read["contract"] != "collaboration-stage-v1" || read["created_by"] != nil {
		t.Fatalf("virtual policy read: %v", read)
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
		if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, tc.key, tc.body)).Want(http.StatusBadRequest).Map(); got["code"] != tc.code || rows() != 0 {
			t.Fatalf("%s: %v", name, got)
		}
	}
	machine := s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), map[string]any{"expected_policy_revision": "0", "enabled": false, "reason": "x"})
	machine.Header.Set("X-Agent-ID", uuid.NewString())
	if got := testutil.Call(t, s.h.UpdateStagePolicy, machine).Want(http.StatusForbidden).Map(); got["code"] != "stage_policy_denied" {
		t.Fatalf("agent actor reached the policy write: %v", got)
	}
	// Disabling is the rollback path and stays available.
	s.revise(t, true)
	read = testutil.Call(t, s.h.GetStagePolicy, s.policyRequest(http.MethodGet, "", s.actor, "", nil)).Want(http.StatusOK).Map()
	if read["policy_revision"] != "1" || read["enabled"] != true || read["activation_available"] != false {
		t.Fatalf("enabled policy read: %v", read)
	}
	key := uuid.NewString()
	disable := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "explicit rollback"}
	first := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, key, disable)).Want(http.StatusOK).Map()
	confirmation, _ := first["stage_confirmation"].(map[string]any)
	if first["code"] != "stage_policy_revised" || confirmation["action"] != "revise_policy" || confirmation["policy_revision"] != "2" || confirmation["enabled"] != false ||
		confirmation["project_id"] != s.project || len(confirmation) != 4 {
		t.Fatalf("disable confirmation: %v", first)
	}
	replay := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, key, disable)).Want(http.StatusOK).Map()
	recovered := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/"+key, s.actor, key, nil)).Want(http.StatusOK).Map()
	for _, got := range []map[string]any{replay, recovered} {
		if c, _ := got["stage_confirmation"].(map[string]any); c["policy_revision"] != "2" || rows() != 2 {
			t.Fatalf("original key did not recover its own revision: %v", got)
		}
	}
	if got := testutil.Call(t, s.h.GetStagePolicyOperation, s.policyRequest(http.MethodGet, "/operations/x", s.actor, uuid.NewString(), nil)).Want(http.StatusNotFound).Map(); got["code"] != "stage_policy_operation_not_found" {
		t.Fatalf("unknown key: %v", got)
	}
	stale := map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "stale"}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), stale)).Want(http.StatusConflict).Map(); got["code"] != "stage_policy_revision_conflict" {
		t.Fatalf("stale revision: %v", got)
	}
	again := map[string]any{"expected_policy_revision": "2", "enabled": true, "reason": "re-enable"}
	if got := testutil.Call(t, s.h.UpdateStagePolicy, s.policyRequest(http.MethodPut, "", s.actor, uuid.NewString(), again)).Want(http.StatusServiceUnavailable).Map(); got["code"] != "stage_policy_unavailable" || rows() != 2 {
		t.Fatalf("re-enable over HTTP: %v rows=%d", got, rows())
	}
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
