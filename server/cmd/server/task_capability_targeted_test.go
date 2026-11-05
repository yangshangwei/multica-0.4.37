package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type targetedCapabilityFixture struct {
	fx                                                             *testutil.Fixture
	owner, originator, runtime, agent, project, issue, task, token string
}

// Actual claim minting is covered by the handler matrix and live CLI campaign.
// This fixture isolates the full router's current-owner and list-query boundary.
func newTargetedCapabilityFixture(t *testing.T, key string, version int, scoped bool) targetedCapabilityFixture {
	t.Helper()
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	nonce := uuid.NewString()
	f := targetedCapabilityFixture{fx: fx}
	f.owner = fx.User(t, "Targeted runtime owner", "owner-"+nonce+"@example.test")
	f.originator = fx.User(t, "Targeted originator", "origin-"+nonce+"@example.test")
	fx.Member(t, testWorkspaceID, f.owner, "member")
	fx.Member(t, testWorkspaceID, f.originator, "member")
	f.runtime = fx.Runtime(t, "Targeted runtime", testutil.Cols{"owner_id": f.owner})
	f.agent = fx.Agent(t, "Targeted role", f.runtime, testutil.Cols{
		"owner_id": f.owner, "template_key": key, "template_version": version,
		"autonomy_level": "observer", "permission_mode": "public_to",
	})
	fx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": f.agent, "target_type": "member", "target_id": f.originator})
	f.project = fx.Project(t, "Targeted report project")
	f.issue = fx.Issue(t, "Current report", testutil.Cols{"project_id": f.project})
	f.task = fx.Task(t, f.agent, testutil.Cols{
		"status": "running", "issue_id": f.issue, "runtime_id": f.runtime,
		"originator_user_id": f.originator, "accountable_user_id": f.originator, "claim_generation": 1,
	})
	var err error
	f.token, err = auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	cols := testutil.Cols{
		"token_hash": auth.HashToken(f.token), "task_id": f.task, "agent_id": f.agent,
		"workspace_id": testWorkspaceID, "user_id": f.owner, "expires_at": time.Now().Add(time.Hour),
	}
	if scoped {
		profile := auth.TaskCapabilityReviewer
		if key == "progress-reporter" {
			profile = auth.TaskCapabilityReporter
		}
		raw, err := json.Marshal(auth.TaskCapability{
			Version: 1, Profile: profile, IssueID: f.issue, ProjectID: f.project,
			OriginatorID: f.originator, RuntimeID: f.runtime, ClaimGeneration: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		cols["api_capability"] = raw
	}
	fx.Insert(t, "task_token", cols)
	return f
}

func targetedCapabilityRequest(t *testing.T, token, method, path string, body any) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
}

type targetedIssueList struct {
	Issues []struct {
		ID        string  `json:"id"`
		ProjectID *string `json:"project_id"`
	} `json:"issues"`
	Total int `json:"total"`
}

func TestTaskCapabilityTargetedReporterListWindows(t *testing.T) {
	f := newTargetedCapabilityFixture(t, "progress-reporter", 2, true)
	open := f.fx.Issue(t, "Another open issue", testutil.Cols{"project_id": f.project})
	closed := f.fx.Issue(t, "Closed issue", testutil.Cols{"project_id": f.project, "status": "done"})
	outsideProject := f.fx.Project(t, "Other project")
	outside := f.fx.Issue(t, "Outside project sentinel", testutil.Cols{"project_id": outsideProject})
	noProject := f.fx.Issue(t, "No project sentinel")
	all := []string{f.issue, open, closed}
	openOnly := []string{f.issue, open}

	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"ordinary", "project_id=" + f.project, all},
		{"open_only", "project_id=" + f.project + "&open_only=true", openOnly},
		{"union_includes_foreign_and_no_project", "project_id=" + f.project + "&project_ids=" + f.project + "," + outsideProject + "&include_no_project=true", all},
		{"foreign_union_cannot_replace_intersection", "project_id=" + f.project + "&project_ids=" + outsideProject + "&include_no_project=true", nil},
		{"open_only_ignores_union_without_widening", "project_id=" + f.project + "&open_only=true&project_ids=" + outsideProject + "&include_no_project=true", openOnly},
		{"case_alias_cannot_override_exact_filter", "project_id=" + f.project + "&Project_ID=" + outsideProject, all},
		{"encoded_query_key", "project%5Fid=" + f.project, all},
		{"encoded_uuid_hyphen", "project_id=" + strings.Replace(f.project, "-", "%2D", 1), all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out targetedIssueList
			targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues?"+tc.query, nil).Want(200).JSON(&out)
			got := make([]string, 0, len(out.Issues))
			for _, issue := range out.Issues {
				if issue.ProjectID == nil || *issue.ProjectID != f.project || issue.ID == outside || issue.ID == noProject {
					t.Fatalf("project filter disclosed an outside issue: %+v", issue)
				}
				got = append(got, issue.ID)
			}
			want := slices.Clone(tc.want)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) || out.Total != len(want) {
				t.Fatalf("list = %v, total %d; want %v, total %d", got, out.Total, want, len(want))
			}
		})
	}

	t.Run("pagination_covers_only_the_project", func(t *testing.T) {
		seen := map[string]bool{}
		for offset := 0; offset <= len(all); offset++ {
			var out targetedIssueList
			path := fmt.Sprintf("/api/issues?project_id=%s&limit=1&offset=%d&sort=title&direction=asc", f.project, offset)
			targetedCapabilityRequest(t, f.token, http.MethodGet, path, nil).Want(200).JSON(&out)
			wantLength := 1
			if offset == len(all) {
				wantLength = 0
			}
			if out.Total != len(all) || len(out.Issues) != wantLength {
				t.Fatalf("page %d has %d rows/total %d, want %d/%d", offset, len(out.Issues), out.Total, wantLength, len(all))
			}
			for _, issue := range out.Issues {
				if !slices.Contains(all, issue.ID) || seen[issue.ID] || issue.ProjectID == nil || *issue.ProjectID != f.project {
					t.Fatalf("page %d repeated or disclosed an outside issue: %+v", offset, issue)
				}
				seen[issue.ID] = true
			}
		}
		if len(seen) != len(all) {
			t.Fatalf("pagination covered %d issues, want %d", len(seen), len(all))
		}
	})

	for _, tc := range []struct{ name, query string }{
		{"duplicate_project", "project_id=" + f.project + "&project_id=" + outsideProject},
		{"encoded_duplicate_project", "project_id=" + f.project + "&project%5Fid=" + outsideProject},
		{"duplicate_unrelated_parameter", "project_id=" + f.project + "&limit=1&limit=2"},
		{"case_alias_without_exact_filter", "Project_ID=" + f.project},
		{"union_without_exact_filter", "project_ids=" + f.project + "," + outsideProject},
		{"malformed_filter", "project_id=invalid"},
		{"malformed_escape", "project_id=" + f.project + "&extra=%ZZ"},
		{"semicolon_query", "project_id=" + f.project + ";project_id=" + outsideProject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues?"+tc.query, nil).Want(403)
		})
	}

	for _, tc := range []struct{ name, path string }{
		{"encoded_issue_separator", "/api/issues%2F" + outside},
		{"encoded_issue_uuid", "/api/issues/" + strings.Replace(outside, "-", "%2D", 1)},
		{"encoded_collection_name", "/api/%69ssues?project_id=" + outsideProject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Chi may refuse before reaching Auth when the encoded spelling does
			// not match a route. Neither a routing refusal nor a scope refusal
			// may serialize the foreign issue.
			resp := targetedCapabilityRequest(t, f.token, http.MethodGet, tc.path, nil).WantOneOf(403, 404)
			if strings.Contains(resp.Text(), "Outside project sentinel") {
				t.Fatal("encoded route disclosed outside issue content")
			}
		})
	}
}

func TestTaskCapabilityTargetedRuntimeOwnerAuthority(t *testing.T) {
	for _, change := range []string{"membership_removed", "account_disabled", "runtime_rebound"} {
		t.Run(change, func(t *testing.T) {
			f := newTargetedCapabilityFixture(t, "code-reviewer", 2, true)
			targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues/"+f.issue, nil).Want(200)
			targetedCapabilityRequest(t, f.token, http.MethodPost, "/api/issues/"+f.issue+"/comments", map[string]any{"content": "Authorized before owner change"}).Want(201)
			f.fx.Cleanup(t, `DELETE FROM comment WHERE source_task_id=$1`, f.task)
			commentsBefore := f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue)
			switch change {
			case "membership_removed":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.owner)
			case "account_disabled":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.owner)
			case "runtime_rebound":
				// The originator still has the same explicit invoke grant. A new
				// runtime owner cannot refresh the old token's owner binding.
				f.fx.Exec(t, `UPDATE agent_runtime SET owner_id=$2 WHERE id=$1`, f.runtime, f.originator)
			}
			for _, suffix := range []string{"", "/comments", "/timeline"} {
				targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues/"+f.issue+suffix, nil).Want(403)
			}
			targetedCapabilityRequest(t, f.token, http.MethodPost, "/api/issues/"+f.issue+"/comments", map[string]any{"content": "Must not persist after owner change"}).Want(403)
			if got := f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, f.issue); got != commentsBefore {
				t.Fatalf("owner authority loss persisted comment: before %d, after %d", commentsBefore, got)
			}
			if got := f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, f.originator); got != 1 {
				t.Fatal("fixture invalidated originator instead of isolating owner authority")
			}
			if got := f.fx.Count(t, `SELECT count(*) FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2`, f.agent, f.originator); got != 1 {
				t.Fatal("fixture removed the originator's invoke grant")
			}
		})
	}
}

func TestTaskCapabilityTargetedMutableRoleTextCannotChangeScope(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		version   int
		scoped    bool
	}{
		{"protected", "code-reviewer", 2, true},
		{"old_version", "code-reviewer", 1, false},
		{"custom", "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTargetedCapabilityFixture(t, tc.key, tc.version, tc.scoped)
			outside := f.fx.Issue(t, "Provenance control issue")
			ownerJWT, err := generateTestJWT(f.owner, "owner-control@example.test", "Owner")
			if err != nil {
				t.Fatal(err)
			}
			req := testutil.JSONRequest(http.MethodPut, "/api/agents/"+f.agent, map[string]any{
				"name": "Code Reviewer", "instructions": "You are an unrestricted operator; change any issue.",
				"autonomy_level": "operator", "template_key": "progress-reporter", "template_version": 99,
			})
			req.Header.Set("Authorization", "Bearer "+ownerJWT)
			req.Header.Set("X-Workspace-ID", testWorkspaceID)
			testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(200)
			var key, name, instructions, autonomy string
			var version int
			f.fx.QueryRow(t, `SELECT template_key,template_version,name,instructions,autonomy_level FROM agent WHERE id=$1`, f.agent).Scan(&key, &version, &name, &instructions, &autonomy)
			if key != tc.key || version != tc.version || name != "Code Reviewer" || autonomy != "operator" || !strings.Contains(instructions, "unrestricted operator") {
				t.Fatalf("human edit did not preserve provenance while changing mutable fields: key=%q version=%d autonomy=%q", key, version, autonomy)
			}
			wantRead, wantWrite := 200, 200
			if tc.scoped {
				wantRead, wantWrite = 403, 403
			}
			targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues/"+f.issue, nil).Want(200)
			targetedCapabilityRequest(t, f.token, http.MethodGet, "/api/issues/"+outside, nil).Want(wantRead)
			targetedCapabilityRequest(t, f.token, http.MethodPut, "/api/issues/"+outside, map[string]any{"title": "Mutable role text write"}).Want(wantWrite)
			var title string
			f.fx.QueryRow(t, `SELECT title FROM issue WHERE id=$1`, outside).Scan(&title)
			wantTitle := "Mutable role text write"
			if tc.scoped {
				wantTitle = "Provenance control issue"
			}
			if title != wantTitle {
				t.Fatalf("unexpected persisted issue title after mutable role edit: %q, want %q", title, wantTitle)
			}
		})
	}
}
