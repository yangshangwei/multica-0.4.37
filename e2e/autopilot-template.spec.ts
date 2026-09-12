import { test, expect, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";
import { waitForPageText } from "./helpers";

// Creating an autopilot from a built-in template, end to end through the real app.
//
// Auth and the workspace go through the real backend, as every spec does. The
// template catalog, the agent list and the create call are intercepted at the
// network boundary: the flow needs a selectable agent to become submittable, and
// a real one would mean a live daemon. Intercepting the create is also how this
// test asserts the thing that matters most — the exact body the flow sends.
//
// That body assertion is the point of the spec. The server deliberately refuses
// to accept prompt / cron / execution_mode / title on from-template, so that a
// client cannot claim a template's provenance while supplying its own prompt. A
// UI regression that started sending those fields would still "work" against a
// forgiving backend, and only this assertion would catch it.

const E2E_WORKER =
  process.env.TEST_PARALLEL_INDEX ?? process.env.TEST_WORKER_INDEX ?? "0";
const E2E_RUN_ID =
  process.env.E2E_RUN_ID ?? `${Date.now().toString(36)}-${process.pid.toString(36)}`;
const EMAIL = `e2e-autopilot-template-${E2E_WORKER}-${E2E_RUN_ID}@multica.ai`;
const NAME = "E2E Autopilot Template User";

const AGENT_ID = "66666666-6666-4666-8666-666666666666";
const RUNTIME_ID = "77777777-7777-4777-8777-777777777777";
const CREATED_AUTOPILOT_ID = "88888888-8888-4888-8888-888888888888";
const CREATED_TRIGGER_ID = "99999999-9999-4999-8999-999999999999";

// Two templates covering both execution modes: the summary kind that opens an
// issue every run, and the patrol kind that opens one only when it finds
// something. The picker must render either without knowing the difference.
const TEMPLATES = [
  {
    key: "daily-change-review",
    version: 1,
    category: "periodic-review",
    category_label: "Periodic Review",
    title: "Daily Change Review",
    description:
      "Scans recent work and flags correctness, UX, and test-coverage risks.",
    cron_expression: "0 18 * * *",
    execution_mode: "create_issue",
    avatar_emoji: "🔎",
    prompt:
      "1. List the changes merged in the last 24 hours\n2. Post the findings as a comment on this issue",
  },
  {
    key: "hourly-queue-check",
    version: 1,
    category: "maintenance",
    category_label: "Maintenance",
    title: "Hourly Queue Check",
    description:
      "Finds stuck work, stale generated files, and failing local checks.",
    cron_expression: "0 * * * *",
    execution_mode: "run_only",
    avatar_emoji: "🧹",
    prompt:
      "1. Look for issues that stopped moving\n2. Before creating an issue, search for a still-open issue this autopilot created and comment on it instead",
  },
];

async function login(page: Page): Promise<{
  slug: string;
  workspaceId: string;
  userId: string;
}> {
  const api = new TestApiClient();
  const auth = await api.login(EMAIL, NAME);
  const userId: unknown = auth.user?.id;
  if (typeof userId !== "string" || !userId) {
    throw new Error("login did not return a user id");
  }
  const workspace = await api.ensureWorkspace(
    `E2E Autopilot Template WS ${E2E_WORKER}`,
    `e2e-ap-tpl-${E2E_WORKER}-${E2E_RUN_ID}`,
  );
  await api.markUserOnboarded();
  const token = api.getToken();
  if (!token) throw new Error("login did not return a token");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica:chat:isOpen", "false");
  }, token);
  return { slug: workspace.slug, workspaceId: workspace.id, userId };
}

/** Mocks the template catalog, one selectable agent, and the create call.
 *  Returns a getter for the create body the flow actually sent. */
async function mockTemplateApis(page: Page, workspaceId: string, userId: string) {
  const captured: { body?: Record<string, unknown> } = {};

  await page.route("**/api/autopilots/templates**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ templates: TEMPLATES }),
    }),
  );

  // The assignee picker reads the workspace agent list. It must contain a
  // runtime-bound, non-archived agent in this workspace that the current user
  // can invoke, or Enable stays disabled and the body assertion cannot run.
  await page.route("**/api/agents?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: AGENT_ID,
          workspace_id: workspaceId,
          runtime_id: RUNTIME_ID,
          name: "E2E Reviewer",
          description: "",
          instructions: "",
          avatar_url: null,
          runtime_mode: "local",
          runtime_config: {},
          custom_args: [],
          visibility: "workspace",
          permission_mode: "public_to",
          invocation_targets: [{ target_type: "workspace", target_id: null }],
          status: "idle",
          runtime_availability: "online",
          max_concurrent_tasks: 3,
          model: "",
          owner_id: userId,
          skills: [],
          template_key: "",
          template_version: 0,
          autonomy_level: "coordinator",
          created_at: "2026-09-01T00:00:00Z",
          updated_at: "2026-09-01T00:00:00Z",
          archived_at: null,
          archived_by: null,
        },
      ]),
    }),
  );

  await page.route("**/api/autopilots/from-template", async (route) => {
    captured.body = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        autopilot: {
          id: CREATED_AUTOPILOT_ID,
          workspace_id: workspaceId,
          title: "Hourly Queue Check",
          description: TEMPLATES[1].prompt,
          assignee_type: "agent",
          assignee_id: AGENT_ID,
          status: "active",
          execution_mode: "run_only",
          issue_title_template: null,
          project_id: null,
          template_key: "hourly-queue-check",
          template_version: 1,
          created_by_type: "member",
          created_by_id: userId,
          created_at: "2026-09-01T00:00:00Z",
          updated_at: "2026-09-01T00:00:00Z",
        },
        trigger: {
          id: CREATED_TRIGGER_ID,
          autopilot_id: CREATED_AUTOPILOT_ID,
          kind: "schedule",
          enabled: true,
          cron_expression: "0 * * * *",
          timezone: "UTC",
          next_run_at: "2026-09-01T01:00:00Z",
          label: null,
          created_at: "2026-09-01T00:00:00Z",
          updated_at: "2026-09-01T00:00:00Z",
        },
      }),
    });
  });

  return () => captured.body;
}

test.describe("autopilot templates", () => {
  test("picks a template, requires an assignee, and creates without claiming the prompt", async ({
    page,
  }) => {
    const { slug, workspaceId, userId } = await login(page);
    const createdBody = await mockTemplateApis(page, workspaceId, userId);

    await page.goto(`/${slug}/autopilots/new/template`);

    // Step one is the template grid. Each card carries the three layers the
    // product design calls for: category, title, description.
    await waitForPageText(page, "Start from an automation template");
    await expect(page.getByText("Periodic Review").first()).toBeVisible();
    await expect(page.getByText("Daily Change Review").first()).toBeVisible();
    await expect(
      page.getByText(
        "Scans recent work and flags correctness, UX, and test-coverage risks.",
      ),
    ).toBeVisible();
    await expect(page.getByText("Maintenance").first()).toBeVisible();
    await expect(page.getByText("Hourly Queue Check").first()).toBeVisible();

    await page.getByRole("button", { name: /Hourly Queue Check/ }).first().click();

    // Step two shows the brief that will be copied, read-only, and the cadence
    // in words. The prompt is not an editable field here: the server decides it.
    await waitForPageText(page, "What the agent is asked to do");
    await expect(
      page.getByText("Look for issues that stopped moving", { exact: false }),
    ).toBeVisible();

    // A template cannot know which agents a workspace has, so the flow cannot be
    // one click: Enable stays disabled until an assignee is chosen.
    const createButton = page.getByRole("button", { name: "Enable automation" });
    await expect(createButton).toBeDisabled();

    // The assignee control is a popover, so the agent only exists in the DOM
    // once its trigger is opened.
    await page.getByRole("button", { name: "Select agent or squad" }).click();
    // The background chat launcher can also name this agent; select the
    // actual option inside the open picker instead of a page-wide text match.
    await page.getByRole("dialog").getByRole("button", { name: /E2E Reviewer/ }).click();
    await expect(createButton).toBeEnabled();
    await createButton.click();

    await expect
      .poll(() => createdBody(), { timeout: 10_000 })
      .toMatchObject({
        template_key: "hourly-queue-check",
        assignee_id: AGENT_ID,
        assignee_type: "agent",
      });

    // The prompt, the cadence and the output mode are the backend's to apply.
    // Sending them from the client would let a caller record a template's
    // provenance on an autopilot that runs something else entirely.
    const body = createdBody() ?? {};
    expect(body).not.toHaveProperty("prompt");
    expect(body).not.toHaveProperty("description");
    expect(body).not.toHaveProperty("title");
    expect(body).not.toHaveProperty("cron_expression");
    expect(body).not.toHaveProperty("execution_mode");
  });
});
