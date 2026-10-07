import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { IterationsPage } from "./iteration-page";
import {
  alpha,
  beta,
  previewFor,
  receipt,
  targetA,
  issuePage,
  settings,
  source,
  statistics,
  ws,
} from "./test-fixtures";
import projects from "../locales/en/projects.json";
import issues from "../locales/en/issues.json";
const membership = vi.hoisted(() => ({ role: "member" }));
const route = vi.hoisted(() => ({ pathname: "/acme/iterations", getShareableUrl: (path: string) => `https://multica.test${path}` }));
const copyLink = vi.hoisted(() => vi.fn().mockResolvedValue(true));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: copyLink }));
vi.mock("../i18n", () => ({
  useLocale: () => "en",
  useT: (namespace?: string) => ({
    t: (fn: (x: unknown) => string) =>
      fn(namespace === "issues" ? issues : projects),
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => ws }));
vi.mock("../navigation", () => ({
  useNavigation: () => route,
  AppLink: ({
    href,
    children,
  }: {
    href: string;
    children: React.ReactNode;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    iterations: () => "/acme/iterations",
    iterationDetail: (id: string) => `/acme/iterations/${id}`,
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => membership,
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) =>
      select({ user: { id: ws } }),
    { getState: () => ({ user: { id: ws } }) },
  ),
}));
vi.mock("@multica/core/projects", () => ({
  useProjectPlanningTimezone: () => ({ mutate: vi.fn() }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: async () => [] }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: async () => [] }),
  squadListOptions: () => ({ queryKey: ["squads"], queryFn: async () => [] }),
}));
vi.mock("./iteration-assignment", () => ({ IterationAssignment: () => null }));
vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  LineChart: () => null,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    getIterationCapabilities: vi.fn(),
    getIterationSettings: vi.fn(),
    listIterations: vi.fn(),
    getIteration: vi.fn(),
    getIterationIssues: vi.fn(),
    previewIteration: vi.fn(),
    applyIterationOperation: vi.fn(),
    getIterationOperation: vi.fn(),
  },
}));
beforeEach(() => {
  membership.role = "member";
  vi.resetAllMocks();
  copyLink.mockResolvedValue(true);
  route.pathname = "/acme/iterations";
  window.localStorage.clear();
  vi.mocked(api.getIterationCapabilities).mockResolvedValue({
    workspace_id: ws,
    supported: true,
    schema_version: 1,
    enabled: true,
    manual: true,
    atomic_handoff: true,
  });
  vi.mocked(api.getIterationSettings).mockResolvedValue(settings);
  vi.mocked(api.listIterations).mockResolvedValue({
    workspace_id: ws,
    items: [source],
    next_cursor: null,
  });
  vi.mocked(api.getIteration).mockResolvedValue({
    workspace_id: ws,
    iteration: source,
    statistics,
    snapshot: null,
  });
  vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha, beta]));
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
    previewFor(draft),
  );
});
function mount() {
  const client = new QueryClient();
  const view = render(
    <QueryClientProvider client={client}>
      <IterationsPage />
    </QueryClientProvider>,
  );
  return {
    client,
    user: userEvent.setup(),
    rerender: () =>
      view.rerender(
        <QueryClientProvider client={client}>
          <IterationsPage />
        </QueryClientProvider>,
      ),
  };
}
describe("iteration pagination and access", () => {
  it("keeps unknown planning modes readable without fresh manual actions", async () => {
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: { ...source, mode: "future-mode" }, statistics, snapshot: null });
    mount();
    await screen.findByText("This planning mode is not supported. History remains readable.");
    expect(screen.queryByRole("button", { name: "End iteration" })).not.toBeInTheDocument();
    expect(screen.queryByText("Edit iteration", { selector: "summary" })).not.toBeInTheDocument();
  });
  it("shows active and future periods independently of a full historical page", async () => {
    vi.mocked(api.listIterations).mockImplementation(async (_ws, params) => ({ workspace_id: ws, items: params?.limit === "100" ? [source, targetA] : Array.from({ length: 50 }, (_, index) => ({ ...source, id: `history-${index}`, name: `History ${index}`, status: "completed" })), next_cursor: null }));
    mount();
    expect(await screen.findByRole("link", { name: source.name })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: targetA.name })).toBeInTheDocument();
  });
  it("copies the platform shareable URL and shows saved mode/coordinator information", async () => {
    route.pathname += `/${source.id}`;
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Copy link" }));
    expect(copyLink).toHaveBeenCalledWith(`https://multica.test/acme/iterations/${source.id}`);
    expect(screen.getByText(/Coordinator:/)).toHaveTextContent("Manual");
  });
  it("groups plans separately and marks the earliest future plan without inventing an active iteration", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [targetA, { ...source, status: "completed" }], next_cursor: null });
    mount();
    expect(await screen.findByText("No iteration is currently active.")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Future plans" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Frozen history" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: targetA.name }).parentElement).toHaveTextContent("Upcoming");
  });
  it("resets a stale list cursor without refetching that stale page", async () => {
    vi.mocked(api.listIterations).mockImplementation(async (_ws, params) => {
      if (params?.limit === "100") return { workspace_id: ws, items: [source], next_cursor: null };
      if (params?.cursor)
        throw new ApiError("Cursor changed", 409, "Conflict", {
          code: "cursor_stale",
        });
      return { workspace_id: ws, items: [source], next_cursor: "cursor" };
    });
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByRole("link", { name: source.name });
    expect(
      vi
        .mocked(api.listIterations)
        .mock.calls.filter(([, params]) => params?.cursor),
    ).toHaveLength(1);
  });
  it("offers a first-page retry for stale issue pagination", async () => {
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIterationIssues).mockImplementation(
      async (_ws, _id, params) => {
        if (params?.cursor)
          throw new ApiError("Cursor changed", 409, "Conflict", {
            code: "cursor_stale",
          });
        return issuePage([alpha], "cursor");
      },
    );
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Next page" }));
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByRole("link", {
      name: `${alpha.identifier} · ${alpha.title}`,
    });
  });
  it("removes history from the view and query cache when access is revoked", async () => {
    route.pathname += `/${source.id}`;
    const { client } = mount();
    await screen.findByRole("heading", { name: source.name });
    window.localStorage.setItem(
      `multica_iteration_command:test:actor:${ws}:create:request`,
      "saved draft",
    );
    vi.mocked(api.getIteration).mockRejectedValue(
      new ApiError("Revoked", 403, "Forbidden"),
    );
    await act(async () => {
      await client.invalidateQueries({
        queryKey: ["iterations", ws, "detail", source.id],
      });
    });
    await screen.findByRole("alert");
    expect(
      screen.queryByRole("heading", { name: source.name }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(
        client.getQueryData(["iterations", ws, "detail", source.id]),
      ).toBeUndefined(),
    );
    expect(
      client
        .getQueriesData({ queryKey: ["iterations", ws, "issues"] })
        .every(([, data]) => data === undefined),
    ).toBe(true);
    expect(window.localStorage.length).toBe(0);
  });
  it("resets entity-local input, filters and preview when the route changes period", async () => {
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIteration).mockImplementation(async (_ws, id) => ({
      workspace_id: ws,
      iteration: id === source.id ? source : targetA,
      statistics,
      snapshot: null,
    }));
    const { user, rerender, client } = mount();
    act(() => {
      client.setQueryData(["iterations", ws, "detail", targetA.id], {
        workspace_id: ws,
        iteration: targetA,
        statistics,
        snapshot: null,
      });
    });
    await screen.findByRole("heading", { name: source.name });
    await user.click(
      screen.getByText("Edit iteration", { selector: "summary" }),
    );
    await user.clear(screen.getByLabelText("Name"));
    await user.type(
      screen.getByLabelText("Name"),
      "First period unsaved draft",
    );
    await user.type(screen.getByLabelText("Select tasks"), "first search");
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    await user.type(
      within(screen.getByRole("dialog")).getByLabelText("Reason"),
      "First period close",
    );
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Confirm changes" });
    route.pathname = `/acme/iterations/${targetA.id}`;
    rerender();
    await screen.findByRole("heading", { name: targetA.name });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(
      screen.getByText("Edit iteration", { selector: "summary" }),
    );
    expect(screen.getByLabelText("Name")).toHaveValue(targetA.name);
    expect(screen.getByLabelText("Select tasks")).toHaveValue("");
  });
  it("keeps original request recovery after the realtime closed state arrives before a lost response", async () => {
    route.pathname += `/${source.id}`;
    let fail!: () => void;
    vi.mocked(api.applyIterationOperation).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          fail = () => reject(new TypeError("Response lost"));
        }),
    );
    vi.mocked(api.getIterationOperation).mockResolvedValue(receipt);
    const { user, client } = mount();
    await user.click(
      await screen.findByRole("button", { name: "End iteration" }),
    );
    await user.type(
      within(screen.getByRole("dialog")).getByLabelText("Reason"),
      "Close original period",
    );
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await user.click(
      await screen.findByRole("button", { name: "Confirm changes" }),
    );
    await waitFor(() =>
      expect(api.applyIterationOperation).toHaveBeenCalledTimes(1),
    );
    const original = vi.mocked(api.applyIterationOperation).mock.calls[0]![1];
    act(() => {
      client.setQueryData(["iterations", ws, "detail", source.id], {
        workspace_id: ws,
        iteration: { ...source, status: "completed" },
        statistics,
        snapshot: null,
      });
    });
    await act(async () => fail());
    expect(api.getIterationOperation).not.toHaveBeenCalled();
    await user.click(
      await screen.findByRole("button", { name: "Check original request" }),
    );
    await waitFor(() =>
      expect(api.getIterationOperation).toHaveBeenCalledWith(
        ws,
        original.request_id,
      ),
    );
    expect(api.applyIterationOperation).toHaveBeenCalledTimes(1);
  });

  it.each(["closed", "disabled", "rollout-off"])(
    "recovers a stored original request when reopening a %s surface",
    async (mode) => {
      const operation = mode === "closed" ? "end" : "disable";
      const scope =
        operation === "end" ? `end:${source.id}` : "disable:workspace";
      const command = {
        kind: "operation",
        body: {
          request_id: ws,
          preview_hash: "original-hash",
          draft: {
            operation,
            iteration_id: operation === "end" ? source.id : null,
            expected_iteration_revision: operation === "end" ? 2 : null,
            expected_scope_revision: operation === "end" ? 3 : null,
            expected_settings_revision: 1,
            reason: "Original reason",
            moves: [],
            start: null,
          },
        },
      };
      window.localStorage.setItem(
        `multica_iteration_command:test:${ws}:${ws}:${scope}:${ws}`,
        JSON.stringify(command),
      );
      if (mode === "closed") {
        route.pathname += `/${source.id}`;
        vi.mocked(api.getIteration).mockResolvedValue({
          workspace_id: ws,
          iteration: { ...source, status: "completed" },
          statistics,
          snapshot: null,
        });
      } else
        vi.mocked(api.getIterationSettings).mockResolvedValue({
          ...settings,
          enabled: false,
        });
      if (mode === "rollout-off")
        vi.mocked(api.getIterationCapabilities).mockResolvedValue({
          workspace_id: ws,
          supported: false,
          enabled: false,
          manual: true,
          schema_version: 1,
          atomic_handoff: false,
        });
      vi.mocked(api.getIterationOperation).mockResolvedValue({
        ...receipt,
        request_id: ws,
        operation,
      });
      const { user } = mount();
      await user.click(
        await screen.findByRole("button", { name: "Check original request" }),
      );
      await waitFor(() =>
        expect(api.getIterationOperation).toHaveBeenCalledWith(ws, ws),
      );
      expect(api.applyIterationOperation).not.toHaveBeenCalled();
      await waitFor(() => expect(window.localStorage.length).toBe(0));
    },
  );

  it("retains disable recovery when settings become disabled before the response is lost", async () => {
    membership.role = "owner";
    let fail!: () => void;
    vi.mocked(api.applyIterationOperation).mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          fail = () => reject(new TypeError("Response lost"));
        }),
    );
    vi.mocked(api.getIterationOperation).mockResolvedValue({
      ...receipt,
      operation: "disable",
    });
    const { user, client } = mount();
    await user.click(
      await screen.findByText("Iteration settings", { selector: "summary" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Disable all iterations" }),
    );
    await user.type(
      within(screen.getByRole("dialog")).getByLabelText("Reason"),
      "Disable original request",
    );
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await user.click(
      await screen.findByRole("button", { name: "Confirm changes" }),
    );
    await waitFor(() =>
      expect(api.applyIterationOperation).toHaveBeenCalledTimes(1),
    );
    const original = vi.mocked(api.applyIterationOperation).mock.calls[0]![1];
    act(() => {
      client.setQueryData(["iterations", ws, "settings"], {
        ...settings,
        enabled: false,
      });
    });
    await act(async () => fail());
    await user.click(
      await screen.findByRole("button", { name: "Check original request" }),
    );
    await waitFor(() =>
      expect(api.getIterationOperation).toHaveBeenCalledWith(
        ws,
        original.request_id,
      ),
    );
    expect(api.applyIterationOperation).toHaveBeenCalledTimes(1);
  });

  it("reports a rejected original request instead of inferring success from an already-closed period", async () => {
    const command = {
      kind: "operation",
      body: {
        request_id: ws,
        preview_hash: "original-hash",
        draft: {
          operation: "end",
          iteration_id: source.id,
          expected_iteration_revision: 2,
          expected_scope_revision: 3,
          expected_settings_revision: 1,
          reason: "Original",
          moves: [],
          start: null,
        },
      },
    };
    window.localStorage.setItem(
      `multica_iteration_command:test:${ws}:${ws}:end:${source.id}:${ws}`,
      JSON.stringify(command),
    );
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({
      workspace_id: ws,
      iteration: { ...source, status: "completed" },
      statistics,
      snapshot: null,
    });
    vi.mocked(api.getIterationOperation).mockRejectedValue(
      new ApiError("Missing", 404, "Not Found", {
        code: "operation_not_found",
      }),
    );
    vi.mocked(api.applyIterationOperation).mockRejectedValue(
      new ApiError("Stale", 409, "Conflict"),
    );
    const { user } = mount();
    await user.click(
      await screen.findByRole("button", { name: "Check original request" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      projects.iterations.conflict,
    );
    expect(
      screen.queryByText(projects.iterations.recoveryConfirmed),
    ).not.toBeInTheDocument();
    expect(vi.mocked(api.applyIterationOperation).mock.calls[0]![1]).toEqual(
      command.body,
    );
    expect(window.localStorage.length).toBe(0);
  });
});
