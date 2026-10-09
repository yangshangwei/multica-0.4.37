import userEvent from "@testing-library/user-event";
import { render, screen, within } from "@testing-library/react";
import type { TriageAction, TriageHistoryEntry } from "@multica/core/triage";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriageHistoryRecord, TriageItemHistory } from "./triage-history";
import { TriageHistoryTimeline } from "./triage-history-timeline";

const state = vi.hoisted(() => ({
  events: [] as TriageAction[],
  retry: vi.fn(),
}));

vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      selector: (value: typeof en) => string,
      values?: Record<string, string | number>,
    ) =>
      selector(en).replace(/{{(\w+)}}/g, (_, key: string) =>
        String(values?.[key] ?? key),
      ),
    i18n: { language: "en-US" },
  }),
}));
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string, id: string) => `${type}: ${id}`,
  }),
}));
vi.mock("../issues/utils/status-label", () => ({
  useStatusLabel: () => (value: string) => value,
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ userId: "reviewer-1" }),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useRetryTriageExecution: () => ({ mutate: state.retry, isPending: false }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({ queryKey }: { queryKey: string[] }) => ({
    data:
      queryKey[2] === "item-history"
        ? { events: state.events }
        : queryKey.at(-1) === "members"
          ? [{ user_id: "reviewer-1", name: "Avery" }]
          : [
              { id: "project-old", title: "Original project" },
              { id: "project-new", title: "Launch" },
            ],
  }),
}));

const record: TriageAction = {
  id: "decision-1",
  issue_id: "issue-1",
  actor_id: "reviewer-1",
  action: "accept",
  round: 2,
  reason: "Keep the original reasoning.\nIncluding the second line.",
  created_at: "2026-10-09T15:12:36Z",
  before: {
    source: "csv",
    issue: {
      priority: "low",
      project_id: "project-old",
      assignee_type: "agent",
      assignee_id: "agent-old",
      revision: 1,
    },
    reviewer_id: null,
  },
  after: {
    source: "csv",
    issue: {
      priority: "high",
      project_id: "project-new",
      assignee_type: "agent",
      assignee_id: "agent-new",
      revision: 2,
    },
    reviewer_id: "departed-reviewer",
  },
  execution_status: "not_requested",
  execution_error: null,
  task_id: null,
};

beforeEach(() => {
  state.events = [];
  state.retry.mockClear();
});

// The pure changed-field matrix is owned by triage-ui.test.ts. This regression
// protects the existing record's names, evidence and disclosure when shared.
it("retains the actor, round, reason and snapshot values behind a native disclosure", async () => {
  const user = userEvent.setup();
  render(<TriageHistoryRecord wsId="ws" entry={record} />);

  expect(screen.getByRole("article")).toHaveTextContent("Avery · Round 2");
  expect(screen.getByText(en.csv)).toBeVisible();
  expect(screen.getByText(/Keep the original reasoning/)).toHaveTextContent(
    "Including the second line.",
  );
  expect(screen.getByText(en.snapshot_names_hint)).not.toBeVisible();
  expect(document.querySelector("time")).toHaveAttribute(
    "datetime",
    record.created_at,
  );

  const summary = screen.getByText(en.changes);
  expect(summary.tagName).toBe("SUMMARY");
  await user.click(summary);
  expect(screen.getByText(en.snapshot_names_hint)).toBeVisible();
  const priorityChange = screen.getByText(en.priority).parentElement;
  expect(priorityChange).toHaveTextContent("Before: Low");
  expect(priorityChange).toHaveTextContent("After: High");
  expect(screen.getByText(/Original project/)).toBeVisible();
  expect(screen.getByText(/Launch/)).toBeVisible();
  expect(screen.getByText(/agent: agent-old/)).toBeVisible();
  expect(screen.getByText(/agent: agent-new/)).toBeVisible();
  expect(screen.getByText(en.removed_member)).toBeVisible();
  expect(screen.queryByText(/revision/i)).not.toBeInTheDocument();
});

it("keeps single-item failed execution attached to its original decision and retry", async () => {
  const user = userEvent.setup();
  state.events = [
    {
      ...record,
      action: "accept_and_execute",
      execution_status: "failed",
      execution_error: "Runtime disconnected",
    },
  ];
  render(<TriageItemHistory wsId="ws" issueId="issue-1" />);

  expect(screen.getByRole("status")).toHaveTextContent(en.execution_failed);
  expect(screen.getByText("Runtime disconnected")).toBeVisible();
  expect(screen.getByRole("article")).toHaveTextContent("Avery · Round 2");
  await user.click(screen.getByRole("button", { name: en.retry_execution }));
  expect(state.retry).toHaveBeenCalledExactlyOnceWith("decision-1");
});

const historyEntry: TriageHistoryEntry = {
  id: "event-1",
  kind: "action",
  issue_id: "issue-1",
  identifier: "MUL-12",
  title: "A task with a recorded decision",
  action: "accept",
  actor_id: record.actor_id,
  created_at: new Date(2026, 9, 10, 10, 5, 7).toISOString(),
  reason: record.reason,
  before: record.before,
  after: record.after,
  batch_id: null,
  filename: null,
  counts: null,
};

// Date boundary, DST and ordering cases live in triage-history-dates.test.ts.
it("renders labelled day lists with full timestamps, preserved evidence and keyboard destinations", async () => {
  const user = userEvent.setup();
  const onOpenIssue = vi.fn();
  const onOpenImport = vi.fn();
  render(
    <TriageHistoryTimeline
      wsId="ws"
      entries={[
        historyEntry,
        {
          ...historyEntry,
          id: "event-2",
          kind: "import",
          action: "import",
          issue_id: null,
          reason: null,
          before: {},
          after: {},
          batch_id: "batch-1",
          filename: "launch.csv",
          counts: { created: 4, skipped: 1, failed: 2 },
        },
        {
          ...historyEntry,
          id: "event-3",
          created_at: new Date(2026, 9, 9, 8).toISOString(),
          issue_id: "issue-2",
          identifier: "MUL-13",
          title: "Previous day task",
          reason: null,
          before: {},
          after: {},
        },
      ]}
      onOpenIssue={onOpenIssue}
      onOpenImport={onOpenImport}
    />,
  );

  expect(screen.getAllByRole("heading", { level: 2 }).map((heading) => heading.textContent)).toEqual([
    "October 10, 2026",
    "October 9, 2026",
  ]);
  expect(within(screen.getByRole("list", { name: "October 10, 2026" })).getAllByRole("listitem")).toHaveLength(2);
  expect(document.querySelectorAll("time")).toHaveLength(3);
  const fullTimestamp = new Intl.DateTimeFormat("en-US", {
    year: "numeric", month: "long", day: "numeric",
    hour: "2-digit", minute: "2-digit", second: "2-digit", timeZoneName: "short",
  }).format(new Date(historyEntry.created_at));
  expect(document.querySelector("time")).toHaveAttribute("datetime", historyEntry.created_at);
  expect(within(document.querySelector("time")!).getByText(fullTimestamp)).toBeInTheDocument();
  expect(screen.getByText(/Keep the original reasoning/)).toBeVisible();

  await user.tab();
  expect(screen.getByRole("button", { name: "MUL-12 A task with a recorded decision" })).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(onOpenIssue).toHaveBeenCalledExactlyOnceWith("issue-1");
  await user.click(screen.getByText(en.changes));
  expect(screen.getByText(en.snapshot_names_hint)).toBeVisible();
  expect(screen.getByText(en.priority).parentElement).toHaveTextContent("Before: Low");
  expect(screen.getByText(en.priority).parentElement).toHaveTextContent("After: High");

  const importButton = screen.getByRole("button", { name: "CSV import: launch.csv" });
  expect(importButton.closest("article")).toHaveTextContent("4 created · 1 skipped · 2 failed");
  expect(screen.queryByText(en.import_csv, { exact: true })).not.toBeInTheDocument();
  await user.click(importButton);
  expect(onOpenImport).toHaveBeenCalledExactlyOnceWith("batch-1");
});

it("renders safe unknown action and timestamp fallbacks without losing the event", () => {
  render(
    <TriageHistoryTimeline
      wsId="ws"
      entries={[{ ...historyEntry, action: "future-action", created_at: "bad-timestamp" }]}
      onOpenIssue={vi.fn()}
      onOpenImport={vi.fn()}
    />,
  );
  expect(screen.getByRole("heading", { name: en.history_timeline.unknown_date })).toBeVisible();
  expect(screen.getByText(en.history_timeline.unknown_time)).toBeVisible();
  expect(screen.getByText("future-action")).toBeVisible();
  expect(screen.getByRole("button", { name: "MUL-12 A task with a recorded decision" })).toBeVisible();
  expect(document.querySelector("time")).not.toBeInTheDocument();
});

it("keeps an expanded event mounted when a newer event arrives on the same day", async () => {
  const user = userEvent.setup();
  const callbacks = { onOpenIssue: vi.fn(), onOpenImport: vi.fn() };
  const { rerender } = render(
    <TriageHistoryTimeline wsId="ws" entries={[historyEntry]} {...callbacks} />,
  );
  await user.click(screen.getByText(en.changes));
  const article = screen.getByRole("article");
  rerender(
    <TriageHistoryTimeline
      wsId="ws"
      entries={[
        {
          ...historyEntry,
          id: "new-event",
          title: "A newer decision",
          created_at: new Date(2026, 9, 10, 10, 6).toISOString(),
          before: {},
          after: {},
        },
        historyEntry,
      ]}
      {...callbacks}
    />,
  );
  expect(screen.getByRole("button", { name: "MUL-12 A task with a recorded decision" }).closest("article")).toBe(article);
  expect(screen.getByText(en.snapshot_names_hint)).toBeVisible();
});
