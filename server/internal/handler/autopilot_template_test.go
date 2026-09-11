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
// creates one has to name it. Deleting the autopilot takes its trigger, rule
// versions and subscribers with it.
func cleanupTemplateAutopilot(t *testing.T, autopilotID string) {
	t.Helper()
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM autopilot WHERE id = $1`, autopilotID)
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

	// Provenance and the trigger row as the database actually holds them — the
	// response could be right while the write was not.
	var templateKey string
	var templateVersion int32
	dbfx.QueryRow(t, `SELECT template_key, template_version FROM autopilot WHERE id = $1`, created.Autopilot.ID).
		Scan(&templateKey, &templateVersion)
	if templateKey != template.Key || templateVersion != template.Version {
		t.Errorf("stored provenance = (%q, %d), want (%q, %d)", templateKey, templateVersion, template.Key, template.Version)
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
	dbfx.QueryRow(t, `SELECT execution_mode FROM autopilot WHERE id = $1`, created.Autopilot.ID).Scan(&executionMode)
	if executionMode != "run_only" {
		t.Errorf("execution_mode = %q, want run_only", executionMode)
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
