package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type workspaceErrorDB struct{ err error }
type workspaceErrorRow struct{ err error }

func (d workspaceErrorDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.err
}
func (d workspaceErrorDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, d.err
}
func (d workspaceErrorDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return workspaceErrorRow{d.err}
}
func (r workspaceErrorRow) Scan(...any) error { return r.err }

func TestWorkspaceUnavailableDoesNotRevokeMembership(t *testing.T) {
	for _, slug := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			err  error
			want int
		}{
			{"not found", pgx.ErrNoRows, 404},
			{"database unavailable", errors.New("database offline"), 503},
		} {
			t.Run(tc.name+map[bool]string{true: " slug", false: " uuid"}[slug], func(t *testing.T) {
				r := httptest.NewRequest("GET", "/api/projects", nil)
				r.Header.Set("X-User-ID", "00000000-0000-0000-0000-000000000001")
				if slug {
					r.Header.Set("X-Workspace-Slug", "p1-errors")
				} else {
					r.Header.Set("X-Workspace-ID", "00000000-0000-0000-0000-000000000002")
				}
				w := httptest.NewRecorder()
				RequireWorkspaceMember(db.New(workspaceErrorDB{tc.err}))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Fatal("denied/unavailable membership reached protected handler")
				})).ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("status=%d body=%s; want %d", w.Code, w.Body.String(), tc.want)
				}
				var body map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if tc.want == 404 && body["code"] != "workspace_access_denied" {
					t.Fatalf("missing denied code: %v", body)
				}
				if tc.want == 503 && body["code"] == "workspace_access_denied" {
					t.Fatal("an availability failure cannot erase the user's workspace state")
				}
			})
		}
	}
}

func (workspaceErrorDB) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("unexpected batch")
}
