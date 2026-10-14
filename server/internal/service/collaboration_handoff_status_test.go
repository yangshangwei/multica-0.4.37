package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationDirectedStatusOptionsUseSourceCatalog(t *testing.T) {
	for _, tc := range []struct{ action, target string }{
		{"request_information", "blocked"},
		{"return_information", "todo"},
		{"submit_acceptance", "in_review"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			if tc.action == "return_information" {
				request := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator,
					"from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "todo"})
				f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, request)
			}
			options, err := f.svc.StatusOptions(collaborationContext(f.handler), f.issue.Issue.ID, tc.action)
			if err != nil {
				t.Fatal(err)
			}
			if options.Action != tc.action || options.DefaultStatus != tc.target || len(options.Options) != 1 || options.Options[0].Key != tc.target || len(options.StatusOptionsVersion) != 64 {
				t.Fatalf("directed action did not project its exact source target: %+v", options)
			}
		})
	}
}

func TestCollaborationDirectedHandoffsRejectChangedCatalogWithSameTarget(t *testing.T) {
	for _, action := range []string{"request_information", "return_information", "submit_acceptance"} {
		for _, field := range []string{"name", "updated_at", "category"} {
			t.Run(action+"/"+field, func(t *testing.T) {
				f := newCollaborationHandoffFixture(t)
				f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "working", "name": "Working", "category": "todo", "color": "#000000"})
				f.fx.Exec(t, `UPDATE issue SET status='working' WHERE id=$1`, f.issue.Issue.ID)
				ctx := collaborationContext(f.handler)
				var invoke func() (CollaborationWorkflowReceipt, error)
				var version, target string
				switch action {
				case "request_information":
					p := handoffAsk(t, f, 1, f.creator, "Original request")
					version, target = p.StatusOptionsVersion, p.ExpectedStatus
					invoke = func() (CollaborationWorkflowReceipt, error) { return f.svc.RequestInformation(ctx, p) }
				case "return_information":
					if _, err := f.svc.RequestInformation(ctx, handoffAsk(t, f, 1, f.creator, "Original question")); err != nil {
						t.Fatal(err)
					}
					ctx = f.ctx
					p := handoffReturn(t, f, 2, activeInformationRequest(t, f), "working")
					version, target = p.StatusOptionsVersion, p.ExpectedStatus
					invoke = func() (CollaborationWorkflowReceipt, error) { return f.svc.ReturnInformation(ctx, p) }
				case "submit_acceptance":
					p := handoffAcceptance(t, f, 1, "Original result")
					version, target = p.StatusOptionsVersion, p.ExpectedStatus
					invoke = func() (CollaborationWorkflowReceipt, error) { return f.svc.SubmitAcceptance(ctx, p) }
				}
				before := mustAccess(t, f, ctx)
				events := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
				handoffs := f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
				key := target
				if field == "category" {
					key = "working"
				}
				catalog, err := f.svc.Queries.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: before.Issue.WorkspaceID, IncludeArchived: true})
				if err != nil {
					t.Fatal(err)
				}
				entry, ok := collaborationCatalogEntry(catalog, key)
				if !ok {
					t.Fatal("missing catalog fixture")
				}
				switch field {
				case "name":
					f.fx.Exec(t, `UPDATE issue_status SET name='Renamed' WHERE workspace_id=$1 AND key=$2`, f.workspace, key)
				case "updated_at":
					f.fx.Exec(t, `UPDATE issue_status SET updated_at=updated_at+interval '1 second' WHERE workspace_id=$1 AND key=$2`, f.workspace, key)
				case "category":
					f.fx.Exec(t, `UPDATE issue_status SET category='in_progress' WHERE workspace_id=$1 AND key=$2`, f.workspace, key)
				}
				refused, err := invoke()
				if !collaborationErrorCode(err, "status_options_changed") || refused.StatusOptions == nil || refused.StatusOptions.DefaultStatus != target || refused.StatusOptions.StatusOptionsVersion == version {
					t.Fatalf("same-key catalog edit was silently accepted: %+v %v", refused, err)
				}
				if after := mustAccess(t, f, ctx); !reflect.DeepEqual(before.Issue, after.Issue) || before.Revision != after.Revision || before.InformationRequestID != after.InformationRequestID {
					t.Fatal("catalog refusal changed the issue or request chain")
				}
				// Make the original digest valid again. Its refused operation must
				// still replay the original refusal rather than execute the handoff.
				f.fx.Exec(t, `UPDATE issue_status SET name=$3, category=$4, updated_at=$5 WHERE workspace_id=$1 AND key=$2`, f.workspace, key, entry.Name, entry.Category, entry.UpdatedAt)
				current, err := f.svc.StatusOptions(ctx, f.issue.Issue.ID, action)
				if err != nil || current.StatusOptionsVersion != version {
					t.Fatalf("fixture did not restore the original confirmation: %+v %v", current, err)
				}
				replay, err := invoke()
				if !collaborationErrorCode(err, refused.Code) || replay.Code != refused.Code || replay.CollaborationRevision != refused.CollaborationRevision {
					t.Fatalf("refusal became an executable intent: %+v %v", replay, err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != events || f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID) != handoffs {
					t.Fatal("refused handoff wrote history or notifications")
				}
			})
		}
	}
}

func TestCollaborationReturnDigestBindsNestedRequestIdentity(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	f.fx.Exec(t, `UPDATE issue SET status='blocked' WHERE id=$1`, f.issue.Issue.ID)
	if _, err := f.svc.RequestInformation(collaborationContext(f.handler), handoffAsk(t, f, 1, f.creator, "First layer")); err != nil {
		t.Fatal(err)
	}
	first := activeInformationRequest(t, f)
	firstOptions, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, "return_information")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RequestInformation(f.ctx, handoffAsk(t, f, 2, f.outsider, "Second layer")); err != nil {
		t.Fatal(err)
	}
	second := activeInformationRequest(t, f)
	p := handoffReturn(t, f, 3, second, "blocked")
	if firstOptions.DefaultStatus != p.ExpectedStatus || firstOptions.StatusOptionsVersion == p.StatusOptionsVersion {
		t.Fatal("same restoration key erased the nested request identity")
	}
	stale := p
	stale.StatusOptionsVersion = firstOptions.StatusOptionsVersion
	if _, err = f.svc.ReturnInformation(collaborationContext(f.outsider), stale); !collaborationErrorCode(err, "status_options_changed") {
		t.Fatalf("accepted a confirmation from a different request layer: %v", err)
	}
	if activeInformationRequest(t, f) != second {
		t.Fatal("mismatched confirmation consumed a request")
	}
	p.RequestID = uuid.NewString()
	if _, err = f.svc.ReturnInformation(collaborationContext(f.outsider), p); err != nil || activeInformationRequest(t, f) != first {
		t.Fatalf("confirmed nested return failed: %v", err)
	}
}

func TestCollaborationDirectedHandoffsRequireStatusConfirmation(t *testing.T) {
	for _, action := range []string{"request_information", "return_information", "submit_acceptance"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			ctx := collaborationContext(f.handler)
			var receipt CollaborationWorkflowReceipt
			var err error
			switch action {
			case "request_information":
				p := handoffAsk(t, f, 1, f.creator, "")
				p.StatusOptionsVersion = ""
				receipt, err = f.svc.RequestInformation(ctx, p)
			case "return_information":
				if _, err = f.svc.RequestInformation(ctx, handoffAsk(t, f, 1, f.creator, "")); err != nil {
					t.Fatal(err)
				}
				p := handoffReturn(t, f, 2, activeInformationRequest(t, f), "todo")
				p.StatusOptionsVersion = ""
				receipt, err = f.svc.ReturnInformation(f.ctx, p)
			case "submit_acceptance":
				p := handoffAcceptance(t, f, 1, "")
				p.StatusOptionsVersion = ""
				receipt, err = f.svc.SubmitAcceptance(ctx, p)
			}
			if !collaborationErrorCode(err, "invalid_status_request") || receipt.Status != 400 {
				t.Fatalf("missing critical digest was accepted: %+v %v", receipt, err)
			}
		})
	}
}

func TestCollaborationInformationRequestRechecksCatalogAfterLockWait(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	p := handoffAsk(t, f, 1, f.creator, "Original intent")
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = db.New(tx).LockIssueStatusCatalog(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	var blocker int
	if err = tx.QueryRow(f.ctx, `SELECT pg_backend_pid()`).Scan(&blocker); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.RequestInformation(collaborationContext(f.handler), p)
		done <- err
	}()
	waitWorkflowBlocked(t, f, blocker, done)
	if _, err = tx.Exec(f.ctx, `UPDATE issue_status SET name='New blocked name' WHERE workspace_id=$1 AND key='blocked'`, f.workspace); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !collaborationErrorCode(err, "status_options_changed") {
		t.Fatalf("handoff used catalog facts from before the lock: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("stale catalog handoff changed responsibility")
	}
}

func TestCollaborationAcceptanceRefusesRevokedCreator(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	p := handoffAcceptance(t, f, 1, "Ready")
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
	if _, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p); !collaborationErrorCode(err, "target_unavailable") {
		t.Fatalf("acceptance revived revoked creator participation: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='todo' AND assignee_id=$2`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("unavailable acceptance target changed responsibility")
	}
}

func TestCollaborationDirectedStatusCannotKeepArchivedCurrentKey(t *testing.T) {
	for _, tc := range []struct{ action, category string }{{"request_information", "blocked"}, {"submit_acceptance", "in_review"}} {
		t.Run(tc.action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			f.fx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.workspace, "key": "old_status", "name": "Archived", "category": tc.category, "color": "#000000", "archived_at": "2026-10-01T00:00:00Z"})
			f.fx.Exec(t, `UPDATE issue SET status='old_status' WHERE id=$1`, f.issue.Issue.ID)
			options := handoffTestOptions(t, f, tc.action, pgtype.UUID{})
			if options.DefaultStatus != tc.category || len(options.Options) != 1 || options.Options[0].Key != tc.category {
				t.Fatalf("archived status remained selectable: %+v", options)
			}
		})
	}
}
