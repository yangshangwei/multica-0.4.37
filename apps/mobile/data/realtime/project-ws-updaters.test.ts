import { QueryClient } from "@tanstack/react-query";
import type { Project } from "@multica/core/types";
import { describe, expect, it, vi } from "vitest";
import { projectKeys } from "../queries/projects";
import { isFormalProjectIssue, patchProjectDetail, patchProjectsList, upsertIntoProjectsList } from "./project-ws-updaters";
vi.mock("@/data/api", () => ({ api: {} }));
const current: Project = {
  id: "project", workspace_id: "workspace", title: "current", description: "Goal", icon: null,
  status: "in_progress", priority: "none", lead_type: null, lead_id: null, start_date: null, due_date: null,
  created_at: "", updated_at: "", issue_count: 5, done_count: 3, resource_count: 0,
  revision: 5, description_revision: 2, statistics_complete: true,
  completed_issue_count: 2, cancelled_issue_count: 1, open_issue_count: 2,
};
describe("mobile project event compatibility", () => {
  it("keeps pending/rejected/duplicate issues outside the formal project scope", () => {
    for (const admission_status of ["pending", "rejected", "duplicate"] as const) {
      expect(isFormalProjectIssue({ project_id: "project", admission_status }, "project")).toBe(false);
    }
    for (const admission_status of [undefined, "not_required", "accepted"] as const) {
      expect(isFormalProjectIssue({ project_id: "project", admission_status }, "project")).toBe(true);
    }
    expect(isFormalProjectIssue({ project_id: "other", admission_status: "accepted" }, "project")).toBe(false);
  });
  it("retains P1 fields under a newer partial update and honours explicit null", () => {
    const qc = new QueryClient();
    qc.setQueryData(projectKeys.detail("workspace", "project"), current);
    qc.setQueryData(projectKeys.list("workspace"), [current]);
    const partial = { id: "project", workspace_id: "workspace", revision: 6, title: "new", description: null };
    patchProjectDetail(qc, "workspace", partial as Project);
    patchProjectsList(qc, "workspace", partial);
    expect(qc.getQueryData(projectKeys.detail("workspace", "project"))).toMatchObject({ title: "new", description: null, completed_issue_count: 2 });
    expect(qc.getQueryData(projectKeys.list("workspace"))).toEqual([expect.objectContaining({ title: "new", completed_issue_count: 2 })]);
  });
  it("rejects stale revisions and invalidates an unversioned event over versioned cache", () => {
    const qc = new QueryClient();
    qc.setQueryData(projectKeys.detail("workspace", "project"), current);
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    patchProjectDetail(qc, "workspace", { ...current, title: "stale", revision: 4 });
    patchProjectDetail(qc, "workspace", { ...current, title: "legacy", revision: undefined });
    expect(qc.getQueryData(projectKeys.detail("workspace", "project"))).toEqual(current);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: projectKeys.detail("workspace", "project") });
  });
  it("never puts another workspace's payload in this workspace", () => {
    const qc = new QueryClient();
    qc.setQueryData(projectKeys.list("workspace"), [current]);
    const foreign = { ...current, workspace_id: "foreign", title: "secret" };
    patchProjectsList(qc, "workspace", foreign);
    patchProjectDetail(qc, "workspace", foreign);
    upsertIntoProjectsList(qc, "workspace", foreign);
    expect(qc.getQueryData(projectKeys.list("workspace"))).toEqual([current]);
    expect(qc.getQueryData(projectKeys.detail("workspace", "project"))).toBeUndefined();
  });
  it("does not seed a fake complete project from an ID-only event", () => {
    const qc = new QueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    patchProjectDetail(qc, "workspace", { id: "project" } as Project);
    expect(qc.getQueryData(projectKeys.detail("workspace", "project"))).toBeUndefined();
    expect(invalidate).toHaveBeenCalled();
  });
});
