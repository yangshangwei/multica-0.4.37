// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { api, ApiError } from "../api";
import type { Issue } from "../types";
import {
  classifyIterationCandidate,
  iterationCandidateSearchOptions,
  ISSUE_REFERENCE_LIMIT,
  iterationUnplannedCandidatesOptions,
  parseIssueReferences,
  UNPLANNED_CANDIDATE_LIMIT,
} from "./candidates";

vi.mock("../api", async (original) => ({
  ...(await original<typeof import("../api")>()),
  api: { searchIssues: vi.fn(), getIssue: vi.fn(), listIssues: vi.fn() },
}));

const planned = { id: "target", status: "planned" };
const active = { id: "target", status: "active" };
const open = new Set(["target", "other"]);
const task = (fields: Partial<Issue> = {}) =>
  ({ status: "todo", admission_status: "not_required", current_iteration_id: null, ...fields }) as Issue;

describe("classifyIterationCandidate", () => {
  it.each([
    ["an unassigned open task", task(), planned, null, false, false],
    ["a task already in the target", task({ current_iteration_id: "target" }), planned, "member", false, false],
    ["a cancelled task", task({ status: "cancelled" }), active, "cancelled", false, false],
    ["a custom status in the cancelled category", task({ status: "dropped", status_category: "cancelled" }), active, "cancelled", false, false],
    ["a task waiting for triage", task({ admission_status: "pending" }), planned, "triage", false, false],
    ["a task whose source iteration closed", task({ current_iteration_id: "closed" }), planned, "closed_source", false, false],
    ["a completed task for a planned target", task({ status: "done" }), planned, "completed_needs_active", false, false],
    ["a completed task for an active target", task({ status: "done" }), active, null, false, true],
    ["a task in another open iteration", task({ current_iteration_id: "other" }), planned, null, true, false],
    ["a task from an older backend without admission", task({ admission_status: undefined }), planned, null, false, false],
  ] as const)("%s", (_name, issue, target, blocked, needsReason, needsCompletedConfirmation) => {
    expect(classifyIterationCandidate(issue, target, open)).toMatchObject({ blocked, needsReason, needsCompletedConfirmation });
  });

  it("leaves an unknown source to the preview while the catalogue loads", () => {
    expect(classifyIterationCandidate(task({ current_iteration_id: "closed" }), planned, null)).toMatchObject({ blocked: null, needsReason: true });
  });
});

describe("parseIssueReferences", () => {
  it("treats identifiers and UUIDs separated by Latin or Chinese punctuation as a list", () => {
    expect(parseIssueReferences("MUL-1, mul-2，MUL-3、30000000-0000-4000-8000-000000000001\nMUL-1")).toEqual({
      tokens: ["MUL-1", "mul-2", "MUL-3", "30000000-0000-4000-8000-000000000001"],
      truncated: false,
    });
  });

  it.each(["", "  ", "login page", "MUL-1 login", "MUL-"])("searches text for %j", (query) => {
    expect(parseIssueReferences(query)).toBeNull();
  });

  it("caps a pasted list and says so", () => {
    const list = Array.from({ length: ISSUE_REFERENCE_LIMIT + 1 }, (_, index) => `MUL-${index + 1}`).join(" ");
    expect(parseIssueReferences(list)).toMatchObject({ truncated: true, tokens: expect.arrayContaining([`MUL-${ISSUE_REFERENCE_LIMIT}`]) });
    expect(parseIssueReferences(list)!.tokens).toHaveLength(ISSUE_REFERENCE_LIMIT);
  });
});

describe("iterationCandidateSearchOptions", () => {
  const issue = (id: string, workspace_id = "ws") => ({ id, workspace_id }) as Issue;

  it("resolves a pasted list exactly and reports what did not resolve", async () => {
    vi.mocked(api.getIssue).mockImplementation(async (id) => {
      if (id === "MUL-1") return issue("one");
      if (id === "MUL-2") return issue("foreign", "elsewhere");
      throw new ApiError("Not found", 404, "Not Found");
    });
    const result = await new QueryClient().fetchQuery(iterationCandidateSearchOptions("ws", "MUL-1 MUL-2 MUL-3"));
    expect(result).toEqual({ issues: [issue("one")], missing: ["MUL-2", "MUL-3"], truncated: false });
    expect(api.searchIssues).not.toHaveBeenCalled();
  });

  it("fails instead of calling a task missing when a lookup cannot answer", async () => {
    vi.mocked(api.getIssue).mockRejectedValue(new TypeError("Offline"));
    await expect(new QueryClient().fetchQuery(iterationCandidateSearchOptions("ws", "MUL-1"))).rejects.toThrow("Offline");
  });

  it("searches text including closed tasks and keeps only this workspace", async () => {
    vi.mocked(api.searchIssues).mockResolvedValue({ issues: [issue("a"), issue("b", "elsewhere")], total: 2 } as Awaited<ReturnType<typeof api.searchIssues>>);
    const result = await new QueryClient().fetchQuery(iterationCandidateSearchOptions("ws", " login "));
    expect(vi.mocked(api.searchIssues).mock.calls[0]![0]).toMatchObject({ q: "login", include_closed: true });
    expect(result.issues.map((value) => value.id)).toEqual(["a"]);
  });
});

describe("iterationUnplannedCandidatesOptions", () => {
  it("asks the server for recent open work outside every iteration", async () => {
    vi.mocked(api.listIssues).mockResolvedValue({ issues: [task({ id: "a", workspace_id: "ws" } as Partial<Issue>)], total: 45 });
    const result = await new QueryClient().fetchQuery(iterationUnplannedCandidatesOptions("ws"));
    expect(vi.mocked(api.listIssues).mock.calls[0]![0]).toEqual({
      include_no_iteration: true,
      status_categories: ["backlog", "todo", "in_progress", "in_review", "blocked"],
      sort_by: "updated_at",
      sort_direction: "desc",
      limit: UNPLANNED_CANDIDATE_LIMIT,
    });
    expect(result).toMatchObject({ issues: [{ id: "a" }], more: true });
  });

  it("drops planned or closed work an older backend returns despite the facets", async () => {
    const issues = [
      task({ id: "open", workspace_id: "ws" } as Partial<Issue>),
      task({ id: "planned", workspace_id: "ws", current_iteration_id: "other" } as Partial<Issue>),
      task({ id: "done", workspace_id: "ws", status: "done" } as Partial<Issue>),
      task({ id: "foreign", workspace_id: "elsewhere" } as Partial<Issue>),
    ];
    vi.mocked(api.listIssues).mockResolvedValue({ issues, total: issues.length });
    const result = await new QueryClient().fetchQuery(iterationUnplannedCandidatesOptions("ws"));
    expect(result.issues.map((issue) => issue.id)).toEqual(["open"]);
    expect(result.more).toBe(false);
  });
});
