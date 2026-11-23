package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	guuid "github.com/google/uuid"
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
	fx.UserID = fx.User(t, "IM boundary actor", "im-boundary-"+guuid.NewString()+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "IM boundary workspace", "im-boundary-"+guuid.NewString())
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
	draft := util.MustParseUUID(guuid.NewString())
	uploaded, err := svc.UploadAttachmentDraft(ctx, service.CollaborationAttachmentUpload{DraftID: draft, IssueID: sharedID, RequestID: guuid.NewString(), Name: "private-fixture.txt", ContentType: "text/plain", Data: []byte(file)})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, uploaded.ID)
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, sharedID)
	fx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, sharedID)
	fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, uploaded.ID)
	if _, err = svc.DeliverAttachments(ctx, service.CollaborationAttachmentDelivery{IssueID: sharedID, DraftID: &draft, AttachmentIDs: []pgtype.UUID{uploaded.ID}, RequestID: guuid.NewString()}); err != nil {
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
	command, err := q.CreateCollaborationCommand(ctx, db.CreateCollaborationCommandParams{SessionID: session, TargetID: target, ChannelID: lane, BatchID: batch, InputEventID: util.MustParseUUID(guuid.NewString()), Seq: 1, AuthorUserID: owner, ParticipantEpoch: 1, AuthorAuthEpoch: 1, LifecycleEpoch: 1, Content: "Private source run input", Attachments: []byte("[]")})
	if err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_command WHERE id=$1`, command.ID)
	binding, installation, deploy := util.MustParseUUID(guuid.NewString()), util.MustParseUUID(guuid.NewString()), util.MustParseUUID(guuid.NewString())
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
	fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": sharedID, "run_id": run.ID, "producer_event_id": guuid.NewString(), "seq": 1, "kind": "outcome", "payload": payload})
	child := fx.Issue(t, "Ordinary child reply", testutil.Cols{"parent_issue_id": shared, "description": "Ordinary child own context", "assignee_type": "agent", "assignee_id": agent})
	chat := fx.ChatSession(t, agent)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": child, "chat_session_id": chat})
	fx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=$1 WHERE id=$1`, task)
	fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": task, "role": "user", "content": "Ordinary own channel-ingested input", "channel_ingested": true})
	output := fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": task, "role": "assistant", "content": "Ordinary child answer"})
	if yes, err := q.IsCollaborationIssue(ctx, util.MustParseUUID(child)); err != nil || yes {
		t.Fatal("ordinary child was incorrectly classified as shared")
	}
	return &collaborationIMFixture{pool: pool, q: q, fx: fx, shared: shared, child: child, run: util.UUIDToString(run.ID), chat: chat, task: task, output: output, private: strings.Join([]string{body, comment, file, record}, "\n")}
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

type collaborationIMWecomQueries struct {
	*db.Queries
	deliveryReads, installationReads, attachmentReads   int
	taskReads, originReads                              int
	deliveryError, taskError, originError, installError error
}

func (q *collaborationIMWecomQueries) GetChannelTaskDelivery(ctx context.Context, id pgtype.UUID) (db.ChannelTaskDelivery, error) {
	q.deliveryReads++
	if q.deliveryError != nil {
		return db.ChannelTaskDelivery{}, q.deliveryError
	}
	return q.Queries.GetChannelTaskDelivery(ctx, id)
}
func (q *collaborationIMWecomQueries) GetChannelInstallation(ctx context.Context, p db.GetChannelInstallationParams) (db.ChannelInstallation, error) {
	q.installationReads++
	if q.installError != nil {
		return db.ChannelInstallation{}, q.installError
	}
	return q.Queries.GetChannelInstallation(ctx, p)
}
func (q *collaborationIMWecomQueries) GetAgentTask(ctx context.Context, id pgtype.UUID) (db.AgentTaskQueue, error) {
	q.taskReads++
	if q.taskError != nil {
		return db.AgentTaskQueue{}, q.taskError
	}
	return q.Queries.GetAgentTask(ctx, id)
}
func (q *collaborationIMWecomQueries) TaskHasChannelIngestedMessages(ctx context.Context, id pgtype.UUID) (bool, error) {
	q.originReads++
	if q.originError != nil {
		return false, q.originError
	}
	return q.Queries.TaskHasChannelIngestedMessages(ctx, id)
}
func (q *collaborationIMWecomQueries) ListAttachmentsByChatMessage(ctx context.Context, p db.ListAttachmentsByChatMessageParams) ([]db.Attachment, error) {
	q.attachmentReads++
	return q.Queries.ListAttachmentsByChatMessage(ctx, p)
}

func collaborationIMWecom(t *testing.T) (*collaborationIMFixture, *collaborationIMWecomQueries, *Outbound, *mediaConn, *fakeObjectStore, db.ChannelInstallation) {
	t.Helper()
	f := newCollaborationIMFixture(t)
	instID, _ := f.channel(t, "wecom", "CHAT_BOUNDARY", []byte(`{"app_id":"fake-local-bot"}`), []byte(`{}`))
	inst, err := f.q.GetChannelInstallation(t.Context(), db.GetChannelInstallationParams{ID: util.MustParseUUID(instID), ChannelType: "wecom"})
	if err != nil {
		t.Fatal(err)
	}
	q := &collaborationIMWecomQueries{Queries: f.q}
	objects := &fakeObjectStore{key: "ordinary-test-key", data: []byte("Ordinary child file bytes")}
	conn := newMediaConn()
	reg := newSendersRegistry()
	reg.set(inst.ID, conn.newSender())
	outbound := NewOutbound(q, reg, nil, WithAttachments(objects))
	outbound.spawn = func(f func()) { f() }
	return f, q, outbound, conn, objects, inst
}

func TestCollaborationOutboundWecomPrivateDirectAndFileOnlyRequireDeliveryBeforeSenderOrStorage(t *testing.T) {
	f, q, outbound, conn, objects, _ := collaborationIMWecom(t)
	bus := events.New()
	outbound.Register(bus)
	for _, candidate := range []struct{ name, id string }{{"shared_source_run", f.run}, {"ordinary_child_without_delivery", f.task}} {
		for _, kind := range []string{"final", "file_only"} {
			t.Run(candidate.name+"/"+kind, func(t *testing.T) {
				content := f.private
				if kind == "file_only" {
					content = ""
				}
				event := f.done(candidate.id, content)
				before := q.deliveryReads
				if err := outbound.processEvent(t.Context(), event); err != nil {
					t.Fatal(err)
				}
				bus.Publish(event)
				if q.deliveryReads != before+2 || q.installationReads != 0 || q.attachmentReads != 0 || objects.readCount() != 0 || len(conn.cmdFrames(cmdSendMsg))+len(conn.cmdFrames(cmdUploadMediaInit)) != 0 {
					t.Fatal("private WeCom reply/file-only entered sender/storage before immutable delivery")
				}
			})
		}
	}
	q.deliveryError = errors.New("local immutable delivery unavailable")
	for _, body := range []string{f.private, ""} {
		event := f.done(f.task, body)
		if err := outbound.processEvent(t.Context(), event); !errors.Is(err, q.deliveryError) {
			t.Fatal("WeCom delivery lookup error was not propagated")
		}
		bus.Publish(event)
	}
	if q.installationReads != 0 || q.attachmentReads != 0 || objects.readCount() != 0 || len(conn.cmdFrames(cmdSendMsg)) != 0 {
		t.Fatal("WeCom delivery dependency failure sent or read private data")
	}
	q.deliveryError = nil
	f.delivery(t, "wecom")
	f.fx.Insert(t, "attachment", testutil.Cols{"workspace_id": f.fx.WorkspaceID, "chat_session_id": f.chat, "chat_message_id": f.output, "task_id": f.task, "uploader_type": "agent", "uploader_id": instAgent(t, f), "filename": "ordinary-child.txt", "url": "https://local.fixture/ordinary-child.txt", "content_type": "text/plain", "size_bytes": len(objects.data)})
	bus.Publish(f.done(f.task, "Ordinary child answer"))
	if texts := markdownSends(t, conn); len(texts) != 1 || texts[0] != "Ordinary child answer" {
		t.Fatal("ordinary child answer did not reach recorded production WeCom sender")
	}
	if len(mediaSends(t, conn)) != 1 || objects.readCount() != 1 || q.attachmentReads != 1 {
		t.Fatal("ordinary own attachment failed actual sender/upload/storage positive")
	}
	for _, frame := range conn.cmdFrames(cmdSendMsg) {
		for _, private := range strings.Split(f.private, "\n") {
			if bytes.Contains(frame.Body, []byte(private)) {
				t.Fatal("ordinary WeCom output inherited private parent body/comment/file/record")
			}
		}
	}
}
func instAgent(t *testing.T, f *collaborationIMFixture) pgtype.UUID {
	t.Helper()
	task, err := f.q.GetAgentTask(t.Context(), util.MustParseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	return task.AgentID
}

// A relay recipient is a separate production consumer. The local dispatcher
// must not turn a live installation socket into permission for a shared run or
// an ordinary child without its immutable channel delivery.
func TestCollaborationOutboundWecomRelayReceiverRejectsPrivateAndUnknownDeliveryBeforeWire(t *testing.T) {
	for _, scenario := range []string{"shared_source_run", "ordinary_child_without_delivery", "delivery_lookup_failure"} {
		t.Run(scenario, func(t *testing.T) {
			f, q, outbound, conn, objects, inst := collaborationIMWecom(t)
			task := f.run
			if scenario != "shared_source_run" {
				task = f.task
			}
			if scenario == "delivery_lookup_failure" {
				q.deliveryError = errors.New("local immutable delivery unavailable")
			}
			router := NewRelayOutbound(nil, nil, RelayConfig{Shards: 1}, nil)
			router.Attach(outbound)
			frame := relayFrame{Kind: relayKindReply, InstallationID: util.UUIDToString(inst.ID), ChatID: "CHAT_BOUNDARY", ChatType: chatTypeGroupInt, Content: f.private, TaskID: task, MessageID: f.output, WorkspaceID: f.fx.WorkspaceID, SessionID: f.chat, CarriesFiles: true}
			router.perform(t.Context(), queued{frame: frame, eventID: guuid.NewString()})
			if q.deliveryReads != 1 || q.installationReads != 0 || q.attachmentReads != 0 || objects.readCount() != 0 || len(conn.cmdFrames(cmdSendMsg))+len(conn.cmdFrames(cmdUploadMediaInit)) != 0 {
				t.Fatalf("relay receiver bypassed immutable delivery: lookup=%d installation=%d attachments=%d wire=%d", q.deliveryReads, q.installationReads, q.attachmentReads, len(conn.cmdFrames(cmdSendMsg)))
			}
		})
	}
}

func TestCollaborationOutboundWecomRelayOrdinaryChildUsesItsExactDeliveryAndOwnMedia(t *testing.T) {
	f, q, outbound, conn, objects, inst := collaborationIMWecom(t)
	delivery := f.delivery(t, "wecom")
	f.fx.Insert(t, "attachment", testutil.Cols{"workspace_id": f.fx.WorkspaceID, "chat_session_id": f.chat, "chat_message_id": f.output, "task_id": f.task, "uploader_type": "agent", "uploader_id": instAgent(t, f), "filename": "ordinary-relay.txt", "url": "https://local.fixture/ordinary-relay.txt", "content_type": "text/plain", "size_bytes": len(objects.data)})
	router := NewRelayOutbound(nil, nil, RelayConfig{Shards: 1}, nil)
	router.Attach(outbound)
	frame := relayFrame{Kind: relayKindReply, InstallationID: util.UUIDToString(delivery.InstallationID), ChatID: delivery.ChannelChatID, ChatType: chatTypeGroupInt, Content: "Ordinary child answer", TaskID: f.task, MessageID: f.output, WorkspaceID: f.fx.WorkspaceID, SessionID: f.chat, CarriesFiles: true}
	router.perform(t.Context(), queued{frame: frame, eventID: guuid.NewString()})
	if got := markdownSends(t, conn); len(got) != 1 || got[0] != "Ordinary child answer" || len(mediaSends(t, conn)) != 1 || objects.readCount() != 1 {
		t.Fatal("ordinary relayed reply/media did not preserve actual local transport")
	}
	if q.deliveryReads != 1 || q.installationReads != 1 || q.attachmentReads != 1 {
		t.Fatal("relayed ordinary child did not revalidate exact delivery/installation/media")
	}
	_ = inst
}

func TestCollaborationOutboundWecomRelayRejectsTamperedOrInactiveDelivery(t *testing.T) {
	for _, scenario := range []string{"installation", "chat", "chat_type", "session", "workspace", "missing_task", "malformed_task", "malformed_session", "malformed_workspace", "inactive_installation", "other_channel", "web_input", "missing_queries"} {
		t.Run(scenario, func(t *testing.T) {
			f, q, outbound, conn, objects, inst := collaborationIMWecom(t)
			kind := "wecom"
			if scenario == "other_channel" {
				f.fx.Exec(t, `UPDATE channel_installation SET channel_type='slack' WHERE id=$1`, inst.ID)
				f.fx.Exec(t, `UPDATE channel_chat_session_binding SET channel_type='slack' WHERE chat_session_id=$1`, f.chat)
				kind = "slack"
			}
			if scenario == "web_input" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET status='completed', completed_at=now() WHERE id=$1`, f.task)
				f.task = f.fx.Task(t, util.UUIDToString(instAgent(t, f)), testutil.Cols{"runtime_id": mustIMTaskRuntime(t, f), "issue_id": f.child, "chat_session_id": f.chat})
				f.fx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=$1 WHERE id=$1`, f.task)
				f.fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": f.chat, "task_id": f.task, "role": "user", "content": "Ordinary input typed only in Web", "channel_ingested": false})
			}
			delivery := f.deliveryWithoutOriginAssertion(t, kind)
			frame := relayFrame{Kind: relayKindReply, InstallationID: util.UUIDToString(delivery.InstallationID), ChatID: delivery.ChannelChatID, ChatType: chatTypeGroupInt, Content: f.private,
				TaskID: f.task, MessageID: f.output, WorkspaceID: f.fx.WorkspaceID, SessionID: f.chat, CarriesFiles: true}
			switch scenario {
			case "installation":
				id := util.MustParseUUID(guuid.NewString())
				frame.InstallationID = util.UUIDToString(id)
				outbound.senders.set(id, conn.newSender()) // a live foreign socket is insufficient
			case "chat":
				frame.ChatID = "OTHER_ROOM"
			case "chat_type":
				frame.ChatType = chatTypeSingleInt
			case "session":
				frame.SessionID = guuid.NewString()
			case "workspace":
				frame.WorkspaceID = guuid.NewString()
			case "missing_task":
				frame.TaskID = ""
			case "malformed_task":
				frame.TaskID = "not-a-task"
			case "malformed_session":
				frame.SessionID = "not-a-session"
			case "malformed_workspace":
				frame.WorkspaceID = "not-a-workspace"
			case "inactive_installation":
				f.fx.Exec(t, `UPDATE channel_installation SET status='revoked' WHERE id=$1`, inst.ID)
			case "missing_queries":
				outbound.q = nil
			}
			router := NewRelayOutbound(nil, nil, RelayConfig{Shards: 1}, nil)
			router.Attach(outbound)
			if !router.perform(t.Context(), queued{frame: frame, eventID: guuid.NewString()}) {
				t.Fatal("immutable mismatch or inactive authority was retried as a temporary delivery")
			}
			if q.attachmentReads != 0 || objects.readCount() != 0 || len(conn.cmdFrames(cmdSendMsg))+len(conn.cmdFrames(cmdUploadMediaInit)) != 0 {
				t.Fatalf("%s relay frame reached private media or wire before exact current delivery", scenario)
			}
			if scenario != "missing_task" && scenario != "malformed_task" && scenario != "missing_queries" && q.deliveryReads != 1 {
				t.Fatal("relay refusal did not consult its exact immutable task delivery")
			}
		})
	}
}

func mustIMTaskRuntime(t *testing.T, f *collaborationIMFixture) pgtype.UUID {
	t.Helper()
	task, err := f.q.GetAgentTask(t.Context(), util.MustParseUUID(f.task))
	if err != nil {
		t.Fatal(err)
	}
	return task.RuntimeID
}

func (f *collaborationIMFixture) deliveryWithoutOriginAssertion(t *testing.T, kind string) db.ChannelTaskDelivery {
	t.Helper()
	row, err := f.q.CreateChannelTaskDeliveryFromSession(t.Context(), db.CreateChannelTaskDeliveryFromSessionParams{TaskID: util.MustParseUUID(f.task), ChatSessionID: util.MustParseUUID(f.chat)})
	if err != nil || row.ChannelType != kind {
		t.Fatalf("real immutable delivery prerequisite: %s %v", row.ChannelType, err)
	}
	f.fx.Cleanup(t, `DELETE FROM channel_task_delivery WHERE task_id=$1`, row.TaskID)
	return row
}

func TestCollaborationOutboundWecomRelayReadFaultRetainsRetryWithoutPrivateWire(t *testing.T) {
	for _, stage := range []string{"delivery", "task", "origin", "installation"} {
		t.Run(stage, func(t *testing.T) {
			f, q, outbound, conn, objects, _ := collaborationIMWecom(t)
			delivery := f.delivery(t, "wecom")
			var logs strings.Builder
			outbound.logger = slog.New(slog.NewTextHandler(&logs, nil))
			fault := errors.New("temporary lookup contains " + f.private)
			switch stage {
			case "delivery":
				q.deliveryError = fault
			case "task":
				q.taskError = fault
			case "origin":
				q.originError = fault
			case "installation":
				q.installError = fault
			}
			router := NewRelayOutbound(nil, nil, RelayConfig{Shards: 1}, nil)
			router.Attach(outbound)
			item := queued{frame: relayFrame{Kind: relayKindReply, InstallationID: util.UUIDToString(delivery.InstallationID), ChatID: delivery.ChannelChatID, ChatType: chatTypeGroupInt,
				Content: "Ordinary child retry answer", TaskID: f.task, MessageID: f.output, WorkspaceID: f.fx.WorkspaceID, SessionID: f.chat, CarriesFiles: true}, eventID: guuid.NewString()}
			if router.perform(t.Context(), item) || q.attachmentReads != 0 || objects.readCount() != 0 || len(conn.cmdFrames(cmdSendMsg))+len(conn.cmdFrames(cmdUploadMediaInit)) != 0 {
				t.Fatal("read fault sent private content/media or consumed the original retry identity")
			}
			if logs.Len() == 0 || strings.Contains(logs.String(), "IM_BOUNDARY_PRIVATE_") {
				t.Fatal("read-fault diagnostic was missing or leaked protected dependency contents")
			}
			q.deliveryError, q.taskError, q.originError, q.installError = nil, nil, nil, nil
			if !router.perform(t.Context(), item) {
				t.Fatal("original relay item remained blocked after the same delivery recovered")
			}
			if got := markdownSends(t, conn); len(got) != 1 || got[0] != "Ordinary child retry answer" {
				t.Fatal("original recovered relay item failed the actual ordinary transport positive")
			}
			if !router.perform(t.Context(), item) || len(markdownSends(t, conn)) != 1 {
				t.Fatal("original relay event identity replay duplicated its delivered reply")
			}
		})
	}
}
