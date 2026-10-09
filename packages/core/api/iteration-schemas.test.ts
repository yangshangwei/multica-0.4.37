// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  IterationCapabilitiesSchema,
  IterationWriteResultSchema,
  parseIteration,
  HistoricalIterationIssueSchema,
  IterationIssuesSchema,
} from "./iteration-schemas";
const ws = "10000000-0000-4000-8000-000000000001";
const historical = { issue_id: ws, identifier: "I1-1", title: "Frozen", project_id: null, project_name: null, assignee_type: null, assignee_id: null, assignee_name: null, status_key: "todo", status_category: "todo", was_completed_at_start: false, rollover_count: 0 };
const issuePage = { workspace_id: ws, iteration_id: ws, scope_revision: 1, items: [historical], total: 1, next_cursor: null };
const filterOptions = {
  statuses: ["todo", "custom_status"],
  projects: [{ id: ws, name: "Frozen project" }, { id: null, name: null }],
  assignees: [{ type: "member", id: ws, name: "Frozen member" }, { type: "agent", id: ws, name: "Frozen agent" }, { type: null, id: null, name: null }],
  labels: [{ id: ws, name: "Frozen label" }],
};
describe("iteration filter metadata compatibility", () => {
  it("retains complete stored choices and permits additive metadata", () => {
    expect(parseIteration({ ...issuePage, filter_options: { ...filterOptions, future: true } }, IterationIssuesSchema, ws).filter_options).toEqual(filterOptions);
  });
  it("keeps valid task pages from older servers without inventing choices", () => {
    expect(parseIteration(issuePage, IterationIssuesSchema, ws).filter_options).toBeUndefined();
  });
  it.each([null, "bad", {}, { ...filterOptions, statuses: [false] }, { ...filterOptions, projects: [{ id: "bad", name: "Broken" }] }, { ...filterOptions, assignees: [{ type: null, id: ws, name: "Unknown" }] }, { ...filterOptions, labels: [{ id: ws, name: null }] }])("discards malformed optional metadata, preserving valid tasks: %j", (filter_options) => {
    const page = parseIteration({ ...issuePage, filter_options }, IterationIssuesSchema, ws);
    expect(page.items).toEqual([historical]);
    expect(page.filter_options).toBeUndefined();
  });
});
it("retains frozen priority and labels while old omissions remain unknown", () => {
  expect(HistoricalIterationIssueSchema.parse({ ...historical, priority: "high", labels: [{ id: ws, name: "Original label" }] })).toMatchObject({ priority: "high", labels: [{ id: ws, name: "Original label" }] });
  expect(HistoricalIterationIssueSchema.parse(historical).labels).toBeUndefined();
  expect(HistoricalIterationIssueSchema.parse({ ...historical, priority: null, labels: null }).labels).toBeNull();
  expect(HistoricalIterationIssueSchema.parse({ ...historical, priority: "none", labels: [] }).labels).toEqual([]);
});
it.each([{ priority: 3 }, { labels: "bad" }, { labels: [{ id: "bad", name: "label" }] }, { labels: [{ id: ws, name: null }] }])("rejects malformed historical display fields: %j", fields => {
  expect(() => HistoricalIterationIssueSchema.parse({ ...historical, ...fields })).toThrow();
});
describe("iteration response boundary", () => {
  it("rejects an empty write receipt instead of inventing success", () => {
    expect(() => parseIteration({}, IterationWriteResultSchema, ws)).toThrow();
  });
  it("rejects another workspace even with a valid capability shape", () => {
    expect(() =>
      parseIteration(
        {
          workspace_id: "20000000-0000-4000-8000-000000000001",
          schema_version: 1,
          supported: true,
          enabled: true,
          manual: true,
          atomic_handoff: false,
        },
        IterationCapabilitiesSchema,
        ws,
      ),
    ).toThrow();
  });
  it("preserves disabled as a valid response and permits additive fields", () => {
    expect(
      parseIteration(
        {
          workspace_id: ws,
          schema_version: 1,
          supported: false,
          enabled: false,
          manual: true,
          atomic_handoff: false,
          future: true,
        },
        IterationCapabilitiesSchema,
        ws,
      ).enabled,
    ).toBe(false);
  });
});

import {
  IterationSchema,
  IterationListSchema,
  IterationSettingsSchema,
} from "./iteration-schemas";
const iteration = {
  id: ws,
  workspace_id: ws,
  name: "Long ".repeat(80),
  description: null,
  coordinator_user_id: null,
  status: "future_status",
  mode: "manual",
  start_date: "2026-10-01",
  end_date: "2026-10-14",
  timezone: "UTC",
  revision: 1,
  scope_revision: 1,
  started_at: null,
  logical_ended_at: null,
  processed_at: null,
};
describe("iteration structural compatibility", () => {
  it("keeps unknown server statuses for explicit read-only rendering", () => {
    expect(parseIteration(iteration, IterationSchema, ws).status).toBe(
      "future_status",
    );
  });
  it.each([0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1])(
    "rejects revision %s",
    (revision) => {
      expect(() =>
        parseIteration({ ...iteration, revision }, IterationSchema, ws),
      ).toThrow();
    },
  );
  it.each([null, {}, "items", [null]])(
    "does not truncate a malformed list into empty success: %s",
    (items) => {
      expect(() =>
        parseIteration(
          { workspace_id: ws, items, next_cursor: null },
          IterationListSchema,
          ws,
        ),
      ).toThrow();
    },
  );
  it("rejects missing timezone instead of inventing UTC", () => {
    expect(() =>
      parseIteration(
        { workspace_id: ws, enabled: true, revision: 1 },
        IterationSettingsSchema,
        ws,
      ),
    ).toThrow();
  });
});

it("rejects mixed-workspace list members", () => {
  expect(() =>
    parseIteration(
      {
        workspace_id: ws,
        items: [
          {
            ...iteration,
            workspace_id: "20000000-0000-4000-8000-000000000001",
          },
        ],
        next_cursor: null,
      },
      IterationListSchema,
      ws,
    ),
  ).toThrow();
});
it("rejects an invalid iteration timezone before calendar rendering", () => {
  expect(() =>
    parseIteration(
      { ...iteration, timezone: "not/a-zone" },
      IterationSchema,
      ws,
    ),
  ).toThrow();
});
