// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  AutopilotTemplateListResponseSchema,
  CreateAutopilotFromTemplateResponseSchema,
  EMPTY_AUTOPILOT_TEMPLATE_LIST,
  EMPTY_CREATE_AUTOPILOT_FROM_TEMPLATE_RESPONSE,
} from "./schemas";
import { parseWithFallback } from "./schema";

// Boundary defence for the autopilot template endpoints. Two properties matter
// here and nowhere else in this suite:
//
//  - A drifted or hostile payload degrades to an empty picker, never a throw.
//  - A newer backend that adds an execution mode this build has never heard of
//    still renders its card, because `execution_mode` is text at the boundary
//    rather than a zod enum.

const TEMPLATE = {
  key: "hourly-queue-check",
  version: 1,
  category: "maintenance",
  category_label: "维护",
  title: "每小时队列巡检",
  description: "Looks for stuck work every hour",
  cron_expression: "0 * * * *",
  execution_mode: "run_only",
  avatar_emoji: "🔁",
  prompt: "# Hourly queue check",
};

const templateList = (payload: unknown) =>
  parseWithFallback(
    payload,
    AutopilotTemplateListResponseSchema,
    { templates: EMPTY_AUTOPILOT_TEMPLATE_LIST },
    { endpoint: "test" },
  );

describe("AutopilotTemplateListResponseSchema", () => {
  it("parses a full payload", () => {
    const parsed = templateList({ templates: [TEMPLATE] });
    expect(parsed.templates[0]).toMatchObject({
      key: "hourly-queue-check",
      category: "maintenance",
      category_label: "维护",
      cron_expression: "0 * * * *",
      execution_mode: "run_only",
      prompt: "# Hourly queue check",
    });
  });

  it("keeps unknown fields a newer backend adds", () => {
    const parsed = templateList({
      templates: [{ ...TEMPLATE, recommended_for: "large-repos" }],
    });
    expect(parsed.templates[0]).toMatchObject({ key: "hourly-queue-check" });
  });

  it("fills every field a leaner backend omits", () => {
    const parsed = templateList({ templates: [{ key: "release-readiness" }] });
    expect(parsed.templates[0]).toEqual({
      key: "release-readiness",
      version: 0,
      category: "",
      category_label: "",
      title: "",
      description: "",
      cron_expression: "",
      execution_mode: "",
      avatar_emoji: "",
      prompt: "",
    });
  });

  it("keeps an execution mode it does not recognise", () => {
    // The proof that this is not a zod enum: widening the backend's vocabulary
    // must not blank the picker. The card still renders and the consuming
    // switch takes its default branch.
    const parsed = templateList({
      templates: [{ ...TEMPLATE, execution_mode: "comment_only" }],
    });
    expect(parsed.templates[0]?.execution_mode).toBe("comment_only");
  });

  it("falls back on a malformed payload instead of throwing", () => {
    const malformed: unknown[] = [
      null,
      undefined,
      "nope",
      42,
      [],
      {},
      { templates: null },
      { templates: "nope" },
      // An entry with no key is not addressable, so the whole list is dropped
      // rather than half-shown.
      { templates: [{}] },
      { templates: [{ key: 7 }] },
      { templates: [{ ...TEMPLATE, version: "one" }] },
      { templates: [{ ...TEMPLATE, prompt: { text: "nope" } }] },
    ];
    for (const payload of malformed) {
      expect(templateList(payload).templates).toEqual([]);
    }
  });
});

const AUTOPILOT = {
  id: "autopilot-1",
  workspace_id: "ws-1",
  title: "Hourly queue check",
  description: "# Hourly queue check",
  project_id: null,
  assignee_type: "agent",
  assignee_id: "agent-1",
  status: "active",
  execution_mode: "run_only",
  issue_title_template: null,
  created_by_type: "member",
  created_by_id: "user-1",
  // Provenance from the template the row was created from. Older servers and
  // hand-created rows omit both.
  template_key: "hourly-queue-check",
  template_version: 3,
  last_run_at: null,
  created_at: "2026-09-11T00:00:00Z",
  updated_at: "2026-09-11T00:00:00Z",
  subscribers: [],
};

const TRIGGER = {
  id: "trigger-1",
  autopilot_id: "autopilot-1",
  kind: "schedule",
  enabled: true,
  cron_expression: "0 * * * *",
  timezone: "Asia/Shanghai",
  next_run_at: "2026-09-11T01:00:00Z",
  webhook_token: null,
  webhook_path: null,
  webhook_url: null,
  label: null,
  last_fired_at: null,
  created_at: "2026-09-11T00:00:00Z",
  updated_at: "2026-09-11T00:00:00Z",
};

const fromTemplate = (payload: unknown) =>
  parseWithFallback(
    payload,
    CreateAutopilotFromTemplateResponseSchema,
    EMPTY_CREATE_AUTOPILOT_FROM_TEMPLATE_RESPONSE,
    { endpoint: "test" },
  );

describe("CreateAutopilotFromTemplateResponseSchema", () => {
  it("parses both rows the call wrote", () => {
    const parsed = fromTemplate({ autopilot: AUTOPILOT, trigger: TRIGGER });
    expect(parsed.autopilot.id).toBe("autopilot-1");
    expect(parsed.autopilot.execution_mode).toBe("run_only");
    // Provenance survives the round trip: the row says which template, and
    // which version of it, its content came from.
    expect(parsed.autopilot.template_key).toBe("hourly-queue-check");
    expect(parsed.autopilot.template_version).toBe(3);
    expect(parsed.trigger).toMatchObject({
      id: "trigger-1",
      cron_expression: "0 * * * *",
      timezone: "Asia/Shanghai",
      enabled: true,
    });
  });

  it("normalizes absent nullable trigger columns to null", () => {
    const parsed = fromTemplate({
      autopilot: AUTOPILOT,
      trigger: { id: "trigger-1", autopilot_id: "autopilot-1" },
    });
    expect(parsed.trigger.cron_expression).toBeNull();
    expect(parsed.trigger.timezone).toBeNull();
    expect(parsed.trigger.next_run_at).toBeNull();
    expect(parsed.trigger.webhook_token).toBeNull();
    expect(parsed.trigger.event_filters).toBeNull();
  });

  it("falls back to a paused, issue-free shape when either half is unreadable", () => {
    const malformed: unknown[] = [
      null,
      "nope",
      {},
      { autopilot: AUTOPILOT },
      { trigger: TRIGGER },
      { autopilot: AUTOPILOT, trigger: null },
      { autopilot: null, trigger: TRIGGER },
      { autopilot: { ...AUTOPILOT, id: 7 }, trigger: TRIGGER },
    ];
    for (const payload of malformed) {
      const parsed = fromTemplate(payload);
      // `id === ""` is the guard a caller checks before navigating to a row it
      // believes it just created.
      expect(parsed.autopilot.id).toBe("");
      // Conservative in both directions: never claim an automation is live, and
      // never claim it opens an issue on every run.
      expect(parsed.autopilot.status).toBe("paused");
      expect(parsed.autopilot.execution_mode).toBe("run_only");
      expect(parsed.trigger.enabled).toBe(false);
    }
  });
});
