package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestSourceContextCollaborationPreviewIsDenied(t *testing.T) {
	issue := dbfx.Issue(t, "Private collaboration source", testutil.Cols{"description": "private-source-body"})
	comment := dbfx.Comment(t, issue, "private-comment-body")
	dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	r := withURLParam(newRequest(http.MethodGet, "/api/comments/"+comment+"/sub-issue-preview", nil), "commentId", comment)
	response := testutil.Call(t, testHandler.PreviewCommentSubIssue, r).Want(http.StatusConflict)
	if response.Map()["code"] != "unsupported_collaboration_action" || strings.Contains(response.Body.String(), "private-") {
		t.Fatalf("collaboration preview did not return a content-free refusal: %s", response.Body.String())
	}
}

type sourceContextHTTPProbe struct {
	db.DBTX
	lookupFailure               bool
	agentReads, attachmentReads int
}

type sourceContextHTTPErrorRow struct{}

func (sourceContextHTTPErrorRow) Scan(...any) error {
	return errors.New("private-source-lookup-failure")
}

func (p *sourceContextHTTPProbe) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: IsCollaborationIssue") && p.lookupFailure {
		return sourceContextHTTPErrorRow{}
	}
	if strings.Contains(sql, "-- name: GetAgent") {
		p.agentReads++
	}
	return p.DBTX.QueryRow(ctx, sql, args...)
}

func (p *sourceContextHTTPProbe) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: ListSourceContext") && strings.Contains(sql, "Attachments") {
		p.attachmentReads++
	}
	return p.DBTX.Query(ctx, sql, args...)
}

func TestSourceContextCollaborationBothModesRejectBeforeCopyOrAgentLookup(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "shared_source", true: "lookup_failure"}[failure], func(t *testing.T) {
			store := &mockStorage{}
			source := dbfx.Issue(t, "Collaboration capture source", testutil.Cols{"description": "private-source-body"})
			anchor := dbfx.Comment(t, source, "private-thread-body")
			key := "source-boundary/" + uuid.NewString()
			store.put(key, []byte("private-delivered-file"))
			dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": source, "comment_id": anchor, "uploader_type": "member", "uploader_id": testUserID, "filename": "private.txt", "url": store.ObjectURL(key), "content_type": "text/plain", "size_bytes": 22})
			build, err := service.BuildSourceContext(context.Background(), testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(anchor))
			if err != nil {
				t.Fatal(err)
			}
			dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": source}, "issue_id=$1", source)
			probe := &sourceContextHTTPProbe{DBTX: testPool, lookupFailure: failure}
			h := *testHandler
			h.Queries, h.DB, h.Storage = db.New(probe), probe, store
			// Valid source tokens must not allow either mode to reach preparation.
			// The provider is a fixture identity; no daemon/provider is started.
			runtime := dbfx.Runtime(t, "Fake capture runtime")
			agent := dbfx.Agent(t, "Fake capture agent", runtime)
			beforeIssues := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, testWorkspaceID)
			beforeAttachments := dbfx.Count(t, `SELECT count(*) FROM attachment WHERE workspace_id=$1`, testWorkspaceID)
			wantStatus, wantCode := http.StatusConflict, "unsupported_collaboration_action"
			if failure {
				wantStatus, wantCode = http.StatusServiceUnavailable, "collaboration_unavailable"
			}
			for _, mode := range []string{"manual", "agent", "preview"} {
				var response *testutil.Response
				if mode == "preview" {
					r := withURLParam(newRequest(http.MethodGet, "/api/comments/"+anchor+"/sub-issue-preview", nil), "commentId", anchor)
					response = testutil.Call(t, h.PreviewCommentSubIssue, r).Want(wantStatus)
				} else {
					body := map[string]any{"mode": mode, "capture_token": build.Token, "issue": map[string]any{"title": "Forbidden copied child"}, "quick_create": map[string]any{"agent_id": agent, "prompt": "Forbidden copied child"}}
					r := withURLParam(newRequest(http.MethodPost, "/api/comments/"+anchor+"/sub-issues", body), "commentId", anchor)
					response = testutil.Call(t, h.CreateCommentSubIssue, r).Want(wantStatus)
				}
				payload := response.Map()
				if payload["code"] != wantCode || len(payload) != 2 || strings.Contains(response.Body.String(), "private-") {
					t.Fatalf("%s did not return the bounded refusal: %s", mode, response.Body.String())
				}
			}
			reads, copies := store.streamCopyCalls()
			if reads != 0 || copies != 0 || probe.attachmentReads != 0 || probe.agentReads != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM issue_source_context WHERE source_issue_id=$1`, source) != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM issue_source_context_object_intent WHERE workspace_id=$1`, testWorkspaceID) != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agent) != 0 ||
				dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, testWorkspaceID) != beforeIssues ||
				dbfx.Count(t, `SELECT count(*) FROM attachment WHERE workspace_id=$1`, testWorkspaceID) != beforeAttachments {
				t.Fatalf("refused capture reached projection/preparation or left artifacts: reads=%d copies=%d attachment_queries=%d agent_queries=%d", reads, copies, probe.attachmentReads, probe.agentReads)
			}
		})
	}
}

func TestSourceContextOrdinaryChildHTTPStillCreatesOwnSubtask(t *testing.T) {
	// Fixtures use high numbers so the production issue counter can allocate
	// the new child independently of rows inserted directly by the test.
	parent := dbfx.Issue(t, "Private collaboration parent", testutil.Cols{"description": "private-parent-content", "number": 9_560_001})
	dbfx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": parent}, "issue_id=$1", parent)
	source := dbfx.Issue(t, "Ordinary source child", testutil.Cols{"parent_issue_id": parent, "description": "ordinary-child-content", "number": 9_560_002})
	anchor := dbfx.Comment(t, source, "ordinary-child-comment")
	r := withURLParam(newRequest(http.MethodGet, "/api/comments/"+anchor+"/sub-issue-preview", nil), "commentId", anchor)
	var preview sourceContextPreviewResponse
	response := testutil.Call(t, testHandler.PreviewCommentSubIssue, r).Want(http.StatusOK)
	response.JSON(&preview)
	if preview.SourceIssue.ID != source || strings.Contains(response.Body.String(), "private-parent-content") {
		t.Fatal("ordinary child inherited its collaboration parent's content or restriction")
	}
	dbfx.Cleanup(t, `DELETE FROM issue_source_context WHERE source_issue_id=$1`, source)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND parent_issue_id=$2`, testWorkspaceID, source)
	body := map[string]any{"mode": "manual", "capture_token": preview.CaptureToken, "issue": map[string]any{"title": "Ordinary captured grandchild"}}
	r = withURLParam(newRequest(http.MethodPost, "/api/comments/"+anchor+"/sub-issues", body), "commentId", anchor)
	out := testutil.Call(t, testHandler.CreateCommentSubIssue, r).Want(http.StatusCreated).Map()
	id, ok := out["id"].(string)
	if !ok {
		t.Fatalf("missing created issue identity: %+v", out)
	}
	if _, err := util.ParseUUID(id); err != nil {
		t.Fatal(err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM issue_source_context WHERE issue_id=$1 AND source_issue_id=$2`, id, source) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1`, id) != 0 ||
		dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 0 {
		t.Fatal("ordinary child capture changed collaboration or execution semantics")
	}
}
