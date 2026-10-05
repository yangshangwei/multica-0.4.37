package service

import (
	"context"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Execution fingerprints hold FOR SHARE on the project through task insertion.
// All resource writes must wait at that parent before changing its resource set.
func TestTriageProjectResourceWritesSerializeWithExecutionSnapshot(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			fx, s, _, _ := triageBoundaryFixture(t)
			project := util.MustParseUUID(fx.Project(t, "Execution snapshot"))
			fx.Cleanup(t, "DELETE FROM project_resource WHERE project_id=$1", project)
			resource := util.MustParseUUID(fx.Insert(t, "project_resource", testutil.Cols{"project_id": project, "workspace_id": fx.WorkspaceID, "resource_type": "github_repo", "resource_ref": `{"url":"https://example.invalid/original"}`}))
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			tx, err := fx.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, "SELECT id FROM project WHERE id=$1 FOR SHARE", project); err != nil {
				t.Fatal(err)
			}
			conn, err := fx.Pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			var pid int32
			if err = conn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				q := db.New(conn)
				var e error
				switch operation {
				case "create":
					_, e = q.CreateProjectResource(ctx, db.CreateProjectResourceParams{ProjectID: project, WorkspaceID: util.MustParseUUID(fx.WorkspaceID), ResourceType: "github_repo", ResourceRef: []byte(`{"url":"https://example.invalid/new"}`)})
				case "update":
					_, e = q.UpdateProjectResource(ctx, db.UpdateProjectResourceParams{ID: resource, ResourceRef: []byte(`{"url":"https://example.invalid/updated"}`)})
				case "delete":
					e = q.DeleteProjectResource(ctx, resource)
				}
				done <- e
			}()
			blocked := false
			for start := time.Now(); time.Since(start) < time.Second; {
				select {
				case err := <-done:
					t.Fatalf("%s escaped execution snapshot lock: %v", operation, err)
				default:
				}
				if err = fx.Pool.QueryRow(ctx, "SELECT cardinality(pg_blocking_pids($1))>0", pid).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if !blocked {
				t.Fatal("resource writer never waited on project snapshot")
			}
			rows, err := s.Queries.ListProjectResources(ctx, project)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || string(rows[0].ResourceRef) != `{"url": "https://example.invalid/original"}` {
				t.Fatalf("snapshot resources changed while locked: %+v", rows)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
