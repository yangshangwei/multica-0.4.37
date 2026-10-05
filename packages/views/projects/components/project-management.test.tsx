import React, { forwardRef, useImperativeHandle, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance, ApiError } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectProgressDraftStore, useProjectDescriptionDraftStore } from "@multica/core/projects";
import type { Project, ProjectUpdateDraft } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ProjectDescription } from "./project-description";
import { ProjectProgress } from "./project-progress";
import { ProjectRiskIssues } from "./project-risk-issues";
import { p1Project, p1Preview, p1WriteResult, p1Overview } from "@multica/core/projects/test-fixtures/p1";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "11111111-1111-4111-8111-111111111111" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/issues/${id}` }) }));
vi.mock("../../navigation", () => ({ AppLink: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
vi.mock("../../editor", () => ({ ContentEditor: forwardRef(function Editor({ value, defaultValue, onUpdate }: { value?: string; defaultValue?: string; onUpdate: (value: string, base: string) => void }, ref) {
  const [text, setText] = useState(value ?? defaultValue ?? "");
  useImperativeHandle(ref, () => ({ getMarkdown: () => text, adoptContent: setText,
    insertMarkdownAtEnd: (append: string) => { const next = `${text}\n\n${append}`; setText(next); onUpdate(next, value ?? ""); return true; } }));
  return <textarea aria-label="Rich text" value={text} onChange={(event) => { setText(event.target.value); onUpdate(event.target.value, value ?? ""); }} />;
}) }));

let qc: QueryClient;
const project: Project = { ...p1Project, status: "in_progress", priority: "medium", lead_type: "member" };
function render(ui: React.ReactElement) { return renderWithI18n(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>); }
beforeEach(() => { qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }); useProjectProgressDraftStore.getState().clearDraft(); useProjectDescriptionDraftStore.getState().clearDraft(); });
afterEach(() => { qc.clear(); vi.restoreAllMocks(); });

describe("project goal editor", () => {
  it("keeps autosave active after the StrictMode effect replay", async () => {
    const updateProject = vi.fn(async (_id, data) => ({ ...project, description: data.description, description_revision: 3 }));
    setApiInstance({ updateProject } as unknown as ApiClient);
    renderWithI18n(<QueryClientProvider client={qc}><ProjectDescription project={project} supported /></QueryClientProvider>, { reactStrictMode: true });
    fireEvent.change(screen.getByLabelText("Rich text"), { target: { value: "Edited in StrictMode" } });
    await waitFor(() => expect(updateProject).toHaveBeenCalledWith(project.id, expect.objectContaining({ description: "Edited in StrictMode" }), { workspaceId: project.workspace_id }), { timeout: 2000 });
  });

  it("preserves a description draft when the editor unmounts before autosave", async () => {
    const updateProject = vi.fn(async (_id, data) => ({ ...project, description: data.description, description_revision: 3 }));
    setApiInstance({ updateProject, getProject: vi.fn().mockResolvedValue(project) } as unknown as ApiClient);
    const first = render(<ProjectDescription project={project} supported />);
    fireEvent.change(screen.getByLabelText("Rich text"), { target: { value: "Unsaved text" } });
    first.unmount();
    expect(updateProject).not.toHaveBeenCalled();
    render(<ProjectDescription project={project} supported />);
    expect(screen.getByLabelText("Rich text")).toHaveValue("Unsaved text");
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(updateProject).toHaveBeenCalledWith(project.id, expect.objectContaining({ description: "Unsaved text", expected_description_revision: 2 }), { workspaceId: project.workspace_id }));
  });

  it("previews selected missing sections before appending without replacing prose", async () => {
    const updateProject = vi.fn(async (_id, data) => ({ ...project, description: data.description, description_revision: 3 }));
    setApiInstance({ updateProject } as unknown as ApiClient);
    render(<ProjectDescription project={project} supported />);
    const user = userEvent.setup(); await user.click(screen.getByRole("button", { name: "Goal template" }));
    await user.click(screen.getByRole("checkbox", { name: "Background" })); await user.click(screen.getByRole("button", { name: "Preview selected sections" }));
    expect(updateProject).not.toHaveBeenCalled(); expect(screen.getByLabelText("Rich text")).toHaveValue(project.description);
    await user.click(screen.getByRole("button", { name: "Append selected sections" }));
    await waitFor(() => expect(updateProject).toHaveBeenCalledWith(project.id, expect.objectContaining({ description: expect.stringContaining("Ship the release\n\n## Background"), expected_description_revision: 2 }), { workspaceId: project.workspace_id }), { timeout: 2000 });
  });
});

describe("manual project progress", () => {
  it("previews, keeps failed input, and retries the exact publication intent", async () => {
    const previewProjectUpdate = vi.fn(async (_ws, _id, draft: ProjectUpdateDraft) => ({ ...p1Preview, draft }));
    const createProjectUpdate = vi.fn().mockRejectedValueOnce(new Error("Connection lost")).mockResolvedValueOnce(p1WriteResult);
    setApiInstance({ getBaseUrl: () => "https://test.example", listProjectUpdates: vi.fn().mockResolvedValue({ workspace_id: project.workspace_id, project_id: project.id, items: [], next_cursor: null }), previewProjectUpdate, createProjectUpdate } as unknown as ApiClient);
    render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
    const user = userEvent.setup(); await user.click(screen.getByRole("button", { name: "Write progress" }));
    fireEvent.change(screen.getByLabelText("Rich text"), { target: { value: "Release ready" } });
    await user.click(screen.getByRole("button", { name: "Preview publication" }));
    await screen.findByText("No members will be notified"); expect(createProjectUpdate).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Publish" })); await screen.findByText("Connection lost");
    expect(screen.getByText("Release ready")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Publish" }));
    await waitFor(() => expect(createProjectUpdate).toHaveBeenCalledTimes(2));
    expect(createProjectUpdate.mock.calls[0]?.[2]).toEqual(createProjectUpdate.mock.calls[1]?.[2]);
  });
  it("requires explicit review of a changed description before previewing new acceptance", async () => {
    const previewProjectUpdate = vi.fn().mockRejectedValueOnce(new ApiError("Description changed", 409, "Conflict", { code: "project_description_conflict" }))
      .mockImplementationOnce(async (_ws, _id, draft: ProjectUpdateDraft) => ({ ...p1Preview, draft, description_revision: 3 }));
    setApiInstance({ getBaseUrl: () => "https://test.example", getProject: vi.fn().mockResolvedValue({ ...project, description: "New goal", description_revision: 3 }),
      listProjectUpdates: vi.fn().mockResolvedValue({ items: [], next_cursor: null }), previewProjectUpdate } as unknown as ApiClient);
    render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
    const user = userEvent.setup(); await user.click(screen.getByRole("button", { name: "Record acceptance" }));
    fireEvent.change(screen.getByLabelText("Rich text"), { target: { value: "Verified the release" } });
    await user.click(screen.getByRole("button", { name: "Preview publication" }));
    await screen.findByText("New goal"); expect(previewProjectUpdate).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Review acceptance against this description" }));
    await user.click(screen.getByRole("button", { name: "Preview publication" }));
    await waitFor(() => expect(previewProjectUpdate).toHaveBeenCalledTimes(2));
    expect(previewProjectUpdate.mock.calls[1]?.[2]).toMatchObject({ expected_description_revision: 3, body: "Verified the release" });
  });

  it("cancelling the editor does not preview or publish", async () => {
    const previewProjectUpdate = vi.fn(); const createProjectUpdate = vi.fn();
    setApiInstance({ getBaseUrl: () => "https://test.example", listProjectUpdates: vi.fn().mockResolvedValue({ items: [], next_cursor: null }), previewProjectUpdate, createProjectUpdate } as unknown as ApiClient);
    render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
    const user = userEvent.setup(); await user.click(screen.getByRole("button", { name: "Write progress" }));
    fireEvent.change(screen.getByLabelText("Rich text"), { target: { value: "Keep draft" } });
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(previewProjectUpdate).not.toHaveBeenCalled(); expect(createProjectUpdate).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Write progress" })); expect(screen.getByLabelText("Rich text")).toHaveValue("Keep draft");
  });
});

describe("exact project risk scope", () => {
  it("renders every returned parent and sub-issue with no personal filters in its request", async () => {
    const getProjectRiskIssues = vi.fn().mockResolvedValue({ workspace_id: project.workspace_id, project_id: project.id, signal: "blocked", items: [
      { id: "parent", identifier: "MUL-1", title: "Parent", status: "custom_blocked", due_date: null },
      { id: "child", identifier: "MUL-2", title: "Child", status: "custom_blocked", due_date: null, parent_issue_id: "parent" },
    ], total: 2, snapshot_version: "new", refreshed: true, overview: p1Overview, next_cursor: "next-new-version" });
    setApiInstance({ getProjectRiskIssues } as unknown as ApiClient);
    render(<ProjectRiskIssues project={project} signal="blocked" version="old" onBack={vi.fn()} onProtectedError={vi.fn()} />);
    expect(await screen.findByRole("link", { name: /Parent/ })).toBeInTheDocument(); expect(screen.getByRole("link", { name: /Child/ })).toBeInTheDocument();
    expect(screen.getByText("The project changed. Counts and results have been refreshed together.")).toBeInTheDocument();
    expect(getProjectRiskIssues).toHaveBeenCalledWith(project.workspace_id, project.id, { signal: "blocked", cursor: undefined, version: "old" }, { signal: expect.any(AbortSignal) });
    await userEvent.setup().click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() => expect(getProjectRiskIssues).toHaveBeenLastCalledWith(project.workspace_id, project.id,
      { signal: "blocked", cursor: "next-new-version", version: "new" }, { signal: expect.any(AbortSignal) }));
  });
});
