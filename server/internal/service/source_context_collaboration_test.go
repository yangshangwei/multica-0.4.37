package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func sourceContextBoundaryFixture(t *testing.T) (*pgxpool.Pool, *testutil.Fixture, string, string) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.UserID = fx.User(t, "Source context actor", "source-boundary-"+uuid.NewString()+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Source context workspace", "source-boundary-"+uuid.NewString())
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	parent := fx.Issue(t, "Private collaboration parent", testutil.Cols{"description": "private-parent-body"})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": parent}, "issue_id=$1", parent)
	source := fx.Issue(t, "Ordinary source child", testutil.Cols{"parent_issue_id": parent, "description": "ordinary-source-body"})
	anchor := fx.Comment(t, source, "ordinary-source-comment")
	return pool, fx, source, anchor
}

type sourceContextBoundaryProbe struct {
	db.DBTX
	failLookup          bool
	checks, projections int
	checkedID           pgtype.UUID
}

type sourceContextBoundaryErrorRow struct{}

func (sourceContextBoundaryErrorRow) Scan(...any) error {
	return errors.New("private-collaboration-lookup-failure")
}

func (p *sourceContextBoundaryProbe) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: IsCollaborationIssue") {
		p.checks++
		p.checkedID = args[0].(pgtype.UUID)
		if p.failLookup {
			return sourceContextBoundaryErrorRow{}
		}
	}
	if strings.Contains(sql, "-- name: GetWorkspace ") {
		p.projections++
	}
	return p.DBTX.QueryRow(ctx, sql, args...)
}

func (p *sourceContextBoundaryProbe) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	for _, name := range []string{"ListSourceContextIssueAttachments", "ListCommentAncestorPath", "ListCommentThreadHistory", "ListSourceContextCommentAttachments"} {
		if strings.Contains(sql, "-- name: "+name+" ") {
			p.projections++
		}
	}
	return p.DBTX.Query(ctx, sql, args...)
}

func TestSourceContextCollaborationBuildersDenyBeforeProjection(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "collaboration", true: "lookup_failure"}[failure], func(t *testing.T) {
			pool, fx, source, anchor := sourceContextBoundaryFixture(t)
			fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": source}, "issue_id=$1", source)
			probe := &sourceContextBoundaryProbe{DBTX: pool, failLookup: failure}
			q := db.New(probe)
			ctx := context.Background()
			snapshot, rows, err := BuildSourceIssueSnapshot(ctx, q, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(source))
			if failure && !collaborationErrorCode(err, "collaboration_unavailable") || !failure && !errors.Is(err, ErrSourceContextCollaboration) {
				t.Fatalf("direct source snapshot did not reject: %v", err)
			}
			if !reflect.DeepEqual(snapshot, SourceContextIssueSnapshot{}) || len(rows) != 0 {
				t.Fatal("refused source snapshot returned private fields or attachments")
			}
			build, err := BuildSourceContext(ctx, q, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(anchor))
			if failure && !collaborationErrorCode(err, "collaboration_unavailable") || !failure && !errors.Is(err, ErrSourceContextCollaboration) {
				t.Fatalf("direct comment capture did not reject: %v", err)
			}
			if !reflect.DeepEqual(build, SourceContextBuild{}) || probe.projections != 0 || probe.checks != 2 || probe.checkedID != util.MustParseUUID(source) {
				t.Fatalf("capture reached source projection or checked the wrong identity: build=%+v projections=%d checks=%d checked=%s", build, probe.projections, probe.checks, util.UUIDToString(probe.checkedID))
			}
		})
	}
}

func TestSourceContextOrdinaryChildRetainsOwnCaptureAndWorkspaceScope(t *testing.T) {
	pool, fx, source, anchor := sourceContextBoundaryFixture(t)
	probe := &sourceContextBoundaryProbe{DBTX: pool}
	q := db.New(probe)
	ctx := context.Background()
	build, err := BuildSourceContext(ctx, q, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(anchor))
	if err != nil || build.Snapshot.SourceIssue.ID != source || build.Snapshot.SourceIssue.Description == nil || *build.Snapshot.SourceIssue.Description != "ordinary-source-body" || len(build.Snapshot.CommentThread) != 1 || build.Snapshot.CommentThread[0].Content != "ordinary-source-comment" {
		t.Fatalf("ordinary child capture changed: %+v %v", build, err)
	}
	encoded, err := json.Marshal(build.Snapshot)
	if err != nil || strings.Contains(string(encoded), "private-parent-body") || probe.checkedID != util.MustParseUUID(source) {
		t.Fatal("ordinary child inherited or exposed its collaboration parent")
	}
	checks := probe.checks
	_, _, err = BuildSourceIssueSnapshot(ctx, q, util.MustParseUUID(uuid.NewString()), util.MustParseUUID(source))
	if !errors.Is(err, ErrSourceIssueDeleted) || probe.checks != checks {
		t.Fatalf("foreign workspace reached collaboration lookup or source data: %v", err)
	}
}

func TestSourceContextCollaborationDirectWritersCannotUsePrebuiltCapture(t *testing.T) {
	for _, mode := range []string{"manual", "agent"} {
		t.Run(mode, func(t *testing.T) {
			pool, fx, source, anchor := sourceContextBoundaryFixture(t)
			ctx := context.Background()
			q := db.New(pool)
			build, err := BuildSourceContext(ctx, q, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(anchor))
			if err != nil {
				t.Fatal(err)
			}
			capture, err := PrepareSourceContextCapture(build, util.MustParseUUID(uuid.NewString()), util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(fx.UserID), time.Now(), nil)
			if err != nil {
				t.Fatal(err)
			}
			// This fixture supplies an otherwise valid old capture; there is no
			// production ordinary-to-collaboration conversion operation.
			fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": source}, "issue_id=$1", source)
			fx.Cleanup(t, `DELETE FROM issue_source_context WHERE source_issue_id=$1`, source)
			fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND parent_issue_id=$2`, fx.WorkspaceID, source)
			if mode == "manual" {
				svc := NewIssueService(q, pool, nil, nil, nil)
				_, err = svc.Create(ctx, IssueCreateParams{WorkspaceID: util.MustParseUUID(fx.WorkspaceID), Title: "Forbidden captured child", Status: "todo", Priority: "none", CreatorType: "member", CreatorID: util.MustParseUUID(fx.UserID), ParentIssueID: util.MustParseUUID(source), SourceContext: &capture}, IssueCreateOpts{})
			} else {
				runtime := fx.Runtime(t, "Fake source context runtime")
				agent := fx.Agent(t, "Fake source context agent", runtime)
				fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id=$1`, agent)
				svc := &TaskService{Queries: q, TxStarter: pool, Bus: events.New()}
				_, err = svc.EnqueueQuickCreateTaskWithSourceContext(ctx, util.MustParseUUID(fx.WorkspaceID), util.MustParseUUID(fx.UserID), util.MustParseUUID(agent), pgtype.UUID{}, "Forbidden captured child", "medium", "", pgtype.UUID{}, util.MustParseUUID(source), nil, capture)
				if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, agent) != 0 {
					t.Fatal("refused capture enqueued execution")
				}
			}
			if !errors.Is(err, ErrSourceContextCollaboration) {
				t.Fatalf("direct %s writer bypassed source boundary: %v", mode, err)
			}
			if fx.Count(t, `SELECT count(*) FROM issue_source_context WHERE source_issue_id=$1`, source) != 0 || fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND parent_issue_id=$2`, fx.WorkspaceID, source) != 0 || fx.Count(t, `SELECT count(*) FROM attachment WHERE source_context_id=$1`, capture.ID) != 0 {
				t.Fatal("refused direct writer left capture, child or cloned attachment data")
			}
		})
	}
}
