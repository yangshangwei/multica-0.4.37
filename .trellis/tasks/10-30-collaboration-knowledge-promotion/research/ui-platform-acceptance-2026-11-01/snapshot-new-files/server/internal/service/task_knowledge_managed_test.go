package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeManagedClaimKeepsOriginalRuntimeBinding(t *testing.T) {
	f, source, runtimeID := managedRuntimeFixture(t)
	for _, table := range []string{"agent", "issue"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	fx := testutil.New(f.pool, source.WorkspaceID, source.UserID)
	fx.Exec(t, `UPDATE agent_runtime SET provider='claude' WHERE id=$1`, runtimeID)
	agentID := fx.Agent(t, "Managed knowledge consumer", util.UUIDToString(runtimeID), testutil.Cols{"runtime_mode": "local"})
	issueID := fx.Issue(t, "Managed knowledge issue", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "originator_user_id": source.UserID, "accountable_user_id": source.UserID})
	ctx := auth.WithPasswordSession(t.Context(), source)
	svc := NewTaskService(f.svc.Queries, f.pool, nil, events.New())
	svc.KnowledgeContextEnabled = true
	task, err := svc.ClaimTaskForRuntime(ctx, runtimeID)
	if err != nil || task == nil {
		t.Fatalf("managed claim failed: %v", err)
	}
	if util.UUIDToString(task.ID) != taskID || util.UUIDToString(task.ExecutionBindingID) != source.BindingID {
		t.Fatal("claim lost managed binding identity")
	}
	inv := &protocol.TaskKnowledgeInvocation{ProtocolVersion: 1, Provider: "claude", Mode: "local", Builtin: true, ConfigurationID: "managed-runtime-config"}
	d, _, err := svc.BuildTaskKnowledgeClaim(ctx, *task, inv, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.FinalizeTaskClaim(ctx, *task, db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_" + uuid.NewString()), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: util.MustParseUUID(source.WorkspaceID), UserID: f.user, ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, d); err != nil {
		t.Fatal(err)
	}
	var tokenHash string
	fx.QueryRow(t, `SELECT token_hash FROM daemon_token WHERE installation_binding_id=$1 ORDER BY created_at DESC LIMIT 1`, source.BindingID).Scan(&tokenHash)
	caller := TaskKnowledgeCaller{RuntimeID: runtimeID, Kind: "daemon_token", TokenHash: tokenHash}
	p, err := svc.PrepareTaskKnowledge(ctx, caller, task.ID, protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Contributions) != 0 {
		t.Fatal("empty managed fixture invented private contributions")
	}
	req := protocol.TaskKnowledgeAuthorizeRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: p.InvocationFingerprint, ManifestSHA256: p.ManifestSHA256, PromptSHA256: strings.Repeat("b", 64), AttemptSequence: 1, AttemptNonce: uuid.NewString()}
	fx.Exec(t, `UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1`, source.BindingID)
	if _, err = svc.AuthorizeTaskKnowledgeLaunch(ctx, caller, task.ID, req); !collaborationErrorCode(err, "knowledge_authorization_denied") {
		t.Fatalf("revoked managed binding authorized launch: %v", err)
	}
}
