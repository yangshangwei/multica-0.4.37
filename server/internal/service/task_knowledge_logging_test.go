package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

const serverLogPrivateSentinel = "RR2_PRIVATE_APPROVED_STDIN_ECHOED_TO_STDERR"
const serverLogLegacySentinel = "RR2_ORDINARY_LEGACY_DIAGNOSTIC"
const serverLogReceipt = "RR2_PERSISTED_EVIDENCE_VERIFIED"

// Run real server logging in a separate test process so capture never mutates
// the process-wide default logger or interferes with parallel package tests.
func TestKnowledgeServerFailureLogsUsePersistedState(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("explicit isolated DATABASE_URL is required")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"service_prepared", "handler_prepared", "service_retained", "handler_retained", "handler_refusal", "handler_stale_fence", "service_legacy", "handler_legacy_with_fence"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestKnowledgeServerLogProbe$", "-test.v", "-test.timeout=25s")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "RR2_SERVER_LOG_CASE=") && !strings.HasPrefix(entry, "MULTICA_AUTH_MODE=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "RR2_SERVER_LOG_CASE="+mode, "MULTICA_AUTH_MODE=legacy")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("server probe failed: %v\n%s", err, output)
			}
			text := string(output)
			if !strings.Contains(text, serverLogReceipt) {
				t.Fatal("probe did not verify stored failure evidence")
			}
			legacy := strings.Contains(mode, "legacy")
			if !legacy && strings.Contains(text, serverLogPrivateSentinel) {
				t.Fatal("approved private stderr reached generic server logging")
			}
			if legacy && !strings.Contains(text, serverLogLegacySentinel) {
				t.Fatal("legacy diagnostic logging unexpectedly changed")
			}
			if mode != "handler_stale_fence" {
				reason := taskfailure.ReasonAgentProcessFailure.String()
				if mode == "handler_refusal" {
					reason = taskfailure.ReasonKnowledgeContextChanged.String()
				}
				records := 0
				for _, line := range strings.Split(text, "\n") {
					if !strings.Contains(line, " WARN task failed ") && !strings.Contains(line, " INFO task failed ") {
						continue
					}
					records++
					if !strings.Contains(line, "task_id=") || !strings.Contains(line, "failure_reason="+reason) {
						t.Fatal("safe task identity or persisted canonical reason was lost")
					}
				}
				want := 1
				if strings.HasPrefix(mode, "handler_") {
					want = 2
				}
				if records != want {
					t.Fatalf("observed %d failure records, want %d", records, want)
				}
			}
		})
	}
}

// This helper is selected only by the subprocess above; it uses its ordinary
// process logger unchanged and returns no private content in its test receipt.
func TestKnowledgeServerLogProbe(t *testing.T) {
	mode := os.Getenv("RR2_SERVER_LOG_CASE")
	if mode == "" {
		return
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Fatal("explicit isolated DATABASE_URL is required")
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = pool.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(pool, "", "")
	owner := fx.User(t, "RR2 owner", "rr2-owner-"+uuid.NewString()+"@example.test")
	other := fx.User(t, "RR2 source handler", "rr2-handler-"+uuid.NewString()+"@example.test")
	workspace := fx.Workspace(t, "RR2 isolated workspace", "rr2-"+uuid.NewString())
	fx.Member(t, workspace, owner, "owner")
	fx.WorkspaceID, fx.UserID = workspace, owner
	q := db.New(pool)
	bus := events.New()
	h := handler.New(q, pool, nil, bus, service.NewEmailService(), nil, nil, analytics.NoopClient{}, handler.Config{})
	svc := h.TaskService
	svc.KnowledgeContextEnabled = true
	daemonID := "rr2-" + uuid.NewString()
	runtimeID := fx.Runtime(t, "RR2 runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": daemonID})
	agentID := fx.Agent(t, "RR2 agent", runtimeID, testutil.Cols{"runtime_mode": "local"})
	consumer := fx.Issue(t, "RR2 consumer", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
	fx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT max(number) FROM issue WHERE workspace_id=$1) WHERE id=$1`, workspace)
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": consumer, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1, "originator_user_id": owner, "accountable_user_id": owner})
	token := "mdt_" + uuid.NewString()
	fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(token), "workspace_id": workspace, "daemon_id": daemonID, "user_id": owner, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	ctx := auth.WithAuthenticatedPrincipal(t.Context(), auth.AuthenticatedPrincipal{UserID: owner, Kind: auth.CredentialJWT})
	task, err := q.GetAgentTask(ctx, util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	fence := &protocol.TaskKnowledgeFence{ClaimGeneration: "1", DispatchedAt: task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano)}
	diagnostic := serverLogLegacySentinel
	if !strings.Contains(mode, "legacy") {
		var gate bool
		fx.QueryRow(t, `SELECT creation_enabled FROM collaboration_execution_gate WHERE id=1`).Scan(&gate)
		fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
		fx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=$1 WHERE id=1`, gate)
		collab := service.NewCollaborationService(q, pool, service.NewIssueService(q, pool, nil, analytics.NoopClient{}, nil))
		source, err := collab.Create(ctx, service.CollaborationCreateParams{SourceWorkspaceID: util.MustParseUUID(workspace), HandlerID: util.MustParseUUID(other), RequestID: uuid.NewString(), Title: "RR2 source", Description: "Private source fixture"})
		if err != nil {
			t.Fatal(err)
		}
		// Create owns these collaboration rows; the ordinary fixture only removes
		// their parents. Clean this exact source first, retaining knowledge history.
		fx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, source.Issue.ID)
		for _, table := range []string{"issue_collaboration", "issue_participant", "collaboration_session", "issue_handoff", "collaboration_event", "collaboration_notification", "collaboration_memory_version"} {
			fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, source.Issue.ID)
		}
		fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, source.Issue.ID)
		fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, workspace)
		fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{owner, other})
		if _, err = collab.UpdateMemory(ctx, service.CollaborationMemoryUpdateParams{IssueID: source.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 0, Content: serverLogPrivateSentinel}); err != nil {
			t.Fatal(err)
		}
		p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: workspace}, Summary: "RR2 summary", Content: serverLogPrivateSentinel, Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: util.UUIDToString(source.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash(serverLogPrivateSentinel)}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}}}
		if _, err = collab.ProposeKnowledge(ctx, source.Issue.ID, p); err != nil {
			t.Fatal(err)
		}
		if _, err = collab.ApproveKnowledge(ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
			t.Fatal(err)
		}
		d, _, err := svc.BuildTaskKnowledgeClaim(ctx, task, &protocol.TaskKnowledgeInvocation{ProtocolVersion: 1, Provider: "claude", Mode: "local", Builtin: true, ConfigurationID: "rr2-config"}, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = svc.FinalizeTaskClaim(ctx, task, db.CreateTaskTokenParams{TokenHash: auth.HashToken("mat_" + uuid.NewString()), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: util.MustParseUUID(workspace), UserID: util.MustParseUUID(owner), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, d); err != nil {
			t.Fatal(err)
		}
		prepared, err := svc.PrepareTaskKnowledge(ctx, service.TaskKnowledgeCaller{RuntimeID: task.RuntimeID, Kind: "daemon_token", TokenHash: auth.HashToken(token)}, task.ID, protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)})
		if err != nil {
			t.Fatal(err)
		}
		if len(prepared.Contributions) != 1 || prepared.Contributions[0].Content != serverLogPrivateSentinel {
			t.Fatal("actual preparation did not authorize fixture source")
		}
		diagnostic = prepared.Contributions[0].Content
		if strings.Contains(mode, "retained") || mode == "handler_stale_fence" {
			fx.Exec(t, `UPDATE agent_task_queue SET claim_generation=2,dispatched_at=now() WHERE id=$1`, task.ID)
			svc.KnowledgeContextEnabled = false
			if mode != "handler_stale_fence" {
				row, err := q.GetAgentTask(ctx, task.ID)
				if err != nil {
					t.Fatal(err)
				}
				fence = &protocol.TaskKnowledgeFence{ClaimGeneration: "2", DispatchedAt: row.DispatchedAt.Time.UTC().Format(time.RFC3339Nano)}
			}
		}
	}
	errorText := "claude exited with error: exit status 1\n[claude stderr] " + diagnostic
	reason := taskfailure.ReasonAgentProcessFailure.String()
	if mode == "handler_refusal" {
		reason = taskfailure.ReasonKnowledgeContextChanged.String()
	}
	if strings.HasPrefix(mode, "service_") {
		if _, err = svc.FailTask(ctx, task.ID, errorText, "", "", "", reason, false, "", ""); err != nil {
			t.Fatal(err)
		}
	} else {
		// Leave the request reason empty for a Provider failure: the handler's
		// telemetry must use the canonical reason returned by persistence.
		requestedReason := reason
		if mode != "handler_refusal" {
			requestedReason = ""
		}
		body := handler.TaskFailRequest{Error: errorText, FailureReason: requestedReason, KnowledgeFence: fence}
		req := testutil.WithURLParams(testutil.JSONRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/fail", body), "taskId", taskID)
		req.Header.Set("Authorization", "Bearer "+token)
		wrapped := middleware.DaemonAuth(q, nil, nil, nil)(http.HandlerFunc(h.FailTask))
		response := testutil.Call(t, wrapped.ServeHTTP, req)
		if mode == "handler_stale_fence" {
			response.Want(409)
		} else {
			response.Want(200)
		}
	}
	stored, err := q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "handler_stale_fence" {
		if stored.Status != "dispatched" || stored.Error.Valid {
			t.Fatal("stale failure changed the current task")
		}
	} else {
		if stored.Status != "failed" || stored.Error.String != errorText || stored.FailureReason.String != reason {
			t.Fatal("stored authorized failure evidence or canonical reason changed")
		}
	}
	encoded, _ := json.Marshal(map[string]any{"mode": mode, "persisted_evidence": true})
	fmt.Println(serverLogReceipt, string(encoded))
}
