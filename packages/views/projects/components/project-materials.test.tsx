import React from "react";
import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError, setApiInstance, type ApiClient } from "@multica/core/api";
import { materialKeys, useMaterialDraftStore, type MaterialDetail, type MaterialSettingsEnvelope } from "@multica/core/materials";
import { useProjectAccessStore } from "@multica/core/projects";
import { paths } from "@multica/core/paths";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider } from "../../navigation";
import { ProjectMaterials } from "./project-materials";
import { ProjectMaterialSettings } from "./project-material-settings";
import { MaterialDetailView } from "./material-detail";

// Schema/default/identity and approval matrix tests belong to core/materials/schema.test.ts.
// This suite exercises the real query hooks and user-visible request ownership.
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign((selector: (state: unknown) => unknown) => selector({ user: { id: "user" } }), { getState: () => ({ user: { id: "user" } }) }) }));
vi.mock("@multica/core/paths", async (original) => {
  const module = await original<typeof import("@multica/core/paths")>();
  return { ...module, useWorkspacePaths: () => module.paths.workspace("acme") };
});
vi.mock("@multica/ui/markdown", () => ({ Markdown: ({ children }: { children: string }) => <div>{children}</div> }));

const sha = "a".repeat(40), digest = "b".repeat(64);
const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });
const push = vi.fn();
const api = {
  getMaterialSettings: vi.fn(), saveMaterialSettings: vi.fn(), listMaterials: vi.fn(), createMaterial: vi.fn(), getMaterial: vi.fn(), sendMaterialCommand: vi.fn(),
  listMembers: vi.fn(), listAgents: vi.fn(), listProjectResources: vi.fn(), listGitHubInstallations: vi.fn(), listVCSConnections: vi.fn(),
};
function envelope(auto = true): MaterialSettingsEnvelope {
  return { workspace_id: "ws", project_id: "project", can_configure: true, settings: {
    version: 1, enabled: true, provider: "github", resource_id: "repo", connection_id: "installation", repository: "acme/materials", base_branch: "main", root_path: "", reviewer_ids: ["user"], plan_agent_id: null, auto_start_plan: auto,
  } };
}
function detail(): MaterialDetail {
  return {
    material: { id: "material", workspace_id: "ws", project_id: "project", title: "Refund requirements", path: "requirements/REQ-1/prd.md", version: 2, status: "in_review", blocker: null, created_at: "2026-11-26T00:00:00Z", updated_at: "2026-11-26T00:01:00Z" },
    revisions: [{ id: "revision", kind: "requirement", content: "The full requirement includes the final acceptance detail.", plan_content: null, content_sha256: digest, sources: [{ resource_id: "repo", commit: sha, path: "src/refund.go" }], acceptance_criteria: ["AC1: Refund succeeds"], blocking_questions: [], source_baseline_id: null, publication_status: "in_review", review_url: "https://github.com/acme/materials/pull/1", head_sha: sha, created_at: "2026-11-26T00:01:00Z" }],
    decisions: [], comments: [], baseline: null, plan: null, capabilities: { can_edit: true, can_review: true, can_start_plan: false, can_retry: true },
  };
}
function render(ui: React.ReactElement, locale: "en" | "zh-Hans" = "en", search = "section=materials") {
  return renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ push, replace: vi.fn(), back: vi.fn(), pathname: "/acme/projects/project", searchParams: new URLSearchParams(search), hash: "", getShareableUrl: (path) => path }}>{ui}</NavigationProvider></QueryClientProvider>, { locale });
}
beforeEach(() => {
  vi.clearAllMocks(); client.clear(); useMaterialDraftStore.getState().clearDraft(); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  api.getMaterialSettings.mockResolvedValue(envelope());
  api.saveMaterialSettings.mockImplementation(async (_id, input) => ({ ...envelope(input.auto_start_plan), settings: { ...envelope(input.auto_start_plan).settings, version: 2 } }));
  api.listMaterials.mockResolvedValue({ workspace_id: "ws", project_id: "project", materials: [detail().material] });
  api.getMaterial.mockImplementation(async () => detail()); api.sendMaterialCommand.mockImplementation(async () => detail());
  api.listMembers.mockResolvedValue([{ user_id: "user", name: "Alice", role: "admin" }]); api.listAgents.mockResolvedValue([]);
  api.listProjectResources.mockResolvedValue({ resources: [{ id: "repo", workspace_id: "ws", project_id: "project", resource_type: "github_repo", resource_ref: { url: "https://github.com/acme/materials" }, label: "Materials repository" }], total: 1 });
  api.listGitHubInstallations.mockResolvedValue({ configured: true, installations: [{ id: "installation", account_login: "acme" }] });
  api.listVCSConnections.mockResolvedValue({ configured: true, connections: [] });
  setApiInstance(api as unknown as ApiClient);
});
afterEach(() => { cleanup(); client.clear(); onlineManager.setOnline(true); });

describe("shared material workflow", () => {
  it("opens a material inside the shared project route", async () => {
    render(<ProjectMaterials workspaceId="ws" projectId="project" />);
    await userEvent.click(await screen.findByRole("button", { name: /Refund requirements/ }));
    expect(push).toHaveBeenCalledWith(paths.workspace("acme").projectMaterial("project", "material"));
  });

  it("saves explicit disabled auto-start and closes only after the server confirms", async () => {
    const close = vi.fn();
    let confirm!: (value: MaterialSettingsEnvelope) => void;
    api.saveMaterialSettings.mockReturnValue(new Promise<MaterialSettingsEnvelope>((resolve) => { confirm = resolve; }));
    render(<ProjectMaterialSettings workspaceId="ws" projectId="project" envelope={envelope()} onClose={close} />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Save" })).toBeEnabled());
    await userEvent.click(screen.getByRole("switch", { name: "Start planning after verified merge" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(close).not.toHaveBeenCalled();
    expect(api.saveMaterialSettings).toHaveBeenCalledWith("project", expect.objectContaining({ auto_start_plan: false, expected_version: 1, connection_id: "installation" }), { workspaceId: "ws" });
    await act(async () => { confirm(envelope(false)); });
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
  });

  it("reads a saved false setting without restoring the default", async () => {
    render(<ProjectMaterialSettings workspaceId="ws" projectId="project" envelope={envelope(false)} onClose={vi.fn()} />);
    expect(screen.getByRole("switch", { name: "Start planning after verified merge" })).not.toBeChecked();
    expect(screen.getByText(/Turning this on later does not start existing waiting tasks/)).toBeInTheDocument();
  });

  it("keeps the settings input mounted when a background configuration refresh fails", async () => {
    render(<ProjectMaterials workspaceId="ws" projectId="project" />);
    await userEvent.click(await screen.findByRole("button", { name: "Repository and workflow" }));
    const repository = screen.getByLabelText("Repository path");
    fireEvent.change(repository, { target: { value: "acme/unsaved-materials" } });
    repository.focus();
    api.getMaterialSettings.mockRejectedValueOnce(new ApiError("unavailable", 503, "Service Unavailable"));
    await act(async () => { await client.invalidateQueries({ queryKey: materialKeys.settings("ws", "project") }); });
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByLabelText("Repository path")).toBe(repository);
    expect(repository).toHaveValue("acme/unsaved-materials");
    expect(repository).toHaveFocus();
  });

  it("preserves the review input and its pinned revision through a failed poll", async () => {
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    const reason = await screen.findByLabelText("Review reason");
    fireEvent.change(reason, { target: { value: "Unsent review explanation" } });
    reason.focus();
    api.getMaterial.mockRejectedValueOnce(new ApiError("unavailable", 503, "Service Unavailable"));
    await act(async () => { await client.invalidateQueries({ queryKey: materialKeys.detail("ws", "project", "material") }); });
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByLabelText("Review reason")).toBe(reason);
    expect(reason).toHaveValue("Unsent review explanation");
    expect(reason).toHaveFocus();
    expect(api.sendMaterialCommand).not.toHaveBeenCalled();
  });

  it("fails an offline approval promptly and waits for an explicit original-request retry", async () => {
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    const approve = await screen.findByRole("button", { name: "Approve this version" });
    api.sendMaterialCommand.mockRejectedValueOnce(new TypeError("Network disconnected"));
    onlineManager.setOnline(false);
    await userEvent.click(approve);
    await screen.findByRole("button", { name: "Retry the same request" });
    expect(api.sendMaterialCommand).toHaveBeenCalledOnce();
    await act(async () => { onlineManager.setOnline(true); });
    expect(api.sendMaterialCommand).toHaveBeenCalledOnce();
    await userEvent.click(screen.getByRole("button", { name: "Retry the same request" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledTimes(2));
    expect(api.sendMaterialCommand.mock.calls[1]).toEqual(api.sendMaterialCommand.mock.calls[0]);
  });

  it("keeps approval pinned when polling discovers a newer revision", async () => {
    const current = detail(); api.getMaterial.mockResolvedValue(current);
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    expect(await screen.findByText(current.revisions[0]!.content)).toBeVisible();
    await userEvent.type(screen.getByLabelText("Review reason"), "Reviewed the original text");
    const changed = detail(); changed.material.version = 3;
    changed.revisions.push({ ...changed.revisions[0]!, id: "new-revision", content: "A changed requirement", head_sha: "c".repeat(40), created_at: "2026-11-26T00:02:00Z" });
    act(() => client.setQueryData(materialKeys.detail("ws", "project", "material"), changed));
    expect(screen.getByText(current.revisions[0]!.content)).toBeVisible();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Approve this version" })).not.toBeInTheDocument());
    expect(api.sendMaterialCommand).not.toHaveBeenCalled();
  });

  it("opens the last server revision and compares the preceding one before any baseline exists", async () => {
    const current = detail();
    const older = current.revisions[0]!;
    current.revisions.push({ ...older, id: "latest", content: "Updated refund policy" });
    api.getMaterial.mockResolvedValue(current);
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    expect(await screen.findByText("Updated refund policy")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Compare with previous revision" }));
    expect(screen.getByText(older.content)).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Approve this version" }));
    expect(api.sendMaterialCommand.mock.calls[0]?.[2].data.revision_id).toBe("latest");
  });

  it("keeps the project and editor draft available after a source permission refusal", async () => {
    api.sendMaterialCommand.mockRejectedValueOnce(new ApiError("Source unavailable", 403, "Forbidden", { code: "material_source_unavailable" }));
    render(<ProjectMaterials workspaceId="ws" projectId="project" />, "en", "section=materials&material=material");
    await userEvent.click(await screen.findByRole("button", { name: "New revision" }));
    fireEvent.change(screen.getByLabelText("Requirement document"), { target: { value: "Pending correction" } });
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("alert");
    expect(screen.getByLabelText("Requirement document")).toHaveValue("Pending correction");
    expect(useProjectAccessStore.getState().denied).toEqual({});
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();
  });

  it("retries the same uncertain approval and does not announce a merge", async () => {
    api.sendMaterialCommand.mockRejectedValueOnce(new TypeError("Network disconnected")).mockResolvedValue(detail());
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "Approve this version" }));
    expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve this version" })).toBeDisabled();
    expect(screen.getByText(/No verified requirement baseline yet/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry the same request" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledTimes(2));
    expect(api.sendMaterialCommand.mock.calls[1]).toEqual(api.sendMaterialCommand.mock.calls[0]);
    expect(api.sendMaterialCommand.mock.calls[0]?.[2].data).toMatchObject({ revision_id: "revision", head_sha: sha, expected_version: 2, decision: "approve" });
  });

  it("requires a fresh review when the same revision receives a different remote head", async () => {
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await screen.findByRole("button", { name: "Approve this version" });
    await userEvent.type(screen.getByLabelText("Review reason"), "Original head reviewed");
    const changed = detail(); changed.revisions[0]!.head_sha = "c".repeat(40); changed.material.version = 3;
    act(() => client.setQueryData(materialKeys.detail("ws", "project", "material"), changed));
    await screen.findByRole("button", { name: "Review the displayed version" });
    expect(screen.queryByRole("button", { name: "Approve this version" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Review the displayed version" }));
    expect(await screen.findByLabelText("Review reason")).toHaveValue("");
    expect(api.sendMaterialCommand).not.toHaveBeenCalled();
  });

  it("saves draft content without implicitly requesting publication", async () => {
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "New revision" }));
    fireEvent.change(screen.getByLabelText("Requirement document"), { target: { value: "An incomplete new draft" } });
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalled());
    expect(api.sendMaterialCommand.mock.calls[0]?.[2]).toMatchObject({ action: "revisions", data: { kind: "requirement", content: "An incomplete new draft", ready: false } });
  });

  it("recovers an uncertain original request after remount", async () => {
    api.sendMaterialCommand.mockRejectedValueOnce(new TypeError("Network disconnected")).mockResolvedValue(detail());
    const view = render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "Approve this version" }));
    await screen.findByText(/The result is not confirmed/);
    const original = api.sendMaterialCommand.mock.calls[0];
    view.unmount();
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "Retry the same request" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledTimes(2));
    expect(api.sendMaterialCommand.mock.calls[1]).toEqual(original);
  });

  it("retains unsaved editor text across navigation", async () => {
    const view = render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "New revision" }));
    fireEvent.change(screen.getByLabelText("Requirement document"), { target: { value: "My unsaved refinement" } });
    view.unmount();
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    await userEvent.click(await screen.findByRole("button", { name: "New revision" }));
    expect(screen.getByLabelText("Requirement document")).toHaveValue("My unsaved refinement");
  });

  it("starts the same waiting planning task, then edits both design and task definitions", async () => {
    const current = detail(); current.material.status = "baseline_ready";
    current.baseline = { id: "baseline", revision_id: "revision", commit: sha, path: current.material.path, content_sha256: digest, review_url: current.revisions[0]!.review_url!, verified_at: "2026-11-26T00:02:00Z" };
    current.plan = { issue_id: "plan-issue", status: "waiting_manual", blocker: null, baseline_id: "baseline", revision_id: null };
    current.capabilities.can_start_plan = true;
    api.getMaterial.mockResolvedValue(current); api.sendMaterialCommand.mockResolvedValue(current);
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    expect(await screen.findByText(/Waiting for manual start/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Start planning" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledWith("project", "material", expect.objectContaining({ action: "start-plan", data: expect.objectContaining({ expected_version: 2 }) }), { workspaceId: "ws" }));
    await userEvent.click(screen.getByRole("button", { name: "Write a plan revision" }));
    fireEvent.change(screen.getByLabelText("Technical design"), { target: { value: "Design from the approved requirement" } });
    fireEvent.change(screen.getByLabelText("Task breakdown"), { target: { value: "TASK1: Implement refund validation" } });
    await userEvent.click(screen.getByLabelText("Clarification is complete; publish this revision automatically when saved"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledTimes(2));
    expect(api.sendMaterialCommand.mock.calls[1]?.[2]).toMatchObject({ action: "revisions", data: { kind: "plan", plan_content: "TASK1: Implement refund validation", source_baseline_id: "baseline", ready: true } });
  });

  it("retries a finished planning attempt only on explicit start and keeps its task link", async () => {
    const current = detail(); current.material.status = "planning";
    current.baseline = { id: "baseline", revision_id: "revision", commit: sha, path: current.material.path, content_sha256: digest, review_url: current.revisions[0]!.review_url!, verified_at: "2026-11-26T00:02:00Z" };
    current.plan = { issue_id: "plan-issue", status: "blocked", blocker: "material_plan_run_finished", baseline_id: "baseline", revision_id: null };
    current.capabilities.can_start_plan = true;
    api.getMaterial.mockResolvedValue(current);
    api.sendMaterialCommand.mockResolvedValue({ ...current, plan: { ...current.plan, status: "queued", blocker: null }, capabilities: { ...current.capabilities, can_start_plan: false } });
    render(<MaterialDetailView workspaceId="ws" projectId="project" materialId="material" onBack={vi.fn()} />);
    const start = await screen.findByRole("button", { name: "Start planning" });
    const task = screen.getByRole("link", { name: "Open planning task" });
    expect(task).toHaveAttribute("href", paths.workspace("acme").issueDetail("plan-issue"));
    expect(api.sendMaterialCommand).not.toHaveBeenCalled();
    await act(async () => { onlineManager.setOnline(false); onlineManager.setOnline(true); });
    expect(api.sendMaterialCommand).not.toHaveBeenCalled();
    await userEvent.click(start);
    await waitFor(() => expect(api.sendMaterialCommand).toHaveBeenCalledOnce());
    expect(api.sendMaterialCommand).toHaveBeenCalledWith("project", "material", {
      action: "start-plan", data: { request_id: expect.any(String), expected_version: current.material.version },
    }, { workspaceId: "ws" });
    expect(screen.getByRole("link", { name: "Open planning task" })).toHaveAttribute("href", paths.workspace("acme").issueDetail("plan-issue"));
  });

  it("removes private material content when access is revoked", async () => {
    render(<ProjectMaterials workspaceId="ws" projectId="project" />, "en", "section=materials&material=material");
    expect(await screen.findByText(detail().revisions[0]!.content)).toBeVisible();
    api.getMaterial.mockRejectedValue(new ApiError("denied", 403, "Forbidden"));
    await act(async () => { await client.invalidateQueries({ queryKey: materialKeys.detail("ws", "project", "material") }); });
    await waitFor(() => expect(screen.queryByText(detail().revisions[0]!.content)).not.toBeInTheDocument());
    expect(useProjectAccessStore.getState().denied[JSON.stringify(["ws", "*"])]).toBe(true);
  });

  it("renders Chinese product copy", async () => {
    render(<ProjectMaterials workspaceId="ws" projectId="project" />, "zh-Hans");
    expect(await screen.findByRole("heading", { name: "工程物料" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "新建需求" })).toBeInTheDocument();
  });
});
