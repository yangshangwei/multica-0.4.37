package slack

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/integrations/channel"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationOutboundDeliveryProbe struct {
	*db.Queries
	deliveryReads     int
	installationReads int
	deliveryError     error
}

func (q *collaborationOutboundDeliveryProbe) GetChannelTaskDelivery(ctx context.Context, id pgtype.UUID) (db.ChannelTaskDelivery, error) {
	q.deliveryReads++
	if q.deliveryError != nil {
		return db.ChannelTaskDelivery{}, q.deliveryError
	}
	return q.Queries.GetChannelTaskDelivery(ctx, id)
}

func (q *collaborationOutboundDeliveryProbe) GetChannelInstallation(ctx context.Context, input db.GetChannelInstallationParams) (db.ChannelInstallation, error) {
	q.installationReads++
	return q.Queries.GetChannelInstallation(ctx, input)
}

// The production subscriber and query layer run unchanged. Only the external
// Slack sender is recorded locally; the positive control must persist the same
// outbound provenance the real adapter writes after a send.
func TestCollaborationOutboundSlackRejectsSharedAndChildWithoutChannelDelivery(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.UserID = fx.User(t, "Outbound boundary actor", "outbound-"+uuid.NewString()+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Outbound boundary workspace", "outbound-"+uuid.NewString())
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	runtime := fx.Runtime(t, "Outbound boundary local fixture")
	agent := fx.Agent(t, "Outbound boundary agent", runtime)
	const body = "IM_PRIVATE_COLLABORATION_BODY"
	const comment = "IM_PRIVATE_COLLABORATION_COMMENT"
	const attachment = "IM_PRIVATE_COLLABORATION_ATTACHMENT"
	const record = "IM_PRIVATE_COLLABORATION_RECORD"
	shared := fx.Issue(t, "Minimal shared reference", testutil.Cols{"description": body, "assignee_type": "member", "assignee_id": fx.UserID})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": shared}, "issue_id=$1", shared)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": shared})
	fx.Comment(t, shared, comment)
	child := fx.Issue(t, "Ordinary child output", testutil.Cols{"parent_issue_id": shared, "description": "Child own context", "assignee_type": "agent", "assignee_id": agent})
	// The shared result sentinel is source-owned retained data. Its run ID is
	// deliberately not an ordinary task ID or a channel delivery identity.
	sharedRun := uuid.NewString()
	fx.Insert(t, "collaboration_run_event", testutil.Cols{
		"issue_id": shared, "run_id": sharedRun, "producer_event_id": uuid.NewString(), "seq": 1,
		"kind": "outcome", "payload": testutil.Raw(`'{"body":"` + record + `","attachment":"` + attachment + `"}'::jsonb`),
	})
	chat := fx.ChatSession(t, agent)
	installation := fx.Insert(t, "channel_installation", testutil.Cols{
		"workspace_id": fx.WorkspaceID, "agent_id": agent, "channel_type": "slack", "config": slackInstallConfigJSON(), "installer_user_id": fx.UserID,
	})
	binding := fx.Insert(t, "channel_chat_session_binding", testutil.Cols{
		"chat_session_id": chat, "installation_id": installation, "channel_type": "slack", "channel_chat_id": "C_OUTBOUND_BOUNDARY", "chat_type": "group",
		"last_message_id": "fixture-input", "last_thread_id": "1.0", "route_revision": 1, "config": testutil.Raw(`'{"channel_id":"C_OUTBOUND_BOUNDARY"}'::jsonb`),
	})
	ordinaryTask := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": child, "chat_session_id": chat})
	ordinary := util.MustParseUUID(ordinaryTask)
	fx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=$1 WHERE id=$1`, ordinary)
	fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": ordinary, "role": "user", "content": "Ordinary channel question", "channel_ingested": true})
	fx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": chat, "task_id": ordinary, "role": "assistant", "content": "Ordinary child answer"})
	probe := &collaborationOutboundDeliveryProbe{Queries: db.New(pool)}
	sender := &fakeSender{result: channel.SendResult{MessageID: "2.0"}}
	outbound := newTestOutbound(probe, sender)
	private := strings.Join([]string{body, comment, attachment, record}, "\n")
	for _, candidate := range []struct{ name, id string }{{"shared_run", sharedRun}, {"ordinary_child_without_delivery", ordinaryTask}} {
		t.Run(candidate.name, func(t *testing.T) {
			before := probe.deliveryReads
			event := chatDoneEvent(chat, private)
			event.TaskID = candidate.id
			if err := outbound.processEvent(t.Context(), event); err != nil {
				t.Fatal(err)
			}
			if probe.deliveryReads != before+1 || probe.installationReads != 0 || sender.called != 0 {
				t.Fatalf("private reply used a live chat binding instead of immutable delivery: lookup=%d installation=%d sends=%d", probe.deliveryReads-before, probe.installationReads, sender.called)
			}
		})
	}
	probe.deliveryError = errors.New("injected channel delivery lookup failure")
	event := chatDoneEvent(chat, private)
	event.TaskID = ordinaryTask
	if err := outbound.processEvent(t.Context(), event); err == nil || !strings.Contains(err.Error(), "lookup slack task delivery") {
		t.Fatalf("unknown origin did not fail closed with lookup error: %v", err)
	}
	if probe.installationReads != 0 || sender.called != 0 {
		t.Fatal("delivery lookup failure accessed credentials or sent a private reply")
	}
	probe.deliveryError = nil
	fx.InsertNoID(t, "channel_task_delivery", testutil.Cols{
		"task_id": ordinaryTask, "binding_id": binding, "installation_id": installation, "channel_type": "slack", "channel_chat_id": "C_OUTBOUND_BOUNDARY",
		"chat_type": "group", "channel_message_id": "fixture-input", "channel_thread_id": "1.0", "route_revision": 1,
		"config": testutil.Raw(`'{"channel_id":"C_OUTBOUND_BOUNDARY"}'::jsonb`),
	}, "task_id=$1", ordinaryTask)
	fx.Cleanup(t, `DELETE FROM channel_outbound_message WHERE installation_id=$1`, installation)
	bus := events.New()
	outbound.Register(bus)
	event = chatDoneEvent(chat, "Ordinary child answer")
	event.TaskID = ordinaryTask
	bus.Publish(event)
	if sender.called != 1 || sender.got.ChatID != "C_OUTBOUND_BOUNDARY" || sender.got.ThreadID != "1.0" || sender.got.Text != "Ordinary child answer" {
		t.Fatalf("ordinary channel positive control did not reach the actual subscribed sender: %+v", sender)
	}
	for _, secret := range []string{body, comment, attachment, record} {
		if strings.Contains(sender.got.Text, secret) {
			t.Fatalf("ordinary child reply contained private parent %q", secret)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM channel_outbound_message WHERE task_id=$1 AND channel_message_id='2.0' AND outbound_kind='task_reply'`, ordinaryTask) != 1 ||
		fx.Count(t, `SELECT count(*) FROM chat_message WHERE task_id=$1 AND role='assistant' AND channel_outbound_type='slack' AND channel_outbound_message_ids=ARRAY['2.0']::text[]`, ordinaryTask) != 1 {
		t.Fatal("ordinary positive control did not retain exact outbound message/provenance")
	}
}
