package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	outboundPrivateBody       = "OUTBOUND_PRIVATE_COLLABORATION_BODY"
	outboundPrivateComment    = "OUTBOUND_PRIVATE_COLLABORATION_COMMENT"
	outboundPrivateAttachment = "OUTBOUND_PRIVATE_COLLABORATION_ATTACHMENT"
	outboundPrivateRecord     = "OUTBOUND_PRIVATE_COLLABORATION_RECORD"
)

type outboundCollaborationLookupFailure struct {
	db.DBTX
	checks int
}

func (q *outboundCollaborationLookupFailure) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: IsCollaborationIssue ") {
		q.checks++
		return errRow{err: errors.New("injected outbound collaboration lookup failure")}
	}
	return q.DBTX.QueryRow(ctx, sql, args...)
}

// The worker's real delivery method is synchronous here so an absent HTTP call
// is checked after processing finishes, without sleeps or a queue timing guess.
// The bridge vocabulary/payload extraction remains canonically tested in
// plugin_event_dispatch_test.go; this test owns actual DB-to-transport refusal.
func TestCollaborationOutboundPluginDispatcherWithholdsPrivateEvents(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	upload.Name, upload.Data = outboundPrivateAttachment+".txt", []byte(outboundPrivateAttachment)
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := f.params(t)
	p.Description, p.DraftID, p.AttachmentIDs = outboundPrivateBody, &upload.DraftID, []pgtype.UUID{attachment.ID}
	shared, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	comment, err := f.svc.CreateComment(f.ctx, CollaborationCreateCommentParams{
		CollaborationQueueRequest: CollaborationQueueRequest{IssueID: shared.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1},
		CollaborationCommentInput: CollaborationCommentInput{Content: outboundPrivateComment},
	})
	if err != nil {
		t.Fatal(err)
	}
	storedComment, err := f.svc.Queries.GetComment(f.ctx, comment.CommentID)
	if err != nil || storedComment.Content != outboundPrivateComment {
		t.Fatalf("private comment setup: %+v %v", storedComment, err)
	}
	file, err := f.svc.GetAttachment(f.ctx, shared.Issue.ID, pgtype.UUID{}, attachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	privateBytes, err := f.svc.ReadAttachmentBytes(f.ctx, file)
	if err != nil || !bytes.Equal(privateBytes, upload.Data) {
		t.Fatalf("private attachment setup: %v", err)
	}
	// A retained record is deliberately nonempty; withholding must not depend
	// on the issue having no history or no runtime-owned result data.
	recordID := f.fx.Insert(t, "collaboration_run_event", testutil.Cols{
		"issue_id": shared.Issue.ID, "run_id": uuid.NewString(), "producer_event_id": uuid.NewString(),
		"seq": 1, "kind": "outcome", "payload": testutil.Raw(`'{"body":"` + outboundPrivateRecord + `"}'::jsonb`),
	})

	harness := newHookTestServer(t)
	plugins := hookTestService(t, harness)
	plugins.Queries, plugins.FeatureFlags = f.svc.Queries, testFlagService(true)
	declaration := hookTestInstallation(t, harness.server.URL+"/hooks/summarize", hookTestHost(harness), []string{plugincontract.TriggerEvent})
	var manifest map[string]any
	if err := json.Unmarshal(declaration.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	hook := manifest["contributes"].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	hook["events"] = []string{plugincontract.EventIssueCreated, plugincontract.EventIssueUpdated, plugincontract.EventCommentCreated, plugincontract.EventTaskCompleted, plugincontract.EventTaskFailed}
	declaration.Manifest, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	installation := f.fx.Insert(t, "plugin_installation", testutil.Cols{
		"workspace_id": f.workspace, "plugin_key": "com.example.outboundboundary", "package_version_id": uuid.NewString(), "version": "1.0.0", "manifest": declaration.Manifest,
		"granted_scopes": declaration.GrantedScopes, "installed_by": f.creator,
	})
	f.fx.Cleanup(t, `DELETE FROM plugin_invocation WHERE installation_id=$1`, installation)
	dispatcher := &PluginEventDispatcher{service: plugins}
	issuePayload := map[string]any{"issue": IssueToMap(shared.Issue, "OUT"), "attachments": []any{CollaborationAttachmentMetadata(file)}, "record_id": recordID, "record": outboundPrivateRecord}
	for _, event := range []struct {
		kind    string
		payload any
	}{
		{plugincontract.EventIssueCreated, issuePayload},
		{plugincontract.EventIssueUpdated, issuePayload},
		{plugincontract.EventCommentCreated, map[string]any{"comment": storedComment, "attachments": []any{CollaborationAttachmentMetadata(file)}, "record": outboundPrivateRecord}},
		{plugincontract.EventTaskCompleted, map[string]any{"issue_id": util.UUIDToString(shared.Issue.ID), "content": outboundPrivateRecord, "description": outboundPrivateBody}},
		{plugincontract.EventTaskFailed, taskEvent(protocol.EventTaskFailed, f.workspace, db.AgentTaskQueue{IssueID: shared.Issue.ID}, taskFailedFields(outboundPrivateRecord+" "+outboundPrivateComment, "provider_failed", false)).Payload},
	} {
		t.Run(event.kind, func(t *testing.T) {
			dispatcher.run(dispatchJob{installation: db.PluginInstallation{WorkspaceID: shared.Issue.WorkspaceID}, eventType: event.kind, payload: event.payload})
			select {
			case <-harness.received:
				t.Fatal("private collaboration event reached the plugin endpoint")
			default:
			}
			if f.fx.Count(t, `SELECT count(*) FROM plugin_invocation WHERE installation_id=$1`, installation) != 0 {
				t.Fatal("withheld collaboration event created a plugin invocation")
			}
		})
	}
	fault := &outboundCollaborationLookupFailure{DBTX: f.pool}
	plugins.Queries = db.New(fault)
	dispatcher.run(dispatchJob{installation: db.PluginInstallation{WorkspaceID: shared.Issue.WorkspaceID}, eventType: plugincontract.EventIssueCreated, payload: issuePayload})
	if fault.checks != 1 {
		t.Fatalf("outbound classification did not hit the real failed query: %d checks", fault.checks)
	}
	select {
	case <-harness.received:
		t.Fatal("unknown collaboration classification sent private payload")
	default:
	}
	plugins.Queries = f.svc.Queries
	ordinaryID := f.fx.Issue(t, "Ordinary child outbound control", testutil.Cols{"parent_issue_id": shared.Issue.ID, "description": "Ordinary child own public context"})
	ordinary, err := f.svc.Queries.GetIssue(f.ctx, util.MustParseUUID(ordinaryID))
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.run(dispatchJob{installation: db.PluginInstallation{WorkspaceID: ordinary.WorkspaceID}, eventType: plugincontract.EventIssueCreated, payload: map[string]any{"issue": IssueToMap(ordinary, "OUT")}})
	select {
	case request := <-harness.received:
		if !bytes.Contains(request.Body, []byte("Ordinary child own public context")) || !bytes.Contains(request.Body, []byte(ordinaryID)) {
			t.Fatal("ordinary plugin control did not send its actual issue")
		}
		for _, secret := range []string{outboundPrivateBody, outboundPrivateComment, outboundPrivateAttachment, outboundPrivateRecord} {
			if bytes.Contains(request.Body, []byte(secret)) {
				t.Fatalf("ordinary child hook inherited private parent %q", secret)
			}
		}
		secret, err := plugins.HookSigningSecret(util.MustParseUUID(installation))
		if err != nil || VerifyHookSignature(secret, request.Timestamp, request.Body, request.Signature, time.Now()) != nil {
			t.Fatalf("ordinary positive control did not use the real signed transport: %v", err)
		}
	default:
		t.Fatal("ordinary child plugin hook was not delivered")
	}
	if f.fx.Count(t, `SELECT count(*) FROM plugin_invocation WHERE installation_id=$1 AND status='ok'`, installation) != 1 {
		t.Fatal("ordinary positive control did not record one successful outbound invocation")
	}
}

func TestCollaborationOutboundPublicWritesStayOffOrdinaryEventBus(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	f, _ := newCollaborationAttachmentFixture(t)
	bus := events.New()
	f.svc.Issues.Bus = bus
	var published []events.Event
	bus.SubscribeAll(func(event events.Event) { published = append(published, event) })
	upload := attachmentUploadParams(f)
	upload.Name, upload.Data = outboundPrivateAttachment+".txt", []byte(outboundPrivateAttachment)
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	create := f.params(t)
	create.Description, create.DraftID, create.AttachmentIDs = outboundPrivateBody, &upload.DraftID, []pgtype.UUID{attachment.ID}
	shared, err := f.svc.Create(f.ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	input := CollaborationCreateCommentParams{
		CollaborationQueueRequest: CollaborationQueueRequest{IssueID: shared.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1},
		CollaborationCommentInput: CollaborationCommentInput{Content: outboundPrivateComment},
	}
	if _, err := f.svc.CreateComment(f.ctx, input); err != nil {
		t.Fatal(err)
	}
	request := CollaborationWorkflowRequest{CollaborationQueueRequest: input.CollaborationQueueRequest, ExpectedCollaborationRevision: 1}
	request.RequestID = uuid.NewString()
	if _, err := f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(f.outsider)}); err != nil {
		t.Fatal(err)
	}
	request.RequestID, request.ExpectedCollaborationRevision = uuid.NewString(), 2
	if _, err := f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(f.outsider)}, Note: outboundPrivateComment}); err != nil {
		t.Fatal(err)
	}
	if len(published) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, shared.Issue.ID) < 4 ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, shared.Issue.ID) != 0 {
		t.Fatalf("public collaboration writes did not keep private retained events separate from ordinary/IM bus: %+v", published)
	}
	ordinary, err := f.svc.Issues.Create(f.ctx, IssueCreateParams{
		WorkspaceID: shared.Issue.WorkspaceID, Title: "Ordinary child bus positive control", Description: pgtype.Text{String: "Ordinary child own context", Valid: true},
		Status: "todo", Priority: "none", CreatorType: "member", CreatorID: util.MustParseUUID(f.creator), ParentIssueID: shared.Issue.ID,
	}, IssueCreateOpts{BroadcastPayload: func(issue db.Issue, _ []db.Attachment, _ []db.IssueLabel) map[string]any {
		return map[string]any{"issue": IssueToMap(issue, "OUT")}
	}})
	if err != nil {
		t.Fatal(err)
	}
	var ordinaryCreates int
	for _, event := range published {
		if event.Type == protocol.EventIssueCreated {
			ordinaryCreates++
			if issueIDFromPayload(event.Payload) != ordinary.Issue.ID {
				t.Fatal("ordinary positive event referred to the private shared parent")
			}
		}
		encoded, err := json.Marshal(event.Payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{outboundPrivateBody, outboundPrivateComment, outboundPrivateAttachment, outboundPrivateRecord} {
			if bytes.Contains(encoded, []byte(secret)) {
				t.Fatalf("ordinary child event leaked parent context %q", secret)
			}
		}
	}
	if ordinaryCreates != 1 {
		t.Fatalf("ordinary positive control published %d create events, want one", ordinaryCreates)
	}
}

func TestCollaborationOutboundManagedResultStaysOffOrdinaryChatEvents(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	bus := events.New()
	f.svc.Issues.Bus = bus
	f.svc.Issues.TaskService = NewTaskService(f.svc.Queries, f.pool, nil, bus)
	var published []events.Event
	bus.SubscribeAll(func(event events.Event) { published = append(published, event) })
	f.submitManaged(t, f.creator, outboundPrivateBody)
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
		Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateRuntimeComment(context.Background(), runID, permit.BusinessToken, CollaborationRuntimeCommentParams{
		CommentID: util.MustParseUUID(uuid.NewString()), Content: outboundPrivateComment,
	}); err != nil {
		t.Fatal(err)
	}
	outcome := recordedOutcome(claim, permit, 1)
	outcome.Body, outcome.Record.Messages[0].Body = outboundPrivateRecord, outboundPrivateRecord
	if err := f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(page)
	if err != nil || !bytes.Contains(encoded, []byte(outboundPrivateRecord)) || !bytes.Contains(encoded, []byte(outboundPrivateComment)) {
		t.Fatalf("public capability writes did not retain real private result/comment records: %v", err)
	}
	var taskID pgtype.UUID
	f.fx.QueryRow(t, `SELECT task_id FROM collaboration_run WHERE id=$1`, runID).Scan(&taskID)
	if taskID.Valid || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 || len(published) != 0 {
		t.Fatalf("proven shared result created an ordinary task/IM event: task=%v bus=%+v", taskID, published)
	}
}
