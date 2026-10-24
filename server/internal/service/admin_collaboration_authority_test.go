package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminCollaborationRejectsMachineAndIncompleteSessionsBeforeDatabase(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	id := util.MustParseUUID(uuid.NewString())
	org := util.MustParseUUID(uuid.NewString())
	for _, scenario := range []struct {
		name    string
		kind    auth.CredentialKind
		session auth.PasswordSession
	}{
		{"pat", auth.CredentialPAT, auth.PasswordSession{UserID: util.UUIDToString(id), Kind: "pat", Version: 1}},
		{"task", auth.CredentialTask, auth.PasswordSession{UserID: util.UUIDToString(id), Kind: "task_token", Version: 1}},
		{"cloud", auth.CredentialCloud, auth.PasswordSession{UserID: util.UUIDToString(id), Kind: "cloud_pat", Version: 1}},
		{"setup", auth.CredentialJWT, auth.PasswordSession{UserID: util.UUIDToString(id), Kind: "jwt", Setup: true}},
		{"change", auth.CredentialJWT, auth.PasswordSession{UserID: util.UUIDToString(id), Kind: "jwt", Version: 1, Change: true}},
		{"wrong_user", auth.CredentialJWT, auth.PasswordSession{UserID: uuid.NewString(), Kind: "jwt", Version: 1}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := auth.WithPasswordSession(auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{
				UserID: util.UUIDToString(id), Kind: scenario.kind,
			}), scenario.session)
			svc := NewAdminCollaborationService(nil, nil, true)
			_, err := svc.ChangeLifecycle(ctx, AdminCollaborationLifecycleParams{OrganizationID: org})
			assertPlatformAdminError(t, err, "admin_session_required")
			_, err = svc.ListIssues(ctx, org, AdminCollaborationPageQuery{Limit: 50})
			assertPlatformAdminError(t, err, "admin_session_required")
			_, err = svc.GetOperation(ctx, org, id, id)
			assertPlatformAdminError(t, err, "admin_session_required")
		})
	}
}

func TestAdminCollaborationRequiresCurrentAdministrationAndDisabledCreator(t *testing.T) {
	for _, scenario := range []struct{ name, code string }{
		{"enabled_creator", "creator_not_disabled"}, {"departed_creator", "creator_not_disabled"},
		{"observer", "admin_forbidden"}, {"workspace_owner", "admin_forbidden"},
		{"disabled_admin", "session_invalid"}, {"stale_session", "session_invalid"}, {"restricted_admin", "session_invalid"},
		{"disabled_feature", "admin_mode_disabled"}, {"legacy_mode", "admin_mode_disabled"},
		{"wrong_organization", "admin_scope_forbidden"}, {"missing_source_scope", "not_found"}, {"inactive_organization", "admin_unavailable"},
		{"no_reason", "invalid_reason"}, {"long_reason", "invalid_reason"}, {"invalid_action", "invalid_lifecycle_request"},
		{"stale_revision", "stale_revision"}, {"stale_status", "stale_status"}, {"stale_catalog", "status_options_changed"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			p := f.params(t, "complete")
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			ctx := f.adminCtx
			switch scenario.name {
			case "enabled_creator", "departed_creator":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.creator)
				if scenario.name == "departed_creator" {
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				}
			case "observer":
				f.fx.Exec(t, `UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1`, f.admin)
			case "workspace_owner":
				f.fx.Exec(t, `DELETE FROM platform_role_binding WHERE user_id=$1`, f.admin)
				f.fx.Member(t, f.workspace, util.UUIDToString(f.admin), "owner")
			case "disabled_admin":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.admin)
			case "stale_session":
				ctx = adminCollaborationContext(f.admin, 2)
			case "restricted_admin":
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.admin)
			case "disabled_feature":
				f.actions.Enabled = false
			case "legacy_mode":
				t.Setenv("MULTICA_AUTH_MODE", "legacy")
			case "wrong_organization":
				p.OrganizationID = util.MustParseUUID(uuid.NewString())
			case "missing_source_scope":
				f.fx.Exec(t, `DELETE FROM organization_workspace WHERE workspace_id=$1`, f.workspace)
			case "inactive_organization":
				f.fx.Exec(t, `UPDATE organization SET state='inactive' WHERE id=$1`, f.org)
				t.Cleanup(func() { f.fx.Exec(t, `UPDATE organization SET state='active' WHERE id=$1`, f.org) })
			case "no_reason":
				p.Reason = " \t\n"
			case "long_reason":
				p.Reason = strings.Repeat("a", 1001)
			case "invalid_action":
				p.Action = "transfer"
			case "stale_revision":
				p.ExpectedCollaborationRevision++
			case "stale_status":
				p.ExpectedStatus = "in_progress"
			case "stale_catalog":
				f.fx.Exec(t, `UPDATE issue_status SET name='Changed completion label' WHERE workspace_id=$1 AND key='done'`, f.workspace)
			}
			before := adminCollaborationStateSnapshot(t, f)
			_, err := f.actions.ChangeLifecycle(ctx, p)
			assertAdminCollaborationError(t, err, scenario.code)
			if after := adminCollaborationStateSnapshot(t, f); before != after {
				t.Fatal("refusal changed collaboration, responsibility, queues or operation/audit rows")
			}
		})
	}
}

func assertAdminCollaborationError(t *testing.T, err error, code string) {
	t.Helper()
	switch code {
	case "session_invalid":
		if !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("error = %v, want revoked password session", err)
		}
	case "not_found":
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("error = %v, want not found", err)
		}
	default:
		assertPlatformAdminError(t, err, code)
	}
}

func adminCollaborationStateSnapshot(t *testing.T, f *adminCollaborationFixture) string {
	t.Helper()
	var workflow string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
		'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
		'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$1),
		'handoffs',(SELECT jsonb_agg(to_jsonb(h) ORDER BY h.id) FROM issue_handoff h WHERE h.issue_id=$1),
		'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$1),
		'admin_operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM admin_operation o WHERE o.target_id=$1),
		'admin_audits',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM admin_audit_event a WHERE a.target_id=$1),
		'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1)
	)::text`, f.issue.Issue.ID).Scan(&workflow)
	return workflow + "\n" + controlStateSnapshot(t, f.collaborationQueueFixture)
}

func (f *adminCollaborationFixture) closeAsCreator(t *testing.T) {
	t.Helper()
	p := lifecycleParams(t, f.collaborationQueueFixture, "complete", "done", 1, 1)
	if _, err := f.svc.ChangeLifecycle(f.ctx, p); err != nil {
		t.Fatal(err)
	}
}

func (f *adminCollaborationFixture) enrollCandidate(t *testing.T, id pgtype.UUID) {
	t.Helper()
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": id}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, id)
	if _, err := f.svc.Queries.EnsureCollaborationAccountFence(f.ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestAdminCollaborationSuccessorIsConditionalAndCurrentlyEligible(t *testing.T) {
	for _, scenario := range []struct{ name, action, code string }{
		{"missing", "reopen", "successor_required"}, {"not_participating", "reopen", "successor_unavailable"},
		{"revoked", "reopen", "successor_unavailable"}, {"disabled", "reopen", "successor_unavailable"},
		{"password_change_required", "reopen", "successor_unavailable"}, {"password_missing", "reopen", "successor_unavailable"},
		{"handler_active", "reopen", "successor_forbidden"}, {"complete", "complete", "successor_forbidden"}, {"cancel", "cancel", "successor_forbidden"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			if scenario.action == "reopen" {
				f.closeAsCreator(t)
			}
			successor := util.MustParseUUID(f.outsider)
			if scenario.name != "not_participating" {
				f.enrollCandidate(t, successor)
			} else {
				// Source membership alone does not create an R40 candidate.
				f.fx.Member(t, f.workspace, f.outsider, "member")
			}
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
			p := f.params(t, scenario.action)
			p.SuccessorUserID = &successor
			switch scenario.name {
			case "missing":
				p.SuccessorUserID = nil
			case "revoked":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, p.IssueID, successor)
			case "disabled":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, successor)
			case "password_change_required":
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, successor)
			case "password_missing":
				f.fx.Exec(t, `DELETE FROM user_password_credential WHERE user_id=$1`, successor)
			case "handler_active":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
			}
			before := adminCollaborationStateSnapshot(t, f)
			_, err := f.actions.ChangeLifecycle(f.adminCtx, p)
			assertPlatformAdminError(t, err, scenario.code)
			if adminCollaborationStateSnapshot(t, f) != before {
				t.Fatal("invalid successor left a partial reopen, reassignment, audit or event")
			}
		})
	}
}

func TestAdminCollaborationReadScopeAndCandidateProjection(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	if _, err := f.actions.GetIssue(f.adminCtx, f.org, f.issue.Issue.ID); err == nil {
		t.Fatal("active creator exposed the administration action projection")
	}
	f.closeAsCreator(t)
	candidate := util.MustParseUUID(f.outsider)
	f.enrollCandidate(t, candidate)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
	detail, err := f.actions.GetIssue(f.adminCtx, f.org, f.issue.Issue.ID)
	if err != nil || !detail.SuccessorRequired || len(detail.Actions) != 1 || detail.Actions[0].Action != "reopen" || detail.Handler.UserID != util.MustParseUUID(f.handler) {
		t.Fatalf("minimal management detail = %+v, %v", detail, err)
	}
	page, err := f.actions.ListIssues(f.adminCtx, f.org, AdminCollaborationPageQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].IssueID != f.issue.Issue.ID || page.NextCursor.Valid {
		t.Fatalf("scoped list = %+v, %v", page, err)
	}
	candidates, err := f.actions.ReopenCandidates(f.adminCtx, f.org, f.issue.Issue.ID, AdminCollaborationPageQuery{Limit: 1})
	if err != nil || len(candidates.Items) != 1 || candidates.Items[0].UserID != candidate || !candidates.Items[0].External || candidates.CollaborationRevision != detail.CollaborationRevision {
		t.Fatalf("existing effective candidates = %+v, %v", candidates, err)
	}
	for _, value := range []any{detail, page, candidates} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var scoped struct {
			Scope string `json:"scope"`
		}
		if err := json.Unmarshal(encoded, &scoped); err != nil || scoped.Scope != util.UUIDToString(f.org) {
			t.Fatal("administration read did not correlate its authorized organization")
		}
		for _, forbidden := range []string{"Private source description", "Cross-workspace collaboration", "password_hash", "payload_hash", "session_id", "provider", "email"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("management projection disclosed %q", forbidden)
			}
		}
	}
	f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, candidate)
	candidates, err = f.actions.ReopenCandidates(f.adminCtx, f.org, f.issue.Issue.ID, AdminCollaborationPageQuery{Limit: 50})
	if err != nil || len(candidates.Items) != 0 {
		t.Fatalf("restricted participant still selectable: %+v, %v", candidates, err)
	}
	f.fx.Exec(t, `UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1`, f.admin)
	_, err = f.actions.GetIssue(f.adminCtx, f.org, f.issue.Issue.ID)
	assertPlatformAdminError(t, err, "admin_forbidden")
	_, err = f.actions.ReopenCandidates(f.adminCtx, f.org, f.issue.Issue.ID, AdminCollaborationPageQuery{Limit: 50})
	assertPlatformAdminError(t, err, "admin_forbidden")
}

func TestAdminCollaborationCandidatesPaginateEligibleParticipants(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		count   int
		denied  []int
		limit   int
		visible []int
	}{
		{name: "denied_prefix", count: 4, denied: []int{0, 1}, limit: 1, visible: []int{2, 3}},
		{name: "denied_page_boundary", count: 3, denied: []int{1}, limit: 2, visible: []int{0, 2}},
		{name: "denied_tail", count: 3, denied: []int{1, 2}, limit: 1, visible: []int{0}},
		{name: "all_denied", count: 2, denied: []int{0, 1}, limit: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			f.closeAsCreator(t)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
			var hash string
			f.fx.QueryRow(t, `SELECT password_hash FROM user_password_credential WHERE user_id=$1`, f.outsider).Scan(&hash)
			candidates := make([]pgtype.UUID, scenario.count)
			for i := range candidates {
				id := util.MustParseUUID(f.fx.User(t, "Existing candidate", uuid.NewString()+"@example.invalid"))
				f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{
					"user_id": id, "username": strings.ReplaceAll(uuid.NewString(), "-", ""), "password_hash": hash,
				}, "user_id=$1", id)
				f.enrollCandidate(t, id)
				f.fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, id)
				candidates[i] = id
			}
			sort.Slice(candidates, func(i, j int) bool { return util.UUIDToString(candidates[i]) < util.UUIDToString(candidates[j]) })
			for i, index := range scenario.denied {
				// Exercise the same emergency account predicate as password login,
				// using only synthetic users in this task's isolated database.
				email := []string{"pdzzer68@embassybase.com", "gtwtrox@mowan666.com"}[i]
				f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, candidates[index], email)
			}
			var cursor pgtype.UUID
			for offset := 0; ; offset += scenario.limit {
				page, err := f.actions.ReopenCandidates(f.adminCtx, f.org, f.issue.Issue.ID, AdminCollaborationPageQuery{Limit: scenario.limit, Cursor: cursor})
				if err != nil {
					t.Fatal(err)
				}
				end := min(offset+scenario.limit, len(scenario.visible))
				if len(page.Items) != end-offset {
					t.Fatalf("page at offset %d has %d eligible candidates, want %d; cursor=%s", offset, len(page.Items), end-offset, util.UUIDToString(page.NextCursor))
				}
				for i, item := range page.Items {
					if item.UserID != candidates[scenario.visible[offset+i]] {
						t.Fatal("candidate page skipped an eligible participant or included a denied account")
					}
				}
				if end == len(scenario.visible) {
					if page.NextCursor.Valid {
						t.Fatal("last eligible page retained a cursor for denied accounts")
					}
					break
				}
				if page.NextCursor != page.Items[len(page.Items)-1].UserID {
					t.Fatal("next cursor does not identify the last visible eligible candidate")
				}
				cursor = page.NextCursor
			}
		})
	}
}

func TestAdminCollaborationTemporaryDisableMatchesAccountAvailability(t *testing.T) {
	for _, scenario := range []struct {
		name, action                 string
		creatorDenied, handlerDenied bool
		handlerPersistentlyDisabled  bool
		creatorPersistentlyDisabled  bool
	}{
		{name: "temporary_creator_complete", action: "complete", creatorDenied: true},
		{name: "temporary_creator_cancel", action: "cancel", creatorDenied: true},
		{name: "temporary_creator_healthy_handler_reopen", action: "reopen", creatorDenied: true},
		{name: "temporary_handler_requires_successor", action: "reopen", creatorPersistentlyDisabled: true, handlerDenied: true},
		{name: "temporary_creator_disabled_handler", action: "reopen", creatorDenied: true, handlerPersistentlyDisabled: true},
		{name: "both_temporarily_disabled", action: "reopen", creatorDenied: true, handlerDenied: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			if scenario.action == "reopen" {
				f.closeAsCreator(t)
			}
			if scenario.creatorDenied {
				f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, f.creator, "\u0085\t PDZZER68@EMBASSYBASE.COM \u3000")
			}
			if scenario.handlerDenied {
				f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, f.handler, "gtwtrox@mowan666.com")
			}
			if scenario.creatorPersistentlyDisabled {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			}
			if scenario.handlerPersistentlyDisabled {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			}
			page, err := f.actions.ListIssues(f.adminCtx, f.org, AdminCollaborationPageQuery{Limit: 1})
			if err != nil || len(page.Items) != 1 || page.Items[0].IssueID != f.issue.Issue.ID || !page.Items[0].Creator.Disabled {
				t.Fatalf("disabled creator's management list = %+v, %v", page, err)
			}
			detail, err := f.actions.GetIssue(f.adminCtx, f.org, f.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			handlerDisabled := scenario.handlerDenied || scenario.handlerPersistentlyDisabled
			if !detail.Creator.Disabled || detail.Handler.Disabled != handlerDisabled || detail.SuccessorRequired != (scenario.action == "reopen" && handlerDisabled) {
				t.Fatalf("management availability differs from authentication: %+v", detail)
			}
			encoded, err := json.Marshal(detail)
			if err != nil || strings.Contains(strings.ToLower(string(encoded)), "email") || strings.Contains(string(encoded), "embassybase") || strings.Contains(string(encoded), "mowan666") {
				t.Fatal("management response exposed internal account denylist inputs")
			}
			p := f.params(t, scenario.action)
			expectedHandler := util.MustParseUUID(f.handler)
			if detail.SuccessorRequired {
				before := adminCollaborationStateSnapshot(t, f)
				_, err := f.actions.ChangeLifecycle(f.adminCtx, p)
				assertPlatformAdminError(t, err, "successor_required")
				if adminCollaborationStateSnapshot(t, f) != before {
					t.Fatal("missing successor changed lifecycle or operation state")
				}
				expectedHandler = util.MustParseUUID(f.outsider)
				f.enrollCandidate(t, expectedHandler)
				p.SuccessorUserID = &expectedHandler
				candidates, err := f.actions.ReopenCandidates(f.adminCtx, f.org, p.IssueID, AdminCollaborationPageQuery{Limit: 1})
				if err != nil || len(candidates.Items) != 1 || candidates.Items[0].UserID != expectedHandler || candidates.NextCursor.Valid {
					t.Fatalf("double-disabled existing candidates = %+v, %v", candidates, err)
				}
			}
			result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
			if err != nil || result.Result.CreatorUserID != util.MustParseUUID(f.creator) || result.Result.HandlerUserID != expectedHandler {
				t.Fatalf("disabled account lifecycle = %+v, %v", result, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, p.IssueID, f.admin) != 0 {
				t.Fatal("lifecycle administration enrolled the administrator")
			}
		})
	}
}

func TestAdminCollaborationPasswordRestrictionsDoNotEnableAdministration(t *testing.T) {
	for _, scenario := range []string{"password_missing", "password_change_required"} {
		t.Run(scenario, func(t *testing.T) {
			f := newAdminCollaborationFixture(t)
			p := f.params(t, "complete")
			if scenario == "password_missing" {
				f.fx.Exec(t, `DELETE FROM user_password_credential WHERE user_id=$1`, f.creator)
			} else {
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.creator)
			}
			page, err := f.actions.ListIssues(f.adminCtx, f.org, AdminCollaborationPageQuery{Limit: 1})
			if err != nil || len(page.Items) != 0 || page.NextCursor.Valid {
				t.Fatalf("password restriction incorrectly enabled management: %+v, %v", page, err)
			}
			_, err = f.actions.GetIssue(f.adminCtx, f.org, p.IssueID)
			assertPlatformAdminError(t, err, "creator_not_disabled")
			before := adminCollaborationStateSnapshot(t, f)
			_, err = f.actions.ChangeLifecycle(f.adminCtx, p)
			assertPlatformAdminError(t, err, "creator_not_disabled")
			if adminCollaborationStateSnapshot(t, f) != before {
				t.Fatal("password restriction allowed lifecycle administration")
			}
		})
	}
}

func TestAdminCollaborationIssueListFiltersDisabledCreatorsBeforeLimit(t *testing.T) {
	fixtures := []*adminCollaborationFixture{newAdminCollaborationFixture(t), newAdminCollaborationFixture(t)}
	sort.Slice(fixtures, func(i, j int) bool {
		return util.UUIDToString(fixtures[i].issue.Issue.ID) < util.UUIDToString(fixtures[j].issue.Issue.ID)
	})
	f, later := fixtures[0], fixtures[1]
	if f.org != later.org {
		t.Fatal("management pagination fixtures must use the same isolated organization")
	}
	later.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, later.creator, "\u0085\t PDZZER68@EMBASSYBASE.COM \u3000")
	page, err := f.actions.ListIssues(f.adminCtx, f.org, AdminCollaborationPageQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].IssueID != later.issue.Issue.ID || page.NextCursor.Valid {
		t.Fatalf("an enabled creator prefix hid the disabled creator's issue: %+v, %v", page, err)
	}
	query := db.ListAdminCollaborationIssuesParams{OrganizationID: f.org, PageLimit: 1,
		TemporarilyDisabledUserIds: []string{}, TemporarilyDisabledUserEmails: []string{}}
	rows, err := f.svc.Queries.ListAdminCollaborationIssues(f.adminCtx, query)
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty emergency filters matched an otherwise enabled account: rows=%d, %v", len(rows), err)
	}
	query.TemporarilyDisabledUserIds = []string{later.creator}
	rows, err = f.svc.Queries.ListAdminCollaborationIssues(f.adminCtx, query)
	if err != nil || len(rows) != 1 || rows[0].IssueID != later.issue.Issue.ID {
		t.Fatalf("internal UUID denylist did not apply before the page limit: rows=%d, %v", len(rows), err)
	}
	query.TemporarilyDisabledUserIds = []string{}
	later.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, later.creator)
	rows, err = f.svc.Queries.ListAdminCollaborationIssues(f.adminCtx, query)
	if err != nil || len(rows) != 1 || rows[0].IssueID != later.issue.Issue.ID {
		t.Fatalf("empty emergency filters hid a persisted account disable: rows=%d, %v", len(rows), err)
	}
}

func TestAdminCollaborationReopenRetainsHealthyHandlerAndDoesNotGrantOrdinaryRights(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.closeAsCreator(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	p := f.params(t, "reopen")
	result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
	if err != nil || result.Result.HandlerUserID != util.MustParseUUID(f.handler) || result.Result.QueueState != "paused" {
		t.Fatalf("healthy handler reopen = %+v, %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen'`, p.IssueID) != 0 {
		t.Fatal("healthy handler received an implicit reassignment")
	}
	if _, err = f.svc.ResolveIssueAccess(f.adminCtx, nil, p.IssueID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("administrator acquired ordinary task content: %v", err)
	}
	if _, err = f.svc.Resources(f.adminCtx, p.IssueID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("administrator acquired resource content/consent: %v", err)
	}
	if _, err = f.svc.GetWorkflowOperation(f.adminCtx, p.IssueID, util.UUIDToString(p.IdempotencyKey)); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("ordinary operation reader exposed administrative receipt: %v", err)
	}
	before := adminCollaborationStateSnapshot(t, f)
	_, err = f.svc.Transfer(f.adminCtx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, result.Result.CollaborationRevision), TargetUserID: util.MustParseUUID(f.outsider)}})
	if !collaborationErrorCode(err, "not_found") || adminCollaborationStateSnapshot(t, f) != before {
		t.Fatalf("ordinary transfer borrowed platform authority: %v", err)
	}
}

func TestAdminCollaborationRecoveryPreservesOriginalKeyAndSuccessor(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.closeAsCreator(t)
	successor := util.MustParseUUID(f.outsider)
	f.enrollCandidate(t, successor)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=ANY($1::uuid[])`, []string{f.creator, f.handler})
	p := f.params(t, "reopen")
	p.SuccessorUserID = &successor
	first, err := f.actions.ChangeLifecycle(f.adminCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	before := adminCollaborationStateSnapshot(t, f)
	second, err := f.actions.ChangeLifecycle(f.adminCtx, p)
	if err != nil || !second.Replayed || second.Operation.ID != first.Operation.ID || !reflect.DeepEqual(second.Result, first.Result) || adminCollaborationStateSnapshot(t, f) != before {
		t.Fatalf("original key replay = %+v, %v", second, err)
	}
	wrong := util.MustParseUUID(f.handler)
	p.SuccessorUserID = &wrong
	_, err = f.actions.ChangeLifecycle(f.adminCtx, p)
	assertPlatformAdminError(t, err, "idempotency_conflict")
	if adminCollaborationStateSnapshot(t, f) != before {
		t.Fatal("different successor changed the original operation")
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.creator)
	p.SuccessorUserID = &successor
	before = adminCollaborationStateSnapshot(t, f)
	_, err = f.actions.ChangeLifecycle(f.adminCtx, p)
	assertPlatformAdminError(t, err, "creator_not_disabled")
	if adminCollaborationStateSnapshot(t, f) != before {
		t.Fatal("restored creator's original POST changed the retained result")
	}
	recovered, err := f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
	if err != nil || !reflect.DeepEqual(recovered.Result, first.Result) {
		t.Fatalf("creator restoration erased original receipt: %+v, %v", recovered, err)
	}
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.admin)
	_, err = f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
	assertAdminCollaborationError(t, err, "session_invalid")
	f.adminCtx = adminCollaborationContext(f.admin, 2)
	if _, err = f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey); err != nil {
		t.Fatalf("current login cannot recover its earlier operation: %v", err)
	}
	p = f.params(t, "complete")
	_, err = f.actions.ChangeLifecycle(f.adminCtx, p)
	assertPlatformAdminError(t, err, "creator_not_disabled")
	// Even another super administrator cannot recover this actor's operation.
	f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": successor, "role": PlatformRoleSuperAdmin}, "user_id=$1", successor)
	_, err = f.actions.GetOperation(adminCollaborationContext(successor, 1), f.org, p.IssueID, first.Operation.IdempotencyKey)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("another actor recovered the original key: %v", err)
	}
	f.fx.Exec(t, `DELETE FROM organization_workspace WHERE workspace_id=$1`, f.workspace)
	_, err = f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, first.Operation.IdempotencyKey)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("recovery retained lost source scope: %v", err)
	}
}

func TestAdminCollaborationSameDisabledCreatorAndHandler(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
	f.closeAsCreator(t)
	successor := util.MustParseUUID(f.handler)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	p := f.params(t, "reopen")
	p.SuccessorUserID = &successor
	result, err := f.actions.ChangeLifecycle(f.adminCtx, p)
	if err != nil || result.Result.CreatorUserID != util.MustParseUUID(f.creator) || result.Result.PreviousHandlerUserID != util.MustParseUUID(f.creator) || result.Result.HandlerUserID != successor {
		t.Fatalf("same disabled account reopening = %+v, %v", result, err)
	}
}

func TestAdminCollaborationRejectsUncorrelatedStoredResult(t *testing.T) {
	f := newAdminCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	p := f.params(t, "complete")
	if _, err := f.actions.ChangeLifecycle(f.adminCtx, p); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE collaboration_operation SET result=jsonb_set(result,'{operation_id}',to_jsonb($3::text)) WHERE scope_id=$1 AND request_id=$2`, p.IssueID, util.UUIDToString(p.IdempotencyKey), uuid.NewString())
	_, err := f.actions.GetOperation(f.adminCtx, f.org, p.IssueID, p.IdempotencyKey)
	assertPlatformAdminError(t, err, "admin_operation_unavailable")
}
