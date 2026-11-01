package handler

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type workspaceDeleteAction string

const (
	workspaceDelete       workspaceDeleteAction = "delete"
	workspaceDeleteDetach workspaceDeleteAction = "detach"
	workspaceDeleteKeep   workspaceDeleteAction = "keep"
	workspaceDeleteSettle workspaceDeleteAction = "settle"
)

// workspaceDeletionManifest is the schema coverage contract for workspace
// teardown. Adding a table requires an explicit ownership decision here; the
// handler deletion graph must then implement that decision before CI passes.
var workspaceDeletionManifest = map[string]workspaceDeleteAction{
	// Collaboration identity, rollout and invalidation state belong to the
	// deployment/accounts, rather than to the workspace being deleted.
	"collaboration_account_fence":        workspaceDeleteKeep,
	"collaboration_account_invalidation": workspaceDeleteKeep,
	"collaboration_config_invalidation":  workspaceDeleteKeep,
	"collaboration_execution_gate":       workspaceDeleteKeep,
	// Scan positions belong to the runtime being deleted, not retained history.
	"collaboration_runtime_scan_cursor": workspaceDelete,
	// WorkspaceHasCollaborationRecords refuses source-workspace deletion,
	// including ended issues. Execution-workspace teardown cannot erase this
	// independently retained source history or its unfinished proof ledger.
	"issue_collaboration":                workspaceDeleteKeep,
	"issue_handoff":                      workspaceDeleteKeep,
	"issue_participant":                  workspaceDeleteKeep,
	"collaboration_attachment":           workspaceDeleteKeep,
	"collaboration_attachment_reference": workspaceDeleteKeep,
	"collaboration_attempt":              workspaceDeleteKeep,
	"collaboration_batch":                workspaceDeleteKeep,
	"collaboration_business_capability":  workspaceDeleteKeep,
	"collaboration_channel":              workspaceDeleteKeep,
	"collaboration_command":              workspaceDeleteKeep,
	"collaboration_delegation":           workspaceDeleteKeep,
	"collaboration_event":                workspaceDeleteKeep,
	"collaboration_execution_group":      workspaceDeleteKeep,
	"collaboration_grant":                workspaceDeleteKeep,
	"collaboration_group_dependency":     workspaceDeleteKeep,
	"collaboration_group_input":          workspaceDeleteKeep,
	"collaboration_memory_version":       workspaceDeleteKeep,
	"collaboration_notification":         workspaceDeleteKeep,
	"collaboration_operation":            workspaceDeleteKeep,
	"collaboration_resource_offer":       workspaceDeleteKeep,
	"collaboration_run":                  workspaceDeleteKeep,
	"collaboration_run_event":            workspaceDeleteKeep,
	"collaboration_runtime_challenge":    workspaceDeleteKeep,
	"collaboration_runtime_enrollment":   workspaceDeleteKeep,
	"collaboration_runtime_reservation":  workspaceDeleteKeep,
	"collaboration_session":              workspaceDeleteKeep,
	"collaboration_switch_consent":       workspaceDeleteKeep,
	"collaboration_switch_proposal":      workspaceDeleteKeep,
	"collaboration_target":               workspaceDeleteKeep,
	"collaboration_termination":          workspaceDeleteKeep,
	// Candidate versions and invalidation routing/history outlive target
	// deletion. Only the target-owned counters are removed, after final hints
	// are retained by DeleteKnowledgeTargetsInTx in the deletion transaction.
	"collaboration_knowledge_candidate":    workspaceDeleteKeep,
	"collaboration_knowledge_invalidation": workspaceDeleteKeep,
	"collaboration_knowledge_target":       workspaceDelete,
	// Workspace teardown purges I1 history/outbox through
	// DeleteWorkspaceIterationData in the existing owner transaction.
	"workspace_iteration_settings": workspaceDelete,
	"iteration":                    workspaceDelete,
	"iteration_participation":      workspaceDelete,
	"iteration_event":              workspaceDelete,
	"iteration_snapshot":           workspaceDelete,
	"iteration_operation":          workspaceDelete,
	"iteration_notification":       workspaceDelete,
	"workspace_triage_settings":    workspaceDelete,
	"issue_triage":                 workspaceDelete,
	"triage_intake_request":        workspaceDelete,
	"triage_action":                workspaceDelete,
	"triage_import_batch":          workspaceDelete,
	"triage_import_row":            workspaceDelete,
	"triage_notification":          workspaceDelete,
	"managed_installation":         workspaceDeleteKeep,
	"installation_user":            workspaceDeleteKeep,
	"installation_report_cursor":   workspaceDeleteKeep,
	"installation_daemon_binding":  workspaceDeleteSettle,
	"installation_challenge":       workspaceDeleteSettle,
	// Alerts and detector cursors belong to the organization, including after
	// a workspace is removed; recovery observes task deletion independently.
	"admin_alert":                        workspaceDeleteKeep,
	"admin_alert_detector_state":         workspaceDeleteKeep,
	"admin_audit_event":                  workspaceDeleteKeep,
	"admin_operation":                    workspaceDeleteKeep,
	"organization":                       workspaceDeleteKeep,
	"organization_workspace":             workspaceDelete,
	"platform_role_binding":              workspaceDeleteKeep,
	"platform_admin_initialization":      workspaceDeleteKeep,
	"activity_log":                       workspaceDelete,
	"agent":                              workspaceDelete,
	"agent_approval_request":             workspaceDelete,
	"agent_builder_draft":                workspaceDelete,
	"agent_invocation_target":            workspaceDelete,
	"agent_runtime":                      workspaceDelete,
	"agent_skill":                        workspaceDelete,
	"agent_task_queue":                   workspaceDelete,
	"agent_to_label":                     workspaceDelete,
	"attachment":                         workspaceDelete,
	"autopilot":                          workspaceDelete,
	"autopilot_collaborator":             workspaceDelete,
	"autopilot_quota_period":             workspaceDelete,
	"autopilot_quota_reservation":        workspaceDelete,
	"autopilot_rule_version":             workspaceDelete,
	"autopilot_run":                      workspaceDelete,
	"autopilot_subscriber":               workspaceDelete,
	"autopilot_trigger":                  workspaceDelete,
	"channel_binding_token":              workspaceDelete,
	"channel_chat_context_generation":    workspaceDelete,
	"channel_chat_session_binding":       workspaceDelete,
	"channel_inbound_audit":              workspaceDelete,
	"channel_inbound_message_dedup":      workspaceDelete,
	"channel_installation":               workspaceDelete,
	"channel_media_pending_object":       workspaceDeleteSettle,
	"channel_outbound_card_message":      workspaceDelete,
	"channel_outbound_message":           workspaceDelete,
	"channel_task_delivery":              workspaceDelete,
	"channel_user_binding":               workspaceDelete,
	"chat_draft_restore":                 workspaceDelete,
	"chat_message":                       workspaceDelete,
	"chat_pinned_agent":                  workspaceDelete,
	"chat_session":                       workspaceDelete,
	"client_usage_daily":                 workspaceDeleteDetach,
	"comment":                            workspaceDelete,
	"comment_reaction":                   workspaceDelete,
	"contact_sales_inquiry":              workspaceDeleteKeep,
	"daemon_connection":                  workspaceDelete,
	"daemon_token":                       workspaceDelete,
	"dingtalk_group_presence":            workspaceDelete,
	"dingtalk_bot_identity":              workspaceDelete,
	"dingtalk_group_route":               workspaceDelete,
	"feedback":                           workspaceDeleteDetach,
	"github_installation":                workspaceDelete,
	"github_pending_check_suite":         workspaceDelete,
	"github_pending_installation":        workspaceDeleteKeep,
	"github_pull_request":                workspaceDelete,
	"github_pull_request_check_run":      workspaceDelete,
	"github_pull_request_check_suite":    workspaceDelete,
	"inbox_item":                         workspaceDelete,
	"issue":                              workspaceDelete,
	"issue_view":                         workspaceDelete,
	"issue_view_preference":              workspaceDelete,
	"issue_dependency":                   workspaceDelete,
	"issue_label":                        workspaceDelete,
	"issue_property":                     workspaceDelete,
	"issue_pull_request":                 workspaceDelete,
	"issue_reaction":                     workspaceDelete,
	"issue_source_context":               workspaceDelete,
	"issue_source_context_object_intent": workspaceDeleteSettle,
	"issue_status":                       workspaceDelete,
	"issue_subscriber":                   workspaceDelete,
	"issue_to_label":                     workspaceDelete,
	"issue_vcs_pull_request":             workspaceDelete,
	"lark_binding_token":                 workspaceDelete,
	"lark_chat_session_binding":          workspaceDelete,
	"lark_inbound_audit":                 workspaceDelete,
	"lark_inbound_message_dedup":         workspaceDelete,
	"lark_installation":                  workspaceDelete,
	"lark_outbound_card_message":         workspaceDelete,
	"lark_user_binding":                  workspaceDelete,
	"member":                             workspaceDelete,
	"agent_mcp_server":                   workspaceDelete,
	"workspace_mcp_server":               workspaceDelete,
	"notification_preference":            workspaceDelete,
	"personal_access_token":              workspaceDeleteKeep,
	"user_password_credential":           workspaceDeleteKeep,
	"pinned_item":                        workspaceDelete,
	"plugin_installation":                workspaceDelete,
	"plugin_hook_schedule":               workspaceDelete,
	"plugin_invocation":                  workspaceDelete,
	"plugin_storage":                     workspaceDelete,
	"plugin_secret":                      workspaceDelete,
	"plugin_package":                     workspaceDelete,
	"plugin_package_version":             workspaceDelete,
	"plugin_package_file":                workspaceDelete,
	"project":                            workspaceDelete,
	"project_resource":                   workspaceDelete,
	"project_state_change":               workspaceDelete,
	"project_update":                     workspaceDelete,
	"project_update_revision":            workspaceDelete,
	"project_update_request":             workspaceDelete,
	"project_update_notification":        workspaceDelete,
	"quick_action":                       workspaceDelete,
	"runtime_profile":                    workspaceDelete,
	"schema_migrations":                  workspaceDeleteKeep,
	"seat_capacity_outbox":               workspaceDeleteSettle,
	"skill":                              workspaceDelete,
	"skill_file":                         workspaceDelete,
	"skill_to_label":                     workspaceDelete,
	"squad":                              workspaceDelete,
	"squad_member":                       workspaceDelete,
	"sys_cron_executions":                workspaceDeleteKeep,
	"task_message":                       workspaceDelete,
	"task_token":                         workspaceDelete,
	"task_usage":                         workspaceDelete,
	"task_usage_hourly":                  workspaceDelete,
	"task_usage_hourly_dirty":            workspaceDelete,
	"task_usage_hourly_rollup_state":     workspaceDeleteKeep,
	"user":                               workspaceDeleteKeep,
	"user_composio_connection":           workspaceDeleteKeep,
	"vcs_commit_status":                  workspaceDelete,
	"vcs_connection":                     workspaceDelete,
	"vcs_pull_request":                   workspaceDelete,
	"verification_code":                  workspaceDeleteKeep,
	"webhook_delivery":                   workspaceDelete,
	"workspace":                          workspaceDelete,
	"workspace_invitation":               workspaceDelete,
	"workspace_share_link":               workspaceDelete,
}

func TestWorkspaceDeletionManifestPreservesKnowledgeHistory(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	source := knowledgeHookWorkspace(t)
	target := knowledgeHookWorkspace(t)
	issue := collaborationBoundaryIssue(t, source)
	dbfx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	ctx := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	svc := service.NewCollaborationService(testHandler.Queries, testPool, testHandler.IssueService)
	if _, err := svc.UpdateMemory(ctx, service.CollaborationMemoryUpdateParams{IssueID: parseUUID(issue), RequestID: uuid.NewString(), Content: "retained knowledge source"}); err != nil {
		t.Fatal(err)
	}
	p := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 1, CandidateID: uuid.NewString(), RequestID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: target}, Summary: "retained summary", Content: "retained candidate body", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: issue, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("retained knowledge source")}}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue"}}}
	if _, err := svc.ProposeKnowledge(ctx, parseUUID(issue), p); err != nil {
		t.Fatal(err)
	}
	approval := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}
	if _, err := svc.ApproveKnowledge(ctx, parseUUID(p.CandidateID), approval); err != nil {
		t.Fatal(err)
	}
	op, err := testHandler.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: parseUUID(issue), ActorUserID: parseUUID(testUserID), RequestID: approval.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	hint, err := testHandler.Queries.GetKnowledgeTargetHintForCause(ctx, db.GetKnowledgeTargetHintForCauseParams{CauseID: op.ID, TargetKind: pgtype.Text{String: "workspace", Valid: true}, TargetID: parseUUID(target)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = testHandler.Queries.MarkKnowledgeTargetHintDelivered(ctx, hint.ID); err != nil {
		t.Fatal(err)
	}
	cause := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if err = service.EnqueueKnowledgeSourceInvalidation(ctx, testHandler.Queries, cause, "source_issue", parseUUID(issue)); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var value string
		dbfx.QueryRow(t, `SELECT jsonb_build_object('candidate',(SELECT to_jsonb(c) FROM collaboration_knowledge_candidate c WHERE id=$1),'retained_jobs',(SELECT jsonb_agg(to_jsonb(j) ORDER BY j.id) FROM collaboration_knowledge_invalidation j WHERE j.id=$2 OR j.cause_id=$3))::text`, p.CandidateID, hint.ID, cause).Scan(&value)
		return value
	}
	before := snapshot()
	r := newRequest("DELETE", "/api/workspaces/"+target, nil)
	r.Header.Set("X-Workspace-ID", target)
	testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(r, "id", target)).Want(204)
	if snapshot() != before {
		t.Fatal("target deletion changed retained candidate or pending/delivered invalidation history")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE workspace_id=$1`, target) != 0 || dbfx.Count(t, `SELECT count(*) FROM workspace WHERE id=$1`, target) != 0 {
		t.Fatal("target deletion retained target-owned counters or parent")
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint' AND state='pending' AND knowledge_revision=2`, target) != 1 {
		t.Fatal("target deletion did not preserve its final hint before removing the counter")
	}
	r = newRequest("DELETE", "/api/workspaces/"+source, nil)
	r.Header.Set("X-Workspace-ID", source)
	response := testutil.Call(t, testHandler.DeleteWorkspace, withURLParam(r, "id", source)).Want(409)
	if response.Map()["code"] != "workspace_has_collaboration_records" || snapshot() != before {
		t.Fatal("source history deletion guard or retained knowledge changed")
	}
}

func TestWorkspaceDeletionManifestCoversPublicSchema(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	rows, err := testPool.Query(context.Background(), `
SELECT tablename
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY tablename
`)
	if err != nil {
		t.Fatalf("list public tables: %v", err)
	}
	defer rows.Close()

	actual := make(map[string]struct{}, len(workspaceDeletionManifest))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan public table: %v", err)
		}
		actual[table] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate public tables: %v", err)
	}

	var unclassified, missing []string
	for table := range actual {
		if _, ok := workspaceDeletionManifest[table]; !ok {
			unclassified = append(unclassified, table)
		}
	}
	for table := range workspaceDeletionManifest {
		if _, ok := actual[table]; !ok {
			missing = append(missing, table)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(missing)
	if len(unclassified) > 0 || len(missing) > 0 {
		t.Fatalf("workspace deletion manifest drift: unclassified=%v missing=%v", unclassified, missing)
	}

	workspaceColumns, err := testPool.Query(context.Background(), `
SELECT table_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND column_name = 'workspace_id'
`)
	if err != nil {
		t.Fatalf("list workspace_id columns: %v", err)
	}
	defer workspaceColumns.Close()

	withWorkspaceID := make(map[string]struct{})
	for workspaceColumns.Next() {
		var table string
		if err := workspaceColumns.Scan(&table); err != nil {
			t.Fatalf("scan workspace_id table: %v", err)
		}
		withWorkspaceID[table] = struct{}{}
	}
	if err := workspaceColumns.Err(); err != nil {
		t.Fatalf("iterate workspace_id tables: %v", err)
	}

	for table, action := range workspaceDeletionManifest {
		_, hasWorkspaceID := withWorkspaceID[table]
		switch action {
		case workspaceDeleteKeep:
			if hasWorkspaceID {
				t.Errorf("KEEP table %s gained workspace_id; classify its teardown behavior", table)
			}
		case workspaceDeleteDetach, workspaceDeleteSettle:
			if !hasWorkspaceID {
				t.Errorf("%s table %s lost workspace_id; update its teardown selector", action, table)
			}
		}
	}
}
