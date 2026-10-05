package handler

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func progressEndpoint(name string) http.HandlerFunc {
	api := testHandler
	switch name {
	case "preview":
		return api.PreviewProjectUpdate
	case "create":
		return api.CreateProjectUpdate
	case "correct":
		return api.CorrectProjectUpdate
	case "history":
		return api.ListProjectUpdateRevisions
	default:
		return api.ListProjectUpdates
	}
}
func progressProject(t *testing.T) string {
	t.Helper()
	id := dbfx.Project(t, "Progress contract", testutil.Cols{"description": "Original target"})
	for _, table := range []string{"project_update", "project_update_revision", "project_update_request", "project_update_notification"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE project_id=$1", id)
	}
	return id
}
func progressDraft(body string) map[string]any {
	return map[string]any{"operation": "create", "update_id": nil, "expected_revision": nil, "kind": "progress", "body": body, "health_judgment": nil, "evidence": []any{}, "acceptance": nil, "expected_description_revision": nil, "include_statistics": false, "correction_reason": nil}
}
func progressCall(t *testing.T, name, project, update string, body any, status int) map[string]any {
	t.Helper()
	method := "POST"
	if name == "correct" {
		method = "PUT"
	}
	if name == "list" || name == "history" {
		method = "GET"
	}
	req := withURLParam(newRequest(method, "/api/projects/"+project+"/updates", body), "id", project)
	if update != "" {
		chi.RouteContext(req.Context()).URLParams.Add("updateId", update)
	}
	var out map[string]any
	testutil.Call(t, progressEndpoint(name), req).Want(status).JSON(&out)
	return out
}
func progressPreview(t *testing.T, project string, draft map[string]any) map[string]any {
	return progressCall(t, "preview", project, "", draft, 200)
}
func progressInput(preview map[string]any) map[string]any {
	return map[string]any{"request_id": uuid.NewString(), "draft": preview["draft"], "preview_hash": preview["preview_hash"], "evidence_versions": preview["evidence_versions"]}
}
func TestProjectUpdateCreateReplayAndImmutableCorrection(t *testing.T) {
	p := progressProject(t)
	draft := progressDraft("  本周完成登录流程\r\n保留说明  ")
	preview := progressPreview(t, p, draft)
	if preview["workspace_id"] != testWorkspaceID || preview["project_id"] != p || preview["statistics_snapshot"] != nil {
		t.Fatalf("preview identity/snapshot: %v", preview)
	}
	input := progressInput(preview)
	first := progressCall(t, "create", p, "", input, 201)
	id := first["update_id"].(string)
	if first["result_revision"] != float64(1) || first["replayed"] != false {
		t.Fatalf("created: %v", first)
	}
	replay := progressCall(t, "create", p, "", input, 200)
	if replay["update_id"] != id || replay["result_revision"] != float64(1) || replay["replayed"] != true {
		t.Fatalf("replay: %v", replay)
	}
	correct := progressDraft("更正正文")
	correct["operation"] = "correct"
	correct["update_id"] = id
	correct["expected_revision"] = 1
	correct["correction_reason"] = "补全细节"
	correction := progressInput(progressPreview(t, p, correct))
	out := progressCall(t, "correct", p, id, correction, 200)
	if out["result_revision"] != float64(2) {
		t.Fatalf("corrected: %v", out)
	}
	replay = progressCall(t, "create", p, "", input, 200)
	if replay["result_revision"] != float64(1) || replay["result"].(map[string]any)["body"] != "本周完成登录流程\n保留说明" {
		t.Fatalf("replay changed historical result: %v", replay)
	}
	history := progressCall(t, "history", p, id, nil, 200)["items"].([]any)
	if len(history) != 2 || history[1].(map[string]any)["revision"] != float64(1) {
		t.Fatalf("history: %v", history)
	}
	input["preview_hash"] = "different"
	conflict := progressCall(t, "create", p, "", input, 409)
	if conflict["code"] != "idempotency_conflict" {
		t.Fatalf("payload conflict: %v", conflict)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 1 {
		t.Fatalf("updates=%d", n)
	}
}
func TestProjectUpdateAcceptanceKeepsOriginalDescription(t *testing.T) {
	p := progressProject(t)
	draft := progressDraft("验收通过")
	draft["kind"] = "acceptance"
	draft["acceptance"] = map[string]any{"conclusion": "passed", "scope": "登录", "explanation": "已完成端到端复核"}
	draft["expected_description_revision"] = 1
	first := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, draft)), 201)
	id := first["update_id"].(string)
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+p, map[string]any{"description": "New target", "expected_description_revision": 1}), "id", p)).Want(200)
	draft["operation"] = "correct"
	draft["update_id"] = id
	draft["expected_revision"] = 1
	draft["correction_reason"] = "修正旧版验收说明"
	out := progressCall(t, "correct", p, id, progressInput(progressPreview(t, p, draft)), 200)
	a := out["result"].(map[string]any)["acceptance"].(map[string]any)
	if a["description_revision"] != float64(1) || a["description_snapshot"] != "Original target" {
		t.Fatalf("old acceptance rebound: %v", a)
	}
}
func TestProjectUpdateValidationAndHumanGate(t *testing.T) {
	p := progressProject(t)
	for _, body := range []string{" ", strings.Repeat("😀", 10001)} {
		progressCall(t, "preview", p, "", progressDraft(body), 422)
	}
	progressPreview(t, p, progressDraft(strings.Repeat("😀", 10000)))
	for _, conclusion := range []string{"passed", "partial"} {
		d := progressDraft("验收")
		d["kind"] = "acceptance"
		d["expected_description_revision"] = 1
		d["acceptance"] = map[string]any{"conclusion": conclusion, "scope": "目标", "explanation": " "}
		progressCall(t, "preview", p, "", d, 422)
	}
	d := progressDraft("链接")
	d["evidence"] = []any{map[string]any{"kind": "url", "id": nil, "url": "javascript:alert(1)"}}
	progressCall(t, "preview", p, "", d, 422)
	for _, source := range []string{"task_token", "cloud_pat"} {
		req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", progressDraft("agent")), "id", p)
		req.Header.Set("X-Actor-Source", source)
		testutil.Call(t, progressEndpoint("preview"), req).Want(403)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 0 {
		t.Fatalf("validation wrote %d updates", n)
	}
}
func TestProjectUpdateEvidenceChangeRequiresFreshPreview(t *testing.T) {
	p := progressProject(t)
	issue := dbfx.Issue(t, "Evidence", testutil.Cols{"project_id": p})
	d := progressDraft("证据确认")
	d["evidence"] = []any{map[string]any{"kind": "issue", "id": issue, "url": nil}}
	input := progressInput(progressPreview(t, p, d))
	dbfx.Exec(t, "UPDATE issue SET revision=revision+1,title='Changed' WHERE id=$1", issue)
	out := progressCall(t, "create", p, "", input, 409)
	if out["code"] != "project_update_preview_stale" {
		t.Fatalf("stale: %v", out)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 0 {
		t.Fatalf("stale wrote %d", n)
	}
}

func TestProjectUpdateStatisticsSnapshotIsServerGeneratedAndImmutable(t *testing.T) {
	p := progressProject(t)
	issue := dbfx.Issue(t, "Snapshot", testutil.Cols{"project_id": p, "status": "todo"})
	d := progressDraft("附统计进展")
	d["include_statistics"] = true
	preview := progressPreview(t, p, d)
	input := progressInput(preview)
	dbfx.Exec(t, "UPDATE issue SET status='done',revision=revision+1 WHERE id=$1", issue)
	stale := progressCall(t, "create", p, "", input, 409)
	if stale["code"] != "project_update_preview_stale" {
		t.Fatalf("statistics stale: %v", stale)
	}
	preview = progressPreview(t, p, d)
	out := progressCall(t, "create", p, "", progressInput(preview), 201)
	id := out["update_id"].(string)
	snapshot := out["result"].(map[string]any)["statistics_snapshot"].(map[string]any)
	if snapshot["complete"] != true || snapshot["counts"].(map[string]any)["completed"] != float64(1) {
		t.Fatalf("snapshot=%v", snapshot)
	}
	if _, ok := snapshot["latest_acceptance"]; ok {
		t.Fatal("recursive acceptance in statistics")
	}
	dbfx.Exec(t, "UPDATE issue SET status='todo',revision=revision+1 WHERE id=$1", issue)
	correction := progressDraft("保持旧统计")
	correction["operation"] = "correct"
	correction["update_id"] = id
	correction["expected_revision"] = 1
	correction["correction_reason"] = "仅更正文案"
	updated := progressCall(t, "correct", p, id, progressInput(progressPreview(t, p, correction)), 200)
	kept := updated["result"].(map[string]any)["statistics_snapshot"].(map[string]any)
	if kept["snapshot_version"] != snapshot["snapshot_version"] || kept["calculated_at"] != snapshot["calculated_at"] {
		t.Fatalf("stored snapshot recomputed: %v", kept)
	}
	plain := progressInput(progressPreview(t, p, progressDraft("不附统计")))
	dbfx.Exec(t, "UPDATE issue SET status='done',revision=revision+1 WHERE id=$1", issue)
	progressCall(t, "create", p, "", plain, 201)
}

func TestProjectUpdateCanonicalFixtureAndNormalization(t *testing.T) {
	raw, e := os.ReadFile("../../../.trellis/tasks/10-05-projects-p1-progress/canonical-fixture.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		Input  any    `json:"input"`
		SHA256 string `json:"sha256"`
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	hash, e := projectDigest(fixture.Input)
	if e != nil || hash != fixture.SHA256 {
		t.Fatalf("hash=%s want=%s err=%v", hash, fixture.SHA256, e)
	}
	p := progressProject(t)
	a := progressPreview(t, p, progressDraft("  line\r\n😀  "))
	b := progressPreview(t, p, progressDraft("line\n😀"))
	if a["preview_hash"] != b["preview_hash"] {
		t.Fatal("normalization or collection time changed hash")
	}
	url := "https://example.invalid/evidence?x=1&y=2#section"
	d := progressDraft("URL only")
	d["evidence"] = []any{map[string]any{"kind": "url", "id": nil, "url": url}, map[string]any{"kind": "url", "id": nil, "url": url}}
	v := progressPreview(t, p, d)
	if len(v["evidence_versions"].([]any)) != 1 || v["evidence_versions"].([]any)[0].(map[string]any)["verification"] != "unverified" {
		t.Fatalf("URL dedup/version: %v", v)
	}
}

func TestProjectUpdateExecutionEvidenceRechecksResultAndPrivacy(t *testing.T) {
	p := progressProject(t)
	runtime := dbfx.Runtime(t, "Evidence runtime")
	agent := dbfx.Agent(t, "Private evidence", runtime, testutil.Cols{"permission_mode": "private"})
	task := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime, "result": "{\"output\":\"original\"}"})
	d := progressDraft("执行复核")
	d["evidence"] = []any{map[string]any{"kind": "execution", "id": task, "url": nil}}
	input := progressInput(progressPreview(t, p, d))
	dbfx.Exec(t, "UPDATE agent_task_queue SET result='{}'::jsonb WHERE id=$1", task)
	stale := progressCall(t, "create", p, "", input, 409)
	if stale["code"] != "project_update_preview_stale" {
		t.Fatalf("result digest ignored: %v", stale)
	}
	first := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, d)), 201)
	reader := dbfx.User(t, "Evidence reader", "progress-evidence-reader@example.invalid")
	dbfx.Member(t, testWorkspaceID, reader, "member")
	req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", d), "id", p)
	req.Header.Set("X-User-ID", reader)
	denied := testutil.Call(t, progressEndpoint("preview"), req).Want(403).Map()
	if denied["code"] != "project_evidence_forbidden" {
		t.Fatalf("source permission must not revoke workspace: %v", denied)
	}
	req = withURLParam(newRequest("GET", "/api/projects/"+p+"/updates", nil), "id", p)
	req.Header.Set("X-User-ID", reader)
	var list map[string]any
	testutil.Call(t, progressEndpoint("list"), req).Want(200).JSON(&list)
	evidence := list["items"].([]any)[0].(map[string]any)["current"].(map[string]any)["evidence"].([]any)[0].(map[string]any)
	if evidence["availability"] != "inaccessible" || evidence["label"] != nil || evidence["href"] != nil || evidence["current_version"] != nil {
		t.Fatalf("private historical evidence leaked: %v", evidence)
	}
	if first["result"].(map[string]any)["evidence"].([]any)[0].(map[string]any)["availability"] != "available" {
		t.Fatal("owner cannot read own evidence")
	}
	chat := dbfx.ChatSession(t, agent, testutil.Cols{"creator_id": reader})
	chatTask := dbfx.Task(t, agent, testutil.Cols{"chat_session_id": chat, "runtime_id": runtime})
	d["evidence"] = []any{map[string]any{"kind": "execution", "id": chatTask, "url": nil}}
	if denied := progressCall(t, "preview", p, "", d, 403); denied["code"] != "project_evidence_forbidden" {
		t.Fatalf("chat source permission code=%v", denied)
	}
}

func TestProjectUpdateFlagOffRetainsHistoryAndExactReplay(t *testing.T) {
	p := progressProject(t)
	input := progressInput(progressPreview(t, p, progressDraft("Before rollback")))
	first := progressCall(t, "create", p, "", input, 201)
	withFeatureFlag(t, testHandler, featureflags.ProjectsP1, false)
	if denied := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, progressDraft("Read-only blocked"))), 403); denied["code"] != "project_updates_disabled" {
		t.Fatalf("disabled feature must not revoke workspace: %v", denied)
	}
	replayed := progressCall(t, "create", p, "", input, 200)
	if replayed["update_id"] != first["update_id"] || replayed["replayed"] != true {
		t.Fatalf("read-only replay: %v", replayed)
	}
	progressCall(t, "list", p, "", nil, 200)
	d := progressDraft("Read-only correction")
	d["operation"] = "correct"
	d["update_id"] = first["update_id"]
	d["expected_revision"] = 1
	d["correction_reason"] = "blocked"
	if denied := progressCall(t, "correct", p, first["update_id"].(string), progressInput(progressPreview(t, p, d)), 403); denied["code"] != "project_updates_disabled" {
		t.Fatalf("disabled correction code=%v", denied)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update_revision WHERE project_id=$1", p); n != 1 {
		t.Fatalf("read-only wrote %d revisions", n)
	}
}

func TestProjectUpdateConcurrentRequestsAndCorrections(t *testing.T) {
	p := progressProject(t)
	input := progressInput(progressPreview(t, p, progressDraft("Concurrent intent")))
	type result struct {
		status int
		out    map[string]any
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	gate := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", input), "id", p)
			response := testutil.Call(t, progressEndpoint("create"), req)
			var out map[string]any
			if e := json.NewDecoder(response.Body).Decode(&out); e != nil {
				t.Error(e)
			}
			results <- result{response.Code, out}
		}()
	}
	close(gate)
	wg.Wait()
	close(results)
	statuses := map[int]int{}
	var id string
	for r := range results {
		statuses[r.status]++
		if got, _ := r.out["update_id"].(string); id != "" && got != id {
			t.Fatalf("duplicate identities: %s/%s", id, got)
		} else {
			id = got
		}
	}
	if statuses[201] != 1 || statuses[200] != 1 {
		t.Fatalf("concurrent create statuses=%v", statuses)
	}
	d := progressDraft("Concurrent correction")
	d["operation"] = "correct"
	d["update_id"] = id
	d["expected_revision"] = 1
	d["correction_reason"] = "one winner"
	preview := progressPreview(t, p, d)
	one, two := progressInput(preview), progressInput(preview)
	corrections := make(chan result, 2)
	correctionGate := make(chan struct{})
	for _, body := range []map[string]any{one, two} {
		wg.Add(1)
		go func(body map[string]any) {
			defer wg.Done()
			<-correctionGate
			req := testutil.WithURLParams(newRequest("PUT", "/api/projects/"+p+"/updates/"+id, body), "id", p, "updateId", id)
			response := testutil.Call(t, progressEndpoint("correct"), req)
			corrections <- result{response.Code, response.Map()}
		}(body)
	}
	close(correctionGate)
	wg.Wait()
	close(corrections)
	statuses = map[int]int{}
	for r := range corrections {
		statuses[r.status]++
		if r.status == 409 && r.out["code"] != "project_update_revision_conflict" {
			t.Fatalf("correction conflict=%v", r.out)
		}
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatalf("concurrent correction statuses=%v", statuses)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update_revision WHERE project_id=$1", p); n != 2 {
		t.Fatalf("concurrent revisions=%d", n)
	}
}

func TestProjectUpdateConcurrentRequestIdentityAcrossProjects(t *testing.T) {
	p1, p2 := progressProject(t), progressProject(t)
	one := progressInput(progressPreview(t, p1, progressDraft("first intent")))
	two := progressInput(progressPreview(t, p2, progressDraft("different intent")))
	two["request_id"] = one["request_id"]
	gate := make(chan struct{})
	results := make(chan int, 2)
	for i, p := range []string{p1, p2} {
		body := []map[string]any{one, two}[i]
		go func(p string, body map[string]any) {
			<-gate
			req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", body), "id", p)
			response := testutil.Call(t, progressEndpoint("create"), req)
			if response.Code == 409 && response.Map()["code"] != "idempotency_conflict" {
				t.Errorf("unexpected request conflict: %s", response.Text())
			}
			results <- response.Code
		}(p, body)
	}
	close(gate)
	statuses := map[int]int{}
	for i := 0; i < 2; i++ {
		select {
		case code := <-results:
			statuses[code]++
		case <-time.After(3 * time.Second):
			t.Fatal("cross-project request competition stalled")
		}
	}
	if statuses[201] != 1 || statuses[409] != 1 {
		t.Fatalf("request competition statuses=%v", statuses)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id IN ($1,$2)", p1, p2); n != 1 {
		t.Fatalf("different intent committed %d updates", n)
	}
}

func TestProjectUpdateHistoryIdentityAndCursorScope(t *testing.T) {
	p := progressProject(t)
	other := progressProject(t)
	one := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, progressDraft("First"))), 201)
	progressCall(t, "create", p, "", progressInput(progressPreview(t, p, progressDraft("Second"))), 201)
	req := withURLParam(newRequest("GET", "/api/projects/"+p+"/updates?limit=1", nil), "id", p)
	var page map[string]any
	testutil.Call(t, progressEndpoint("list"), req).Want(200).JSON(&page)
	cursor := page["next_cursor"].(string)
	req = withURLParam(newRequest("GET", "/api/projects/"+p+"/updates?limit=1&cursor="+cursor, nil), "id", p)
	testutil.Call(t, progressEndpoint("list"), req).Want(200).JSON(&page)
	if page["items"].([]any)[0].(map[string]any)["id"] != one["update_id"] || page["next_cursor"] != nil {
		t.Fatalf("timeline page=%v", page)
	}
	req = withURLParam(newRequest("GET", "/api/projects/"+other+"/updates?cursor="+cursor, nil), "id", other)
	testutil.Call(t, progressEndpoint("list"), req).Want(400)
	departed := dbfx.User(t, "Departed author", "progress-departed@example.invalid")
	missing := uuid.NewString()
	dbfx.Exec(t, "UPDATE project_update SET author_user_id=$1 WHERE id=$2", departed, one["update_id"])
	dbfx.Exec(t, "UPDATE project_update_revision SET editor_user_id=$1 WHERE update_id=$2", missing, one["update_id"])
	items := progressCall(t, "list", p, "", nil, 200)["items"].([]any)
	old := items[1].(map[string]any)
	author := old["author"].(map[string]any)
	editor := old["current"].(map[string]any)["editor"].(map[string]any)
	if author["availability"] != "departed" || author["name"] != "Departed author" || editor["availability"] != "deleted" || editor["name"] != nil {
		t.Fatalf("history attribution: %v", old)
	}
	if _, ok := author["email"]; ok {
		t.Fatal("email exposed in historical identity")
	}
}

func TestProjectUpdateSourceLockRetriesInsteadOfReadingStaleVersion(t *testing.T) {
	p := progressProject(t)
	issue := dbfx.Issue(t, "Locked evidence", testutil.Cols{"project_id": p})
	d := progressDraft("Source lock")
	d["evidence"] = []any{map[string]any{"kind": "issue", "id": issue, "url": nil}}
	input := progressInput(progressPreview(t, p, d))
	ctx := context.Background()
	tx, e := testPool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "UPDATE issue SET revision=revision+1 WHERE id=$1", issue); e != nil {
		t.Fatal(e)
	}
	done := make(chan int, 1)
	entered := make(chan struct{})
	var once sync.Once
	h := *testHandler
	h.TxStarter = progressTxStarter{base: testHandler.TxStarter, wrap: func(tx pgx.Tx) pgx.Tx {
		return progressHookTx{Tx: tx, afterQuery: func(sql string) {
			if strings.Contains(sql, "-- name: LockProjectUpdateEvidenceIssue") {
				once.Do(func() { close(entered) })
			}
		}}
	}}
	go func() {
		req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", input), "id", p)
		done <- testutil.Call(t, h.CreateProjectUpdate, req).Code
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("evidence lock barrier not reached")
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case status := <-done:
		if status != 409 {
			t.Fatalf("stale evidence status=%d", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("source retry stalled")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE project_id=$1", p); n != 0 {
		t.Fatalf("stale source wrote %d updates", n)
	}
}

func TestProjectUpdateAccessRevocationAndCrossWorkspace(t *testing.T) {
	p := progressProject(t)
	input := progressInput(progressPreview(t, p, progressDraft("Access")))
	progressCall(t, "create", p, "", input, 201)
	outsider := dbfx.User(t, "Outsider", "progress-outsider@example.invalid")
	req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", input), "id", p)
	req.Header.Set("X-User-ID", outsider)
	testutil.Call(t, progressEndpoint("create"), req).Want(403)
	other := dbfx.Workspace(t, "Other progress workspace", "progress-other-"+uuid.NewString())
	req = withURLParam(newRequest("GET", "/api/projects/"+p+"/updates", nil), "id", p)
	req.Header.Set("X-Workspace-ID", other)
	testutil.Call(t, progressEndpoint("list"), req).Want(403)
	deleted := progressProject(t)
	old := progressInput(progressPreview(t, deleted, progressDraft("Deleted")))
	progressCall(t, "create", deleted, "", old, 201)
	testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+deleted, nil), "id", deleted)).Want(204)
	progressCall(t, "create", deleted, "", old, 404)
}

func TestProjectUpdateCharacterEvidenceAndMalformedInputBoundaries(t *testing.T) {
	p := progressProject(t)
	for _, n := range []int{1, 1000, 1001} {
		reason := strings.Repeat("中", n)
		d := ProjectUpdateDraft{Operation: "correct", UpdateID: new(string), ExpectedRevision: new(int64), Kind: "progress", Body: "x", CorrectionReason: &reason}
		*d.UpdateID = uuid.NewString()
		*d.ExpectedRevision = 1
		_, e := normalizeProjectUpdateDraft(d)
		if (n > 1000) != (e != nil) {
			t.Fatalf("reason length %d: %v", n, e)
		}
	}
	for _, n := range []int{50, 51} {
		d := progressDraft("Evidence bounds")
		refs := make([]any, n)
		for i := range refs {
			refs[i] = map[string]any{"kind": "issue", "id": uuid.NewString(), "url": nil}
		}
		d["evidence"] = refs
		raw, _ := json.Marshal(d)
		var typed ProjectUpdateDraft
		if e := json.Unmarshal(raw, &typed); e != nil {
			t.Fatal(e)
		}
		_, e := normalizeProjectUpdateDraft(typed)
		if (n > 50) != (e != nil) {
			t.Fatalf("evidence limit %d: %v", n, e)
		}
	}
	d := progressDraft("Malformed protocol")
	d["recipient_ids"] = []string{testUserID}
	progressCall(t, "preview", p, "", d, 400)
	d = progressDraft("No forged statistics")
	d["statistics_snapshot"] = map[string]any{"complete": true}
	progressCall(t, "preview", p, "", d, 400)
	for _, body := range []string{`{"body":123}`, `{"operation":"create"} {"operation":"create"}`, `null`} {
		req := testutil.WithURLParams(testutil.WithHeaders(testutil.JSONRequest("POST", "/api/projects/"+p+"/updates/preview", body), "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID), "id", p)
		testutil.Call(t, progressEndpoint("preview"), req).WantOneOf(400, 422)
	}
}

func TestProjectUpdateLegacyAgentRejectedAndEventsContainOnlyIdentity(t *testing.T) {
	p := progressProject(t)
	runtime := dbfx.Runtime(t, "Legacy runtime")
	agent := dbfx.Agent(t, "Legacy actor", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{"status": "running", "runtime_id": runtime, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", progressDraft("Legacy forbidden")), "id", p)
	req.Header.Set("X-Agent-ID", agent)
	req.Header.Set("X-Task-ID", task)
	testutil.Call(t, progressEndpoint("preview"), req).Want(403)
	d := progressDraft("Secret body [@Agent](mention://agent/" + agent + ")")
	input := progressInput(progressPreview(t, p, d))
	h := *testHandler
	h.Bus = events.New()
	captured := []events.Event{}
	h.Bus.SubscribeAll(func(event events.Event) { captured = append(captured, event) })
	req = withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", input), "id", p)
	testutil.Call(t, h.CreateProjectUpdate, req).Want(201)
	if len(captured) != 1 || captured[0].Type != protocol.EventProjectUpdatePublished {
		t.Fatalf("unexpected execution/comment events: %+v", captured)
	}
	payload := captured[0].Payload.(map[string]any)
	if len(payload) != 4 || payload["project_id"] != p || payload["workspace_id"] != testWorkspaceID || payload["revision"] != int64(1) {
		t.Fatalf("identity event=%v", payload)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", agent); n != 1 {
		t.Fatalf("mention created executions: %d", n)
	}
}

func TestProjectUpdateHumanMembersKeepActualAuthorship(t *testing.T) {
	for _, role := range []string{"member", "admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			p := progressProject(t)
			user := dbfx.User(t, "Author "+role, "progress-author-"+uuid.NewString()+"@example.invalid")
			dbfx.Member(t, testWorkspaceID, user, role)
			req := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", progressDraft("Real author")), "id", p)
			req.Header.Set("X-User-ID", user)
			preview := testutil.Call(t, testHandler.PreviewProjectUpdate, req).Want(200).Map()
			req = withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", progressInput(preview)), "id", p)
			req.Header.Set("X-User-ID", user)
			out := testutil.Call(t, testHandler.CreateProjectUpdate, req).Want(201).Map()
			if out["author"].(map[string]any)["id"] != user || out["result"].(map[string]any)["editor"].(map[string]any)["id"] != user {
				t.Fatalf("authorship=%v", out)
			}
		})
	}
}

func TestProjectUpdateInternalEvidenceUsesTypedTargets(t *testing.T) {
	p := progressProject(t)
	issue := dbfx.Issue(t, "Typed issue", testutil.Cols{"project_id": p})
	runtime := dbfx.Runtime(t, "Typed execution")
	agent := dbfx.Agent(t, "Evidence agent", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime})
	url := "https://example.invalid/evidence"
	d := progressDraft("Typed references")
	d["evidence"] = []any{map[string]any{"kind": "issue", "id": issue, "url": nil}, map[string]any{"kind": "execution", "id": task, "url": nil}, map[string]any{"kind": "url", "id": nil, "url": url}}
	out := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, d)), 201)
	evidence := out["result"].(map[string]any)["evidence"].([]any)
	for _, raw := range evidence {
		view := raw.(map[string]any)
		input := view["input"].(map[string]any)
		if input["kind"] == "url" {
			if view["href"] != url {
				t.Fatalf("external URL lost: %v", view)
			}
		} else if view["href"] != nil {
			t.Fatalf("internal reference must use typed workspace navigation, not fabricated href: %v", view)
		}
	}
}

func projectExecutionEvidenceHandler() http.HandlerFunc {
	return testHandler.GetProjectUpdateExecutionEvidence
}
func TestProjectUpdateExecutionEvidenceOpensRealAuthorizedTranscript(t *testing.T) {
	p := progressProject(t)
	runtime := dbfx.Runtime(t, "Transcript runtime")
	agent := dbfx.Agent(t, "Transcript agent", runtime, testutil.Cols{"permission_mode": "private"})
	task := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime, "result": `{"output":"real result"}`})
	dbfx.Insert(t, "task_message", testutil.Cols{"task_id": task, "seq": 1, "type": "text", "content": "real transcript"})
	d := progressDraft("Review execution")
	d["evidence"] = []any{map[string]any{"kind": "execution", "id": task, "url": nil}}
	published := progressCall(t, "create", p, "", progressInput(progressPreview(t, p, d)), 201)
	update := published["update_id"].(string)
	request := func(taskID, user string) *http.Request {
		req := testutil.WithURLParams(newRequest("GET", "/api/projects/"+p+"/updates/"+update+"/revisions/1/executions/"+taskID, nil), "id", p, "updateId", update, "revision", "1", "taskId", taskID)
		req.Header.Set("X-User-ID", user)
		return req
	}
	out := testutil.Call(t, projectExecutionEvidenceHandler(), request(task, testUserID)).Want(200).Map()
	if out["workspace_id"] != testWorkspaceID || out["project_id"] != p || out["update_id"] != update || out["revision"] != float64(1) {
		t.Fatalf("evidence identity: %v", out)
	}
	actualTask := out["task"].(map[string]any)
	messages := out["messages"].([]any)
	if actualTask["id"] != task || actualTask["status"] != "completed" || len(messages) != 1 || messages[0].(map[string]any)["content"] != "real transcript" {
		t.Fatalf("not the real execution: %v", out)
	}
	otherTask := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime})
	testutil.Call(t, projectExecutionEvidenceHandler(), request(otherTask, testUserID)).Want(404)
	reader := dbfx.User(t, "Unprivileged evidence reader", "progress-transcript-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, reader, "member")
	testutil.Call(t, projectExecutionEvidenceHandler(), request(task, reader)).Want(403)
	otherProject := progressProject(t)
	foreignProjectRequest := request(task, testUserID)
	chi.RouteContext(foreignProjectRequest.Context()).URLParams.Values[0] = otherProject
	testutil.Call(t, projectExecutionEvidenceHandler(), foreignProjectRequest).Want(404)
	otherWorkspace := dbfx.Workspace(t, "Other transcript workspace", "progress-transcript-"+uuid.NewString())
	dbfx.Member(t, otherWorkspace, testUserID, "member")
	foreignWorkspaceRequest := request(task, testUserID)
	foreignWorkspaceRequest.Header.Set("X-Workspace-ID", otherWorkspace)
	testutil.Call(t, projectExecutionEvidenceHandler(), foreignWorkspaceRequest).Want(404)
	chat := dbfx.ChatSession(t, agent, testutil.Cols{"creator_id": reader})
	chatTask := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime, "chat_session_id": chat})
	chatDraft := progressDraft("Private chat evidence")
	chatDraft["evidence"] = []any{map[string]any{"kind": "execution", "id": chatTask, "url": nil}}
	previewReq := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates/preview", chatDraft), "id", p)
	previewReq.Header.Set("X-User-ID", reader)
	chatPreview := testutil.Call(t, testHandler.PreviewProjectUpdate, previewReq).Want(200).Map()
	createReq := withURLParam(newRequest("POST", "/api/projects/"+p+"/updates", progressInput(chatPreview)), "id", p)
	createReq.Header.Set("X-User-ID", reader)
	chatUpdate := testutil.Call(t, testHandler.CreateProjectUpdate, createReq).Want(201).Map()["update_id"].(string)
	chatReq := testutil.WithURLParams(newRequest("GET", "/api/projects/"+p+"/updates/"+chatUpdate+"/revisions/1/executions/"+chatTask, nil), "id", p, "updateId", chatUpdate, "revision", "1", "taskId", chatTask)
	testutil.Call(t, projectExecutionEvidenceHandler(), chatReq).Want(403)
	chatReq.Header.Set("X-User-ID", reader)
	testutil.Call(t, projectExecutionEvidenceHandler(), chatReq).Want(200)
	dbfx.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, reader)
	testutil.Call(t, projectExecutionEvidenceHandler(), chatReq).Want(403)
}

func TestProjectUpdateSourcePermissionRecoveryPreservesWorkspaceAccess(t *testing.T) {
	p := progressProject(t)
	reader := dbfx.User(t, "Source reader", "progress-source-code-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, reader, "member")
	runtime := dbfx.Runtime(t, "Source permission runtime")
	agent := dbfx.Agent(t, "Source permission agent", runtime, testutil.Cols{"permission_mode": "public_to"})
	task := dbfx.Task(t, agent, testutil.Cols{"status": "completed", "runtime_id": runtime})
	dbfx.InsertNoID(t, "agent_invocation_target", testutil.Cols{"agent_id": agent, "target_type": "member", "target_id": reader}, "agent_id=$1 AND target_id=$2", agent, reader)
	call := func(method string, handler http.HandlerFunc, body any, status int) map[string]any {
		t.Helper()
		req := withURLParam(newRequest(method, "/api/projects/"+p+"/updates", body), "id", p)
		req.Header.Set("X-User-ID", reader)
		return testutil.Call(t, handler, req).Want(status).Map()
	}
	d := progressDraft("Published protected source reference")
	d["evidence"] = []any{map[string]any{"kind": "execution", "id": task, "url": nil}}
	published := call("POST", testHandler.CreateProjectUpdate, progressInput(call("POST", testHandler.PreviewProjectUpdate, d, 200)), 201)
	d["body"] = "My new draft remains recoverable"
	pending := progressInput(call("POST", testHandler.PreviewProjectUpdate, d, 200))
	dbfx.Exec(t, "DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2", agent, reader)
	for _, denied := range []map[string]any{call("POST", testHandler.PreviewProjectUpdate, d, 403), call("POST", testHandler.CreateProjectUpdate, pending, 403)} {
		if denied["code"] != "project_evidence_forbidden" {
			t.Fatalf("source revocation classification=%v", denied)
		}
	}
	history := call("GET", testHandler.ListProjectUpdates, nil, 200)
	current := history["items"].([]any)[0].(map[string]any)["current"].(map[string]any)
	evidence := current["evidence"].([]any)[0].(map[string]any)
	if current["body"] != "Published protected source reference" || evidence["availability"] != "inaccessible" || evidence["label"] != nil || evidence["href"] != nil || evidence["current_version"] != nil {
		t.Fatalf("source-only history redaction=%v", current)
	}
	evidenceReq := testutil.WithURLParams(newRequest("GET", "/api/projects/"+p+"/updates/evidence", nil), "id", p, "updateId", published["update_id"].(string), "revision", "1", "taskId", task)
	evidenceReq.Header.Set("X-User-ID", reader)
	if denied := testutil.Call(t, testHandler.GetProjectUpdateExecutionEvidence, evidenceReq).Want(403).Map(); denied["code"] != "project_evidence_forbidden" {
		t.Fatalf("evidence reader classification=%v", denied)
	}
	d["evidence"] = []any{}
	recovered := call("POST", testHandler.PreviewProjectUpdate, d, 200)
	if recovered["draft"].(map[string]any)["body"] != "My new draft remains recoverable" {
		t.Fatal("source removal lost the pending body")
	}
	leave := withURLParam(newRequest("POST", "/api/workspaces/"+testWorkspaceID+"/leave", nil), "id", testWorkspaceID)
	leave.Header.Set("X-User-ID", reader)
	testutil.Call(t, testHandler.LeaveWorkspace, leave).Want(204)
	if denied := call("POST", testHandler.PreviewProjectUpdate, d, 403); denied["code"] != "forbidden" {
		t.Fatalf("real workspace revocation must retain forbidden: %v", denied)
	}
}
