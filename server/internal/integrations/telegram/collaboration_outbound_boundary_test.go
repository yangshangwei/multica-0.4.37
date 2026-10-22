package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The private records are real database rows. The encrypted attachment uses the
// actual service; the retained prepared run is a source identity, not a channel
// task or a claim that a provider/kernel process executed.
type collaborationIMFixture struct {
	pool                                   *pgxpool.Pool
	q                                      *db.Queries
	fx                                     *testutil.Fixture
	shared, child, run, chat, task, output string
	attachment                             string
	private                                string
}

func newCollaborationIMFixture(t *testing.T) *collaborationIMFixture {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("requires task-owned schema737 DATABASE_URL")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(t.Context()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.UserID = fx.User(t, "IM boundary actor", "im-boundary-"+uuid.NewString()+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "IM boundary workspace", "im-boundary-"+uuid.NewString())
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	runtime := fx.Runtime(t, "IM boundary offline resource", testutil.Cols{"status": "offline"})
	agent := fx.Agent(t, "IM boundary own agent", runtime)
	q := db.New(pool)
	const body, comment, file, record = "IM_BOUNDARY_PRIVATE_BODY", "IM_BOUNDARY_PRIVATE_COMMENT", "IM_BOUNDARY_PRIVATE_FILE", "IM_BOUNDARY_PRIVATE_RECORD"
	shared := fx.Issue(t, "Minimal shared reference", testutil.Cols{"description": body, "assignee_type": "member", "assignee_id": fx.UserID})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
	sessionID := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": shared})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": shared, "user_id": fx.UserID}, "issue_id=$1 AND user_id=$2", shared, fx.UserID)
	fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": fx.UserID}, "user_id=$1", fx.UserID)
	commentID := fx.Comment(t, shared, comment, testutil.Cols{"dispatch_eligible": false})
	sharedID, session := util.MustParseUUID(shared), util.MustParseUUID(sessionID)
	ctx := auth.WithAuthenticatedPrincipal(t.Context(), auth.AuthenticatedPrincipal{UserID: fx.UserID, Kind: auth.CredentialJWT})
	svc := service.NewCollaborationService(q, pool, service.NewIssueService(q, pool, nil, nil, nil))
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	svc.Storage = storage.NewLocalStorageFromEnv()
	draft := util.MustParseUUID(uuid.NewString())
	uploaded, err := svc.UploadAttachmentDraft(ctx, service.CollaborationAttachmentUpload{DraftID: draft, IssueID: sharedID, RequestID: uuid.NewString(), Name: "private-fixture.txt", ContentType: "text/plain", Data: []byte(file)})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, uploaded.ID)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, sharedID)
	fx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, sharedID)
	fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, uploaded.ID)
	if _, err = svc.DeliverAttachments(ctx, service.CollaborationAttachmentDelivery{IssueID: sharedID, DraftID: &draft, AttachmentIDs: []pgtype.UUID{uploaded.ID}, RequestID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	attachment, err := svc.GetAttachment(ctx, sharedID, pgtype.UUID{}, uploaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	clear, err := svc.ReadAttachmentBytes(ctx, attachment)
	if err != nil || !bytes.Equal(clear, []byte(file)) {
		t.Fatal("private attachment prerequisite did not read its own exact bytes")
	}
	object, err := svc.Storage.GetReader(ctx, attachment.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := io.ReadAll(object)
	closeErr := object.Close()
	if err != nil || closeErr != nil || bytes.Contains(ciphertext, []byte(file)) {
		t.Fatal("private attachment prerequisite is not encrypted")
	}
	if yes, err := q.IsCollaborationIssue(ctx, sharedID); err != nil || !yes {
		t.Fatal("private source prerequisite was not classified as collaboration")
	}
	if row, err := q.GetComment(ctx, util.MustParseUUID(commentID)); err != nil || row.Content != comment {
		t.Fatal("private source comment prerequisite missing")
	}
	agentID, runtimeID, owner, workspace := util.MustParseUUID(agent), util.MustParseUUID(runtime), util.MustParseUUID(fx.UserID), util.MustParseUUID(fx.WorkspaceID)
	roster, err := json.Marshal([]collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: runtime, OwnerUserID: fx.UserID, ExecutionWorkspaceID: fx.WorkspaceID}})
	if err != nil {
		t.Fatal(err)
	}
	targetID := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agentID, "roster": roster})
	target := util.MustParseUUID(targetID)
	channelID := fx.Insert(t, "collaboration_channel", testutil.Cols{"session_id": session, "root_agent_id": agentID})
	lane := util.MustParseUUID(channelID)
	if _, err = q.BindCollaborationTargetChannel(ctx, db.BindCollaborationTargetChannelParams{ChannelID: lane, SessionID: session, TargetID: target}); err != nil {
		t.Fatal(err)
	}
	batchID := fx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": lane, "effective_target_id": target, "seq": 1})
	batch := util.MustParseUUID(batchID)
	command, err := q.CreateCollaborationCommand(ctx, db.CreateCollaborationCommandParams{SessionID: session, TargetID: target, ChannelID: lane, BatchID: batch, InputEventID: util.MustParseUUID(uuid.NewString()), Seq: 1, AuthorUserID: owner, ParticipantEpoch: 1, AuthorAuthEpoch: 1, LifecycleEpoch: 1, Content: "Private source run input", Attachments: []byte("[]")})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_command WHERE id=$1`, command.ID)
	binding, installation, deploy := util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString()), util.MustParseUUID(uuid.NewString())
	grantID := fx.Insert(t, "collaboration_grant", testutil.Cols{"issue_id": sharedID, "target_id": target, "config_version": 1, "role": "agent", "owner_user_id": owner, "owner_auth_epoch": 1, "owner_participant_epoch": 1, "target_epoch": 1, "configuration_fingerprint": strings.Repeat("a", 64), "execution_workspace_id": workspace, "agent_id": agentID, "runtime_id": runtimeID, "identity_kind": "enrolled", "installation_id": installation, "binding_id": binding, "binding_epoch": 1})
	grant := util.MustParseUUID(grantID)
	groupRoster, err := json.Marshal([]collaboration.GroupResource{{TargetResource: collaboration.TargetResource{Role: "agent", AgentID: agent, RuntimeID: runtime, OwnerUserID: fx.UserID, ExecutionWorkspaceID: fx.WorkspaceID}, GrantID: grantID, GrantEpoch: 1}})
	if err != nil {
		t.Fatal(err)
	}
	group, err := q.CreateCollaborationRuntimeGroup(ctx, db.CreateCollaborationRuntimeGroupParams{SessionID: session, TargetID: target, CommandID: command.ID, Generation: 1, TargetEpoch: 1, ConfigVersion: 1, Roster: groupRoster, ChannelID: lane, BatchID: batch})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_execution_group WHERE id=$1`, group.ID)
	run, err := q.CreateCollaborationRuntimeRun(ctx, db.CreateCollaborationRuntimeRunParams{SessionID: session, TargetID: target, GroupID: group.ID, CommandID: command.ID, StepKind: "agent", AgentID: agentID, TargetEpoch: 1, ConfigVersion: 1, GrantID: grant, GrantEpoch: 1, ResourceEpoch: 1, ExecutionWorkspaceID: workspace, RuntimeID: runtimeID, InstallationID: installation, BindingID: binding, BindingEpoch: 1, ChannelID: lane, BatchID: batch, DeploymentID: deploy, ResourceOwnerUserID: owner})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_run WHERE id=$1`, run.ID)
	if run.TaskID.Valid || run.SessionID != session || run.TargetID != target {
		t.Fatal("source run became an ordinary task identity")
	}
	payload, err := json.Marshal(map[string]any{"body": record, "record": map[string]any{"version": 1, "completeness": "partial", "messages": []map[string]any{{"kind": "message", "role": "assistant", "body": record}}}})
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": sharedID, "run_id": run.ID, "producer_event_id": uuid.NewString(), "seq": 1, "kind": "outcome", "payload": payload})
	child := fx.Issue(t, "Ordinary child reply", testutil.Cols{"parent_issue_id": shared, "description": "Ordinary child own context", "assignee_type": "agent", "assignee_id": agent})
	chat := fx.ChatSession(t, agent)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": child, "chat_session_id": chat})
	fx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=$1 WHERE id=$1`, task)
	fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": task, "role": "user", "content": "Ordinary own channel-ingested input", "channel_ingested": true})
	output := fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": task, "role": "assistant", "content": "Ordinary child answer"})
	if yes, err := q.IsCollaborationIssue(ctx, util.MustParseUUID(child)); err != nil || yes {
		t.Fatal("ordinary child was incorrectly classified as shared")
	}
	return &collaborationIMFixture{pool: pool, q: q, fx: fx, shared: shared, child: child, run: util.UUIDToString(run.ID), chat: chat, task: task, output: output, attachment: util.UUIDToString(uploaded.ID), private: strings.Join([]string{body, comment, file, record}, "\n")}
}

func (f *collaborationIMFixture) channel(t *testing.T, kind, chatID string, config, route []byte) (string, string) {
	t.Helper()
	task, err := f.q.GetAgentTask(t.Context(), util.MustParseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	inst := f.fx.Insert(t, "channel_installation", testutil.Cols{"workspace_id": f.fx.WorkspaceID, "agent_id": task.AgentID, "channel_type": kind, "config": config, "installer_user_id": f.fx.UserID})
	binding := f.fx.Insert(t, "channel_chat_session_binding", testutil.Cols{"chat_session_id": f.chat, "installation_id": inst, "channel_type": kind, "channel_chat_id": chatID, "chat_type": "group", "last_message_id": "17", "last_thread_id": "19", "route_revision": 1, "config": route})
	return inst, binding
}

func (f *collaborationIMFixture) delivery(t *testing.T, kind string) db.ChannelTaskDelivery {
	t.Helper()
	row, err := f.q.CreateChannelTaskDeliveryFromSession(t.Context(), db.CreateChannelTaskDeliveryFromSessionParams{TaskID: util.MustParseUUID(f.task), ChatSessionID: util.MustParseUUID(f.chat)})
	if err != nil || row.ChannelType != kind {
		t.Fatal("ordinary positive missing its exact immutable delivery")
	}
	f.fx.Cleanup(t, `DELETE FROM channel_task_delivery WHERE task_id=$1`, row.TaskID)
	if yes, err := f.q.TaskHasChannelIngestedMessages(t.Context(), row.TaskID); err != nil || !yes {
		t.Fatal("ordinary positive missing actual channel input provenance")
	}
	return row
}

func (f *collaborationIMFixture) done(task, content string) events.Event {
	return events.Event{Type: protocol.EventChatDone, WorkspaceID: f.fx.WorkspaceID, ChatSessionID: f.chat, TaskID: task, Payload: protocol.ChatDonePayload{TaskID: task, ChatSessionID: f.chat, MessageID: f.output, Content: content}}
}
func (f *collaborationIMFixture) failed(task, content string) events.Event {
	return events.Event{Type: protocol.EventTaskFailed, WorkspaceID: f.fx.WorkspaceID, ChatSessionID: f.chat, TaskID: task, Payload: map[string]any{"task_id": task, "chat_session_id": f.chat, "error": content, "retry_pending": false}}
}

type collaborationIMTelegramQueries struct {
	*db.Queries
	deliveryReads, installationReads atomic.Int64
	mu                               sync.Mutex
	deliveryError                    error
}

func (q *collaborationIMTelegramQueries) GetChannelTaskDelivery(ctx context.Context, id pgtype.UUID) (db.ChannelTaskDelivery, error) {
	q.deliveryReads.Add(1)
	q.mu.Lock()
	err := q.deliveryError
	q.mu.Unlock()
	if err != nil {
		return db.ChannelTaskDelivery{}, err
	}
	return q.Queries.GetChannelTaskDelivery(ctx, id)
}
func (q *collaborationIMTelegramQueries) GetChannelInstallation(ctx context.Context, p db.GetChannelInstallationParams) (db.ChannelInstallation, error) {
	q.installationReads.Add(1)
	return q.Queries.GetChannelInstallation(ctx, p)
}
func (q *collaborationIMTelegramQueries) fail(err error) {
	q.mu.Lock()
	q.deliveryError = err
	q.mu.Unlock()
}

func TestCollaborationOutboundTelegramPrivateFinalFailureAndStreamNeverReachHTTPWithoutDelivery(t *testing.T) {
	f := newCollaborationIMFixture(t)
	f.channel(t, "telegram", "42", []byte(`{"app_id":"local-test-bot","bot_token_encrypted":"MTIzOmxvY2Fs"}`), []byte(`{"chat_id":"42"}`))
	q := &collaborationIMTelegramQueries{Queries: f.q}
	var creds atomic.Int64
	var mu sync.Mutex
	var sent []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") && !strings.HasSuffix(r.URL.Path, "/editMessageText") {
			t.Error("text-only outbound attempted an unexpected media/HTTP endpoint")
		}
		var body struct {
			Text   string `json:"text"`
			ChatID int64  `json:"chat_id"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("local Telegram request body invalid")
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		sent = append(sent, body.Text)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":99,"chat":{"id":42,"type":"group"},"date":0}}`))
	}))
	t.Cleanup(api.Close)
	outbound := NewOutbound(q, func([]byte) ([]byte, error) { creds.Add(1); return []byte("123:fake-revoked-local-token"), nil }, api.URL, api.Client(), nil)
	bus := events.New()
	outbound.Register(bus)
	ctx, cancel := context.WithCancel(t.Context())
	outbound.Start(ctx)
	t.Cleanup(func() {
		cancel()
		if !outbound.WaitWithTimeout(2 * time.Second) {
			t.Error("local Telegram worker did not stop")
		}
	})
	idle := func() {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			outbound.terminalMu.Lock()
			pending := outbound.queuedTerminalReplyCount
			outbound.terminalMu.Unlock()
			if pending == 0 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("terminal delivery worker did not settle its bounded test event")
			}
			time.Sleep(time.Millisecond)
		}
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(sent) }
	stream := func(task, body string) events.Event {
		issue := f.shared
		if task == f.task {
			issue = f.child
		}
		return events.Event{Type: protocol.EventTaskMessage, TaskID: task, ChatSessionID: f.chat, WorkspaceID: f.fx.WorkspaceID, Payload: protocol.TaskMessagePayload{TaskID: task, IssueID: issue, Type: "text", Content: body}}
	}
	attachmentEvent := func(task, content string) events.Event {
		return events.Event{Type: protocol.EventChatDone, WorkspaceID: f.fx.WorkspaceID, TaskID: task, ChatSessionID: f.chat, Payload: map[string]any{
			"task_id": task, "chat_session_id": f.chat, "message_id": f.output, "content": content,
			"attachment_ids": []string{f.attachment}, "attachments": []map[string]any{{"id": f.attachment, "filename": "private-fixture.txt", "content": "IM_BOUNDARY_PRIVATE_FILE"}},
		}}
	}
	cancelled := func(task string) events.Event {
		return events.Event{Type: protocol.EventTaskCancelled, WorkspaceID: f.fx.WorkspaceID, TaskID: task, ChatSessionID: f.chat, Payload: map[string]any{"task_id": task, "chat_session_id": f.chat, "status": "cancelled", "content": f.private, "attachment_ids": []string{f.attachment}}}
	}
	for _, candidate := range []struct{ name, id string }{{"shared_source_run", f.run}, {"ordinary_child_without_delivery", f.task}} {
		for _, kind := range []string{"final", "failure", "stream", "text_and_attachment", "file_only", "cancelled"} {
			t.Run(candidate.name+"/"+kind, func(t *testing.T) {
				before := q.deliveryReads.Load()
				expectedReads := int64(1)
				switch kind {
				case "final":
					bus.Publish(f.done(candidate.id, f.private))
					idle()
				case "failure":
					bus.Publish(f.failed(candidate.id, f.private))
				case "stream":
					bus.Publish(stream(candidate.id, f.private))
				case "text_and_attachment":
					bus.Publish(attachmentEvent(candidate.id, f.private))
					idle()
				case "file_only":
					bus.Publish(attachmentEvent(candidate.id, ""))
					expectedReads = 0 // Empty completions clear state before enqueue/resolveTarget.
				case "cancelled":
					bus.Publish(cancelled(candidate.id))
					expectedReads = 0 // Cancellation only clears already tracked local streams.
				}
				if q.deliveryReads.Load() != before+expectedReads || q.installationReads.Load() != 0 || creds.Load() != 0 || count() != 0 {
					t.Fatal("private Telegram payload crossed its text/cancellation/delivery boundary")
				}
			})
		}
	}
	injected := errors.New("local immutable delivery unavailable")
	q.fail(injected)
	for _, kind := range []string{"final", "failure", "stream", "text_and_attachment"} {
		event := f.done(f.task, f.private)
		if kind == "failure" {
			event = f.failed(f.task, f.private)
		}
		if kind == "stream" {
			event = stream(f.task, f.private)
		}
		if kind == "text_and_attachment" {
			event = attachmentEvent(f.task, f.private)
		}
		if _, err := outbound.resolveTarget(t.Context(), event, false); !errors.Is(err, injected) {
			t.Fatal("delivery lookup dependency failure was not propagated")
		}
		bus.Publish(event)
		idle()
	}
	if q.installationReads.Load() != 0 || creds.Load() != 0 || count() != 0 {
		t.Fatal("failed delivery lookup leaked Telegram private data")
	}
	q.fail(nil)
	f.delivery(t, "telegram")
	// Stream precedes completion, as it does in production. A stream frame
	// immediately after a final send is correctly throttled by that chat's
	// existing cooldown and cannot serve as a transport positive.
	bus.Publish(stream(f.task, "Ordinary child streamed text"))
	if count() != 1 {
		t.Fatal("ordinary stream positive did not reach local HTTP")
	}
	outbound.mu.Lock()
	tracked := outbound.streams[f.task]
	outbound.mu.Unlock()
	if tracked == nil || tracked.messageID != 99 {
		t.Fatal("ordinary stream positive did not retain its own wire identity")
	}
	before := q.deliveryReads.Load()
	q.fail(injected)
	bus.Publish(cancelled(f.task))
	bus.Publish(cancelled(f.task))
	outbound.mu.Lock()
	remaining := len(outbound.streams)
	refs := tracked.schedule.refs
	outbound.mu.Unlock()
	if q.deliveryReads.Load() != before || q.installationReads.Load() != 1 || creds.Load() != 1 || count() != 1 || remaining != 0 || refs != 0 {
		t.Fatal("cancel subscriber queried delivery or altered wire instead of exact local cleanup")
	}
	q.fail(nil)
	bus.Publish(attachmentEvent(f.task, "Ordinary child answer"))
	idle()
	before = q.deliveryReads.Load()
	bus.Publish(attachmentEvent(f.task, ""))
	if q.deliveryReads.Load() != before || count() != 2 {
		t.Fatal("ordinary file-only completion did not drop before transport")
	}
	bus.Publish(f.failed(f.task, "Ordinary child failure"))
	mu.Lock()
	actual := append([]string(nil), sent...)
	mu.Unlock()
	if len(actual) != 3 || actual[0] != "Ordinary child streamed text" || actual[1] != "Ordinary child answer" || actual[2] != taskFailedText {
		t.Fatalf("ordinary child stream/final/failure local HTTP transport = %q", actual)
	}
	for _, body := range actual {
		for _, private := range strings.Split(f.private, "\n") {
			if strings.Contains(body, private) || strings.Contains(body, f.attachment) {
				t.Fatal("ordinary Telegram child output inherited private source data")
			}
		}
	}
	if q.installationReads.Load() != 3 || creds.Load() != 3 {
		t.Fatal("ordinary child positive did not use its exact installation identity")
	}
}
