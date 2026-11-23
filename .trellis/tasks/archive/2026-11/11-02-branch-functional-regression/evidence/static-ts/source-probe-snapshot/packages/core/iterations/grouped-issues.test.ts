// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { beforeEach, expect, it, vi } from "vitest";
import { api } from "../api";
import { iterationGroupedIssuesOptions } from "./index";
vi.mock("../api", async (original) => ({ ...await original<typeof import("../api")>(), api: { getIterationIssues: vi.fn() } }));
const issue = (id: string) => ({ issue_id: id, identifier: id, title: id, project_id: null, project_name: null, assignee_type: null, assignee_id: null, assignee_name: null, status_key: "todo", status_category: "todo", was_completed_at_start: false, rollover_count: 0 });
const page = (id: string, next: string | null, revision = 1) => ({ workspace_id: "w", iteration_id: "i", scope_revision: revision, items: [issue(id)], total: 2, next_cursor: next });
beforeEach(() => vi.mocked(api.getIterationIssues).mockReset());
it("groups the complete filtered set, including later pages", async () => {
  vi.mocked(api.getIterationIssues).mockResolvedValueOnce(page("a", "next")).mockResolvedValueOnce(page("b", null));
  const result = await new QueryClient().fetchQuery(iterationGroupedIssuesOptions("w", "i", { priority: "high" }));
  expect(result.items.map(item => item.issue_id)).toEqual(["a", "b"]);
  expect(api.getIterationIssues).toHaveBeenLastCalledWith("w", "i", { priority: "high", limit: "100", cursor: "next" }, expect.anything());
});
it("preserves the complete unfiltered choices from the first coherent page", async () => {
  const filter_options = { statuses: ["todo", "later"], projects: [{ id: "project", name: "Frozen project" }], assignees: [{ type: "agent", id: "agent", name: "Frozen agent" }], labels: [{ id: "label", name: "Frozen label" }] };
  vi.mocked(api.getIterationIssues).mockResolvedValueOnce({ ...page("a", "next"), filter_options }).mockResolvedValueOnce(page("b", null));
  const result = await new QueryClient().fetchQuery(iterationGroupedIssuesOptions("w", "i", { search: "match" }));
  expect(result.filter_options).toEqual(filter_options);
  expect(result.items.map(item => item.issue_id)).toEqual(["a", "b"]);
});
it("rejects scope changes rather than combining old and new pages", async () => {
  vi.mocked(api.getIterationIssues).mockResolvedValueOnce(page("a", "next")).mockResolvedValueOnce(page("b", null, 2));
  await expect(new QueryClient().fetchQuery(iterationGroupedIssuesOptions("w", "i"))).rejects.toMatchObject({ status: 409 });
});
it("rejects repeated members and incomplete traversals", async () => {
  vi.mocked(api.getIterationIssues).mockResolvedValueOnce(page("a", "next")).mockResolvedValueOnce(page("a", null));
  await expect(new QueryClient().fetchQuery(iterationGroupedIssuesOptions("w", "i"))).rejects.toThrow();
  vi.mocked(api.getIterationIssues).mockResolvedValueOnce(page("a", null));
  await expect(new QueryClient().fetchQuery(iterationGroupedIssuesOptions("w", "i"))).rejects.toThrow();
});
