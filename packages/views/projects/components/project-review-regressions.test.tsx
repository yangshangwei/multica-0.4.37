import React, { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { clearClientSessionData } from "@multica/core/platform";
import { setApiInstance, ApiError } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectProgressDraftStore, useProjectDescriptionDraftStore, useProjectAccessStore, clearProtectedProjectContent, markProjectDeleted } from "@multica/core/projects";
import { p1Project, p1Preview, p1Revision, p1Member, P1_UPDATE_ID } from "@multica/core/projects/test-fixtures/p1";
import type { Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ProjectDescription } from "./project-description";
import { ProjectProgress } from "./project-progress";
import { useProjectPropertyEditor } from "./project-property-recovery";

const editorEmitted = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "11111111-1111-4111-8111-111111111111" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/source-workspace/issues/${id}` }) }));
vi.mock("../../navigation", () => ({ AppLink: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
vi.mock("../../common/task-transcript", () => ({ AgentTranscriptDialog: ({ task }: { task: { id: string } }) => <div role="dialog">{task.id}</div>, buildTimeline: (messages: unknown) => messages }));
vi.mock("../../editor", () => ({ ContentEditor: forwardRef(function Editor({ value, defaultValue, onUpdate, debounceMs = 300, flushPendingOnUnmount = false }: { value?: string; defaultValue?: string; onUpdate: (value: string, base: string) => void; debounceMs?: number; flushPendingOnUnmount?: boolean }, ref) {
  const [text, setText] = useState(value ?? defaultValue ?? ""); const current = useRef(text); const emitted = useRef(text); const base = useRef(value ?? ""); const pending = useRef(false); const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined); const callback = useRef(onUpdate); callback.current = onUpdate;
  useImperativeHandle(ref, () => ({ getMarkdown: () => current.current, focus: () => {}, adoptContent: (next: string) => { current.current = next; emitted.current = next; base.current = next; setText(next); } }));
  useEffect(() => { if (value !== undefined && current.current === emitted.current) { current.current = value; emitted.current = value; base.current = value; setText(value); } }, [value]);
  useEffect(() => () => { clearTimeout(timer.current); if (flushPendingOnUnmount && pending.current) { emitted.current = current.current; callback.current(current.current, base.current); } }, [flushPendingOnUnmount]);
  return <textarea aria-label="Editor" value={text} onChange={(event) => { current.current = event.target.value; setText(current.current); pending.current = true; clearTimeout(timer.current); timer.current = setTimeout(() => { pending.current = false; emitted.current = current.current; editorEmitted(current.current); callback.current(current.current, base.current); }, debounceMs); }} />;
}) }));
let qc: QueryClient;
const project: Project = { ...p1Project, status: "in_progress", priority: "medium", lead_type: "member" };
function render(ui: React.ReactElement) { return renderWithI18n(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>); }
function install(extra: object = {}) {
  setApiInstance({ getBaseUrl: () => "https://source.test", listProjectUpdates: vi.fn().mockResolvedValue({ items: [], next_cursor: null }), ...extra } as unknown as ApiClient);
}
beforeEach(() => { editorEmitted.mockClear(); qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }); useProjectProgressDraftStore.getState().clearDraft(); useProjectDescriptionDraftStore.getState().clearDraft(); });
afterEach(() => { qc.clear(); vi.restoreAllMocks(); });

it("FR05 explicitly discards the rejected description when choosing the server version", async () => {
  const server = { ...project, description: "Chosen server text", description_revision: 8 };
  install({ updateProject: vi.fn().mockRejectedValue(new ApiError("Conflict", 409, "Conflict", { code: "project_description_conflict", current: server })) });
  const first = render(<ProjectDescription project={project} supported />);
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Rejected local text" } });
  await screen.findByRole("button", { name: "Use server version" }, { timeout: 2000 });
  fireEvent.click(screen.getByRole("button", { name: "Use server version" }));
  expect(useProjectDescriptionDraftStore.getState().draft.entries).toEqual({});
  first.unmount(); render(<ProjectDescription project={server} supported />);
  expect(screen.getByLabelText("Editor")).toHaveValue("Chosen server text");
  expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
});
it("FR04 a description403 clears the editor, drafts and mutation payload without WS", async () => {
  install({ updateProject: vi.fn().mockRejectedValue(new ApiError("Forbidden", 403, "Forbidden")) });
  render(<ProjectDescription project={project} supported />);
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "confidential text" } });
  await waitFor(() => expect(screen.queryByLabelText("Editor")).not.toBeInTheDocument(), { timeout: 2000 });
  expect(useProjectDescriptionDraftStore.getState().draft.entries).toEqual({});
  expect(JSON.stringify(qc.getMutationCache().getAll().map((mutation) => mutation.state))).not.toContain("confidential");
});
it.each(["cancel", "navigate"])("FR06 keeps text typed within the debounce before %s", async (mode) => {
  install(); const first = render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Last keystroke" } });
  if (mode === "cancel") fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
  first.unmount(); render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  expect(screen.getByLabelText("Editor")).toHaveValue("Last keystroke");
});
it("FR06 revocation unmount cannot re-persist pending protected editor text", () => {
  install(); const first = render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Must erase" } });
  act(() => clearProtectedProjectContent(qc, project.workspace_id)); first.unmount();
  expect(useProjectProgressDraftStore.getState().draft.entries).toEqual({});
});
it("FR08 deletion retains only unsent text, then true revocation erases it", () => {
  install(); const first = render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Copy this final text" } });
  act(() => markProjectDeleted(qc, project.workspace_id, project.id)); first.unmount();
  expect(useProjectAccessStore.getState().deleted[JSON.stringify([project.workspace_id, project.id])]).toEqual(["Copy this final text"]);
  expect(useProjectProgressDraftStore.getState().draft.entries).toEqual({});
  act(() => clearProtectedProjectContent(qc, project.workspace_id)); expect(useProjectAccessStore.getState().deleted).toEqual({});
});
it("FR07 issue evidence uses the source workspace route and execution uses its authorized reader", async () => {
  const executionId = "66666666-6666-4666-8666-666666666666";
  const evidence = (kind: "issue" | "execution", id: string) => ({ input: { kind, id, url: null }, observed_version: { kind, id, revision: 1 }, collected_at: p1Revision.created_at, availability: "available", current_version: null, label: kind, href: null });
  const getProjectExecutionEvidence = vi.fn().mockResolvedValue({ task: { id: executionId }, messages: [] });
  install({ getProjectExecutionEvidence, listProjectUpdates: vi.fn().mockResolvedValue({ items: [{ workspace_id: project.workspace_id, project_id: project.id, id: P1_UPDATE_ID, author: p1Member, published_at: p1Revision.created_at, current_revision: 1, current: { ...p1Revision, evidence: [evidence("issue", project.id), evidence("execution", executionId)] } }], next_cursor: null }) });
  render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  expect(await screen.findByRole("link", { name: "issue" })).toHaveAttribute("href", `/source-workspace/issues/${project.id}`);
  fireEvent.click(screen.getByRole("button", { name: "execution" }));
  await screen.findByRole("dialog");
  expect(getProjectExecutionEvidence).toHaveBeenCalledWith(project.workspace_id, project.id, P1_UPDATE_ID, 1, executionId, { signal: expect.any(AbortSignal) });
});
function PropertyHarness() { const editor = useProjectPropertyEditor(project); return <><button onClick={() => editor.send({ status: "paused" })}>Change status</button><button onClick={() => editor.send({ start_date: "2026-12-10" })}>Change date</button>{editor.recovery}</>; }
it("FR09 preserves a failed status patch and retries only explicitly against the latest revision", async () => {
  const updateProject = vi.fn().mockRejectedValueOnce(new ApiError("Project changed", 409, "Conflict", { code: "project_revision_conflict" })).mockResolvedValueOnce({ ...project, revision: 10, status: "paused" });
  install({ updateProject, getProject: vi.fn().mockResolvedValue({ ...project, revision: 9 }) });
  render(<PropertyHarness />); fireEvent.click(screen.getByRole("button", { name: "Change status" }));
  await screen.findByText("Project changes were not saved"); expect(screen.getByText("paused")).toBeInTheDocument();
  expect(updateProject).toHaveBeenCalledTimes(1); fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  await waitFor(() => expect(updateProject).toHaveBeenCalledTimes(2));
  expect(updateProject.mock.calls[1]?.[1]).toEqual({ status: "paused", expected_revision: 9 });
});

it("source-only403 preserves the user's draft and allows removing the evidence", async () => {
  install({ previewProjectUpdate: vi.fn().mockRejectedValue(new ApiError("Source inaccessible", 403, "Forbidden", { code: "project_evidence_forbidden" })) });
  render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Keep this local explanation" } });
  fireEvent.click(screen.getByRole("button", { name: "Preview publication" }));
  await screen.findByText("An evidence source is no longer accessible. Your text is kept; remove that evidence before previewing again.");
  expect(screen.getByLabelText("Editor")).toHaveValue("Keep this local explanation");
  expect(useProjectAccessStore.getState().denied).toEqual({});
  expect(Object.values(useProjectProgressDraftStore.getState().draft.entries)[0]?.draft.body).toBe("Keep this local explanation");
});

it("operation-level403 after role downgrade keeps the attempted fields and workspace readable", async () => {
  install({ updateProject: vi.fn().mockRejectedValue(new ApiError("Administrator required", 403, "Forbidden", { code: "project_permission_denied" })) });
  render(<PropertyHarness />); fireEvent.click(screen.getByRole("button", { name: "Change status" }));
  await screen.findByText("Administrator required");
  expect(screen.getByText("paused")).toBeInTheDocument(); expect(useProjectAccessStore.getState().denied).toEqual({});
});
it("FR09 date validation keeps the attempted date and shows the specific correction needed", async () => {
  install({ updateProject: vi.fn().mockRejectedValue(new ApiError("Validation failed", 422, "Unprocessable", { field_errors: [{ field: "start_date", message: "Start date must precede the due date" }] })) });
  render(<PropertyHarness />); fireEvent.click(screen.getByRole("button", { name: "Change date" }));
  await screen.findByText("Start date must precede the due date"); expect(screen.getByText("2026-12-10")).toBeInTheDocument();
});

it("RR02 a fast successful preview survives the pending editor unmount flush", async () => {
  const previewProjectUpdate = vi.fn(async (_ws, _id, draft) => ({ ...p1Preview, draft }));
  install({ previewProjectUpdate }); render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Fast preview body" } });
  fireEvent.click(screen.getByRole("button", { name: "Preview publication" }));
  await screen.findByRole("button", { name: "Publish" });
  expect(screen.queryByLabelText("Editor")).not.toBeInTheDocument(); expect(screen.getByText("Fast preview body")).toBeInTheDocument();
  expect(previewProjectUpdate).toHaveBeenCalledTimes(1);
});
it("RR03 choosing authorized serverv2 survives stale controlled v1 props and uses v2 on the next edit", async () => {
  const server = { ...project, description: "Chosen server v2", description_revision: 8 };
  const updateProject = vi.fn().mockRejectedValueOnce(new ApiError("Conflict", 409, "Conflict", { code: "project_description_conflict", current: server }))
    .mockImplementationOnce(async (_id, data) => ({ ...server, description: data.description, description_revision: 9 }));
  install({ updateProject, getProject: vi.fn().mockRejectedValue(new Error("GET unavailable")) });
  render(<ProjectDescription project={project} supported />);
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Rejected local body" } });
  await screen.findByRole("button", { name: "Use server version" }, { timeout: 2000 });
  fireEvent.click(screen.getByRole("button", { name: "Use server version" }));
  expect(screen.getByLabelText("Editor")).toHaveValue("Chosen server v2");
  expect(useProjectDescriptionDraftStore.getState().draft.entries).toEqual({});
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Edited after choosing v2" } });
  await waitFor(() => expect(updateProject).toHaveBeenCalledTimes(2), { timeout: 2000 });
  expect(updateProject.mock.calls[1]?.[1]).toMatchObject({ expected_description_revision: 8 });
});

it("RR02 a slow successful preview still remains reviewable after the debounce already emitted", async () => {
  let finish!: (value: unknown) => void;
  install({ previewProjectUpdate: vi.fn(() => new Promise((resolve) => { finish = resolve; })) });
  render(<ProjectProgress project={project} onProtectedError={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Slow preview body" } });
  fireEvent.click(screen.getByRole("button", { name: "Preview publication" }));
  await waitFor(() => expect(editorEmitted).toHaveBeenCalledWith("Slow preview body"));
  await act(async () => finish({ ...p1Preview, draft: { ...p1Preview.draft, body: "Slow preview body" } }));
  await screen.findByRole("button", { name: "Publish" }); expect(screen.queryByLabelText("Editor")).not.toBeInTheDocument();
});

it.each(["progress", "description"])("RR01 session teardown cannot re-persist pending %s text during unmount", (surface) => {
  install();
  const first = render(surface === "progress" ? <ProjectProgress project={project} onProtectedError={vi.fn()} /> : <ProjectDescription project={project} supported />);
  if (surface === "progress") fireEvent.click(screen.getByRole("button", { name: "Write progress" }));
  fireEvent.change(screen.getByLabelText("Editor"), { target: { value: "Previous user's last keystroke" } });
  act(() => clearClientSessionData(qc)); first.unmount();
  expect(useProjectProgressDraftStore.getState().draft.entries).toEqual({});
  expect(useProjectDescriptionDraftStore.getState().draft.entries).toEqual({});
  expect(useProjectAccessStore.getState().deleted).toEqual({});
});
