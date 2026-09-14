package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// This fixture executes the reporting protocol through real handlers; it does
// not evaluate whether a language model follows the reporting instructions.
func TestAutopilotTemplateCreate_ProgressReportsCloseOutOnlyTheirIssue(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name     string
		key      string
		autonomy string
	}{
		{"daily contributor", "daily-progress-report", "contributor"},
		{"weekly contributor", "weekly-progress-report", "contributor"},
		{"observer needs review permission", "daily-progress-report", "observer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			template, ok := service.AutopilotTemplateByKey(tt.key)
			if !ok {
				t.Fatal("progress report template missing")
			}
			projectID := dbfx.Project(t, "Progress report scope")
			// Create through the API so the workspace counter is also advanced
			// before the real autopilot dispatch allocates its report number.
			var source IssueResponse
			testutil.Call(t, testHandler.CreateIssue,
				newRequest(http.MethodPost, "/api/issues", map[string]any{
					"title": "Business work remains in progress", "project_id": projectID,
					"status": "in_progress", "priority": "high",
					"assignee_type": "member", "assignee_id": testUserID,
				})).Want(http.StatusCreated).JSON(&source)
			sourceID := source.ID
			dbfx.Cleanup(t, `DELETE FROM issue WHERE id = $1`, sourceID)
			sourceBefore, err := testHandler.Queries.GetIssue(ctx, parseUUID(sourceID))
			if err != nil {
				t.Fatal(err)
			}
			daemonID := "progress-report-" + tt.key + "-" + tt.autonomy
			runtimeID := dbfx.Runtime(t, "Progress report runtime", testutil.Cols{
				"daemon_id": daemonID, "runtime_mode": "local", "provider": "codex",
				"metadata": testutil.Raw(`'{"capabilities":["rpc-v1"],"cli_version":"0.4.40"}'::jsonb`),
			})
			agentID := dbfx.Agent(t, "进展报告员", runtimeID, testutil.Cols{
				"runtime_mode": "local", "autonomy_level": tt.autonomy,
			})
			var created CreateAutopilotFromTemplateResponse
			testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
				newRequest(http.MethodPost, "/api/autopilots/from-template", map[string]any{
					"template_key": tt.key, "assignee_id": agentID, "project_id": projectID,
					"language": "zh", "timezone": "Asia/Shanghai",
				})).Want(http.StatusCreated).JSON(&created)
			cleanupTemplateAutopilot(t, created.Autopilot.ID)
			dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1`, created.Autopilot.ID)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)

			autopilot, err := testHandler.Queries.GetAutopilot(ctx, parseUUID(created.Autopilot.ID))
			if err != nil {
				t.Fatal(err)
			}
			run, err := testHandler.AutopilotService.DispatchAutopilotForPlan(ctx, autopilot,
				parseUUID(created.Trigger.ID), "schedule", nil, time.Now().UTC())
			if err != nil || run == nil || !run.IssueID.Valid {
				t.Fatalf("report dispatch: run=%v err=%v", run, err)
			}
			reportID := uuidToString(run.IssueID)
			dbfx.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, reportID)
			var claimed struct {
				Task *AgentTaskResponse `json:"task"`
			}
			testutil.Call(t, testHandler.ClaimTaskByRuntime,
				withURLParam(newDaemonTokenRequest(http.MethodPost,
					"/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil, testWorkspaceID, daemonID),
					"runtimeId", runtimeID)).Want(http.StatusOK).JSON(&claimed)
			if claimed.Task == nil || claimed.Task.AgentID != agentID || claimed.Task.IssueID != reportID {
				t.Fatal("the selected reporter did not receive this run's precreated issue")
			}
			taskID := claimed.Task.ID
			testutil.Call(t, testHandler.StartTask,
				withURLParam(newDaemonTokenRequest(http.MethodPost,
					"/api/daemon/tasks/"+taskID+"/start", nil, testWorkspaceID, daemonID),
					"taskId", taskID)).Want(http.StatusOK)

			// A rejected delivery must not persist a report comment.
			testutil.Call(t, testHandler.CreateComment,
				withURLParam(asAgent(newRequest(http.MethodPost, "/api/issues/"+reportID+"/comments",
					map[string]any{"content": ""}), agentID, taskID), "id", reportID)).Want(http.StatusBadRequest)
			if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1`, reportID); count != 0 {
				t.Fatalf("rejected delivery persisted %d comments", count)
			}

			content := fmt.Sprintf("## %s\n\n进行中：[%s](/issues/%s)，状态为 in_progress；未修改业务任务。",
				template.Title("zh"), sourceID, sourceID)
			var comment CommentResponse
			testutil.Call(t, testHandler.CreateComment,
				withURLParam(asAgent(newRequest(http.MethodPost, "/api/issues/"+reportID+"/comments",
					map[string]any{"content": content}), agentID, taskID), "id", reportID)).
				Want(http.StatusCreated).JSON(&comment)
			if comment.Content != content || comment.IssueID != reportID || comment.AuthorID != agentID || comment.AuthorType != "agent" {
				t.Fatal("the report comment lost its content, destination, or agent attribution")
			}
			if comment.SourceTaskID == nil || *comment.SourceTaskID != taskID {
				t.Fatal("the report comment lost its execution provenance")
			}
			afterComment, err := testHandler.Queries.GetAutopilotRun(ctx, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if afterComment.Status == "completed" || afterComment.CompletedAt.Valid {
				t.Fatal("comment publication alone must not pretend the automation run has completed")
			}

			wantStatus := http.StatusOK
			if tt.autonomy == "observer" {
				wantStatus = http.StatusForbidden
			}
			testutil.Call(t, testHandler.UpdateIssue,
				withURLParam(asAgent(newRequest(http.MethodPut, "/api/issues/"+reportID,
					map[string]any{"status": "in_review"}), agentID, taskID), "id", reportID)).Want(wantStatus)
			report, err := testHandler.Queries.GetIssue(ctx, run.IssueID)
			if err != nil {
				t.Fatal(err)
			}
			// The handler fixture does not register cmd/server event listeners.
			// Invoke the production callback on the persisted issue; browser E2E
			// owns the full listener wiring from issue:updated to this callback.
			testHandler.AutopilotService.SyncRunFromIssue(ctx, report)
			finalRun, err := testHandler.Queries.GetAutopilotRun(ctx, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.autonomy == "observer" {
				if report.Status == "in_review" || finalRun.Status == "completed" || finalRun.CompletedAt.Valid {
					t.Fatal("an observer bypassed the report review permission")
				}
			} else if report.Status != "in_review" || finalRun.Status != "completed" || !finalRun.CompletedAt.Valid {
				t.Fatalf("report/run = %s/%s, completed_at=%v; want in_review/completed with a timestamp",
					report.Status, finalRun.Status, finalRun.CompletedAt)
			}
			if uuidToString(report.ProjectID) != projectID || uuidToString(finalRun.IssueID) != reportID {
				t.Fatal("report closeout changed the project or run issue linkage")
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1`, created.Autopilot.ID); count != 1 {
				t.Fatalf("report created %d summary issues, want exactly one", count)
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1`, reportID); count != 1 {
				t.Fatalf("report persisted %d comments, want exactly one", count)
			}
			sourceAfter, err := testHandler.Queries.GetIssue(ctx, parseUUID(sourceID))
			if err != nil {
				t.Fatal(err)
			}
			if sourceAfter.Status != sourceBefore.Status || sourceAfter.Priority != sourceBefore.Priority ||
				sourceAfter.AssigneeID != sourceBefore.AssigneeID || sourceAfter.AssigneeType != sourceBefore.AssigneeType ||
				sourceAfter.Revision != sourceBefore.Revision || sourceAfter.UpdatedAt != sourceBefore.UpdatedAt {
				t.Fatal("report publication or closeout changed a source business task")
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1`, sourceID); count != 0 {
				t.Fatalf("report added %d comments to a source business task", count)
			}
		})
	}
}

func TestAutopilotTemplateCreate_WeeklyUpgradePreservesExistingCopy(t *testing.T) {
	const oldPrompt = "# 团队周报\n\n沿用团队自定义的统计口径，由人工提交审核。"
	const oldTitleTemplate = "团队周报 — {{date}}"
	agentID := createHandlerTestAgent(t, "Existing weekly reporter", nil)
	oldID := dbfx.Insert(t, "autopilot", testutil.Cols{
		"workspace_id": testWorkspaceID, "title": "团队周报", "description": oldPrompt,
		"assignee_id": agentID, "execution_mode": "create_issue", "issue_title_template": oldTitleTemplate,
		"created_by_type": "member", "created_by_id": testUserID,
		"template_key": "weekly-progress-report", "template_version": 1,
	})
	for _, language := range []string{"zh", "en", "ko", "ja"} {
		testutil.Call(t, testHandler.ListAutopilotTemplates,
			newRequest(http.MethodGet, "/api/autopilots/templates?language="+language, nil)).Want(http.StatusOK)
	}
	var created CreateAutopilotFromTemplateResponse
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest(http.MethodPost, "/api/autopilots/from-template", map[string]any{
			"template_key": "weekly-progress-report", "assignee_id": agentID, "language": "zh",
		})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAutopilot(t, created.Autopilot.ID)
	if created.Autopilot.TemplateVersion != 2 {
		t.Fatalf("new weekly version = %d, want 2", created.Autopilot.TemplateVersion)
	}
	var response struct {
		Autopilot AutopilotResponse `json:"autopilot"`
	}
	testutil.Call(t, testHandler.GetAutopilot,
		withURLParam(newRequest(http.MethodGet, "/api/autopilots/"+oldID, nil), "id", oldID)).Want(http.StatusOK).JSON(&response)
	existing := response.Autopilot
	if existing.TemplateVersion != 1 || existing.Title != "团队周报" ||
		existing.Description == nil || *existing.Description != oldPrompt ||
		existing.IssueTitleTemplate == nil || *existing.IssueTitleTemplate != oldTitleTemplate {
		t.Fatal("reading or adopting the new weekly default rewrote the existing workspace copy")
	}
}
