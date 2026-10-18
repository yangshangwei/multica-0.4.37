package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type collaborationExecutionFixtureIDs struct {
	issue, session, target, squad, member string
}

func collaborationExecutionReadFixture(t *testing.T) collaborationExecutionFixtureIDs {
	t.Helper()
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	session, err := testHandler.Queries.GetCollaborationSession(context.Background(), parseUUID(issue))
	if err != nil {
		t.Fatal(err)
	}
	sessionID := uuidToString(session.ID)
	dbfx.Exec(t, `UPDATE collaboration_session SET target_set_version=9007199254740999,control_version=9007199254740995 WHERE id=$1`, sessionID)
	inputEvent := uuid.NewString()
	roster := json.RawMessage(`[{"private_directory":"PRIVATE_DIRECTORY","credential":"PRIVATE_CREDENTIAL"}]`)
	ids := collaborationExecutionFixtureIDs{issue: issue, session: sessionID}
	for _, kind := range []string{"agent", "squad"} {
		agent := uuid.NewString()
		resourceID := agent
		if kind == "squad" {
			resourceID = uuid.NewString()
		}
		channel := dbfx.Insert(t, "collaboration_channel", testutil.Cols{
			"session_id": sessionID, "root_agent_id": agent, "group_generation": 1,
			"control_version": int64(9007199254740995), "next_batch_seq": 2,
		})
		target := dbfx.Insert(t, "collaboration_target", testutil.Cols{
			"session_id": sessionID, "channel_id": channel, "kind": kind, "resource_id": resourceID, "roster": roster,
			"target_epoch": int64(9007199254740993), "config_version": int64(9007199254740993),
			"control_version": int64(9007199254740995), "next_command_seq": int64(9007199254740994),
		})
		batch := dbfx.Insert(t, "collaboration_batch", testutil.Cols{
			"session_id": sessionID, "channel_id": channel, "effective_target_id": target,
			"seq": 1, "state": "in_flight", "started_at": time.Now(),
		})
		command := dbfx.Insert(t, "collaboration_command", testutil.Cols{
			"session_id": sessionID, "target_id": target, "input_event_id": inputEvent, "seq": int64(9007199254740993),
			"channel_id": channel, "batch_id": batch,
			"author_user_id": testUserID, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1,
			"content": "PRIVATE_COMMAND_INPUT", "state": "in_flight",
		})
		group := dbfx.Insert(t, "collaboration_execution_group", testutil.Cols{
			"session_id": sessionID, "target_id": target, "command_id": command, "generation": 1,
			"channel_id": channel, "batch_id": batch,
			"target_epoch": int64(9007199254740993), "config_version": int64(9007199254740993), "roster": roster,
		})
		dbfx.Exec(t, `UPDATE collaboration_target SET group_generation=1,active_group_id=$2 WHERE id=$1`, target, group)
		dbfx.Exec(t, `UPDATE collaboration_channel SET active_group_id=$2 WHERE id=$1`, channel, group)
		dbfx.Exec(t, `UPDATE collaboration_batch SET root_command_id=$2,group_id=$3 WHERE id=$1`, batch, command, group)
		rootKind := "agent"
		if kind == "squad" {
			rootKind = "leader"
		}
		run := testutil.Cols{
			"session_id": sessionID, "target_id": target, "group_id": group, "command_id": command, "agent_id": agent,
			"channel_id": channel, "batch_id": batch,
			"step_key": "root", "step_kind": rootKind, "grant_id": uuid.NewString(), "grant_epoch": 1, "resource_epoch": 1,
			"execution_workspace_id": uuid.NewString(), "runtime_id": uuid.NewString(), "installation_id": uuid.NewString(),
			"binding_id": uuid.NewString(), "binding_epoch": 1, "target_epoch": int64(9007199254740993), "config_version": int64(9007199254740993),
			"progress_state": "ending", "forced_disposition": "stopped", "authorized_attempt_seq": 1,
		}
		if kind == "squad" {
			run["progress_state"], run["proof_state"], run["business_outcome"] = "settled", "exited_proven", "success"
			run["settled_at"], run["sealed_attempt_seq"], run["forced_disposition"] = time.Now(), 1, nil
		}
		root := dbfx.Insert(t, "collaboration_run", run)
		if kind == "squad" {
			run["step_key"], run["step_kind"], run["parent_run_id"], run["agent_id"] = "member-1", "member", root, uuid.NewString()
			run["progress_state"], run["proof_state"], run["business_outcome"] = "ending", "unknown", nil
			run["settled_at"], run["sealed_attempt_seq"], run["forced_disposition"], run["recording_incomplete"] = nil, nil, "stopped", true
			ids.member = dbfx.Insert(t, "collaboration_run", run)
			ids.squad = target
		} else {
			ids.target = target
		}
		dbfx.Exec(t, `UPDATE collaboration_execution_group SET state='ending',expansion_sealed_at=now(),forced_disposition='stopped' WHERE id=$1`, group)
	}
	return ids
}

func TestCollaborationExecutionHTTPProjection(t *testing.T) {
	ids := collaborationExecutionReadFixture(t)
	viewer := dbfx.User(t, "External execution viewer", "private-execution-"+uuid.NewString()+"@example.invalid")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": ids.issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", ids.issue, viewer)
	r := collaborationFieldsReadRequest(ids.issue, viewer, "")
	var out CollaborationIssueResponse
	response := testutil.Call(t, testHandler.GetCollaborationIssue, r).Want(http.StatusOK).JSON(&out)
	execution := out.Execution
	if execution.ProtocolVersion != 3 || execution.IssueID != ids.issue || execution.SessionID != ids.session || execution.TargetSetVersion != "9007199254740999" ||
		execution.Control.Scope != "issue" || execution.Control.ControlVersion != out.Session.ControlVersion || execution.Control.QueueState != out.Session.QueueState {
		t.Fatalf("execution did not carry the exact issue/control snapshot: %+v", execution)
	}
	if execution.Availability != "unavailable" || execution.UnavailableReason != "shared_runtime_unavailable" || out.Capabilities.ControlAI || out.Capabilities.SubmitCommand || !out.Capabilities.Comment {
		t.Fatalf("foundation projection exposed execution or lost human comments: %+v", out)
	}
	// These historical fixtures have no grants or live runtime rows. The mask
	// reflects missing public protocol support, not a grant requirement for Stop.
	access, err := service.NewCollaborationService(testHandler.Queries, testHandler.TxStarter, testHandler.IssueService).ResolveIssueAccess(r.Context(), nil, parseUUID(ids.issue))
	if err != nil || !access.Capabilities.ControlAI || !access.Capabilities.SubmitCommand {
		t.Fatalf("read availability masking changed internal participant authority: %+v, %v", access.Capabilities, err)
	}
	if len(execution.Targets) != 2 || len(execution.Commands) != 2 || len(execution.Groups) != 2 || len(execution.Runs) != 3 {
		t.Fatalf("typed targets or squad members were lost: %+v", execution)
	}
	channels := make(map[string]string)
	for _, target := range execution.Targets {
		if target.Control.Scope != "target" || target.ConfigVersion != "9007199254740993" || target.Control.ControlVersion != "9007199254740995" || target.GroupGeneration != "1" || target.ActiveGroupID == nil || target.ChannelID == nil {
			t.Fatalf("invalid target scope/version: %+v", target)
		}
		channels[target.ID] = *target.ChannelID
	}
	for _, command := range execution.Commands {
		if command.Sequence != "9007199254740993" || command.AuthorUserID != testUserID || command.ChannelID != channels[command.TargetID] || command.BatchID == "" {
			t.Fatalf("command lost sequence, author or batch identity: %+v", command)
		}
	}
	groups := make(map[string]service.CollaborationExecutionGroupView)
	for _, group := range execution.Groups {
		groups[group.ID] = group
	}
	memberFound := false
	for _, run := range execution.Runs {
		group := groups[run.GroupID]
		if run.ChannelID != channels[run.TargetID] || run.ChannelID != group.ChannelID || run.BatchID == "" || run.BatchID != group.BatchID {
			t.Fatalf("run lost its retained group/channel/batch identity: %+v", run)
		}
		if run.ID == ids.member {
			memberFound = run.TargetID == ids.squad && run.ParentRunID != nil && run.StepKind == "member" && run.ProofState == "unknown" && run.SettledAt == nil && run.RecordingIncomplete
		}
	}
	if !memberFound {
		t.Fatal("leader proof hid the member's unconfirmed stop")
	}
	for _, forbidden := range []string{"PRIVATE_", `"roster"`, `"grant_id"`, `"runtime_id"`, `"installation_id"`, `"binding_id"`, `"launch_nonce"`, `"execution_workspace_id"`, `"email"`, `"username"`} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("projection exposed %s", forbidden)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" {
		t.Fatal("execution read inherited workspace or cache scope")
	}
	for _, principal := range []auth.AuthenticatedPrincipal{
		{UserID: viewer, Kind: auth.CredentialTask}, {UserID: viewer, Kind: auth.CredentialCloud},
		{UserID: viewer, Kind: auth.CredentialJWT, AgentActor: true}, {UserID: viewer, Kind: auth.CredentialPAT, AgentActor: true},
		{UserID: viewer, Kind: auth.CredentialJWT, Restricted: true}, {UserID: viewer}, {UserID: uuid.NewString(), Kind: auth.CredentialJWT},
	} {
		request := collaborationFieldsReadRequest(ids.issue, viewer, "")
		request = request.WithContext(auth.WithAuthenticatedPrincipal(request.Context(), principal))
		denied := testutil.Call(t, testHandler.GetCollaborationIssue, request).Want(http.StatusNotFound)
		if strings.Contains(denied.Body.String(), ids.target) || strings.Contains(denied.Body.String(), "execution") {
			t.Fatal("machine or unrelated caller received execution details")
		}
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, ids.session) != 2 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, ids.session) != 3 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, ids.issue) != 0 ||
		dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, ids.issue) != 0 {
		t.Fatal("execution read changed durable work")
	}
}

func TestCollaborationExecutionHTTPEmptyCollections(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	var out CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(issue, testUserID, "")).Want(http.StatusOK).JSON(&out)
	if out.Execution.Targets == nil || out.Execution.Commands == nil || out.Execution.Groups == nil || out.Execution.Runs == nil {
		t.Fatalf("empty execution collections must be explicit: %+v", out.Execution)
	}
}

type collaborationExecutionReadStarter struct {
	service.TxStarter
	beforeQuery func(context.Context, string) error
}

type collaborationExecutionReadTx struct {
	pgx.Tx
	beforeQuery func(context.Context, string) error
}

func (s *collaborationExecutionReadStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &collaborationExecutionReadTx{Tx: tx, beforeQuery: s.beforeQuery}, nil
}

func (tx *collaborationExecutionReadTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := tx.beforeQuery(ctx, sql); err != nil {
		return nil, err
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestCollaborationExecutionHTTPUsesAuthorizedSnapshot(t *testing.T) {
	ids := collaborationExecutionReadFixture(t)
	viewer := dbfx.User(t, "Snapshot viewer", "execution-snapshot-"+uuid.NewString()+"@example.invalid")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": ids.issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", ids.issue, viewer)
	ready, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	scoped := *testHandler
	scoped.TxStarter = &collaborationExecutionReadStarter{TxStarter: testHandler.TxStarter, beforeQuery: func(ctx context.Context, sql string) error {
		if strings.Contains(sql, "-- name: ListCollaborationTargets") {
			once.Do(func() { close(ready) })
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}}
	r := collaborationFieldsReadRequest(ids.issue, viewer, "")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, scoped.GetCollaborationIssue, r.WithContext(ctx)) }()
	select {
	case <-ready:
	case early := <-result:
		t.Fatalf("detail ended before execution read barrier: %d %s", early.Code, early.Body.String())
	case <-ctx.Done():
		t.Fatal("detail did not reach execution read barrier")
	}
	dbfx.Exec(t, `WITH changed AS (UPDATE collaboration_session SET target_set_version=target_set_version+1 WHERE id=$1 RETURNING id)
		UPDATE collaboration_target SET control_version=control_version+1 WHERE session_id IN (SELECT id FROM changed)`, ids.session)
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, ids.issue, viewer)
	unblock()
	var before CollaborationIssueResponse
	(<-result).Want(http.StatusOK).JSON(&before)
	if before.Execution.TargetSetVersion != "9007199254740999" || before.Execution.Targets[0].Control.ControlVersion != "9007199254740995" {
		t.Fatalf("execution spliced newer target data into older authorization: %+v", before.Execution)
	}
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(ids.issue, viewer, "")).Want(http.StatusNotFound)
	var after CollaborationIssueResponse
	testutil.Call(t, testHandler.GetCollaborationIssue, collaborationFieldsReadRequest(ids.issue, testUserID, "")).Want(http.StatusOK).JSON(&after)
	if after.Execution.TargetSetVersion != "9007199254741000" || after.Execution.Targets[0].Control.ControlVersion != "9007199254740996" {
		t.Fatalf("new authorized snapshot did not observe committed controls: %+v", after.Execution)
	}
}

func TestCollaborationExecutionHTTPReadFailureIsUnavailable(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	scoped := *testHandler
	scoped.TxStarter = &collaborationExecutionReadStarter{TxStarter: testHandler.TxStarter, beforeQuery: func(_ context.Context, sql string) error {
		if strings.Contains(sql, "-- name: ListCollaborationRuns") {
			return errors.New("PRIVATE_EXECUTION_STORAGE_FAILURE")
		}
		return nil
	}}
	response := testutil.Call(t, scoped.GetCollaborationIssue, collaborationFieldsReadRequest(issue, testUserID, "")).Want(http.StatusServiceUnavailable)
	if response.Map()["code"] != "collaboration_unavailable" || strings.Contains(response.Body.String(), "PRIVATE_") || strings.Contains(response.Body.String(), `"targets"`) {
		t.Fatal("failed execution read returned partial idle data or private diagnostics")
	}
}
