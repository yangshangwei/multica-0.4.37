package service

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationRecipientSearchExternalNeedsSourceMembership(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	suffix := pickerSuffix()
	mate, stranger := pickerAccount(t, f.collaborationFixture, "Ext "+suffix), pickerAccount(t, f.collaborationFixture, "Ext "+suffix)
	f.fx.Member(t, f.workspace, mate, "member")
	f.fx.Exec(t, `UPDATE "user" SET name=$2 WHERE id=$1`, f.handler, "Ext "+suffix)
	external := func(items []CollaborationRecipientCandidate) map[string]*bool {
		out := map[string]*bool{}
		for _, item := range items {
			out[util.UUIDToString(item.UserID)] = item.External
		}
		return out
	}
	for _, query := range []CollaborationRecipientSearchQuery{
		{SourceWorkspaceID: util.MustParseUUID(f.workspace), Query: suffix, Directory: true},
		{IssueID: f.issue.Issue.ID, Query: suffix, Directory: true},
	} {
		result, err := f.svc.SearchRecipients(f.ctx, query)
		got := external(result.Recipients)
		if err != nil || !result.SourceMember || len(got) != 3 || got[mate] == nil || *got[mate] || got[stranger] == nil || !*got[stranger] || got[f.handler] == nil || !*got[f.handler] {
			t.Fatalf("source member must see who is external: %+v, %v", result, err)
		}
		query.MembersOnly = true
		result, err = f.svc.SearchRecipients(f.ctx, query)
		if err != nil || !slices.Equal(pickerIDs(result.Recipients), []string{mate}) {
			t.Fatalf("members_only listed %v, want %s: %v", pickerIDs(result.Recipients), mate, err)
		}
	}
	// The external handler manages the issue but is not a source member.
	ctx := collaborationContext(f.handler)
	result, err := f.svc.SearchRecipients(ctx, CollaborationRecipientSearchQuery{IssueID: f.issue.Issue.ID, Query: suffix, Directory: true})
	got := external(result.Recipients)
	if err != nil || result.SourceMember || len(got) != 2 || got[mate] != nil || got[stranger] != nil || result.SourceWorkspaceID != f.issue.Issue.WorkspaceID {
		t.Fatalf("a non-member must not learn source membership from the list: %+v, %v", result, err)
	}
	for _, q := range []string{suffix, ""} {
		probe := &recipientProbeDB{DBTX: f.pool, lookup: "SearchCollaborationRecipients"}
		result, err = NewCollaborationService(db.New(probe), nil, nil).SearchRecipients(ctx, CollaborationRecipientSearchQuery{IssueID: f.issue.Issue.ID, Query: q, MembersOnly: true, Directory: true})
		if err != nil || result.SourceMember || len(result.Recipients) != 0 || len(result.Recent) != 0 || result.NextCursor != "" || probe.lookups != 0 {
			t.Fatalf("members_only for a non-member must be an empty 200 without reading accounts: %+v, %v, lookups=%d", result, err, probe.lookups)
		}
	}
}

func TestCollaborationRecipientSearchSharesResolveAuthorization(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		code   string
	}{
		{"unrelated_issue", 404, "not_found"},
		{"ordinary_issue", 404, "not_found"},
		{"participant_not_manager", 403, "forbidden"},
		{"source_nonmember", 403, "forbidden"},
		{"missing_source", 403, "forbidden"},
		{"source_gate_closed", 503, "collaboration_unavailable"},
		{"issue_ignores_creation_gate", 200, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			ctx := f.ctx
			query := CollaborationRecipientSearchQuery{IssueID: f.issue.Issue.ID, Directory: true}
			switch scenario.name {
			case "unrelated_issue":
				ctx = collaborationContext(f.outsider)
			case "ordinary_issue":
				query.IssueID = util.MustParseUUID(f.fx.Issue(t, "Ordinary issue is not a collaboration context"))
			case "participant_not_manager":
				f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
				ctx = collaborationContext(f.outsider)
			case "source_nonmember", "missing_source", "source_gate_closed":
				query = CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Directory: true}
				if scenario.name == "source_nonmember" {
					ctx = collaborationContext(f.outsider)
				} else if scenario.name == "missing_source" {
					query.SourceWorkspaceID = util.MustParseUUID(uuid.NewString())
				}
			}
			if scenario.name == "source_gate_closed" || scenario.name == "issue_ignores_creation_gate" {
				f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
			}
			probe := &recipientProbeDB{DBTX: f.pool, lookup: "SearchCollaborationRecipients"}
			out, err := NewCollaborationService(db.New(probe), nil, nil).SearchRecipients(ctx, query)
			if scenario.status == http.StatusOK {
				if err != nil || probe.lookups != 1 || out.SourceWorkspaceID != f.issue.Issue.WorkspaceID || !out.SourceMember {
					t.Fatalf("valid issue context failed: %+v, %v, lookups=%d", out, err, probe.lookups)
				}
				return
			}
			wantRecipientError(t, err, scenario.status, scenario.code)
			if probe.lookups != 0 {
				t.Fatalf("unauthorized context reached the account query %d times", probe.lookups)
			}
		})
	}
}

func TestCollaborationRecipientSearchRechecksAuthorityAfterQuery(t *testing.T) {
	for _, change := range []string{"transfer", "removal", "source_departure", "creation_disabled", "manager_left_source"} {
		t.Run(change, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			ctx := collaborationContext(f.handler)
			query := CollaborationRecipientSearchQuery{IssueID: f.issue.Issue.ID, Directory: true}
			wantStatus, wantCode := http.StatusForbidden, "forbidden"
			switch change {
			case "removal":
				wantStatus, wantCode = http.StatusNotFound, "not_found"
			case "source_departure", "creation_disabled":
				ctx, query = f.ctx, CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Directory: true}
				if change == "creation_disabled" {
					wantStatus, wantCode = http.StatusServiceUnavailable, "collaboration_unavailable"
				}
			case "manager_left_source":
				// A source member who is also a co-participant would be listed.
				mate := pickerAccount(t, f.collaborationFixture, "Mate "+pickerSuffix())
				f.fx.Member(t, f.workspace, mate, "member")
				f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": mate}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, mate)
				ctx, query.MembersOnly = f.ctx, true
			}
			probe := &recipientProbeDB{DBTX: f.pool, lookup: "ListRecentCollaborationRecipients", beforeLookup: func() {
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
				case "source_departure", "manager_left_source":
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				case "creation_disabled":
					f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
				}
			}}
			out, err := NewCollaborationService(db.New(probe), nil, nil).SearchRecipients(ctx, query)
			if probe.lookups != 1 {
				t.Fatal("authority change was not injected between authorization and the account query")
			}
			if change == "manager_left_source" {
				// The departed creator still manages the issue, but the members
				// read under the old membership must not be returned.
				if err != nil || out.SourceMember || len(out.Recent) != 0 || len(out.Recipients) != 0 {
					t.Fatalf("stale source-member results: %+v, %v", out, err)
				}
				return
			}
			wantRecipientError(t, err, wantStatus, wantCode)
		})
	}
}

func TestCollaborationRecipientSearchDependenciesFailClosed(t *testing.T) {
	for _, queryName := range []string{"GetUser", "GetCollaborationRecipientSource", "GetCollaborationGate", "GetCollaborationAccess", "ListRecentCollaborationRecipients", "SearchCollaborationRecipients"} {
		t.Run(queryName, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			input := CollaborationRecipientSearchQuery{SourceWorkspaceID: util.MustParseUUID(f.workspace), Directory: true}
			if queryName == "GetCollaborationAccess" {
				input = CollaborationRecipientSearchQuery{IssueID: f.issue.Issue.ID, Directory: true}
			}
			probe := &recipientProbeDB{DBTX: f.pool, failQuery: "-- name: " + queryName + " "}
			_, err := NewCollaborationService(db.New(probe), nil, nil).SearchRecipients(f.ctx, input)
			var domain *CollaborationError
			if err == nil || errors.As(err, &domain) {
				t.Fatalf("dependency failure was allowed or disguised as a domain answer: %v", err)
			}
		})
	}
}
