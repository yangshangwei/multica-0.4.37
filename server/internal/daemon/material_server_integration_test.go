//go:build materialintegration

package daemon

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type materialRemoteFixture struct {
	Execution protocol.MaterialExecution
	Merged    bool
	MergedAt  *time.Time
	Note      *protocol.MaterialAuditNote
}

func TestMaterialCompositeExecutable(t *testing.T) {
	if os.Getenv("MATERIAL_COMPOSITE_HELPER") != "1" {
		t.Skip("test-created executable only")
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	statePath := os.Getenv("MATERIAL_COMPOSITE_STATE")
	var state materialRemoteFixture
	if b, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(b, &state)
	}
	if len(args) > 0 && args[0] == "api" {
		materialCompositeReadAPI(state, args[1])
		os.Exit(0)
	}
	// A real native backend invokes this child with Claude's stream-json protocol.
	line, _ := bufio.NewReader(os.Stdin).ReadBytes('\n')
	var input any
	_ = json.Unmarshal(line, &input)
	var prompt string
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case string:
			if strings.Contains(x, "multica project material operation ") {
				prompt = x
			}
		case []any:
			for _, i := range x {
				visit(i)
			}
		case map[string]any:
			for _, i := range x {
				visit(i)
			}
		}
	}
	visit(input)
	id := regexp.MustCompile(`multica project material operation ([a-f0-9-]{36})`).FindStringSubmatch(prompt)
	if len(id) != 2 {
		fmt.Fprintln(os.Stderr, "no operation in native prompt")
		os.Exit(2)
	}
	call := func(action string, out any) {
		body, _ := json.Marshal(map[string]string{"operation_id": id[1], "action": action, "request_id": uuid.NewString()})
		req, _ := http.NewRequest("POST", "http://127.0.0.1:"+os.Getenv("MULTICA_DAEMON_PORT")+"/material", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+os.Getenv("MULTICA_TOKEN"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			os.Exit(3)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			fmt.Fprintln(os.Stderr, action, string(raw))
			os.Exit(4)
		}
		if out != nil {
			if json.Unmarshal(raw, out) != nil {
				os.Exit(5)
			}
		}
	}
	var e protocol.MaterialExecution
	call("operation", &e)
	if e.Action == "merge" {
		call("observe", nil)
	}
	if e.Action != "verify_baseline" {
		call("permit", nil)
	}
	switch e.Action {
	case "publish":
		state = materialRemoteFixture{Execution: e}
	case "review_sync":
		state.Note = e.AuditNote
	case "merge":
		state.Merged = true
		now := time.Now().UTC()
		state.MergedAt = &now
	case "verify_baseline":
	default:
		os.Exit(6)
	}
	if e.Action != "verify_baseline" {
		raw, _ := json.Marshal(state)
		if os.WriteFile(statePath, raw, 0600) != nil {
			os.Exit(7)
		}
		call("observe", nil)
	}
	count, _ := os.OpenFile(statePath+".runs", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if count != nil {
		fmt.Fprintln(count, e.Action)
		_ = count.Close()
	}
	fmt.Println(`{"type":"system","session_id":"material-native"}`)
	fmt.Println(`{"type":"result","subtype":"success","is_error":false,"session_id":"material-native","result":"runtime stage finished"}`)
	os.Exit(0)
}
func materialCompositeReadAPI(s materialRemoteFixture, endpoint string) {
	u, _ := url.Parse(endpoint)
	p := u.Path
	e := s.Execution
	head := strings.Repeat("a", 40)
	merged := strings.Repeat("c", 40)
	blob := strings.Repeat("b", 40)
	var out any
	switch {
	case strings.HasSuffix(p, "/merge_requests"):
		out = []any{}
		if e.OperationID != "" {
			out = []any{map[string]any{"iid": 1}}
		}
	case strings.HasSuffix(p, "/merge_requests/1"):
		state := "opened"
		if s.Merged {
			state = "merged"
		}
		out = map[string]any{"iid": 1, "project_id": 7, "source_project_id": 7, "target_project_id": 7, "web_url": "https://gitlab.com/fixture/material/-/merge_requests/1", "sha": head, "source_branch": e.SourceBranch, "target_branch": e.BaseBranch, "state": state, "merge_commit_sha": merged, "merged_at": s.MergedAt, "merge_user": map[string]string{"username": "runtime-user"}, "detailed_merge_status": "mergeable", "has_conflicts": false, "changes_count": strconv.Itoa(len(e.Files)), "diff_refs": map[string]string{"head_sha": head}, "user": map[string]bool{"can_merge": true}}
	case strings.HasSuffix(p, "/merge_requests/1/diffs"):
		rows := []any{}
		for _, f := range e.Files {
			rows = append(rows, map[string]any{"old_path": f.Path, "new_path": f.Path, "a_mode": "0", "b_mode": "100644", "new_file": true})
		}
		out = rows
	case strings.HasSuffix(p, "/repository/tree"):
		rows := []any{}
		seen := map[string]bool{}
		for _, f := range e.Files {
			parts := strings.Split(f.Path, "/")
			for i := range parts {
				full := strings.Join(parts[:i+1], "/")
				if seen[full] {
					continue
				}
				seen[full] = true
				mode, typ, sha := "040000", "tree", head
				if i == len(parts)-1 {
					mode, typ, sha = "100644", "blob", blob
				}
				rows = append(rows, map[string]string{"path": full, "name": parts[i], "mode": mode, "type": typ, "id": sha})
			}
		}
		out = rows
	case strings.Contains(p, "/repository/files/"):
		name := strings.SplitN(p, "/repository/files/", 2)[1]
		for _, f := range e.Files {
			if f.Path == name {
				out = map[string]any{"file_path": name, "commit_id": u.Query().Get("ref"), "last_commit_id": head, "blob_id": blob, "encoding": "base64", "size": len(f.Content), "content": base64.StdEncoding.EncodeToString([]byte(f.Content))}
			}
		}
	case strings.HasSuffix(p, "/merge_requests/1/notes"):
		out = []any{}
		if s.Note != nil {
			out = []any{map[string]any{"body": s.Note.Body, "author": map[string]string{"username": "runtime-user"}}}
		}
	case strings.HasSuffix(p, "/pulls"):
		if e.OperationID == "" {
			out = []any{}
		} else {
			out = []any{map[string]any{"number": 1}}
		}
	case strings.HasSuffix(p, "/pulls/1"):
		out = map[string]any{"number": 1, "html_url": "https://github.com/fixture/material/pull/1", "state": "open", "merged": s.Merged, "merge_commit_sha": merged, "merged_at": s.MergedAt, "merged_by": map[string]string{"login": "runtime-user"}, "mergeable": true, "mergeable_state": "clean", "changed_files": len(e.Files), "head": map[string]any{"sha": head, "ref": e.SourceBranch, "repo": map[string]string{"full_name": e.Repository}}, "base": map[string]any{"ref": e.BaseBranch, "repo": map[string]string{"full_name": e.Repository}}}
	case strings.HasSuffix(p, "/pulls/1/files"):
		rows := []any{}
		for _, f := range e.Files {
			rows = append(rows, map[string]string{"filename": f.Path, "status": "added"})
		}
		out = rows
	case strings.Contains(p, "/git/commits/"):
		out = map[string]any{"sha": filepath.Base(p), "tree": map[string]string{"sha": head}}
	case strings.Contains(p, "/git/trees/"):
		rows := []any{}
		seen := map[string]bool{}
		for _, f := range e.Files {
			parts := strings.Split(f.Path, "/")
			for i, part := range parts {
				if seen[part] {
					continue
				}
				seen[part] = true
				mode, typ, sha := "040000", "tree", head
				if i == len(parts)-1 {
					mode, typ, sha = "100644", "blob", blob
				}
				rows = append(rows, map[string]string{"path": part, "mode": mode, "type": typ, "sha": sha})
			}
		}
		out = map[string]any{"sha": head, "truncated": false, "tree": rows}
	case strings.Contains(p, "/git/blobs/"):
		content := e.Files[0].Content
		out = map[string]any{"sha": blob, "encoding": "base64", "size": len(content), "content": base64.StdEncoding.EncodeToString([]byte(content))}
	case strings.Contains(p, "/rules/branches/"):
		out = []any{}
	case strings.Contains(p, "/branches/"):
		out = map[string]bool{"protected": false}
	case p == "/graphql":
		out = map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"headRefOid": head, "baseRefName": e.BaseBranch, "mergeStateStatus": "CLEAN", "reviewDecision": "APPROVED", "commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": head, "statusCheckRollup": map[string]string{"state": "SUCCESS"}}}}}}}}}
	case strings.HasSuffix(p, "/comments"):
		out = []any{}
		if s.Note != nil {
			out = []any{map[string]any{"body": s.Note.Body, "user": map[string]string{"login": "runtime-user"}}}
		}
	default:
		fmt.Print("HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\n\r\n{}")
		return
	}
	fmt.Print("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n")
	_ = json.NewEncoder(os.Stdout).Encode(out)
}

func TestMaterialCompositeQueueRuntimeObservation(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) { testMaterialCompositeQueueRuntimeObservation(t, provider) })
	}
}

func testMaterialCompositeQueueRuntimeObservation(t *testing.T, provider string) {
	raw := os.Getenv("DATABASE_URL")
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil || raw == "" || !strings.HasPrefix(cfg.ConnConfig.Database, "multica_material_runtime_") {
		t.Fatal("requires explicitly owned material runtime database")
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var identity string
	if pool.QueryRow(t.Context(), "select current_database()").Scan(&identity) != nil || identity != cfg.ConnConfig.Database {
		t.Fatal("wrong database")
	}
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	user := fx.User(t, "Runtime owner", suffix+"@example.test")
	ws := fx.Workspace(t, "Runtime materials", "material-"+suffix)
	fx.Member(t, ws, user, "owner")
	fx.UserID, fx.WorkspaceID = user, ws
	project := fx.Project(t, "Materials")
	daemonID := "material-" + suffix
	rid := fx.Runtime(t, "Selected fake runtime", testutil.Cols{"provider": "claude", "daemon_id": daemonID, "runtime_mode": "local"})
	resourceRef, _ := json.Marshal(map[string]string{"url": "https://" + provider + ".com/fixture/material.git"})
	resource := fx.Insert(t, "project_resource", testutil.Cols{"workspace_id": ws, "project_id": project, "resource_type": "github_repo", "resource_ref": string(resourceRef), "position": 0})
	q := db.New(pool)
	if err = q.SeedIssueStatusEntries(t.Context(), util.MustParseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	token := "mdt_" + uuid.NewString()
	fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(token), "workspace_id": ws, "daemon_id": daemonID, "user_id": user, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	hub := realtime.NewHub()
	go hub.Run()
	h := handler.New(q, pool, hub, events.New(), service.NewEmailService(), nil, nil, nil, handler.Config{MessagingIntegrationsDisabled: true}, daemonws.NewHub())
	svc := service.NewMaterialService(q, pool, h.IssueService, h.TaskService)
	credential := service.ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}}
	access := service.MaterialAccess{WorkspaceID: util.MustParseUUID(ws), ProjectID: util.MustParseUUID(project), Credential: credential}
	ctx := auth.WithAuthenticatedPrincipal(t.Context(), credential.Principal)
	auto := false
	_, err = svc.Configure(ctx, access, material.SettingsInput{RequestID: uuid.NewString(), Enabled: true, Provider: provider, ResourceID: resource, ExecutionRuntimeID: &rid, Repository: "fixture/material", BaseBranch: "main", RootPath: "engineering", ReviewerIDs: []string{user}, AutoStartPlan: &auto})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.Create(ctx, access, material.CreateInput{RequestID: uuid.NewString(), Title: "Deterministic runtime material"})
	if err != nil {
		t.Fatal(err)
	}
	detail, err = svc.Revise(ctx, access, detail.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version, Kind: "requirement", Content: strings.Repeat("The runtime writes and the daemon independently verifies bytes.\n", 3), Sources: []material.Source{{ResourceID: resource, Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: Runtime receipts bind current claims"}, Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Group(func(r chi.Router) {
		r.Use(middleware.DaemonAuth(q, nil, nil, nil))
		r.Post("/api/daemon/runtimes/{runtimeId}/tasks/claim", h.ClaimTaskByRuntime)
		r.Get("/api/daemon/tasks/{taskId}/material-operation", h.GetDaemonMaterialOperation)
		r.Post("/api/daemon/tasks/{taskId}/material-observations", h.ObserveDaemonMaterialOperation)
		r.Post("/api/daemon/tasks/{taskId}/start", h.StartTask)
		r.Post("/api/daemon/tasks/{taskId}/complete", h.CompleteTask)
		r.Post("/api/daemon/tasks/{taskId}/progress", h.ReportTaskProgress)
		r.Post("/api/daemon/tasks/{taskId}/messages", h.ReportTaskMessages)
		r.Post("/api/daemon/runtimes/{runtimeId}/tasks/{taskId}/prepare-lease", h.ExtendTaskPrepareLease)
	})
	router.Group(func(r chi.Router) {
		r.Use(middleware.Auth(q, nil, nil))
		r.Post("/api/projects/{id}/materials/{materialID}/operations/{operationID}/permit", h.PermitMaterialOperation)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		if rec.Code >= 400 {
			t.Logf("HTTP %s %s => %d %s", r.Method, r.URL.Path, rec.Code, rec.Body.String())
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	}))
	defer server.Close()
	root := t.TempDir()
	native := filepath.Join(root, "fake-runtime")
	gh := filepath.Join(root, "gh")
	binary, _ := os.Executable()
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(binary, "'", "'\\''") + "' -test.run '^TestMaterialCompositeExecutable$' -- \"$@\"\n"
	for _, f := range []string{native, gh, filepath.Join(root, "glab")} {
		if os.WriteFile(f, []byte(script), 0700) != nil {
			t.Fatal("write fake executable")
		}
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	// Every simulated API read launches this test binary. Disable only the
	// race runtime's one-second exit sleep in those child fixtures; retain the
	// product execution deadline and all race detection.
	t.Setenv("GORACE", os.Getenv("GORACE")+" atexit_sleep_ms=0")
	t.Setenv("MATERIAL_COMPOSITE_HELPER", "1")
	statePath := filepath.Join(root, "remote.json")
	t.Setenv("MATERIAL_COMPOSITE_STATE", statePath)
	var logs bytes.Buffer
	d := New(Config{ServerBaseURL: server.URL, WorkspacesRoot: filepath.Join(root, "workspaces"), DaemonID: daemonID, AgentTimeout: 20 * time.Second, Agents: map[string]AgentEntry{"claude": {Path: native}}}, slog.New(slog.NewTextHandler(&logs, nil)))
	d.executionEnvironmentCommand = nil
	d.client.SetToken(token)
	d.runtimeIndex[rid] = Runtime{ID: rid, Provider: "claude"}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		d.materialHandler()(rec, r)
		if rec.Code >= 400 {
			t.Logf("LOCAL %d %s", rec.Code, rec.Body.String())
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	}))
	defer local.Close()
	u, _ := url.Parse(local.URL)
	d.cfg.HealthPort, _ = strconv.Atoi(u.Port())
	run := func(action string) {
		t.Helper()
		for i := 0; i < 5; i++ {
			fx.Exec(t, "UPDATE git_material_job SET next_at=now() WHERE workspace_id=$1", ws)
			if _, err := svc.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			var n int
			if pool.QueryRow(ctx, "select count(*) from agent_task_queue where runtime_id=$1 and status='queued'", rid).Scan(&n) == nil && n > 0 {
				break
			}
		}
		task, err := d.client.ClaimTask(ctx, rid)
		if err != nil || task == nil {
			t.Fatalf("claim %s: %v", action, err)
		}
		if task.MaterialExecution == nil || task.MaterialExecution.Action != action {
			t.Fatalf("wrong claimed stage %+v", task.MaterialExecution)
		}
		result, err := d.runTask(ctx, *task, "claude", 0, d.logger)
		if err != nil || result.Status != "completed" {
			t.Fatalf("run %s: %v status=%s logs=%s", action, err, result.Status, logs.String())
		}
		if err := d.client.CompleteTask(ctx, task.ID, result.Comment, result.BranchName, result.SessionID, result.WorkDir, false, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	run("publish")
	detail, err = svc.Detail(ctx, access, detail.Material.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Revisions[0].HeadSHA == nil || detail.Material.Status != "in_review" {
		t.Fatalf("publication not verified: %+v", detail)
	}
	detail, err = svc.Decide(ctx, access, detail.Material.ID, material.DecisionInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version, RevisionID: detail.Revisions[0].ID, HeadSHA: *detail.Revisions[0].HeadSHA, Decision: "approve", Reason: "Verified candidate"})
	if err != nil {
		t.Fatal(err)
	}
	run("review_sync")
	run("merge")
	detail, err = svc.Detail(ctx, access, detail.Material.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Baseline == nil || detail.Baseline.Commit != strings.Repeat("c", 40) {
		t.Fatalf("missing verified baseline: %+v", detail)
	}
	for i := 0; i < 3; i++ {
		_, _ = svc.ProcessNext(ctx)
	}
	runs, _ := os.ReadFile(statePath + ".runs")
	if string(runs) != "publish\nreview_sync\nmerge\n" {
		t.Fatalf("duplicate or missing native stages: %q", runs)
	}
	detail, err = svc.Detail(ctx, access, detail.Material.ID)
	if err != nil || detail.Plan == nil || detail.Plan.Status != "waiting_manual" || detail.Plan.IssueID == nil {
		t.Fatalf("auto-off plan did not wait: %+v %v", detail.Plan, err)
	}
	issueID := *detail.Plan.IssueID
	for attempt := 0; attempt < 2; attempt++ {
		detail, err = svc.Command(ctx, access, detail.Material.ID, "start-plan", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version})
		if err != nil {
			t.Fatal(err)
		}
		run("verify_baseline")
		detail, err = svc.Detail(ctx, access, detail.Material.ID)
		if err != nil || detail.Plan == nil || detail.Plan.IssueID == nil || *detail.Plan.IssueID != issueID {
			t.Fatal("manual retry replaced plan issue")
		}
	}
	runs, _ = os.ReadFile(statePath + ".runs")
	if string(runs) != "publish\nreview_sync\nmerge\nverify_baseline\nverify_baseline\n" {
		t.Fatalf("missing/duplicate runtime calls: %q", runs)
	}
	var retained int
	if err := pool.QueryRow(ctx, "select count(*) from agent_task_queue where issue_id=$1 and status='completed'", issueID).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("prior plan run lost: %d %v", retained, err)
	}
	t.Log("real queue/claim/native executable/read-only host observation/server baseline and manual retry passed; no App, server credentials or live model/provider")
}
