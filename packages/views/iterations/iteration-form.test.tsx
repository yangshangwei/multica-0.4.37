import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { IterationForm } from "./iteration-form";
import { receipt, source, statistics } from "./test-fixtures";
import type { Iteration } from "@multica/core/iterations";
import projects from "../locales/en/projects.json";
const session = vi.hoisted(() => ({ value: "session" }));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (fn: (x: typeof projects) => string) => fn(projects) }),
}));
vi.mock("../navigation", () => ({ useNavigation: () => ({ push: vi.fn() }) }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    iterationDetail: (id: string) => `/iterations/${id}`,
  }),
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) =>
      select({ user: { id: "actor" } }),
    { getState: () => ({ user: { id: "actor" } }) },
  ),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: async () => [] }),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => session.value,
    listIterations: vi.fn(),
    createIteration: vi.fn(),
    updateIteration: vi.fn(),
    getIterationOperation: vi.fn(),
    getIteration: vi.fn(),
  },
}));
beforeEach(() => {
  vi.clearAllMocks();
  session.value = "session";
  window.localStorage.clear();
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: source.workspace_id, items: [], next_cursor: null });
  vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: source.workspace_id, iteration: source, statistics, snapshot: null });
});

function mountEdit(initial: Iteration = source) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const ui = (iteration: Iteration) => <QueryClientProvider client={client}><IterationForm wsId={iteration.workspace_id} timezone={iteration.timezone} iteration={iteration} /></QueryClientProvider>;
  const view = render(ui(initial));
  fireEvent.click(screen.getByText("Edit iteration", { selector: "summary" }));
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Correct the plan" } });
  return { client, rerender: (iteration: Iteration) => view.rerender(ui(iteration)) };
}
const change = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } });
const submitEdit = () => fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

describe("iteration edit snapshot", () => {
  it("locks input until the post-commit refresh finishes, even when props update first", async () => {
    let resolveRead!: (value: Awaited<ReturnType<typeof api.getIteration>>) => void;
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockImplementation(() => new Promise((resolve) => { resolveRead = resolve; }));
    const view = mountEdit(); change("Name", "Saved name"); submitEdit();
    await waitFor(() => expect(api.getIteration).toHaveBeenCalledOnce());
    const current = { ...source, name: "Authoritative name", revision: 8 };
    view.rerender(current);
    expect(screen.getByLabelText("Name")).toBeDisabled();
    expect(screen.getByLabelText("Name")).toHaveValue("Saved name");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    await act(async () => resolveRead({ workspace_id: source.workspace_id, iteration: current, statistics, snapshot: null }));
    expect(screen.getByLabelText("Name")).toBeEnabled();
    expect(screen.getByLabelText("Name")).toHaveValue("Authoritative name");
  });
  it("does not adopt a delayed refresh from the previous API session", async () => {
    let resolveRead!: (value: Awaited<ReturnType<typeof api.getIteration>>) => void;
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockImplementation(() => new Promise((resolve) => { resolveRead = resolve; }));
    mountEdit(); change("Name", "Saved name"); submitEdit();
    await waitFor(() => expect(api.getIteration).toHaveBeenCalledOnce());
    session.value = "replacement-session";
    await act(async () => resolveRead({ workspace_id: source.workspace_id, iteration: { ...source, name: "Late old-session text", revision: 8 }, statistics, snapshot: null }));
    expect(screen.getByLabelText("Name")).not.toHaveValue("Late old-session text");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(api.updateIteration).toHaveBeenCalledTimes(1);
  });
  it("hides protected input if the post-commit refresh loses access", async () => {
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockRejectedValue(new ApiError("Forbidden", 403, "Forbidden"));
    mountEdit(); change("Name", "Saved name"); submitEdit();
    await screen.findByRole("alert");
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
    expect(api.updateIteration).toHaveBeenCalledTimes(1);
  });
  it("recovers the exact unknown-result payload despite new input props", async () => {
    vi.mocked(api.updateIteration).mockRejectedValueOnce(new Error("Response lost")).mockResolvedValue(receipt);
    vi.mocked(api.getIterationOperation).mockRejectedValue(new ApiError("Not found", 404, "Not Found", { code: "operation_not_found" }));
    const view = mountEdit(); change("Name", "Original intent"); submitEdit();
    await screen.findByRole("alert");
    const original = vi.mocked(api.updateIteration).mock.calls[0]![2];
    view.rerender({ ...source, name: "Remote name", revision: 9 });
    expect(screen.getByLabelText("Name")).toBeDisabled();
    submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.updateIteration).mock.calls[1]![2]).toEqual(original);
    expect(api.getIterationOperation).toHaveBeenCalledWith(source.workspace_id, original.request_id);
  });
  it("does not submit fields made immutable by a remote status change", async () => {
    vi.mocked(api.updateIteration).mockRejectedValue(new ApiError("Conflict", 409, "Conflict"));
    const view = mountEdit({ ...source, status: "planned" });
    change("Name", "Local name"); change("Start date", "2026-10-02"); change("End date", "2026-10-20");
    view.rerender({ ...source, status: "completed", revision: 3 });
    expect(screen.getByLabelText("Start date")).toBeDisabled();
    expect(screen.getByLabelText("End date")).toBeDisabled();
    expect(screen.getByLabelText("Coordinator")).toBeDisabled();
    submitEdit(); await waitFor(() => expect(api.updateIteration).toHaveBeenCalledOnce());
    expect(vi.mocked(api.updateIteration).mock.calls[0]![2].fields).toEqual({ name: "Local name" });
  });
  it("retains authoritative status restrictions after a successful refresh with older props", async () => {
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: source.workspace_id, iteration: { ...source, name: "Saved name", status: "completed", revision: 8 }, statistics, snapshot: null });
    mountEdit(); change("Name", "Saved name"); submitEdit();
    await waitFor(() => expect(api.getIteration).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByLabelText("Name")).toBeEnabled());
    expect(screen.getByLabelText("End date")).toBeDisabled();
    expect(screen.getByLabelText("Coordinator")).toBeDisabled();
  });
  it("keeps the dirty draft's revision and patches only changed fields after a remote refresh", async () => {
    vi.mocked(api.updateIteration).mockRejectedValue(new ApiError("Conflict", 409, "Conflict"));
    const view = mountEdit();
    change("Name", "My name");
    view.rerender({ ...source, description: "Remote description", revision: 3 });
    submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledOnce());
    expect(vi.mocked(api.updateIteration).mock.calls[0]![2]).toMatchObject({ expected_revision: 2, fields: { name: "My name" } });
    expect(vi.mocked(api.updateIteration).mock.calls[0]![2].fields).toEqual({ name: "My name" });
  });
  it("adopts a refreshed clean snapshot into both visible fields and the next edit baseline", async () => {
    vi.mocked(api.updateIteration).mockRejectedValue(new ApiError("Conflict", 409, "Conflict"));
    const view = mountEdit();
    view.rerender({ ...source, name: "Remote name", description: "Remote description", revision: 3 });
    expect(screen.getByLabelText("Name")).toHaveValue("Remote name");
    expect(screen.getByLabelText("Description")).toHaveValue("Remote description");
    change("Name", "My correction");
    submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledOnce());
    expect(vi.mocked(api.updateIteration).mock.calls[0]![2]).toMatchObject({ expected_revision: 3, fields: { name: "My correction" } });
  });
  it.each(["Use server version", "Keep my changes"])("retains input on conflict and explicitly resolves with %s", async (action) => {
    const current = { ...source, name: "Server name", description: "Remote description", revision: 3 };
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: source.workspace_id, iteration: current, statistics, snapshot: null });
    vi.mocked(api.updateIteration).mockRejectedValue(new ApiError("Conflict", 409, "Conflict", { code: "iteration_revision_conflict" }));
    mountEdit(); change("Name", "Local name"); submitEdit();
    await screen.findByRole("button", { name: action });
    expect(screen.getByLabelText("Name")).toHaveValue("Local name");
    expect(screen.getByRole("alert")).toHaveTextContent("Remote description");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: action }));
    expect(screen.getByLabelText("Description")).toHaveValue("Remote description");
    expect(screen.getByLabelText("Name")).toHaveValue(action === "Use server version" ? "Server name" : "Local name");
    expect(api.updateIteration).toHaveBeenCalledTimes(1);
    change("Name", "Final name"); change("Reason", "Reviewed conflict"); submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.updateIteration).mock.calls[1]![2]).toMatchObject({ expected_revision: 3, fields: { name: "Final name" } });
    expect(vi.mocked(api.updateIteration).mock.calls[1]![2].fields).toEqual({ name: "Final name" });
  });
  it("uses the authoritative refreshed revision after success for a second edit", async () => {
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: source.workspace_id, iteration: { ...source, name: "Saved name", revision: 8 }, statistics, snapshot: null });
    mountEdit(); change("Name", "Saved name"); submitEdit();
    await waitFor(() => expect(api.getIteration).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByLabelText("Name")).toBeEnabled());
    change("Name", "Second name"); change("Reason", "Second correction"); submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.updateIteration).mock.calls[1]![2]).toMatchObject({ expected_revision: 8, fields: { name: "Second name" } });
  });
  it("retries only the resource refresh after a committed edit and failed read", async () => {
    vi.mocked(api.updateIteration).mockResolvedValue(receipt);
    vi.mocked(api.getIteration).mockRejectedValueOnce(new Error("Offline")).mockResolvedValue({ workspace_id: source.workspace_id, iteration: { ...source, name: "Saved name", revision: 8 }, statistics, snapshot: null });
    mountEdit(); change("Name", "Saved name"); submitEdit();
    await screen.findByText("Changes saved. Refresh the iteration before editing again.");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(screen.getByLabelText("Name")).toHaveValue("Saved name");
    fireEvent.click(screen.getByRole("button", { name: "Retry refresh" }));
    await waitFor(() => expect(screen.getByLabelText("Name")).toBeEnabled());
    expect(api.updateIteration).toHaveBeenCalledTimes(1);
    change("Name", "Second name"); change("Reason", "Second correction"); submitEdit();
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.updateIteration).mock.calls[1]![2].expected_revision).toBe(8);
  });
});
describe("iteration form interaction", () => {
  it("warns about overlapping open periods without moving or disabling dates", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: source.workspace_id, items: [source], next_cursor: null });
    render(<QueryClientProvider client={new QueryClient()}><IterationForm wsId={source.workspace_id} timezone="UTC" /></QueryClientProvider>);
    fireEvent.change(screen.getByLabelText("Start date"), { target: { value: source.start_date } });
    fireEvent.change(screen.getByLabelText("End date"), { target: { value: source.end_date } });
    expect(await screen.findByRole("status")).toHaveTextContent("These dates overlap");
    expect(screen.getByLabelText("Start date")).toHaveValue(source.start_date);
    expect(screen.getByRole("button", { name: "Create iteration", hidden: true })).toBeEnabled();
  });
  it("supports keyboard submit and retains long input after a confirmed conflict", async () => {
    vi.mocked(api.createIteration).mockRejectedValue(
      new ApiError("Conflict", 409, "Conflict"),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <IterationForm wsId="w" timezone="UTC" />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(
      screen.getByText("Create iteration", { selector: "summary" }),
    );
    const name = "Iteration ".repeat(18);
    await user.type(screen.getByLabelText("Name"), name);
    fireEvent.change(screen.getByLabelText("Start date"), {
      target: { value: "2026-10-01" },
    });
    fireEvent.change(screen.getByLabelText("End date"), {
      target: { value: "2026-10-14" },
    });
    screen.getByRole("button", { name: "Create iteration" }).focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("alert");
    expect(screen.getByLabelText("Name")).toHaveValue(name);
    await waitFor(() => expect(screen.getByLabelText("Name")).toBeEnabled());
    const first = vi.mocked(api.createIteration).mock.calls[0]![1];
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Revised");
    await user.click(screen.getByRole("button", { name: "Create iteration" }));
    await waitFor(() => expect(api.createIteration).toHaveBeenCalledTimes(2));
    expect(
      vi.mocked(api.createIteration).mock.calls[1]![1].request_id,
    ).not.toBe(first.request_id);
  });
  it("explains oversized Unicode names without truncating the input", async () => {
    vi.mocked(api.createIteration).mockRejectedValue(
      new ApiError("Conflict", 409, "Conflict"),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <IterationForm wsId="w" timezone="UTC" />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(
      screen.getByText("Create iteration", { selector: "summary" }),
    );
    const input = screen.getByLabelText("Name");
    fireEvent.change(input, { target: { value: "🚀".repeat(201) } });
    await user.click(screen.getByRole("button", { name: "Create iteration" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("200");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAccessibleDescription(expect.stringContaining("200"));
    expect(input).toHaveValue("🚀".repeat(201));
    expect(api.createIteration).not.toHaveBeenCalled();
    fireEvent.change(input, { target: { value: "🚀".repeat(200) } });
    await user.click(screen.getByRole("button", { name: "Create iteration" }));
    await waitFor(() => expect(api.createIteration).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.createIteration).mock.calls[0]![1].name).toBe(
      "🚀".repeat(200),
    );
    expect(input).not.toHaveAttribute("maxlength");
  });
  it("keeps a documented deleted-iteration rejection editable for a new intent", async () => {
    vi.mocked(api.updateIteration).mockRejectedValue(
      new ApiError("Deleted", 404, "Not Found", {
        code: "iteration_not_found",
      }),
    );
    render(
      <QueryClientProvider client={new QueryClient()}>
        <IterationForm
          wsId={source.workspace_id}
          timezone={source.timezone}
          iteration={source}
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(
      screen.getByText("Edit iteration", { selector: "summary" }),
    );
    await user.type(screen.getByLabelText("Reason"), "Correct name");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await screen.findByRole("alert");
    expect(screen.getByLabelText("Name")).toBeEnabled();
    expect(screen.getByLabelText("Name")).toHaveValue(source.name);
    const first = vi.mocked(api.updateIteration).mock.calls[0]![2].request_id;
    await user.type(screen.getByLabelText("Name"), " revised");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(api.updateIteration).toHaveBeenCalledTimes(2));
    expect(
      vi.mocked(api.updateIteration).mock.calls[1]![2].request_id,
    ).not.toBe(first);
  });
});
