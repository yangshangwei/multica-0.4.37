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
	"sync/atomic"
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
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type materialRemoteFixture struct {
	BaseCommit  string
	Head        string
	MergeCommit string
	Trees       map[string]map[string]string

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
	if state.Trees == nil {
		state.Trees = map[string]map[string]string{strings.Repeat("c", 40): {"README.md": "Keep user documentation\n", "uat/history.bin": string([]byte{0, 42})}}
		state.BaseCommit = strings.Repeat("c", 40)
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
		state.Execution = e
		state.Merged = false
		state.MergedAt = nil
		state.Note = nil
		state.Head = material.Digest(e.OperationID + "head")
		state.MergeCommit = material.Digest(e.OperationID + "merge")
		next := map[string]string{}
		for k, v := range state.Trees[state.BaseCommit] {
			next[k] = v
		}
		for _, f := range e.Files {
			next[f.Path] = f.Content
		}
		state.Trees[state.Head] = next
	case "review_sync":
		state.Note = e.AuditNote
	case "merge":
		state.Merged = true
		state.Trees[state.MergeCommit] = state.Trees[state.Head]
		state.BaseCommit = state.MergeCommit
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
	if s.Trees == nil {
		s.Trees = map[string]map[string]string{strings.Repeat("c", 40): {"README.md": "Keep user documentation\n", "uat/history.bin": string([]byte{0, 42})}}
		s.BaseCommit = strings.Repeat("c", 40)
	}
	head := s.Head
	merged := s.MergeCommit
	base := strings.Repeat("c", 40)
	if e.RepositoryExecution != nil && e.RepositoryExecution.BaseCommit != "" {
		base = e.RepositoryExecution.BaseCommit
	}
	trees := map[string][]map[string]string{}
	blobs := map[string]string{}
	rootTrees := map[string]string{}
	var build func(map[string]string, string, string) string
	build = func(files map[string]string, commit, dir string) string {
		sha := material.Digest(commit + ":" + dir)
		rows := []map[string]string{}
		dirs := map[string]bool{}
		for name, body := range files {
			if !strings.HasPrefix(name, dir) {
				continue
			}
			rel := strings.TrimPrefix(name, dir)
			part, rest, has := strings.Cut(rel, "/")
			if has {
				if !dirs[part] {
					dirs[part] = true
					child := build(files, commit, dir+part+"/")
					rows = append(rows, map[string]string{"path": part, "mode": "040000", "type": "tree", "sha": child})
				}
				_ = rest
			} else {
				blob := material.Digest(body)
				blobs[blob] = body
				rows = append(rows, map[string]string{"path": part, "mode": "100644", "type": "blob", "sha": blob})
			}
		}
		trees[sha] = rows
		return sha
	}
	for commit, files := range s.Trees {
		rootTrees[commit] = build(files, commit, "")
	}
	recursive := func(commit string) []map[string]string {
		rows := []map[string]string{}
		seen := map[string]bool{}
		for name, body := range s.Trees[commit] {
			parts := strings.Split(name, "/")
			for i := range parts {
				full := strings.Join(parts[:i+1], "/")
				if seen[full] {
					continue
				}
				seen[full] = true
				mode, typ, sha := "040000", "tree", material.Digest(commit+":"+full+"/")
				if i == len(parts)-1 {
					mode, typ, sha = "100644", "blob", material.Digest(body)
				}
				rows = append(rows, map[string]string{"path": full, "name": parts[i], "mode": mode, "type": typ, "id": sha, "sha": sha})
			}
		}
		return rows
	}
	var out any
	switch {
	case strings.Contains(p, "/git/ref/heads/"):
		out = map[string]any{"ref": "refs/heads/" + filepath.Base(p), "object": map[string]string{"type": "commit", "sha": s.BaseCommit}}
	case strings.Contains(p, "/repository/branches/"):
		out = map[string]any{"name": filepath.Base(p), "commit": map[string]string{"id": s.BaseCommit}}
	case strings.HasSuffix(p, "/pulls") || strings.HasSuffix(p, "/merge_requests"):
		out = []any{}
		if e.OperationID != "" {
			out = []any{map[string]any{"number": 1, "iid": 1}}
		}
	case strings.HasSuffix(p, "/pulls/1"):
		out = map[string]any{"number": 1, "html_url": "https://github.com/fixture/material/pull/1", "state": "open", "merged": s.Merged, "merge_commit_sha": merged, "merged_at": s.MergedAt, "merged_by": map[string]string{"login": "runtime-user"}, "mergeable": true, "mergeable_state": "clean", "changed_files": len(e.Files), "head": map[string]any{"sha": head, "ref": e.SourceBranch, "repo": map[string]string{"full_name": e.Repository}}, "base": map[string]any{"sha": base, "ref": e.BaseBranch, "repo": map[string]string{"full_name": e.Repository}}}
	case strings.HasSuffix(p, "/merge_requests/1"):
		state := "opened"
		if s.Merged {
			state = "merged"
		}
		out = map[string]any{"iid": 1, "project_id": 7, "source_project_id": 7, "target_project_id": 7, "web_url": "https://gitlab.com/fixture/material/-/merge_requests/1", "sha": head, "source_branch": e.SourceBranch, "target_branch": e.BaseBranch, "state": state, "merge_commit_sha": merged, "merged_at": s.MergedAt, "merge_user": map[string]string{"username": "runtime-user"}, "detailed_merge_status": "mergeable", "has_conflicts": false, "changes_count": strconv.Itoa(len(e.Files)), "diff_refs": map[string]string{"head_sha": head, "base_sha": base}, "user": map[string]bool{"can_merge": true}}
	case strings.HasSuffix(p, "/pulls/1/files") || strings.HasSuffix(p, "/merge_requests/1/diffs"):
		rows := []any{}
		for _, f := range e.Files {
			status, amode, added := "added", "0", true
			if e.RepositoryExecution != nil && len(e.RepositoryExecution.Maintenance) > 0 {
				status, amode, added = "modified", "100644", false
			}
			rows = append(rows, map[string]any{"filename": f.Path, "status": status, "old_path": f.Path, "new_path": f.Path, "a_mode": amode, "b_mode": "100644", "new_file": added})
		}
		out = rows
	case strings.Contains(p, "/git/commits/"):
		commit := filepath.Base(p)
		out = map[string]any{"sha": commit, "tree": map[string]string{"sha": rootTrees[commit]}}
	case strings.Contains(p, "/git/trees/"):
		sha := filepath.Base(p)
		rows := trees[sha]
		if u.Query().Get("recursive") == "1" {
			for commit, root := range rootTrees {
				if root == sha {
					rows = recursive(commit)
				}
			}
		}
		out = map[string]any{"sha": sha, "truncated": false, "tree": rows}
	case strings.HasSuffix(p, "/repository/tree"):
		out = recursive(u.Query().Get("ref"))
	case strings.HasSuffix(p, "/raw"):
		sha := filepath.Base(filepath.Dir(p))
		fmt.Print("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\n\r\n")
		_, _ = os.Stdout.Write([]byte(blobs[sha]))
		return
	case strings.Contains(p, "/blobs/"):
		sha := filepath.Base(p)
		body := blobs[sha]
		out = map[string]any{"sha": sha, "encoding": "base64", "size": len(body), "content": base64.StdEncoding.EncodeToString([]byte(body))}
	case strings.Contains(p, "/repository/files/"):
		name := strings.SplitN(p, "/repository/files/", 2)[1]
		commit := u.Query().Get("ref")
		body := s.Trees[commit][name]
		out = map[string]any{"file_path": name, "commit_id": commit, "last_commit_id": commit, "blob_id": material.Digest(body), "encoding": "base64", "size": len(body), "content": base64.StdEncoding.EncodeToString([]byte(body))}
	case strings.Contains(p, "/rules/branches/"):
		out = []any{}
	case strings.Contains(p, "/branches/"):
		out = map[string]bool{"protected": false}
	case p == "/graphql":
		out = map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"headRefOid": head, "baseRefName": e.BaseBranch, "mergeStateStatus": "CLEAN", "reviewDecision": "APPROVED", "commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": head, "statusCheckRollup": map[string]string{"state": "SUCCESS"}}}}}}}}}
	case strings.HasSuffix(p, "/comments") || strings.HasSuffix(p, "/notes"):
		out = []any{}
		if s.Note != nil {
			out = []any{map[string]any{"body": s.Note.Body, "user": map[string]string{"login": "runtime-user"}, "author": map[string]string{"username": "runtime-user"}}}
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
		t.Run(provider, func(t *testing.T) { testMaterialCompositeQueueRuntimeObservation(t, provider, false) })
	}
}

func TestMaterialRepositoryMaintenanceUpgrade(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) { testMaterialCompositeQueueRuntimeObservation(t, provider, true) })
	}
}
func testMaterialCompositeQueueRuntimeObservation(t *testing.T, provider string, maintenance bool) {
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
		r.Post("/api/projects/{id}/materials/initialization/operations/{operationID}/permit", h.PermitMaterialOperation)
	})
	var negativeReceipts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var loseNegative bool
		if strings.HasSuffix(r.URL.Path, "/material-observations") {
			raw, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(raw))
			if bytes.Contains(raw, []byte(`"outcome":"rules_changed"`)) {
				loseNegative = negativeReceipts.Add(1) == 1
			}
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		if loseNegative && rec.Code == 200 {
			w.WriteHeader(503)
			return
		}
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
	if maintenance {
		repository, err := q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{WorkspaceID: util.MustParseUUID(ws), ProjectID: util.MustParseUUID(project)})
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := materialrepo.BuildCatalogVersion(materialrepo.Binding{SystemID: util.UUIDToString(repository.ID), RootPath: "engineering"}, "1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		tree := map[string]string{"README.md": "Keep user documentation\n", "uat/history.bin": string([]byte{0, 42})}
		for _, f := range catalog.Files {
			tree[f.Path] = f.Content
		}
		tree["engineering/README.md"] = "Keep material maintainer documentation\n"
		seed := materialRemoteFixture{BaseCommit: strings.Repeat("c", 40), Trees: map[string]map[string]string{strings.Repeat("c", 40): tree}}
		raw, _ := json.Marshal(seed)
		if os.WriteFile(statePath, raw, 0600) != nil {
			t.Fatal("seed old supported catalog")
		}
	}
	var logs bytes.Buffer
	d := New(Config{ServerBaseURL: server.URL, WorkspacesRoot: filepath.Join(root, "workspaces"), DaemonID: daemonID, AgentTimeout: 2 * time.Minute, Agents: map[string]AgentEntry{"claude": {Path: native}}}, slog.New(slog.NewTextHandler(&logs, nil)))
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
	run("inspect_repository")
	if maintenance {
		initial, err := svc.Initialization(ctx, access)
		if err != nil || !initial.Readiness.PublishAllowed || initial.Readiness.BundleVersion == nil || *initial.Readiness.BundleVersion != "1.0.0" {
			t.Fatalf("prior supported bundle not ready: %+v %v", initial, err)
		}
		if data, _ := os.ReadFile(statePath + ".runs"); len(data) != 0 {
			t.Fatal("compatible existing init ran model")
		}
		_, err = svc.InitializationCommand(ctx, access, "maintenance", material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: initial.Version, ExpectedSettingsVersion: initial.SettingsVersion, TargetBundleVersion: materialrepo.BundleVersion})
		if err != nil {
			t.Fatal(err)
		}
		run("inspect_repository")
		initial, err = svc.Initialization(ctx, access)
		if err != nil || !initial.Readiness.PublishAllowed {
			t.Fatal("proposal revoked compatible prior proof")
		}
	}
	run("publish")
	initialization, err := svc.Initialization(ctx, access)
	if err != nil || initialization.Candidate == nil {
		t.Fatalf("initialization candidate unavailable: %v %+v", err, initialization)
	}
	candidate := initialization.Candidate
	if maintenance && (candidate.Kind != "maintenance" || len(candidate.Files) != 2) {
		t.Fatalf("wrong maintenance diff: %+v", candidate)
	}
	initialization, err = svc.InitializationDecide(ctx, access, material.InitializationDecisionInput{RequestID: uuid.NewString(), ExpectedVersion: initialization.Version, ExpectedSettingsVersion: initialization.SettingsVersion, CandidateID: candidate.ID, HeadSHA: *candidate.HeadSHA, ManifestSHA256: candidate.ManifestSHA256, Decision: "approve", Reason: "Maintain complete catalog"})
	if err != nil {
		t.Fatal(err)
	}
	run("review_sync")
	run("merge")
	initialization, err = svc.Initialization(ctx, access)
	if err != nil || !initialization.Readiness.PublishAllowed {
		t.Fatalf("repository not ready: %v %+v", err, initialization)
	}
	if maintenance {
		if len(initialization.History) == 0 || initialization.History[len(initialization.History)-1].BundleVersion == nil || *initialization.History[len(initialization.History)-1].BundleVersion != "1.0.0" {
			t.Fatal("prior accepted proof history lost")
		}
		if initialization.Readiness.BundleVersion == nil || *initialization.Readiness.BundleVersion != materialrepo.BundleVersion {
			t.Fatal("upgrade proof version wrong")
		}
		var state materialRemoteFixture
		raw, _ := os.ReadFile(statePath)
		if json.Unmarshal(raw, &state) != nil {
			t.Fatal("fixture state")
		}
		tree := state.Trees[state.BaseCommit]
		if tree["README.md"] != "Keep user documentation\n" || tree["engineering/README.md"] != "Keep material maintainer documentation\n" || tree["uat/history.bin"] != string([]byte{0, 42}) {
			t.Fatal("maintenance changed preserved bytes")
		}
		counts, _ := os.ReadFile(statePath + ".runs")
		if string(counts) != "publish\nreview_sync\nmerge\n" {
			t.Fatalf("duplicate maintenance model: %q", counts)
		}
		_, err = svc.InitializationCommand(ctx, access, "maintenance", material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: initialization.Version, ExpectedSettingsVersion: initialization.SettingsVersion, TargetBundleVersion: materialrepo.BundleVersion})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			_, _ = svc.ProcessNext(ctx)
		}
		if task, err := d.client.ClaimTask(ctx, rid); err != nil || task != nil {
			t.Fatal("same version maintenance queued work")
		}
		t.Log("supported 1.0.0→1.0.1 native maintenance preserved old proof until approved merge, all unrelated bytes and zero duplicate tasks")
		return
	}
	beforeNoop, _ := os.ReadFile(statePath + ".runs")
	for _, action := range []string{"check", "maintenance"} {
		initialization, err = svc.InitializationCommand(ctx, access, action, material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: initialization.Version, ExpectedSettingsVersion: initialization.SettingsVersion, TargetBundleVersion: materialrepo.BundleVersion})
		if err != nil {
			t.Fatal(err)
		}
		if action == "check" {
			run("inspect_repository")
		}
		initialization, err = svc.Initialization(ctx, access)
		if err != nil || !initialization.Readiness.PublishAllowed {
			t.Fatalf("compatible no-op lost readiness: %v", err)
		}
	}
	afterNoop, _ := os.ReadFile(statePath + ".runs")
	if !bytes.Equal(beforeNoop, afterNoop) {
		t.Fatal("read-only/no-op inspection launched a model")
	}
	detail, err := svc.Create(ctx, access, material.CreateInput{RequestID: uuid.NewString(), Title: "Deterministic runtime material"})
	if err != nil {
		t.Fatal(err)
	}
	detail, err = svc.Revise(ctx, access, detail.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version, Kind: "requirement", Content: strings.Repeat("The runtime writes and the daemon independently verifies bytes.\n", 3), Sources: []material.Source{{ResourceID: resource, Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: Runtime receipts bind current claims"}, Ready: true})
	if err != nil {
		t.Fatal(err)
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
	if detail.Baseline == nil || detail.Baseline.Commit == "" {
		t.Fatalf("missing verified baseline: %+v", detail)
	}
	for i := 0; i < 3; i++ {
		_, _ = svc.ProcessNext(ctx)
	}
	runs, _ := os.ReadFile(statePath + ".runs")
	if string(runs) != "publish\nreview_sync\nmerge\npublish\nreview_sync\nmerge\n" {
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
	if string(runs) != "publish\nreview_sync\nmerge\npublish\nreview_sync\nmerge\nverify_baseline\nverify_baseline\n" {
		t.Fatalf("missing/duplicate runtime calls: %q", runs)
	}
	var retained int
	if err := pool.QueryRow(ctx, "select count(*) from agent_task_queue where issue_id=$1 and status='completed'", issueID).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("prior plan run lost: %d %v", retained, err)
	}
	// A new current-base rule edit must revoke readiness and refuse native launch.
	var state materialRemoteFixture
	rawState, _ := os.ReadFile(statePath)
	if json.Unmarshal(rawState, &state) != nil {
		t.Fatal("invalid fixture")
	}
	driftCommit := material.Digest("external rule drift")
	driftTree := map[string]string{}
	for k, v := range state.Trees[state.BaseCommit] {
		driftTree[k] = v
	}
	driftTree["engineering/AGENTS.md"] = "unreviewed rules"
	state.Trees[driftCommit] = driftTree
	state.BaseCommit = driftCommit
	rawState, _ = json.Marshal(state)
	if os.WriteFile(statePath, rawState, 0600) != nil {
		t.Fatal("cannot change remote fixture")
	}
	next, err := svc.Create(ctx, access, material.CreateInput{RequestID: uuid.NewString(), Title: "Drift refusal"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Revise(ctx, access, next.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: next.Material.Version, Kind: "requirement", Content: strings.Repeat("Candidate before drift discovered.\n", 3), Sources: []material.Source{{ResourceID: resource, Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: drift prevents launch"}, Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE git_material_job SET next_at=now() WHERE workspace_id=$1", ws)
	for i := 0; i < 5; i++ {
		_, _ = svc.ProcessNext(ctx)
	}
	refused, err := d.client.ClaimTask(ctx, rid)
	if err != nil || refused == nil {
		t.Fatalf("drift task claim %v", err)
	}
	if _, err = d.runTask(ctx, *refused, "claude", 0, d.logger); err == nil {
		t.Fatal("drift launched native runtime")
	}
	d.replayMaterialObservations(ctx)
	if negativeReceipts.Load() != 2 {
		t.Fatalf("negative receipt did not replay exactly: %d", negativeReceipts.Load())
	}
	entries, _ := os.ReadDir(d.materialReceiptDirectory())
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			t.Fatal("acknowledged negative receipt retained")
		}
	}
	initialization, err = svc.Initialization(ctx, access)
	if err != nil || initialization.Readiness.PublishAllowed {
		t.Fatal("drift did not revoke ready proof")
	}
	finalRuns, _ := os.ReadFile(statePath + ".runs")
	if !bytes.Equal(finalRuns, runs) {
		t.Fatal("drift started another native process")
	}
	t.Log("native initialization, no-op checks, ordinary material/manual retry, and drift refusal passed for this provider")
}
