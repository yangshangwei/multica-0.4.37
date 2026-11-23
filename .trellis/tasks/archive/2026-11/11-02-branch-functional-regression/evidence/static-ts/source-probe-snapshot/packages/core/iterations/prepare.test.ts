// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { prepareIterationDraft } from "./prepare";
import type {
  Iteration,
  IterationDraft,
  IterationPreview,
} from "../api/iteration-schemas";
vi.mock("../api", () => ({
  api: {
    previewIteration: vi.fn(),
    getIterationIssues: vi.fn(),
    getIssue: vi.fn(),
  },
}));
const iteration: Iteration = {
  id: "i",
  workspace_id: "w",
  name: "I",
  status: "active",
  mode: "manual",
  revision: 2,
  scope_revision: 3,
  description: null,
  coordinator_user_id: null,
  start_date: "2026-10-01",
  end_date: "2026-10-14",
  timezone: "UTC",
  started_at: null,
  logical_ended_at: null,
  processed_at: null,
};
function issue(
  id: string,
  category = "todo",
  source = "i",
): IterationPreview["issues"][number] {
  return {
    issue_id: id,
    identifier: id,
    revision: 4,
    source_id: source,
    status_category: category,
    title: id,
    running_execution_count: 0,
    rollover_count: 0,
    project: { id: null, name: null, type: null, available: false },
    assignee: { id: null, name: null, type: null, available: false },
  };
}
function preview(
  draft: IterationDraft,
  issues: IterationPreview["issues"],
  complete = true,
): IterationPreview {
  return {
    workspace_id: "w",
    actor_user_id: "actor",
    draft,
    preview_hash: "preliminary",
    previewed_at: "2026-10-06T00:00:00Z",
    start_preview: null,
    iterations: [iteration],
    issues,
    statistics: {},
    recipients: [],
    invalid_items: [{ issue_id: null, code: "remaining_moves_required" }],
    total_affected: issues.length,
    complete,
  };
}
beforeEach(() => vi.resetAllMocks());
describe("complete closure draft preparation", () => {
  it("derives 1000 remaining moves from one full server preview without per-issue requests", async () => {
    const issues = [
      ...Array.from({ length: 1000 }, (_, i) => issue(String(i))),
      issue("done", "done"),
      issue("cancelled", "cancelled"),
    ];
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
      preview(draft, issues),
    );
    const draft = await prepareIterationDraft(
      "w",
      iteration,
      1,
      "end",
      "end",
      "target",
      "scheduled",
      false,
    );
    expect(draft.moves).toHaveLength(1000);
    expect(
      draft.moves.every(
        (move) =>
          move.target_id === "target" && move.expected_issue_revision === 4,
      ),
    ).toBe(true);
    expect(api.previewIteration).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.previewIteration).mock.calls[0]![1].moves).toEqual([]);
    expect(api.getIssue).not.toHaveBeenCalled();
    expect(api.getIterationIssues).not.toHaveBeenCalled();
  });
  it("rejects an incomplete server preview instead of deriving a partial closure", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
      preview(draft, [issue("a")], false),
    );
    await expect(
      prepareIterationDraft(
        "w",
        iteration,
        1,
        "end",
        "end",
        null,
        "scheduled",
        false,
      ),
    ).rejects.toThrow("complete");
  });
  it("propagates a stale preliminary preview without bypassing its revision check", async () => {
    const stale = new Error("iteration_preview_stale");
    vi.mocked(api.previewIteration).mockRejectedValue(stale);
    await expect(
      prepareIterationDraft(
        "w",
        iteration,
        1,
        "end",
        "end",
        null,
        "scheduled",
        false,
      ),
    ).rejects.toBe(stale);
    expect(api.previewIteration).toHaveBeenCalledTimes(1);
  });
  it("separates source remaining work from next-iteration terminal choices", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
      preview(draft, [
        issue("remaining"),
        issue("source-done", "done"),
        issue("target-done", "done", "target"),
        issue("target-open", "todo", "target"),
      ]),
    );
    const draft = await prepareIterationDraft(
      "w",
      iteration,
      1,
      "handoff",
      "handoff",
      "target",
      "today",
      false,
    );
    expect(draft.moves.map((move) => move.issue_id)).toEqual(["remaining"]);
    expect(draft.start).toEqual({
      target_id: "target",
      mode: "today",
      terminal_choices: [{ issue_id: "target-done", retain: false }],
    });
  });
  it.each(["disable", "delete", "cancel"] as const)(
    "does not read individual members for %s without active closure",
    async (operation) => {
      const draft = await prepareIterationDraft(
        "w",
        operation === "disable" ? null : { ...iteration, status: "planned" },
        1,
        operation,
        "reason",
        null,
        "scheduled",
        false,
      );
      expect(draft.moves).toEqual([]);
      expect(api.previewIteration).not.toHaveBeenCalled();
      expect(api.getIssue).not.toHaveBeenCalled();
    },
  );
});
