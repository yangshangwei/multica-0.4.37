package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Creating a standing automation from a built-in autopilot template, end to end
// through the HTTP handlers against a real database.

// cleanupTemplateAutopilot removes an autopilot this test created through the
// API. Rows the API creates are outside dbfx's cleanup ledger, so each test that
// creates one has to name it. Subscribers and rule versions have no foreign
// keys, so clean them up explicitly before deleting the autopilot.
func cleanupTemplateAutopilot(t *testing.T, autopilotID string) {
	t.Helper()
	t.Cleanup(func() {
		dbfx.Exec(t, `DELETE FROM autopilot_subscriber WHERE autopilot_id = $1`, autopilotID)
		dbfx.Exec(t, `DELETE FROM autopilot_rule_version WHERE autopilot_id = $1`, autopilotID)
		dbfx.Exec(t, `DELETE FROM autopilot WHERE id = $1`, autopilotID)
	})
}

// installAutopilotTriggerInsertFailure makes every INSERT into autopilot_trigger
// raise, so a from-template call fails at exactly the step whose rollback is the
// reason this endpoint is one transaction. Modelled on
// installAutopilotSubscriberInsertFailure, which does the same for subscribers.
func installAutopilotTriggerInsertFailure(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	functionName := fmt.Sprintf("autopilot_trigger_fail_fn_%d", suffix)
	triggerName := fmt.Sprintf("autopilot_trigger_fail_%d", suffix)
	t.Cleanup(func() {
		testPool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON autopilot_trigger`, triggerName))
		testPool.Exec(ctx, fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, functionName))
	})

	if _, err := testPool.Exec(ctx, fmt.Sprintf(`
CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	RAISE EXCEPTION 'forced autopilot trigger insert failure';
END;
$$;
`, functionName)); err != nil {
		t.Fatalf("install failure function: %v", err)
	}
	if _, err := testPool.Exec(ctx, fmt.Sprintf(`
CREATE TRIGGER %s
BEFORE INSERT ON autopilot_trigger
FOR EACH ROW EXECUTE FUNCTION %s();
`, triggerName, functionName)); err != nil {
		t.Fatalf("install failure trigger: %v", err)
	}
}

func TestListAutopilotTemplates_ReturnsTheRosterWithPrompts(t *testing.T) {
	var out struct {
		Templates []AutopilotTemplateResponse `json:"templates"`
	}
	testutil.Call(t, testHandler.ListAutopilotTemplates,
		newRequest("GET", "/api/autopilots/templates?language=zh", nil)).
		Want(http.StatusOK).JSON(&out)

	// The endpoint must serve the whole roster, not a truncated page of it. The
	// registry test owns which templates are in it; this owns that the handler
	// hands all of them over.
	if want := len(service.AutopilotTemplates()); len(out.Templates) != want {
		t.Fatalf("templates = %d, want %d", len(out.Templates), want)
	}
	for _, template := range out.Templates {
		if template.Key == "" {
			t.Errorf("template %+v is missing its key", template)
		}
		if template.Category == "" || template.CategoryLabel == "" {
			t.Errorf("%s: category = %q / label = %q, want both", template.Key, template.Category, template.CategoryLabel)
		}
		if strings.TrimSpace(template.Prompt) == "" {
			// The picker shows the prompt before a person adopts it. An empty body
			// here would mean the embedded file failed to load in the built binary.
			t.Errorf("%s: prompt is empty", template.Key)
		}
		canonical, ok := service.AutopilotTemplateByKey(template.Key)
		if !ok || template.Prompt != canonical.Prompt() {
			t.Errorf("%s: preview differs from the body used by creation", template.Key)
		}
		if template.ExecutionMode != "create_issue" && template.ExecutionMode != "run_only" {
			t.Errorf("%s: execution_mode = %q, want create_issue or run_only", template.Key, template.ExecutionMode)
		}
		// The cadence a card advertises must be the one the scheduler will accept.
		if _, err := service.ComputeNextRun(template.CronExpression, "UTC"); err != nil {
			t.Errorf("%s: cron %q does not parse: %v", template.Key, template.CronExpression, err)
		}
	}
	// language=zh must actually change the copy, or the parameter is decoration.
	audit := findAutopilotTemplate(t, out.Templates, "workday-repo-audit")
	if audit.Title == "Workday Repo Audit" {
		t.Errorf("title for language=zh = %q, want the localized label", audit.Title)
	}
}

// Exercise the real creation, schedule dispatch and daemon claim for every
// template. The registry owns wording checks; this owns unchanged delivery.
func TestAutopilotTemplateCreate_ChineseTemplatesDispatchVerbatim(t *testing.T) {
	ctx := context.Background()
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	for _, template := range service.AutopilotTemplates() {
		t.Run(template.Key, func(t *testing.T) {
			daemonID := "chinese-template-" + template.Key
			runtimeID := dbfx.Runtime(t, "Chinese template runtime", testutil.Cols{
				"daemon_id":    daemonID,
				"runtime_mode": "local",
				"provider":     "codex",
				"metadata":     testutil.Raw(`'{"capabilities":["rpc-v1"],"cli_version":"0.4.40"}'::jsonb`),
			})
			agentID := dbfx.Agent(t, "自动化验收智能体", runtimeID, testutil.Cols{"runtime_mode": "local"})
			var created CreateAutopilotFromTemplateResponse
			testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
				newRequest(http.MethodPost, "/api/autopilots/from-template", map[string]any{
					"template_key": template.Key,
					"assignee_id":  agentID,
					"language":     "zh",
					"timezone":     "Asia/Shanghai",
					// Unknown content fields must not override the registry snapshot.
					"title":                "Client override",
					"description":          "Client override",
					"prompt":               "Client override",
					"cron_expression":      "* * * * *",
					"execution_mode":       "invalid",
					"issue_title_template": "Client override",
					"subscribers": []map[string]string{
						{"user_type": "member", "user_id": testUserID},
					},
				})).Want(http.StatusCreated).JSON(&created)
			cleanupTemplateAutopilot(t, created.Autopilot.ID)
			dbfx.Cleanup(t, `DELETE FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1`, created.Autopilot.ID)
			dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)

			stored, err := testHandler.Queries.GetAutopilot(ctx, parseUUID(created.Autopilot.ID))
			if err != nil {
				t.Fatal(err)
			}
			if stored.Title != template.Title("zh") || !stored.Description.Valid || stored.Description.String != template.Prompt() {
				t.Fatal("stored Chinese title/body differs from the template preview")
			}
			if stored.ExecutionMode != template.ExecutionMode || stored.IssueTitleTemplate.String != template.IssueTitleTemplate {
				t.Fatal("client content fields overrode the template's execution configuration")
			}
			if created.Autopilot.TemplateKey != template.Key || created.Autopilot.TemplateVersion != template.Version {
				t.Fatal("creation lost template provenance")
			}
			if created.Trigger.CronExpression == nil || *created.Trigger.CronExpression != template.CronExpression ||
				created.Trigger.Timezone == nil || *created.Trigger.Timezone != "Asia/Shanghai" || created.Trigger.NextRunAt == nil {
				t.Fatal("creation lost the template's schedule or selected timezone")
			}

			run, err := testHandler.AutopilotService.DispatchAutopilotForPlan(ctx, stored, parseUUID(created.Trigger.ID), "schedule", nil, time.Now().UTC())
			if err != nil || run == nil {
				t.Fatalf("scheduled dispatch: run=%v err=%v", run, err)
			}
			if template.ExecutionMode == "create_issue" {
				if !run.IssueID.Valid {
					t.Fatalf("summary dispatch did not create an issue: status=%s", run.Status)
				}
				var title, description string
				dbfx.QueryRow(t, `SELECT title, description FROM issue WHERE id = $1`, run.IssueID).Scan(&title, &description)
				wantTitle := template.Title("zh") + " — " + run.TriggeredAt.Time.In(location).Format("2006-01-02")
				if title != wantTitle || !strings.HasPrefix(description, template.Prompt()+"\n") {
					t.Fatalf("summary lost its Chinese dated title or prompt: title=%q, want=%q", title, wantTitle)
				}
				if count := dbfx.Count(t, `SELECT count(*) FROM issue_subscriber WHERE issue_id = $1 AND user_type = 'member' AND user_id = $2 AND reason = 'autopilot'`, run.IssueID, testUserID); count != 1 {
					t.Fatalf("summary subscribers = %d, want 1", count)
				}
			} else {
				if run.IssueID.Valid || !run.TaskID.Valid {
					t.Fatalf("patrol must enqueue a task without pre-creating an issue: status=%s", run.Status)
				}
				if count := dbfx.Count(t, `SELECT count(*) FROM issue WHERE origin_type = 'autopilot' AND origin_id = $1`, stored.ID); count != 0 {
					t.Fatalf("patrol created %d issues before the agent found anything", count)
				}
			}

			var claimed struct {
				Task *AgentTaskResponse `json:"task"`
			}
			claimRequest := withURLParam(newDaemonTokenRequest(http.MethodPost,
				"/api/daemon/runtimes/"+runtimeID+"/claim", nil, testWorkspaceID, daemonID), "runtimeId", runtimeID)
			testutil.Call(t, testHandler.ClaimTaskByRuntime, claimRequest).Want(http.StatusOK).JSON(&claimed)
			if claimed.Task == nil || claimed.Task.AgentID != agentID {
				t.Fatal("dispatch did not produce a claimable task for the selected agent")
			}
			if template.ExecutionMode == "run_only" {
				if claimed.Task.AutopilotDescription != template.Prompt() || claimed.Task.AutopilotTitle != template.Title("zh") || claimed.Task.IssueID != "" {
					t.Fatal("daemon claim changed the Chinese patrol prompt or introduced an issue")
				}
			} else if claimed.Task.IssueID != uuidToString(run.IssueID) {
				t.Fatal("daemon claim points to a different summary issue")
			}
		})
	}
}

func TestAutopilotTemplateCreate_UserEditsRemainIndependent(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Editable Chinese automation", nil)
	var created CreateAutopilotFromTemplateResponse
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest(http.MethodPost, "/api/autopilots/from-template", map[string]any{
			"template_key": "daily-change-review", "assignee_id": agentID, "language": "zh",
		})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAutopilot(t, created.Autopilot.ID)

	const customBody = "# 团队变更回顾\n\n只检查本项目的导入流程，用中文列出实际发现。"
	const customTitle = "团队回顾 — {{date}}"
	request := withURLParam(newRequest(http.MethodPatch, "/api/autopilots/"+created.Autopilot.ID, map[string]any{
		"description": customBody, "issue_title_template": customTitle,
	}), "id", created.Autopilot.ID)
	testutil.Call(t, testHandler.UpdateAutopilot, request).Want(http.StatusOK)

	for _, language := range []string{"zh", "en", "ja", "ko"} {
		testutil.Call(t, testHandler.ListAutopilotTemplates,
			newRequest(http.MethodGet, "/api/autopilots/templates?language="+language, nil)).Want(http.StatusOK)
	}
	stored, err := testHandler.Queries.GetAutopilot(context.Background(), parseUUID(created.Autopilot.ID))
	if err != nil {
		t.Fatal(err)
	}
	if stored.Description.String != customBody || stored.IssueTitleTemplate.String != customTitle {
		t.Fatal("template reads overwrote the workspace's customized body or issue title")
	}
	if stored.Title != "每日变更回顾" {
		t.Fatalf("viewing another language renamed the persisted automation: %q", stored.Title)
	}
}

func findAutopilotTemplate(t *testing.T, templates []AutopilotTemplateResponse, key string) AutopilotTemplateResponse {
	t.Helper()
	for _, template := range templates {
		if template.Key == key {
			return template
		}
	}
	t.Fatalf("template %q not in response", key)
	return AutopilotTemplateResponse{}
}

// TestAutopilotTemplateCreate_WritesAutopilotAndTriggerTogether is the core
// contract: one call produces an ordinary autopilot AND its schedule trigger,
// carrying the template's prompt, cadence, execution mode and provenance.
func TestAutopilotTemplateCreate_WritesAutopilotAndTriggerTogether(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autopilot Template Assignee", nil)
	template, ok := service.AutopilotTemplateByKey("daily-change-review")
	if !ok {
		t.Fatal("daily-change-review template missing from the registry")
	}

	var created CreateAutopilotFromTemplateResponse
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest("POST", "/api/autopilots/from-template", map[string]any{
			"template_key": "daily-change-review",
			"assignee_id":  agentID,
			"timezone":     "Asia/Shanghai",
			"language":     "en",
		})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAutopilot(t, created.Autopilot.ID)

	if created.Autopilot.Title != template.Title("en") {
		t.Errorf("title = %q, want the template's %q", created.Autopilot.Title, template.Title("en"))
	}
	// The autopilot has no separate prompt column: description IS the brief the
	// dispatch path hands the agent (and writes into a create_issue run's issue
	// body). Storing the localized card blurb here instead would leave every run
	// briefed with one sentence of marketing copy.
	if created.Autopilot.Description == nil || *created.Autopilot.Description != template.Prompt() {
		t.Error("created autopilot's description is not the template's prompt verbatim")
	}
	if created.Autopilot.ExecutionMode != template.ExecutionMode {
		t.Errorf("execution_mode = %q, want %q", created.Autopilot.ExecutionMode, template.ExecutionMode)
	}
	// A create_issue template stamps {{date}} into the issue title so a month
	// of runs produces distinguishable issues rather than thirty rows named
	// "Daily Change Review". Absent here, dispatch falls back to the autopilot
	// title and the whole point of the template's cadence is lost.
	if created.Autopilot.IssueTitleTemplate == nil || *created.Autopilot.IssueTitleTemplate != template.IssueTitleTemplate {
		t.Errorf("issue_title_template = %v, want the template's %q", created.Autopilot.IssueTitleTemplate, template.IssueTitleTemplate)
	}
	// Provenance surfaced on the response, not just in the database: a client
	// needs it to offer the upgrade diff a later template version implies.
	if created.Autopilot.TemplateKey != template.Key || created.Autopilot.TemplateVersion != template.Version {
		t.Errorf("response provenance = (%q, %d), want (%q, %d)",
			created.Autopilot.TemplateKey, created.Autopilot.TemplateVersion, template.Key, template.Version)
	}
	if created.Autopilot.Status != "active" {
		t.Errorf("status = %q, want active", created.Autopilot.Status)
	}
	if created.Trigger.Kind != "schedule" || !created.Trigger.Enabled {
		t.Errorf("trigger kind = %q enabled = %v, want an enabled schedule", created.Trigger.Kind, created.Trigger.Enabled)
	}
	if created.Trigger.CronExpression == nil || *created.Trigger.CronExpression != template.CronExpression {
		t.Errorf("trigger cron = %v, want the template's %q", created.Trigger.CronExpression, template.CronExpression)
	}
	if created.Trigger.NextRunAt == nil {
		t.Error("trigger has no next_run_at; the scheduler claims work by that column")
	}

	// Provenance, the issue title template and the trigger row as the database
	// actually holds them — the response could be right while the write was not.
	var templateKey string
	var templateVersion int32
	var issueTitleTemplate *string
	dbfx.QueryRow(t, `SELECT template_key, template_version, issue_title_template FROM autopilot WHERE id = $1`, created.Autopilot.ID).
		Scan(&templateKey, &templateVersion, &issueTitleTemplate)
	if templateKey != template.Key || templateVersion != template.Version {
		t.Errorf("stored provenance = (%q, %d), want (%q, %d)", templateKey, templateVersion, template.Key, template.Version)
	}
	if issueTitleTemplate == nil || *issueTitleTemplate != template.IssueTitleTemplate {
		t.Errorf("stored issue_title_template = %v, want the template's %q", issueTitleTemplate, template.IssueTitleTemplate)
	}

	var cron, timezone string
	dbfx.QueryRow(t,
		`SELECT cron_expression, timezone FROM autopilot_trigger WHERE autopilot_id = $1 AND kind = 'schedule'`,
		created.Autopilot.ID).Scan(&cron, &timezone)
	if cron != template.CronExpression {
		t.Errorf("stored cron = %q, want %q", cron, template.CronExpression)
	}
	if timezone != "Asia/Shanghai" {
		t.Errorf("stored timezone = %q, want the requested Asia/Shanghai", timezone)
	}

	// A rule version per substantive publish — the autopilot's own v1 plus the
	// one the new trigger republishes. Without them a dispatch has no accountable
	// human (MUL-4302 §3.4).
	if versions := dbfx.Count(t, `SELECT count(*) FROM autopilot_rule_version WHERE autopilot_id = $1`, created.Autopilot.ID); versions != 2 {
		t.Errorf("rule versions = %d, want 2 (autopilot create + trigger create)", versions)
	}
}

// TestAutopilotTemplateCreate_RunOnlyTemplateKeepsItsMode pins the product
// decision that a patrol template does not pre-create an issue every hour: the
// mode travels from the template onto the row the scheduler reads.
func TestAutopilotTemplateCreate_RunOnlyTemplateKeepsItsMode(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autopilot Template Patrol Assignee", nil)

	var created CreateAutopilotFromTemplateResponse
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest("POST", "/api/autopilots/from-template", map[string]any{
			"template_key": "workday-repo-audit",
			"assignee_id":  agentID,
		})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAutopilot(t, created.Autopilot.ID)

	var executionMode string
	var issueTitleTemplate *string
	dbfx.QueryRow(t, `SELECT execution_mode, issue_title_template FROM autopilot WHERE id = $1`, created.Autopilot.ID).Scan(&executionMode, &issueTitleTemplate)
	if executionMode != "run_only" {
		t.Errorf("execution_mode = %q, want run_only", executionMode)
	}
	// run_only templates carry no issue title template: they never create the
	// issue themselves, so prefilling a title would be dead, misleading config.
	if issueTitleTemplate != nil {
		t.Errorf("issue_title_template = %q, want NULL for a run_only template", *issueTitleTemplate)
	}
	// Absent timezone means UTC, the same fallback the scheduler applies.
	if created.Trigger.Timezone == nil || *created.Trigger.Timezone != "UTC" {
		t.Errorf("trigger timezone = %v, want UTC by default", created.Trigger.Timezone)
	}
}

func TestAutopilotTemplateCreate_RejectsUnknownTemplateKey(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autopilot Template Unknown Key", nil)

	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest("POST", "/api/autopilots/from-template", map[string]any{
			"template_key": "no-such-template",
			"assignee_id":  agentID,
		})).Want(http.StatusBadRequest)
}

func TestAutopilotTemplateCreate_RequiresAssignee(t *testing.T) {
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest("POST", "/api/autopilots/from-template", map[string]any{
			"template_key": "hourly-queue-check",
		})).Want(http.StatusBadRequest)
}

// TestAutopilotTemplateCreate_ObserverAgentIsRefused closes the door the
// autonomy gate exists for: an agent that POST /api/autopilots would refuse must
// not be able to stand up the same standing automation through a template.
func TestAutopilotTemplateCreate_ObserverAgentIsRefused(t *testing.T) {
	actorID, taskID := autonomyTestAgent(t, "Autopilot Template Observer", "observer")
	assignee := createHandlerTestAgent(t, "Autopilot Template Observer Target", nil)

	req := asAgent(newRequest("POST", "/api/autopilots/from-template", map[string]any{
		"template_key": "release-readiness",
		"assignee_id":  assignee,
	}), actorID, taskID)
	testutil.Call(t, testHandler.CreateAutopilotFromTemplate, req).Want(http.StatusForbidden)

	if rows := dbfx.Count(t,
		`SELECT count(*) FROM autopilot WHERE workspace_id = $1 AND assignee_id = $2`,
		testWorkspaceID, assignee); rows != 0 {
		t.Fatalf("autopilot rows after the 403 = %d, want 0; the gate must run before the write", rows)
	}
}

// TestAutopilotTemplateCreate_RollsBackAutopilotWhenTriggerInsertFails is
// the reason this endpoint exists at all. The two-call client flow it replaces
// could leave an autopilot with no schedule behind — silent, inert, and the
// client's problem to clean up. Here the failing trigger insert must take the
// autopilot with it.
func TestAutopilotTemplateCreate_RollsBackAutopilotWhenTriggerInsertFails(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autopilot Template Rollback Assignee", nil)
	installAutopilotTriggerInsertFailure(t)

	testutil.Call(t, testHandler.CreateAutopilotFromTemplate,
		newRequest("POST", "/api/autopilots/from-template", map[string]any{
			"template_key": "hourly-queue-check",
			"assignee_id":  agentID,
		})).Want(http.StatusInternalServerError)

	// The status code alone would pass with the autopilot still committed, which
	// is exactly the bug. Ask the database. Scoped to this test's own assignee so
	// the count cannot be diluted by another test's rows.
	if rows := dbfx.Count(t,
		`SELECT count(*) FROM autopilot WHERE workspace_id = $1 AND assignee_id = $2`,
		testWorkspaceID, agentID); rows != 0 {
		t.Fatalf("orphan autopilot rows after the failed trigger insert = %d, want 0", rows)
	}
	if rows := dbfx.Count(t,
		`SELECT count(*) FROM autopilot_trigger t
		 JOIN autopilot a ON a.id = t.autopilot_id
		 WHERE a.assignee_id = $1`,
		agentID); rows != 0 {
		t.Fatalf("trigger rows after the failed insert = %d, want 0", rows)
	}
}
