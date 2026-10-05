package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func resourceMultipartRequest(t *testing.T, path, token string, fields [][2]string, files ...string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	for _, field := range fields {
		if err := m.WriteField(field[0], field[1]); err != nil {
			t.Fatal(err)
		}
	}
	for _, data := range files {
		f, err := m.CreateFormFile("file", "SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func TestResourceStalledMultipartReleasesUploadCapacity(t *testing.T) {
	for i := 0; i < cap(resourceUploadSlots)-1; i++ {
		resourceUploadSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < cap(resourceUploadSlots)-1; i++ {
			<-resourceUploadSlots
		}
	})
	server := httptest.NewServer(middleware.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()
		r = r.WithContext(ctx)
		release, ok := acquireResourceUpload(w, r)
		if !ok {
			return
		}
		defer release()
		if _, err := readResourceMultipart(w, r, map[string]bool{"key": true}); err != nil {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
	})))
	defer server.Close()
	connection, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err = connection.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(connection, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: multipart/form-data; boundary=stall\r\nContent-Length: 10000\r\n\r\n--stall\r\nContent-Disposition: form-data; name=\"file\"; filename=\"SKILL.md\"\r\n\r\npartial")
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		t.Fatalf("stalled upload did not release its slot within the deadline: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatalf("stalled status=%d", response.StatusCode)
	}
	request := resourceMultipartRequest(t, server.URL, "", [][2]string{{"key", "healthy"}}, "content")
	request.RequestURI = ""
	client := &http.Client{Timeout: 2 * time.Second}
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("later upload could not acquire released slot: %d", response.StatusCode)
	}
}

type resourceAuditUnavailable struct{ db.DBTX }

func (q resourceAuditUnavailable) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("injected audit storage failure")
}

func TestResourceCommitGuardDoesNotApplyWithoutDurableAudit(t *testing.T) {
	login := adminHandlerSetup(t)
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{Kind: "jwt", UserID: login.User.ID, Version: 1})
	actor, err := testHandler.platformAdminService().Authorize(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	handler := *testHandler
	handler.Queries = db.New(resourceAuditUnavailable{DBTX: testPool})
	p := service.ResourceMutation{Kind: "skill", Key: "durable", OrganizationID: uuidToString(actor.OrganizationID), ActorID: login.User.ID, OperationID: uuid.NewString(), Reason: "Require durable request audit"}
	called := false
	_, err = handler.resourceCommitGuard(actor, p, "publish", uuid.NewString())(ctx, func() (service.ResourceMutationResult, error) {
		called = true
		return service.ResourceMutationResult{}, nil
	})
	if err == nil || called {
		t.Fatalf("unaudited apply: called=%v error=%v", called, err)
	}
}

func TestResourceCommitGuardFencesAuthorityAndRecordsRequestBeforeApply(t *testing.T) {
	login := adminHandlerSetup(t)
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{Kind: "jwt", UserID: login.User.ID, Version: 1})
	actor, err := testHandler.platformAdminService().Authorize(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	p := service.ResourceMutation{Kind: "skill", Key: "fenced", OrganizationID: uuidToString(actor.OrganizationID), ActorID: login.User.ID, OperationID: uuid.NewString(), Reason: "Verify authority fence"}
	guard := testHandler.resourceCommitGuard(actor, p, "publish", uuid.NewString())
	called := false
	_, err = guard(ctx, func() (service.ResourceMutationResult, error) {
		called = true
		if dbfx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE actor_user_id=$1 AND phase='requested'", login.User.ID) != 1 {
			t.Fatal("request audit was not durably visible before apply")
		}
		tx, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `SELECT id FROM "user" WHERE id=$1 FOR UPDATE NOWAIT`, login.User.ID)
		var postgres *pgconn.PgError
		if !errors.As(err, &postgres) || postgres.Code != "55P03" {
			t.Fatalf("actor lock missing during apply: %v", err)
		}
		return service.ResourceMutationResult{OperationID: p.OperationID, Resource: service.Resource{Version: uuid.NewString(), State: "published"}}, nil
	})
	if err != nil || !called {
		t.Fatalf("guard: called=%v err=%v", called, err)
	}
	dbfx.Exec(t, "UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1", login.User.ID)
	called = false
	_, err = guard(ctx, func() (service.ResourceMutationResult, error) {
		called = true
		return service.ResourceMutationResult{}, nil
	})
	var denied *service.PlatformAdminError
	if called || !errors.As(err, &denied) || denied.Status != 403 {
		t.Fatalf("stale authority applied: called=%v err=%v", called, err)
	}
}

func TestResourceAuditFinalizeFailureHasRecoverableReceipt(t *testing.T) {
	login := adminHandlerSetup(t)
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{Kind: "jwt", UserID: login.User.ID, Version: 1})
	actor, err := testHandler.platformAdminService().Authorize(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	publisher := &service.ResourcePublisher{Root: t.TempDir()}
	data := []byte("---\nname: unknown-outcome\ndescription: Receipt fixture\n---\n\nContents\n")
	preview, err := publisher.Preview(ctx, "skill", "unknown-outcome", "SKILL.md", data, uuidToString(actor.OrganizationID))
	if err != nil {
		t.Fatal(err)
	}
	p := service.ResourceMutation{Kind: "skill", Key: "unknown-outcome", Filename: "SKILL.md", Data: data, PreviewDigest: preview.PreviewDigest,
		OrganizationID: uuidToString(actor.OrganizationID), ActorID: login.User.ID, OperationID: uuid.NewString(), Reason: "Recover failed acknowledgement"}
	old := testHandler.TxStarter
	testHandler.TxStarter = rollbackOnCommitTxStarter{pool: testPool}
	t.Cleanup(func() { testHandler.TxStarter = old })
	_, err = publisher.Publish(ctx, p, testHandler.resourceCommitGuard(actor, p, "publish", uuid.NewString()))
	var uncertain *service.ResourceError
	if !errors.As(err, &uncertain) || uncertain.Code != "resource_outcome_unknown" || uncertain.OperationID != p.OperationID {
		t.Fatalf("expected unknown outcome: %v", err)
	}
	receipt, err := publisher.Receipt(ctx, p.OrganizationID, p.ActorID, p.OperationID)
	if err != nil || receipt.Resource.State != "published" {
		t.Fatalf("receipt missing after filesystem commit: %+v %v", receipt, err)
	}
	if _, err := publisher.Receipt(ctx, p.OrganizationID, uuid.NewString(), p.OperationID); err == nil {
		t.Fatal("other actor read private receipt")
	}
	if dbfx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE actor_user_id=$1 AND phase='requested'", login.User.ID) != 1 {
		t.Fatal("failed outcome erased request evidence")
	}
}

func TestResourceMultipartRejectsAmbiguousAndOversizedParts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields [][2]string
		files  []string
		valid  bool
	}{
		{"valid", [][2]string{{"key", "example"}}, []string{"content"}, true},
		{"duplicate field", [][2]string{{"key", "one"}, {"key", "two"}}, []string{"content"}, false},
		{"unknown field", [][2]string{{"root", "/tmp"}}, []string{"content"}, false},
		{"duplicate file", nil, []string{"one", "two"}, false},
		{"missing file", nil, nil, false},
		{"oversized field", [][2]string{{"key", strings.Repeat("x", 4097)}}, []string{"content"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resourceMultipartRequest(t, "/", "", tc.fields, tc.files...)
			_, err := readResourceMultipart(httptest.NewRecorder(), r, map[string]bool{"key": true})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestResourceAuditProjectionPreservesOnlySafeResourceMetadata(t *testing.T) {
	version, operation := uuid.NewString(), uuid.NewString()
	digest := "sha256:" + strings.Repeat("a", 64)
	raw, _ := json.Marshal(map[string]any{"kind": "skill", "key": "reference-skill", "version": version, "expected_version": version, "operation_id": operation, "state": "published", "content_digest": digest, "file_count": 2, "byte_count": 512,
		"content": "PRIVATE", "config": map[string]string{"token": "PRIVATE"}, "password": "PRIVATE", "reason": "PRIVATE"})
	projected := adminAuditTargetSnapshot(raw, "resource")
	encoded, _ := json.Marshal(projected)
	if strings.Contains(string(encoded), "PRIVATE") || projected["version"] != version || projected["operation_id"] != operation || projected["content_digest"] != digest || projected["kind"] != "skill" || projected["key"] != "reference-skill" {
		t.Fatalf("invalid resource projection: %s", encoded)
	}
	for _, bad := range []string{
		`{"kind":"PRIVATE","key":"/secret/path","version":"PRIVATE","content_digest":"PRIVATE","operation_id":"PRIVATE","state":"PRIVATE","byte_count":-1,"file_count":999999}`,
		`{"kind":{"value":"skill"},"key":["reference"]}`,
	} {
		if len(adminAuditTargetSnapshot([]byte(bad), "resource")) != 0 {
			t.Fatal("invalid resource metadata escaped projection")
		}
	}
	if _, ok := adminAuditTargetSnapshot(raw, "task")["key"]; ok {
		t.Fatal("resource allowlist broadened other audit targets")
	}
}

func resourceTestRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Auth(testHandler.Queries, nil, nil))
	r.Get("/api/admin/resources", testHandler.AdminResources)
	r.Get("/api/admin/resources/operations/{id}", testHandler.AdminResourceOperation)
	r.Post("/api/admin/resources/{kind}/preview", testHandler.AdminResourcePreview)
	r.Post("/api/admin/resources/{kind}/{key}/publish", testHandler.AdminResourcePublish)
	r.Post("/api/admin/resources/{kind}/{key}/withdraw", testHandler.AdminResourceWithdraw)
	return r
}

func TestAdminResourcesPermissionAndPublication(t *testing.T) {
	login := adminHandlerSetup(t)
	old := testHandler.ResourcePublisher
	publisher := &service.ResourcePublisher{Root: t.TempDir()}
	testHandler.ResourcePublisher = publisher
	t.Cleanup(func() { testHandler.ResourcePublisher = old })
	router := resourceTestRouter()
	call := func(r *http.Request, status int) *testutil.Response {
		return testutil.Call(t, router.ServeHTTP, r).Want(status)
	}
	request := func(method, path string, body any) *http.Request {
		r := testutil.JSONRequest(method, path, body)
		r.Header.Set("Authorization", "Bearer "+login.Token)
		return r
	}
	const content = "---\nname: publication-fixture\ndescription: Publication fixture\n---\n\nPrivate uploaded body\n"
	var preview service.ResourcePreview
	call(resourceMultipartRequest(t, "/api/admin/resources/skill/preview", login.Token, [][2]string{{"key", "publication-fixture"}}, content), 200).JSON(&preview)
	op := uuid.NewString()
	publish := func(version string) *http.Request {
		r := resourceMultipartRequest(t, "/api/admin/resources/skill/publication-fixture/publish", login.Token, [][2]string{{"preview_digest", preview.PreviewDigest}, {"expected_version", version}, {"reason", "Publish documentation"}}, content)
		r.Header.Set("Idempotency-Key", op)
		return r
	}
	var first, replay service.ResourceMutationResult
	call(publish(""), 200).JSON(&first)
	call(publish(""), 200).JSON(&replay)
	if first.OperationID != op || !replay.Replayed || replay.Resource.Version != first.Resource.Version {
		t.Fatalf("bad replay: %+v %+v", first, replay)
	}
	call(request("GET", "/api/admin/resources/operations/"+op, nil), 200)
	if dbfx.Count(t, "SELECT count(*) FROM admin_operation WHERE actor_id=$1", login.User.ID) != 0 {
		t.Fatal("resource operation created a fake admin operation")
	}
	if dbfx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE actor_user_id=$1 AND action='resource.publish' AND phase='applied'", login.User.ID) < 1 {
		t.Fatal("missing applied audit")
	}
	if dbfx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE actor_user_id=$1 AND (before_state::text LIKE '%Private uploaded body%' OR after_state::text LIKE '%Private uploaded body%')", login.User.ID) != 0 {
		t.Fatal("uploaded contents leaked into audit")
	}
	withdraw := request("POST", "/api/admin/resources/skill/publication-fixture/withdraw", map[string]string{"expected_version": first.Resource.Version, "reason": "Withdraw documentation"})
	withdraw.Header.Set("Idempotency-Key", uuid.NewString())
	var withdrawn service.ResourceMutationResult
	call(withdraw, 200).JSON(&withdrawn)
	if withdrawn.Resource.State != "withdrawn" {
		t.Fatalf("withdraw: %+v", withdrawn)
	}
	call(publish(""), 200).JSON(&replay)
	if replay.Resource.Version != first.Resource.Version {
		t.Fatal("replay did not retain original response")
	}
	dbfx.Exec(t, "UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1", login.User.ID)
	var listing struct {
		CanPublish bool `json:"can_publish"`
		Enabled    bool `json:"enabled"`
	}
	call(request("GET", "/api/admin/resources?kind=skill", nil), 200).JSON(&listing)
	if !listing.Enabled || listing.CanPublish {
		t.Fatalf("observer capability: %+v", listing)
	}
	call(resourceMultipartRequest(t, "/api/admin/resources/skill/preview", login.Token, [][2]string{{"key", "publication-fixture"}}, content), 403)
	call(publish(""), 403)
	call(withdraw.Clone(context.Background()), 403)
	testHandler.ResourcePublisher = &service.ResourcePublisher{}
	call(publish(""), 403)
	dbfx.Exec(t, "DELETE FROM platform_role_binding WHERE user_id=$1", login.User.ID)
	call(request("GET", "/api/admin/resources?kind=skill", nil), 403)
}
