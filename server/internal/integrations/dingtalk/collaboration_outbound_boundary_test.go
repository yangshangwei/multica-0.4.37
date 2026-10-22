package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

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

type collaborationIMDingQueries struct {
	*db.Queries
	deliveryReads, installationReads int
	deliveryError                    error
}

func (q *collaborationIMDingQueries) GetChannelTaskDelivery(ctx context.Context, id pgtype.UUID) (db.ChannelTaskDelivery, error) {
	q.deliveryReads++
	if q.deliveryError != nil {
		return db.ChannelTaskDelivery{}, q.deliveryError
	}
	return q.Queries.GetChannelTaskDelivery(ctx, id)
}
func (q *collaborationIMDingQueries) GetChannelInstallation(ctx context.Context, p db.GetChannelInstallationParams) (db.ChannelInstallation, error) {
	q.installationReads++
	return q.Queries.GetChannelInstallation(ctx, p)
}

func TestCollaborationOutboundDingTalkPrivateFinalAndFailureNeverReachHTTPWithoutDelivery(t *testing.T) {
	f := newCollaborationIMFixture(t)
	f.channel(t, "dingtalk", "cid_boundary", []byte(`{"app_id":"fake-local-app","robot_code":"fake-local-robot","app_secret_encrypted":"bG9jYWw="}`), []byte(`{"conversation_id":"cid_boundary","conversation_type":"2"}`))
	q := &collaborationIMDingQueries{Queries: f.q}
	creds := 0
	api := newDingtalkSendServer(t)
	outbound := NewOutbound(q, func([]byte) ([]byte, error) { creds++; return []byte("local-revoked-test-secret"), nil }, NewClient(api.srv.Client(), api.srv.URL), nil)
	bus := events.New()
	outbound.Register(bus)
	for _, candidate := range []struct{ name, id string }{{"shared_source_run", f.run}, {"ordinary_child_without_delivery", f.task}} {
		for _, kind := range []string{"final", "failure"} {
			t.Run(candidate.name+"/"+kind, func(t *testing.T) {
				event := f.done(candidate.id, f.private)
				if kind == "failure" {
					event = f.failed(candidate.id, f.private)
				}
				before := q.deliveryReads
				if err := outbound.processEvent(t.Context(), event); err != nil {
					t.Fatal(err)
				}
				bus.Publish(event)
				if q.deliveryReads != before+2 || q.installationReads != 0 || creds != 0 || atomic.LoadInt32(&api.tokenCalls) != 0 || atomic.LoadInt32(&api.sendCalls) != 0 {
					t.Fatal("private DingTalk reply used credentials or reached HTTP before immutable delivery")
				}
			})
		}
	}
	q.deliveryError = errors.New("local immutable delivery unavailable")
	for _, event := range []events.Event{f.done(f.task, f.private), f.failed(f.task, f.private)} {
		if err := outbound.processEvent(t.Context(), event); !errors.Is(err, q.deliveryError) {
			t.Fatal("delivery lookup error was not propagated")
		}
		bus.Publish(event)
	}
	if q.installationReads != 0 || creds != 0 || atomic.LoadInt32(&api.tokenCalls)+atomic.LoadInt32(&api.sendCalls) != 0 {
		t.Fatal("delivery lookup error sent private DingTalk output")
	}
	q.deliveryError = nil
	f.delivery(t, "dingtalk")
	bus.Publish(f.done(f.task, "Ordinary child answer"))
	if api.lastPath != pathSendGroup || api.lastBody["openConversationId"] != "cid_boundary" {
		t.Fatal("ordinary child did not reach actual local group HTTP transport")
	}
	var text markdownParam
	raw, ok := api.lastBody["msgParam"].(string)
	if !ok || json.Unmarshal([]byte(raw), &text) != nil || text.Text != "Ordinary child answer" {
		t.Fatal("ordinary child actual HTTP reply body was incorrect")
	}
	bus.Publish(f.failed(f.task, "Ordinary child failure"))
	raw, ok = api.lastBody["msgParam"].(string)
	if !ok || json.Unmarshal([]byte(raw), &text) != nil || text.Text != "⚠️ Ordinary child failure" {
		t.Fatal("ordinary failure did not reach actual HTTP transport")
	}
	for _, private := range strings.Split(f.private, "\n") {
		if strings.Contains(raw, private) {
			t.Fatal("ordinary child failure leaked shared parent data")
		}
	}
	if atomic.LoadInt32(&api.sendCalls) != 2 || q.installationReads != 2 || creds != 2 {
		t.Fatal("ordinary positive did not preserve final/failure outbound behavior")
	}
}

func TestCollaborationOutboundDingTalkPrivateAttachmentPayloadsCannotBypassTextBoundary(t *testing.T) {
	f := newCollaborationIMFixture(t)
	f.channel(t, "dingtalk", "cid_boundary", []byte(`{"app_id":"fake-local-app","robot_code":"fake-local-robot","app_secret_encrypted":"bG9jYWw="}`), []byte(`{"conversation_id":"cid_boundary","conversation_type":"2"}`))
	q := &collaborationIMDingQueries{Queries: f.q}
	creds := 0
	api := newDingtalkSendServer(t)
	// Record every HTTP operation, including any accidental media endpoint,
	// instead of treating an absent text send as proof that no wire was used.
	var wireCalls atomic.Int64
	transport := api.srv.Client().Transport
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		wireCalls.Add(1)
		return transport.RoundTrip(r)
	})}
	outbound := NewOutbound(q, func([]byte) ([]byte, error) { creds++; return []byte("local-revoked-test-secret"), nil }, NewClient(client, api.srv.URL), nil)
	bus := events.New()
	outbound.Register(bus)
	attachmentEvent := func(task, content string) events.Event {
		return events.Event{Type: protocol.EventChatDone, WorkspaceID: f.fx.WorkspaceID, TaskID: task, ChatSessionID: f.chat, Payload: map[string]any{
			"task_id": task, "chat_session_id": f.chat, "message_id": f.output, "content": content,
			"attachment_ids": []string{f.attachment}, "attachments": []map[string]any{{"id": f.attachment, "filename": "private-fixture.txt", "content": "IM_BOUNDARY_PRIVATE_FILE"}},
		}}
	}
	for _, candidate := range []struct{ name, id string }{{"shared_source_run", f.run}, {"ordinary_child_without_delivery", f.task}} {
		for _, kind := range []string{"text_and_attachment", "file_only"} {
			t.Run(candidate.name+"/"+kind, func(t *testing.T) {
				content := f.private
				expectedReads := 1
				if kind == "file_only" {
					content = ""
					expectedReads = 0 // The existing content parser drops empty completions first.
				}
				before := q.deliveryReads
				bus.Publish(attachmentEvent(candidate.id, content))
				if q.deliveryReads != before+expectedReads || q.installationReads != 0 || creds != 0 || wireCalls.Load() != 0 {
					t.Fatal("private attachment-bearing event crossed text parser/delivery boundary")
				}
			})
		}
	}
	q.deliveryError = errors.New("local immutable delivery unavailable")
	if err := outbound.processEvent(t.Context(), attachmentEvent(f.task, f.private)); !errors.Is(err, q.deliveryError) {
		t.Fatal("attachment-bearing delivery fault was not propagated")
	}
	bus.Publish(attachmentEvent(f.task, f.private))
	if q.installationReads != 0 || creds != 0 || wireCalls.Load() != 0 {
		t.Fatal("delivery fault accessed media/credentials or HTTP")
	}
	q.deliveryError = nil
	f.delivery(t, "dingtalk")
	bus.Publish(attachmentEvent(f.task, "Ordinary child own answer"))
	before := q.deliveryReads
	bus.Publish(attachmentEvent(f.task, ""))
	var text markdownParam
	raw, ok := api.lastBody["msgParam"].(string)
	if !ok || json.Unmarshal([]byte(raw), &text) != nil || text.Text != "Ordinary child own answer" || api.lastBody["msgKey"] != msgKeyMarkdown || api.lastPath != pathSendGroup || api.lastBody["openConversationId"] != "cid_boundary" {
		t.Fatal("ordinary attachment-bearing positive did not retain its text-only wire")
	}
	if q.deliveryReads != before || q.installationReads != 1 || creds != 1 || atomic.LoadInt32(&api.tokenCalls) != 1 || atomic.LoadInt32(&api.sendCalls) != 1 || wireCalls.Load() != 2 {
		t.Fatal("ordinary positive or file-only parser drop changed")
	}
	for _, private := range strings.Split(f.private, "\n") {
		if strings.Contains(raw, private) || strings.Contains(raw, f.attachment) {
			t.Fatal("ordinary text wire serialized private attachment fields")
		}
	}
}
