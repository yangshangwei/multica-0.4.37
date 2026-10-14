package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func recipientCredential(t *testing.T, f *collaborationFixture, user, username string, restricted bool) {
	t.Helper()
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{
		"user_id": user, "username": username, "password_hash": "test-only-not-a-password", "must_change_password": restricted,
	}, "user_id=$1", user)
}

func TestCollaborationRecipientExactLookupAndNoWrites(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	username := "recipient_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	mailbox := username + "+tag@example.test"
	recipientCredential(t, f.collaborationFixture, f.outsider, username, false)
	f.fx.Exec(t, `UPDATE "user" SET email=$2,name='Confirmed recipient',avatar_url='https://example.test/avatar.png' WHERE id=$1`, f.outsider, mailbox)
	before := recipientState(t, f)

	for _, query := range []struct{ kind, value string }{
		{"username", "\t" + strings.ToUpper(username) + "\u3000"},
		{"email", "\u00a0" + strings.ToUpper(mailbox) + "\r\n"},
		{"user_id", strings.ToUpper(f.outsider)},
		{"user_id", strings.ReplaceAll(f.outsider, "-", "")},
	} {
		t.Run(query.kind+"/"+query.value, func(t *testing.T) {
			out, err := f.svc.ResolveRecipient(f.ctx, CollaborationRecipientQuery{
				Kind: query.kind, Value: query.value, SourceWorkspaceID: util.MustParseUUID(f.workspace),
			})
			if err != nil || out.IssueID.Valid || out.SourceWorkspaceID != util.MustParseUUID(f.workspace) ||
				out.Recipient.UserID != util.MustParseUUID(f.outsider) || out.Recipient.Name != "Confirmed recipient" ||
				out.Recipient.AvatarURL != "https://example.test/avatar.png" || !out.Recipient.External {
				t.Fatalf("exact recipient projection: %+v, %v", out, err)
			}
		})
	}
	if after := recipientState(t, f); after != before {
		t.Fatalf("recipient lookup changed membership, authority, responsibility or execution: before %s, after %s", before, after)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=$1`, f.outsider) != 0 {
		t.Fatal("lookup enrolled a recipient before an explicit invitation")
	}
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = tx.Exec(f.ctx, "SET TRANSACTION READ ONLY"); err != nil {
		t.Fatal(err)
	}
	if _, err = NewCollaborationService(db.New(tx), nil, nil).ResolveRecipient(f.ctx, CollaborationRecipientQuery{
		Kind: "username", Value: username, SourceWorkspaceID: util.MustParseUUID(f.workspace),
	}); err != nil {
		t.Fatalf("confirmation lookup must succeed in a database-enforced read-only transaction: %v", err)
	}
	if err = tx.Rollback(f.ctx); err != nil {
		t.Fatal(err)
	}

	// Historical case/whitespace variants use the same normalization as input.
	f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, f.outsider, "\t\u00a0"+strings.ToUpper(mailbox)+"\u3000\r")
	f.fx.Member(t, f.workspace, f.outsider, "member")
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,creation_enabled=false WHERE id=1`)
	out, err := f.svc.ResolveRecipient(collaborationContext(f.handler), CollaborationRecipientQuery{
		Kind: "email", Value: mailbox, IssueID: f.issue.Issue.ID,
	})
	if err != nil || out.IssueID != f.issue.Issue.ID || out.SourceWorkspaceID != f.issue.Issue.WorkspaceID || out.Recipient.External {
		t.Fatalf("issue management must work with both gates closed and classify source membership: %+v, %v", out, err)
	}
	for _, query := range []struct{ kind, value string }{{"username", username[:len(username)-1]}, {"email", username + "@example.test"}, {"username", "Confirmed_recipient"}} {
		_, err := f.svc.ResolveRecipient(f.ctx, CollaborationRecipientQuery{Kind: query.kind, Value: query.value, IssueID: f.issue.Issue.ID})
		wantRecipientError(t, err, http.StatusNotFound, "recipient_not_found")
	}
}

func recipientState(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
        'issue', (SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
        'collaboration', (SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
        'session', (SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$1),
        'participants', (SELECT count(*) FROM issue_participant WHERE issue_id=$1),
        'members', (SELECT count(*) FROM member WHERE workspace_id=$2),
        'grants', (SELECT count(*) FROM collaboration_grant WHERE issue_id=$1),
        'commands', (SELECT count(*) FROM collaboration_command WHERE session_id=$3),
        'operations', (SELECT count(*) FROM collaboration_operation WHERE scope_id IN ($1,$2)),
        'events', (SELECT count(*) FROM collaboration_event WHERE issue_id=$1),
        'notifications', (SELECT count(*) FROM collaboration_notification WHERE issue_id=$1)
    )::text`, f.issue.Issue.ID, f.issue.Issue.WorkspaceID, f.issue.SessionID).Scan(&snapshot)
	return snapshot
}

func TestCollaborationRecipientRejectsInvalidQueryBeforeDatabase(t *testing.T) {
	svc := NewCollaborationService(nil, nil, nil)
	ctx := collaborationContext(uuid.NewString())
	for _, query := range []struct{ kind, value string }{
		{"", "recipient"}, {"name", "Recipient"}, {"EMAIL", "recipient@example.test"},
		{"username", ""}, {"username", "  "}, {"username", "ab"}, {"username", "name%"}, {"username", "name*"},
		{"username", "Full Name"}, {"username", strings.Repeat("x", 33)},
		{"email", ""}, {"email", "recipient"}, {"email", "recipient@"}, {"email", "@example.test"},
		{"email", "Recipient <recipient@example.test>"}, {"email", "one@example.test,two@example.test"},
		{"user_id", ""}, {"user_id", "not-a-uuid"}, {"user_id", "MUL-123"},
	} {
		t.Run(query.kind+"/"+query.value, func(t *testing.T) {
			_, err := svc.ResolveRecipient(ctx, CollaborationRecipientQuery{Kind: query.kind, Value: query.value, SourceWorkspaceID: util.MustParseUUID(uuid.NewString())})
			wantRecipientError(t, err, http.StatusBadRequest, "invalid_recipient_query")
		})
	}
	for _, query := range []CollaborationRecipientQuery{
		{Kind: "user_id", Value: uuid.NewString()},
		{Kind: "user_id", Value: uuid.NewString(), IssueID: util.MustParseUUID(uuid.NewString()), SourceWorkspaceID: util.MustParseUUID(uuid.NewString())},
	} {
		_, err := svc.ResolveRecipient(ctx, query)
		wantRecipientError(t, err, http.StatusBadRequest, "invalid_recipient_context")
	}
}

func TestCollaborationRecipientRejectsUntrustedPrincipalBeforeDependencies(t *testing.T) {
	svc := NewCollaborationService(nil, nil, nil)
	for _, principal := range []auth.AuthenticatedPrincipal{
		{}, {UserID: uuid.NewString(), Kind: auth.CredentialTask}, {UserID: uuid.NewString(), Kind: auth.CredentialCloud},
		{UserID: uuid.NewString(), Kind: "plugin"}, {UserID: uuid.NewString(), Kind: "unknown"},
		{UserID: uuid.NewString(), Kind: auth.CredentialJWT, Restricted: true},
		{UserID: uuid.NewString(), Kind: auth.CredentialJWT, AgentActor: true},
		{UserID: uuid.NewString(), Kind: auth.CredentialPAT, AgentActor: true},
		{UserID: "not-a-uuid", Kind: auth.CredentialJWT},
	} {
		ctx := auth.WithAuthenticatedPrincipal(context.Background(), principal)
		_, err := svc.ResolveRecipient(ctx, CollaborationRecipientQuery{})
		wantRecipientError(t, err, http.StatusForbidden, "forbidden")
	}
}

func TestCollaborationRecipientAmbiguousAndUnavailable(t *testing.T) {
	for _, kind := range []string{"username", "email"} {
		t.Run(kind, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			value := "recipient_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
			if kind == "username" {
				recipientCredential(t, f.collaborationFixture, f.outsider, value, false)
				recipientCredential(t, f.collaborationFixture, f.handler, strings.ToUpper(value), false)
			} else {
				value += "@example.test"
				f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, f.outsider, value)
				f.fx.Exec(t, `UPDATE "user" SET email=$2 WHERE id=$1`, f.handler, "\t"+strings.ToUpper(value)+" ")
			}
			query := CollaborationRecipientQuery{Kind: kind, Value: value, IssueID: f.issue.Issue.ID}
			_, err := f.svc.ResolveRecipient(f.ctx, query)
			wantRecipientError(t, err, http.StatusConflict, "recipient_ambiguous")
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			_, err = f.svc.ResolveRecipient(f.ctx, query)
			wantRecipientError(t, err, http.StatusConflict, "recipient_ambiguous")
		})
	}
	for _, state := range []string{"missing", "disabled", "restricted", "temporary_denial"} {
		t.Run(state, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			query := CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, IssueID: f.issue.Issue.ID}
			switch state {
			case "missing":
				query.Value = uuid.NewString()
			case "disabled":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.outsider)
			case "restricted":
				recipientCredential(t, f.collaborationFixture, f.outsider, "restricted_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:12], true)
			case "temporary_denial":
				f.fx.Exec(t, `UPDATE "user" SET email='pdzzer68@embassybase.com' WHERE id=$1`, f.outsider)
			}
			before := recipientState(t, f)
			_, err := f.svc.ResolveRecipient(f.ctx, query)
			wantRecipientError(t, err, http.StatusNotFound, "recipient_not_found")
			if recipientState(t, f) != before {
				t.Fatal("unavailable recipient lookup changed collaboration state")
			}
		})
	}
}

func TestCollaborationRecipientContextAuthorizationPrecedesLookup(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		code   string
	}{
		{"unrelated_issue", 404, "not_found"},
		{"ordinary_issue", 404, "not_found"},
		{"missing_issue", 404, "not_found"},
		{"participant_not_manager", 403, "forbidden"},
		{"source_member_not_participant", 403, "forbidden"},
		{"revoked_handler", 404, "not_found"},
		{"revoked_handler_with_source_membership", 403, "forbidden"},
		{"disabled_manager", 404, "not_found"},
		{"source_nonmember_gate_open", 403, "forbidden"},
		{"source_nonmember_gate_closed", 403, "forbidden"},
		{"missing_source", 403, "forbidden"},
		{"disabled_creator", 403, "forbidden"},
		{"source_gate_closed", 503, "collaboration_unavailable"},
		{"ordinary_member_can_create", 200, ""},
		{"external_handler_ignores_creation_gate", 200, ""},
		{"departed_creator_remains_manager", 200, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			ctx := f.ctx
			query := CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, IssueID: f.issue.Issue.ID}
			switch scenario.name {
			case "unrelated_issue":
				ctx = collaborationContext(f.outsider)
			case "ordinary_issue":
				query.IssueID = util.MustParseUUID(f.fx.Issue(t, "Ordinary issue is not a collaboration context"))
			case "missing_issue":
				query.IssueID = util.MustParseUUID(uuid.NewString())
			case "participant_not_manager":
				f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
				ctx = collaborationContext(f.outsider)
			case "source_member_not_participant":
				f.fx.Member(t, f.workspace, f.outsider, "member")
				ctx = collaborationContext(f.outsider)
			case "revoked_handler", "revoked_handler_with_source_membership":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
				if scenario.name == "revoked_handler_with_source_membership" {
					f.fx.Member(t, f.workspace, f.handler, "member")
				}
				ctx = collaborationContext(f.handler)
			case "disabled_manager":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			case "source_nonmember_gate_open", "source_nonmember_gate_closed", "missing_source", "disabled_creator", "source_gate_closed", "ordinary_member_can_create":
				query = CollaborationRecipientQuery{Kind: "user_id", Value: f.handler, SourceWorkspaceID: util.MustParseUUID(f.workspace)}
				switch scenario.name {
				case "source_nonmember_gate_open", "source_nonmember_gate_closed":
					ctx = collaborationContext(f.outsider)
				case "missing_source":
					query.SourceWorkspaceID = util.MustParseUUID(uuid.NewString())
				case "disabled_creator":
					f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
				case "ordinary_member_can_create":
					f.fx.Member(t, f.workspace, f.outsider, "member")
					ctx = collaborationContext(f.outsider)
				}
				if scenario.name == "source_nonmember_gate_closed" || scenario.name == "source_gate_closed" {
					f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
				}
			case "external_handler_ignores_creation_gate":
				ctx = collaborationContext(f.handler)
				f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,creation_enabled=false WHERE id=1`)
			case "departed_creator_remains_manager":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
			}
			probe := &recipientProbeDB{DBTX: f.pool}
			if scenario.name == "external_handler_ignores_creation_gate" {
				probe.failQuery = "-- name: GetCollaborationGate "
			}
			svc := NewCollaborationService(db.New(probe), nil, nil)
			out, err := svc.ResolveRecipient(ctx, query)
			if scenario.status == http.StatusOK {
				if err != nil || !out.Recipient.UserID.Valid || probe.lookups != 1 {
					t.Fatalf("valid context failed: %+v, %v, lookups=%d", out, err, probe.lookups)
				}
			} else {
				wantRecipientError(t, err, scenario.status, scenario.code)
				if probe.lookups != 0 {
					t.Fatalf("unauthorized context reached account lookup %d times", probe.lookups)
				}
			}
		})
	}
}

func TestCollaborationRecipientRechecksAuthorityAfterCandidateLookup(t *testing.T) {
	for _, change := range []string{"transfer", "removal", "disable", "source_departure", "creation_disabled"} {
		t.Run(change, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			ctx := collaborationContext(f.handler)
			query := CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, IssueID: f.issue.Issue.ID}
			wantStatus, wantCode := http.StatusForbidden, "forbidden"
			if change == "source_departure" || change == "creation_disabled" {
				ctx = f.ctx
				query = CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, SourceWorkspaceID: util.MustParseUUID(f.workspace)}
			}
			if change == "removal" || change == "disable" {
				wantStatus, wantCode = http.StatusNotFound, "not_found"
			} else if change == "creation_disabled" {
				wantStatus, wantCode = http.StatusServiceUnavailable, "collaboration_unavailable"
			}
			probe := &recipientProbeDB{DBTX: f.pool, beforeLookup: func() {
				switch change {
				case "transfer":
					receipt, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
						CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider),
					}})
					if err != nil || receipt.Code != "transferred" {
						t.Fatalf("concurrent transfer: %+v, %v", receipt, err)
					}
				case "removal":
					receipt, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
					if err != nil || receipt.Code != "participant_removed" {
						t.Fatalf("concurrent removal: %+v, %v", receipt, err)
					}
				case "disable":
					f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
				case "source_departure":
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				case "creation_disabled":
					f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
				}
			}}
			svc := NewCollaborationService(db.New(probe), nil, nil)
			_, err := svc.ResolveRecipient(ctx, query)
			wantRecipientError(t, err, wantStatus, wantCode)
			if probe.lookups != 1 {
				t.Fatal("authority change was not injected between context authorization and candidate lookup")
			}
		})
	}
}

func TestCollaborationRecipientPasswordAccountStateIsCurrent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	recipientCredential(t, f.collaborationFixture, f.creator, "creator_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:16], false)
	t.Setenv("MULTICA_AUTH_MODE", "password")
	ctx := auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.creator, Version: 1, Kind: "jwt"})
	query := CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, IssueID: f.issue.Issue.ID}
	_, err := f.svc.ResolveRecipient(ctx, query)
	wantRecipientError(t, err, http.StatusNotFound, "recipient_not_found")
	recipientCredential(t, f.collaborationFixture, f.outsider, "recipient_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:16], false)
	if _, err := f.svc.ResolveRecipient(ctx, query); err != nil {
		t.Fatalf("complete password accounts cannot resolve: %v", err)
	}
	f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.outsider)
	_, err = f.svc.ResolveRecipient(ctx, query)
	wantRecipientError(t, err, http.StatusNotFound, "recipient_not_found")
	f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=false WHERE user_id=$1`, f.outsider)
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.creator)
	_, err = f.svc.ResolveRecipient(ctx, query)
	wantRecipientError(t, err, http.StatusNotFound, "not_found")
	query = CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, SourceWorkspaceID: util.MustParseUUID(f.workspace)}
	_, err = f.svc.ResolveRecipient(ctx, query)
	wantRecipientError(t, err, http.StatusForbidden, "forbidden")
	current := auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.creator, Version: 2, Kind: "jwt"})
	if _, err := f.svc.ResolveRecipient(current, query); err != nil {
		t.Fatalf("fresh password session cannot use a valid creation context: %v", err)
	}
	f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.creator)
	_, err = f.svc.ResolveRecipient(current, query)
	wantRecipientError(t, err, http.StatusForbidden, "forbidden")
}

func TestCollaborationRecipientDependenciesFailClosed(t *testing.T) {
	for _, queryName := range []string{"GetUser", "GetCollaborationRecipientSource", "GetCollaborationGate", "GetCollaborationAccess", "FindCollaborationRecipients"} {
		t.Run(queryName, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			input := CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, SourceWorkspaceID: util.MustParseUUID(f.workspace)}
			if queryName == "GetCollaborationAccess" {
				input = CollaborationRecipientQuery{Kind: "user_id", Value: f.outsider, IssueID: f.issue.Issue.ID}
			}
			probe := &recipientProbeDB{DBTX: f.pool, failQuery: "-- name: " + queryName + " "}
			before := recipientState(t, f)
			_, err := NewCollaborationService(db.New(probe), nil, nil).ResolveRecipient(f.ctx, input)
			if err == nil || collaborationErrorCode(err, "not_found") || collaborationErrorCode(err, "recipient_not_found") {
				t.Fatalf("dependency failure was allowed or disguised as an absent account: %v", err)
			}
			if queryName != "FindCollaborationRecipients" && probe.lookups != 0 {
				t.Fatal("failed authorization dependency fell through to account lookup")
			}
			if recipientState(t, f) != before {
				t.Fatal("dependency failure mutated collaboration state")
			}
		})
	}
}

// The real database still executes all reads. The probe only injects failures
// or a concurrent role change at the candidate-query boundary.
type recipientProbeDB struct {
	db.DBTX
	lookups      int
	failQuery    string
	beforeLookup func()
}

func (p *recipientProbeDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: FindCollaborationRecipients ") {
		p.lookups++
		if p.beforeLookup != nil {
			p.beforeLookup()
		}
	}
	if p.failQuery != "" && strings.Contains(sql, p.failQuery) {
		return nil, errors.New("private-recipient-sentinel@example.test database failure")
	}
	return p.DBTX.Query(ctx, sql, args...)
}

func (p *recipientProbeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if p.failQuery != "" && strings.Contains(sql, p.failQuery) {
		return recipientFailureRow{}
	}
	return p.DBTX.QueryRow(ctx, sql, args...)
}

func (p *recipientProbeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("recipient resolution must not write")
}

type recipientFailureRow struct{}

func (recipientFailureRow) Scan(...any) error {
	return errors.New("private-recipient database failure")
}

func wantRecipientError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var target *CollaborationError
	if !errors.As(err, &target) || target.Status != status || target.Code != code {
		t.Fatalf("recipient error = %v; want HTTP %d %s", err, status, code)
	}
}
