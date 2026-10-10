import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createInstance } from "i18next";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
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
import chineseProjects from "../locales/zh-Hans/projects.json";
import issues from "../locales/en/issues.json";
import chineseIssues from "../locales/zh-Hans/issues.json";
const membership = vi.hoisted(() => ({ role: "member" }));
const language = vi.hoisted(() => ({ locale: "en" }));
const route = vi.hoisted(() => ({ pathname: "/acme/iterations", push: vi.fn(), getShareableUrl: (path: string) => `https://multica.test${path}` }));
const copyLink = vi.hoisted(() => vi.fn().mockResolvedValue(true));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: copyLink }));
vi.mock("../i18n", () => ({
  useLocale: () => language.locale,
  useT: (namespace: "projects" | "issues" = "projects") => ({
    t: translations.getFixedT(language.locale, namespace),
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
    root: () => "/acme/issues",
    iterations: () => "/acme/iterations",
    settings: () => "/acme/settings",
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
vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: async () => [] }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: async () => [] }),
  squadListOptions: () => ({ queryKey: ["squads"], queryFn: async () => [] }),
}));
vi.mock("./iteration-assignment", () => ({ IterationAssignment: () => null, IterationSelect: () => null }));
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
    createIteration: vi.fn(),
    listIterations: vi.fn(),
    getIteration: vi.fn(),
    getIterationIssues: vi.fn(),
    getIterationEvents: vi.fn(),
    previewIteration: vi.fn(),
    applyIterationOperation: vi.fn(),
    getIterationOperation: vi.fn(),
  },
}));
const translations = createInstance();
beforeAll(async () => {
  await translations.init({
    lng: "en", fallbackLng: "en", interpolation: { escapeValue: false },
    resources: { en: { projects, issues }, "zh-Hans": { projects: chineseProjects, issues: chineseIssues } },
  });
});
beforeEach(() => {
  membership.role = "member";
  language.locale = "en";
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
  vi.mocked(api.getIterationEvents).mockResolvedValue({ workspace_id: ws, iteration_id: source.id, items: [], next_cursor: null });
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
    previewFor(draft),
  );
});
function mount({ mainLandmark = false }: { mainLandmark?: boolean } = {}) {
  const client = new QueryClient();
  const view = render(
    <QueryClientProvider client={client}>
      <IterationsPage mainLandmark={mainLandmark} />
    </QueryClientProvider>,
  );
  return {
    client,
    user: userEvent.setup(),
    rerender: () =>
      view.rerender(
        <QueryClientProvider client={client}>
          <IterationsPage mainLandmark={mainLandmark} />
        </QueryClientProvider>,
      ),
  };
}
async function openEditor(user = userEvent.setup()) {
  await user.click(await screen.findByRole("button", { name: "More actions" }));
  await user.click(await screen.findByRole("menuitem", { name: "Edit iteration" }));
}

describe("iteration pagination and access", () => {
  it("uses the saved Shanghai default for new iterations without a page timezone editor", async () => {
    vi.mocked(api.getIterationSettings).mockResolvedValue({
      ...settings,
      planning_timezone: null,
      effective_timezone: "Asia/Shanghai",
      timezone_configured: false,
    });
    vi.mocked(api.createIteration).mockResolvedValue({ ...receipt, operation: "create" });
    const { user } = mount();
    expect(screen.queryByLabelText("Planning timezone")).not.toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: "Create iteration" }));
    await user.type(screen.getByLabelText("Name"), "Shanghai plan");
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Create iteration" }));
    await waitFor(() => expect(api.createIteration).toHaveBeenCalledWith(ws, expect.objectContaining({
      name: "Shanghai plan",
      confirmed_timezone: "Asia/Shanghai",
    })));
    await waitFor(() => expect(route.push).toHaveBeenCalledWith(`/acme/iterations/${source.id}`));
  });
  it.each(["UTC", "Asia/Shanghai"])("always labels the saved detail timezone: %s", async (timezone) => {
    route.pathname = `/acme/iterations/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: { ...source, timezone }, statistics, snapshot: null });
    mount();
    const header = (await screen.findByRole("heading", { name: source.name })).closest("header");
    expect(header).toHaveTextContent(timezone);
  });
  it("keeps a frozen period's saved timezone visible after the workspace timezone changes", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({
      workspace_id: ws,
      iteration: { ...source, status: "completed" },
      statistics,
      snapshot: {
        schema_version: 1, workspace_id: ws, iteration_id: source.id, operation_id: ws,
        end_type: "completed", reason: "Finished", logical_ended_at: "2026-10-06T00:00:00Z",
        processed_at: "2026-10-06T00:00:00Z", original: [], scope: [], events: [], statistics, destinations: [],
      },
    });
    const { client } = mount();
    const header = (await screen.findByRole("heading", { name: source.name })).closest("header");
    expect(header).toHaveTextContent("UTC");
    act(() => client.setQueryData(["iterations", ws, "settings"], { ...settings, effective_timezone: "Asia/Shanghai" }));
    expect(header).toHaveTextContent("UTC");
    expect(header).not.toHaveTextContent("Asia/Shanghai");
  });
  it.each(["workspace", "detail"])("retries a failed first %s read once without replaying writes", async (scope) => {
    let finishRead!: () => void;
    if (scope === "workspace") {
      vi.mocked(api.getIterationSettings).mockRejectedValueOnce(new TypeError("Offline"))
        .mockImplementationOnce(() => new Promise((resolve) => { finishRead = () => resolve(settings); }));
    } else {
      route.pathname = `/acme/iterations/${source.id}`;
      vi.mocked(api.getIteration).mockRejectedValueOnce(new TypeError("Offline"))
        .mockImplementationOnce(() => new Promise((resolve) => { finishRead = () => resolve({ workspace_id: ws, iteration: source, statistics, snapshot: null }); }));
    }
    const { user } = mount();
    const retry = await screen.findByRole("button", { name: "Retry" });
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load iteration data. Try again.");
    expect(screen.getByRole("link", { name: scope === "workspace" ? "Back to workspace" : "Back to iterations" }))
      .toHaveAttribute("href", scope === "workspace" ? "/acme/issues" : "/acme/iterations");
    await user.click(retry);
    expect(retry).toBeDisabled();
    await user.click(retry);
    expect(scope === "workspace" ? api.getIterationSettings : api.getIteration).toHaveBeenCalledTimes(2);
    await act(async () => finishRead());
    if (scope === "workspace") await screen.findByRole("link", { name: source.name });
    else await screen.findByRole("heading", { name: source.name });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(api.createIteration).not.toHaveBeenCalled();
    expect(api.applyIterationOperation).not.toHaveBeenCalled();
    expect(api.getIterationOperation).not.toHaveBeenCalled();
  });
  it("treats an initial workspace access-denied 404 as permission loss, not missing data", async () => {
    vi.mocked(api.getIterationSettings).mockRejectedValue(new ApiError("Revoked", 404, "Not Found", { code: "workspace_access_denied" }));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent("You no longer have access to these iterations.");
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to workspace" })).toHaveAttribute("href", "/acme/issues");
    expect(api.getIteration).not.toHaveBeenCalled();
  });
  it.each([
    ["detail", new Error("Offline")],
    ["workspace", new Error("Offline")],
    ["detail", new ApiError("Timed out", 408, "Request Timeout")],
    ["workspace", new ApiError("Timed out", 408, "Request Timeout")],
    ["detail", new ApiError("Rate limited", 429, "Too Many Requests")],
    ["workspace", new ApiError("Rate limited", 429, "Too Many Requests")],
  ])("retains an open dirty editor through a transient %s refresh failure (%s)", async (scope, error) => {
    route.pathname += `/${source.id}`;
    const { client } = mount();
    await openEditor();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Unsent correction" } });
    vi.mocked(api.getIteration).mockRejectedValue(error);
    if (scope === "workspace") {
      vi.mocked(api.getIterationSettings).mockRejectedValue(error);
      vi.mocked(api.getIterationCapabilities).mockRejectedValue(error);
    }
    await act(async () => { await client.invalidateQueries({ queryKey: scope === "workspace" ? ["iterations", ws] : ["iterations", ws, "detail", source.id] }); });
    expect(screen.getByLabelText("Name")).toHaveValue("Unsent correction");
    expect(screen.getByText(source.name, { selector: "h1" })).toBeInTheDocument();
    expect((await screen.findAllByRole("alert", { hidden: true })).length).toBeGreaterThan(0);
  });
  it("hides the editor and stale detail after definitive resource deletion", async () => {
    route.pathname += `/${source.id}`;
    const { client } = mount();
    await openEditor();
    vi.mocked(api.getIteration).mockRejectedValue(new ApiError("Deleted", 404, "Not Found", { code: "iteration_not_found" }));
    await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws, "detail", source.id] }); });
    await waitFor(() => expect(screen.queryByLabelText("Name")).not.toBeInTheDocument());
    expect(screen.queryByRole("heading", { name: source.name })).not.toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("The requested iteration data is no longer available.");
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to iterations" })).toHaveAttribute("href", "/acme/iterations");
  });
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
    const { user } = mount();
    expect(await screen.findByText("No iteration is currently active.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: targetA.name }).closest("article")).toHaveTextContent("Upcoming");
    expect(api.getIteration).not.toHaveBeenCalled();
    await user.click(screen.getByRole("tab", { name: "Past iterations" }));
    expect(screen.getByRole("link", { name: source.name })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: targetA.name })).not.toBeInTheDocument();
  });
  it("restarts a stale catalogue before exposing a partial timeline", async () => {
    let starts = 0;
    vi.mocked(api.listIterations).mockImplementation(async (_ws, params) => {
      if (params?.cursor) throw new ApiError("Cursor changed", 409, "Conflict", { code: "cursor_stale" });
      starts += 1;
      return { workspace_id: ws, items: starts === 1 ? [targetA] : [source], next_cursor: starts === 1 ? "stale" : null };
    });
    mount();
    expect(await screen.findByRole("link", { name: source.name })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: targetA.name })).not.toBeInTheDocument();
    expect(starts).toBe(2);
    expect(vi.mocked(api.listIterations).mock.calls.filter(([, params]) => params?.cursor)).toHaveLength(1);
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
      name: alpha.title,
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
    expect(screen.getByRole("alert")).toHaveTextContent("You no longer have access to these iterations.");
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Back to workspace" })).toHaveAttribute("href", "/acme/issues");
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
  it("resets entity-local editor input and filters when the route changes period", async () => {
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIteration).mockImplementation(async (_ws, id) => ({ workspace_id: ws, iteration: id === source.id ? source : targetA, statistics, snapshot: null }));
    const { user, rerender, client } = mount();
    act(() => client.setQueryData(["iterations", ws, "detail", targetA.id], { workspace_id: ws, iteration: targetA, statistics, snapshot: null }));
    await user.type(await screen.findByLabelText(projects.iterations.audit.searchTasks), "first search");
    await openEditor(user);
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "First period unsaved draft");
    route.pathname = `/acme/iterations/${targetA.id}`;
    rerender();
    await screen.findByRole("heading", { name: targetA.name });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByLabelText(projects.iterations.audit.searchTasks)).toHaveValue("");
    await openEditor(user);
    expect(screen.getByLabelText("Name")).toHaveValue(targetA.name);
  });
  it("drops the previous period's open confirmation after cached navigation", async () => {
    route.pathname += `/${source.id}`;
    vi.mocked(api.getIteration).mockImplementation(async (_ws, id) => ({ workspace_id: ws, iteration: id === source.id ? source : targetA, statistics, snapshot: null }));
    const { user, rerender, client } = mount();
    act(() => client.setQueryData(["iterations", ws, "detail", targetA.id], { workspace_id: ws, iteration: targetA, statistics, snapshot: null }));
    await user.click(await screen.findByRole("button", { name: "End iteration" }));
    await user.type(within(screen.getByRole("dialog")).getByLabelText(/Reason/), "First period close");
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "End iteration" });
    route.pathname = `/acme/iterations/${targetA.id}`;
    rerender();
    await screen.findByRole("heading", { name: targetA.name });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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
      within(screen.getByRole("dialog")).getByLabelText(/Reason/),
      "Close original period",
    );
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await user.click(
      await screen.findByRole("button", { name: "End iteration" }),
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
        await screen.findByRole("button", { name: operation === "end" ? /^Check request: End iteration — / : /^Check request: Disable all iterations — / }),
      );
      await waitFor(() =>
        expect(api.getIterationOperation).toHaveBeenCalledWith(ws, ws),
      );
      expect(api.applyIterationOperation).not.toHaveBeenCalled();
      await waitFor(() => expect(window.localStorage.length).toBe(0));
    },
  );

  // Workspace-disable recovery is covered in iteration-settings-tab.test.tsx.

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
      await screen.findByRole("button", { name: /^Check request: End iteration — / }),
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

describe("approved iteration business pages", () => {
  it("explains a blocked start on a populated planned iteration", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: targetA, statistics, snapshot: null });
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA], next_cursor: null });
    const { client } = mount();
    const start = await screen.findByRole("button", { name: "Start iteration" });
    await waitFor(() => expect(start).toHaveAccessibleDescription(projects.iterations.pages.startBlocked));
    expect(start).toBeDisabled();
    expect(screen.getByText(projects.iterations.pages.startBlocked)).toBeVisible();
    expect(screen.getByRole("link", { name: source.name })).toHaveAttribute("href", `/acme/iterations/${source.id}`);
    act(() => client.setQueryData(["iterations", ws, "catalogue"], [targetA]));
    await waitFor(() => expect(start).toBeEnabled());
    expect(start).not.toHaveAttribute("aria-describedby");
    expect(api.previewIteration).not.toHaveBeenCalled();
  });

  it("explains why start is unavailable while the active iteration check is pending", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    let finishCatalogue!: () => void;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: targetA, statistics, snapshot: null });
    vi.mocked(api.listIterations).mockImplementation(() => new Promise((resolve) => {
      finishCatalogue = () => resolve({ workspace_id: ws, items: [targetA], next_cursor: null });
    }));
    mount();
    const start = await screen.findByRole("button", { name: "Start iteration" });
    expect(start).toBeDisabled();
    expect(start).toHaveAccessibleDescription(projects.iterations.audit.startChecking);
    expect(screen.getByText(projects.iterations.audit.startChecking)).toBeVisible();
    await act(async () => finishCatalogue());
    await waitFor(() => expect(start).toBeEnabled());
  });

  it("keeps start blocked after a failed check and retries that read without replaying writes", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: targetA, statistics, snapshot: null });
    vi.mocked(api.listIterations).mockRejectedValueOnce(new TypeError("Offline"))
      .mockResolvedValueOnce({ workspace_id: ws, items: [targetA], next_cursor: null });
    const { user } = mount();
    const start = await screen.findByRole("button", { name: "Start iteration" });
    await waitFor(() => expect(start).toHaveAccessibleDescription(projects.iterations.audit.startUnavailable));
    expect(start).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(start).toBeEnabled());
    expect(api.listIterations).toHaveBeenCalledTimes(2);
    expect(api.previewIteration).not.toHaveBeenCalled();
    expect(api.applyIterationOperation).not.toHaveBeenCalled();
  });

  it("labels the overview status control and filters through the shared select", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA], next_cursor: null });
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Filters" }));
    const status = screen.getByRole("combobox", { name: "Status" });
    const label = screen.getByText("Status", { selector: "label" });
    expect(label).toBeVisible();
    expect(label).toHaveAttribute("for", status.id);
    await user.click(status);
    await user.click(await screen.findByRole("option", { name: "Planned" }));
    expect(status).toHaveTextContent("Planned");
    expect(screen.getByRole("link", { name: targetA.name })).toBeVisible();
    expect(screen.queryByRole("link", { name: source.name })).not.toBeInTheDocument();
  });

  it("reveals more planned iterations without describing it as page navigation", async () => {
    const plans = Array.from({ length: 6 }, (_, index) => ({ ...targetA, id: `plan-${index}`, name: `Plan ${index}` }));
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: plans, next_cursor: null });
    const { user } = mount();
    const more = await screen.findByRole("button", { name: "Load more" });
    expect(screen.getAllByRole("link", { name: /^Plan / })).toHaveLength(5);
    await user.click(more);
    expect(screen.getAllByRole("link", { name: /^Plan / })).toHaveLength(6);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    expect(api.listIterations).toHaveBeenCalledTimes(1);
  });

  it.each(["overview", "detail"])("leaves the application main landmark to the shell on %s", async (surface) => {
    if (surface === "detail") route.pathname = `/acme/iterations/${source.id}`;
    mount();
    await screen.findByRole("heading", { name: surface === "detail" ? source.name : "Iterations" });
    expect(screen.queryByRole("main")).not.toBeInTheDocument();
  });

  it.each(["overview", "detail"])("owns exactly one main landmark when requested by the %s platform route", async (surface) => {
    if (surface === "detail") route.pathname = `/acme/iterations/${source.id}`;
    mount({ mainLandmark: true });
    const heading = await screen.findByRole("heading", { name: surface === "detail" ? source.name : "Iterations" });
    const landmarks = screen.getAllByRole("main");
    expect(landmarks).toHaveLength(1);
    expect(landmarks[0]).toContainElement(heading);
  });

  it("keeps frozen delivery and unfinished closure destinations distinct from scope events", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    const carry = { ...alpha, id: "30000000-0000-4000-8000-000000000003", identifier: "ITR-3", title: "Carry task C" };
    const remove = { ...beta, id: "30000000-0000-4000-8000-000000000004", identifier: "ITR-4", title: "Remove task D", status: "blocked" };
    const added = { ...alpha, id: "30000000-0000-4000-8000-000000000005", identifier: "ITR-5", title: "Added completed task", status: "done" };
    const frozenScope = issuePage([{ ...alpha, status: "done" }, { ...beta, status: "cancelled" }, carry, remove, added]).items;
    const frozen = {
      ...statistics,
      original: 4, current: 5, cancelled: 1, effective: 4, completed: 2,
      original_completed: 1, initial_effective: 4, added_unique: 1, cancel_events: 1,
      effective_ratio: 0.5, original_ratio: 0.25,
      chart: [{ date: "2026-10-06", effective: 4, completed: 2, original: 4 }],
    };
    const snapshot = {
      schema_version: 1 as const, workspace_id: ws, iteration_id: source.id, operation_id: ws,
      end_type: "completed", reason: "Close frozen delivery", logical_ended_at: "2026-10-06T12:00:00Z",
      processed_at: "2026-10-06T12:00:00Z", original: frozenScope.slice(0, 4), scope: frozenScope,
      events: [], statistics: frozen,
      destinations: frozenScope.map((issue) => ({
        issue_id: issue.issue_id, target_iteration_id: issue.issue_id === carry.id ? targetA.id : null,
        rollover_count_before: 0, rollover_count_after: issue.issue_id === carry.id ? 1 : 0,
      })),
    };
    const detail = { workspace_id: ws, iteration: { ...source, status: "completed" }, statistics: { ...statistics, current: 99, effective: 98, original: 97 }, snapshot };
    vi.mocked(api.getIteration).mockResolvedValue(detail);
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [detail.iteration, targetA], next_cursor: null });
    const { user, client } = mount();
    const tasks = await screen.findByRole("tabpanel", { name: /^Task snapshot/ });
    for (const node of screen.getAllByText("Original commitment", { selector: "dt" })) expect(tasks).toContainElement(node);
    await user.click(screen.getByRole("tab", { name: "Progress" }));
    const progress = screen.getByRole("tabpanel", { name: "Progress" });
    const delivery = within(progress).getByRole("region", { name: "Delivery summary" });
    const primary = within(delivery.querySelector("dl")!);
    const effective = primary.getByText("Effective scope completion").parentElement!;
    const original = primary.getByText("Original commitment completion").parentElement!;
    expect(effective).toHaveTextContent("2 / 4");
    expect(effective).toHaveTextContent("50%");
    expect(original).toHaveTextContent("1 / 4");
    expect(original).toHaveTextContent("25%");
    expect(primary.getByText("Remaining").parentElement).toHaveTextContent("2");
    expect(primary.getAllByRole("term")).toHaveLength(3);
    expect(within(progress).getAllByText("50%")).toHaveLength(1);
    expect(screen.getAllByText(/This history is frozen at closure/)).toHaveLength(1);
    const outcomes = within(progress).getByRole("region", { name: "Unfinished work at closure" });
    expect(within(outcomes).getByText("Tasks carried over at closure").parentElement).toHaveTextContent("1");
    expect(within(outcomes).getByText("Tasks removed at closure").parentElement).toHaveTextContent("1");
    const unfinished = within(outcomes).getByRole("list", { name: "Unfinished work at closure" });
    expect(within(unfinished).getAllByRole("listitem")).toHaveLength(2);
    expect(within(unfinished).getByText("ITR-3 · Carry task C")).toBeVisible();
    expect(await within(unfinished).findByRole("link", { name: targetA.name })).toBeVisible();
    expect(within(unfinished).getByText("ITR-4 · Remove task D")).toBeVisible();
    expect(within(unfinished).queryByText(alpha.title, { exact: false })).not.toBeInTheDocument();
    expect(within(unfinished).queryByText(beta.title, { exact: false })).not.toBeInTheDocument();
    act(() => client.setQueryData(["iterations", ws, "detail", source.id], { ...detail, statistics: { ...statistics, completed: 88, remaining: 77 } }));
    expect(effective).toHaveTextContent("2 / 4");
    expect(original).toHaveTextContent("1 / 4");
    await user.click(within(progress).getByText("View closure details", { selector: "summary" }));
    const closure = within(progress).getByText("View closure details", { selector: "summary" }).closest("details")!;
    expect(within(closure).getAllByRole("listitem")).toHaveLength(5);
    expect(within(closure).getByText(`ITR-1 · ${alpha.title}`)).toBeVisible();
    await user.click(screen.getByRole("tab", { name: "Scope changes" }));
    const scope = screen.getByRole("tabpanel", { name: "Scope changes" });
    await user.click(within(scope).getByText("View all scope counts", { selector: "summary" }));
    expect(within(scope).getByText("Removed during iteration").parentElement).toHaveTextContent("0 events");
    expect(api.getIterationEvents).not.toHaveBeenCalled();
    await user.click(screen.getByRole("tab", { name: "Progress" }));
    expect(within(closure).getByText(`ITR-1 · ${alpha.title}`)).toBeVisible();
  });

  it("connects the overview tabs to named panels during keyboard navigation", async () => {
    vi.mocked(api.listIterations).mockResolvedValue({
      workspace_id: ws,
      items: [source, { ...targetA, name: "Past period", status: "completed" }],
      next_cursor: null,
    });
    const { user } = mount();
    const currentTab = await screen.findByRole("tab", { name: "Current and planned" });
    const currentPanel = screen.getByRole("tabpanel", { name: "Current and planned" });
    expect(currentTab).toHaveAttribute("aria-controls", currentPanel.id);
    expect(await within(currentPanel).findByRole("link", { name: source.name })).toBeVisible();
    currentTab.focus();
    await user.keyboard("{ArrowRight}");
    const historyTab = screen.getByRole("tab", { name: "Past iterations" });
    expect(historyTab).toHaveFocus();
    await user.keyboard("{Enter}");
    const historyPanel = screen.getByRole("tabpanel", { name: "Past iterations" });
    expect(historyTab).toHaveAttribute("aria-selected", "true");
    expect(historyTab).toHaveAttribute("aria-controls", historyPanel.id);
    await user.tab();
    expect(historyPanel).toHaveFocus();
    expect(within(historyPanel).getByRole("link", { name: "Past period" })).toBeVisible();
    expect(screen.queryByRole("link", { name: source.name })).not.toBeInTheDocument();
    expect(api.getIteration).toHaveBeenCalledTimes(1);
  });

  it.each([false, true])("opens the current chart's data without loading other details (frozen=%s)", async (frozen) => {
    const points = [
      { date: "2026-10-01", effective: 5, completed: 0, original: 5 },
      { date: "2026-10-02", effective: 7, completed: 2, original: 5 },
    ];
    vi.mocked(api.getIteration).mockResolvedValue({
      workspace_id: ws,
      iteration: { ...source, status: frozen ? "completed" : "active" },
      statistics: { ...statistics, chart: frozen ? [{ date: "2026-10-03", effective: 99, completed: 98, original: 97 }] : points },
      snapshot: frozen ? {
        schema_version: 1, workspace_id: ws, iteration_id: source.id, operation_id: ws,
        end_type: "completed", reason: "Finished", logical_ended_at: "2026-10-06T00:00:00Z",
        processed_at: "2026-10-06T00:00:00Z", original: [], scope: [], events: [],
        statistics: { ...statistics, chart: points }, destinations: [],
      } : null,
    });
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [source, targetA, { ...targetA, id: "history", name: "Past period", status: "completed" }], next_cursor: null });
    const { user } = mount();
    const disclosure = await screen.findByText("View chart data", { selector: "summary" });
    expect(screen.getByText("Effective scope completion").parentElement).toHaveTextContent("0%");
    expect(screen.getByRole("table", { name: "Chart data" })).not.toBeVisible();
    disclosure.focus();
    expect(disclosure).toHaveFocus();
    await user.click(disclosure);
    const table = screen.getByRole("table", { name: "Chart data" });
    expect(table).toBeVisible();
    expect(within(table).getAllByRole("rowheader").map((row) => row.textContent)).toEqual(points.map((point) => point.date));
    expect(within(table).getAllByRole("row").slice(1).map((row) => within(row).getAllByRole("cell").map((cell) => cell.textContent)))
      .toEqual([["5", "0", "5"], ["7", "2", "5"]]);
    await user.click(screen.getByRole("tab", { name: "Past iterations" }));
    expect(api.getIteration).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.getIteration).mock.calls[0]?.[1]).toBe(source.id);
  });

  it("removes cached overview chart data after its iteration is definitively missing", async () => {
    const { user, client } = mount();
    await user.click(await screen.findByText("View chart data", { selector: "summary" }));
    expect(screen.getByRole("table", { name: "Chart data" })).toBeVisible();
    vi.mocked(api.getIteration).mockRejectedValue(new ApiError("Deleted", 404, "Not Found", { code: "iteration_not_found" }));
    await act(async () => { await client.invalidateQueries({ queryKey: ["iterations", ws, "detail", source.id] }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("The requested iteration data is no longer available.");
    expect(screen.queryByRole("table", { name: "Chart data" })).not.toBeInTheDocument();
    expect(screen.queryByRole("img", { name: "Scope and completion over time" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  it.each([["en", "Nov", "30"], ["zh-Hans", "11月", "30日"]])("keeps localized date parts together in the %s timeline", async (locale, month, day) => {
    language.locale = locale!;
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [{ ...targetA, start_date: "2026-11-30", end_date: "2026-12-13" }], next_cursor: null });
    mount();
    const row = (await screen.findByRole("link", { name: targetA.name })).closest("article")!;
    expect(within(row).getByText(month!, { selector: "time span" })).toHaveClass("whitespace-nowrap");
    expect(within(row).getByText(day!, { selector: "time span" })).toHaveClass("whitespace-nowrap");
    expect(row.querySelector("time")).toHaveAttribute("datetime", "2026-11-30");
    expect(api.getIteration).not.toHaveBeenCalled();
  });

  it("keeps workspace configuration out of the timeline and exposes history separately", async () => {
    mount();
    await screen.findByRole("link", { name: source.name });
    expect(screen.getByRole("tab", { name: "Current and planned" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Past iterations" })).toBeInTheDocument();
    expect(screen.queryByText("Iteration settings", { selector: "summary" })).not.toBeInTheDocument();
  });

  it("retains task filters when moving between detail tabs", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    vi.mocked(api.getIterationIssues).mockImplementation(async (_ws, _id, params) => issuePage(params?.search ? [alpha] : [alpha, beta]));
    const { user } = mount();
    const search = await screen.findByRole("textbox", { name: projects.iterations.audit.searchTasks });
    const tasks = screen.getByRole("tabpanel", { name: /^Tasks/ });
    expect(within(tasks).queryByText(/Matching tasks:/)).not.toBeInTheDocument();
    await user.type(search, "Alpha");
    await waitFor(() => expect(within(tasks).getByRole("status")).toHaveTextContent("Matching tasks: 1"));
    expect(within(tasks).getByText("Current scope", { selector: "dt" }).parentElement).toHaveTextContent("2");
    await user.click(screen.getByRole("button", { name: "About task counts" }));
    expect(await screen.findByText(projects.iterations.overallScope)).toBeVisible();
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Filter tasks" }));
    await user.click(screen.getByRole("combobox", { name: "Priority" }));
    await user.click(await screen.findByRole("option", { name: "High" }));
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("tab", { name: "Progress" }));
    await user.click(screen.getByRole("tab", { name: /^Tasks/ }));
    expect(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks })).toHaveValue("Alpha");
    expect(screen.getByRole("button", { name: "Remove Priority: High filter" })).toBeVisible();
  });

  // The lifecycle matrix is owned by core/iterations/scope.test.ts; these assert page-to-list wiring.
  it.each(["planned", "cancelled", "future-status"])("does not offer an original commitment for an unstarted %s period", async (status) => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: { ...targetA, status }, statistics, snapshot: null });
    mount();
    await screen.findByRole("textbox", { name: projects.iterations.audit.searchTasks });
    expect(screen.queryByRole("combobox", { name: "Current scope" })).not.toBeInTheDocument();
    const tasks = screen.getByRole("tabpanel", { name: /^Tasks/ });
    expect(within(tasks).queryByRole("heading", { name: "Tasks" })).not.toBeInTheDocument();
    if (status === "planned") expect(within(tasks).getByText("Planned tasks").parentElement).toHaveTextContent("2");
  });

  it("offers the original commitment after an actual start with a zero baseline", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: source, statistics: { ...statistics, original: 0, initial_effective: 0 }, snapshot: null });
    const { user } = mount();
    await user.click(await screen.findByRole("combobox", { name: "Current scope" }));
    await user.click(await screen.findByRole("option", { name: "Original commitment" }));
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { scope: "original" }, expect.anything()));
    expect(screen.getByText("Original commitment", { selector: "dt" }).parentElement).toHaveTextContent("0");
  });

  it("returns to current scope when start facts become unavailable without losing task refinements", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    const { user, client } = mount();
    await user.type(await screen.findByRole("textbox", { name: projects.iterations.audit.searchTasks }), "Alpha");
    await user.click(screen.getByRole("combobox", { name: "Current scope" }));
    await user.click(await screen.findByRole("option", { name: "Original commitment" }));
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { scope: "original", search: "Alpha" }, expect.anything()));
    act(() => client.setQueryData(["iterations", ws, "detail", source.id], { workspace_id: ws, iteration: { ...source, started_at: null }, statistics, snapshot: null }));
    await waitFor(() => expect(screen.queryByRole("combobox", { name: "Current scope" })).not.toBeInTheDocument());
    expect(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks })).toHaveValue("Alpha");
    expect(screen.queryByText(/Matching tasks:/)).toBeInTheDocument();
    await user.clear(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks }));
    await user.type(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks }), "Beta");
    await waitFor(() => expect(api.getIterationIssues).toHaveBeenLastCalledWith(ws, source.id, { search: "Beta" }, expect.anything()));
  });

  it("gives an empty planned iteration explicit existing-task and new-task actions", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: targetA, statistics: { ...statistics, original: 0, current: 0, effective: 0, remaining: 0 }, snapshot: null });
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([]), iteration_id: targetA.id });
    mount();
    expect(await screen.findByRole("button", { name: "Add existing tasks" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New task" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks })).toBeVisible();
  });

  it("keeps the empty plan's adjustment history and page identity reachable", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: targetA, statistics: { ...statistics, original: 0, current: 0, effective: 0, initial_effective: 0 }, snapshot: null });
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([]), iteration_id: targetA.id });
    vi.mocked(api.getIterationEvents).mockResolvedValue({ workspace_id: ws, iteration_id: targetA.id, next_cursor: null, items: [{ id: ws, sequence: 1, operation_id: ws, iteration_id: targetA.id, issue_id: alpha.id, kind: "planned_activity", actor: null, before_facts: { title: "Removed from plan", source_iteration_id: targetA.id, target_iteration_id: null }, after_facts: null, occurred_at: source.started_at!, sampled_at: source.started_at!, reason: "Planning changed" }] });
    const { user } = mount();
    await user.click(await screen.findByRole("tab", { name: "Planning adjustments" }));
    const panel = screen.getByRole("tabpanel", { name: "Planning adjustments" });
    expect(await within(panel).findByText("Removed from plan")).toBeVisible();
    expect(within(panel).getByRole("button", { name: "View Planned tasks: 0 tasks" })).toBeVisible();
    expect(screen.getByRole("heading", { name: targetA.name, level: 1 })).toBeVisible();
  });

  it("keeps the main task search while opening and clearing independent scope details", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    const { user } = mount();
    const taskSearch = await screen.findByRole("textbox", { name: projects.iterations.audit.searchTasks });
    await user.type(taskSearch, "Alpha");
    await user.click(screen.getByRole("tab", { name: "Scope changes" }));
    await user.click(screen.getByRole("button", { name: "View Current effective scope: 2 tasks" }));
    const detail = within(screen.getByRole("region", { name: "Scope metric details" }));
    await user.type(await detail.findByRole("textbox", { name: "Search task records" }), "Beta");
    await user.click(detail.getByRole("button", { name: "Back to activity" }));
    await user.click(screen.getByRole("tab", { name: /^Tasks/ }));
    expect(screen.getByRole("textbox", { name: projects.iterations.audit.searchTasks })).toHaveValue("Alpha");
    expect(taskSearch).toBeInTheDocument();
  });

  it("uses the frozen header and read-only task actions despite stale active metadata", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: source, statistics, snapshot: {
      schema_version: 1, workspace_id: ws, iteration_id: source.id, operation_id: ws, end_type: "completed", reason: "Finished",
      logical_ended_at: "2026-10-06T12:00:00Z", processed_at: "2026-10-06T12:00:00Z", original: issuePage([alpha, beta]).items,
      scope: issuePage([alpha, beta]).items, statistics, events: [], destinations: [],
    } });
    mount();
    expect(await screen.findByText(projects.iterations.completed, { selector: "[data-slot=badge]" })).toBeVisible();
    expect(screen.queryByRole("button", { name: projects.iterations.end })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add existing tasks" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /^Task snapshot/ })).toBeVisible();
  });

  it("does not present delivery statistics for a plan cancelled before starting", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.getIteration).mockResolvedValue({ workspace_id: ws, iteration: { ...targetA, status: "cancelled" }, statistics: { ...statistics, original: 0, current: 0, effective: 0 }, snapshot: null });
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([]), iteration_id: targetA.id });
    const { user } = mount();
    await user.click(await screen.findByRole("tab", { name: "Progress" }));
    const progress = within(screen.getByRole("tabpanel", { name: "Progress" }));
    expect(progress.getByText(projects.iterations.activityPanel.cancelledPlanHint)).toBeVisible();
    expect(progress.queryByText("Delivery summary")).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Planning adjustments" })).toBeVisible();
  });
});

describe("iteration page review regressions", () => {
  it("keeps the current iteration visible beyond fifty future plans", async () => {
    const plans = Array.from({ length: 51 }, (_, index) => ({ ...targetA, id: `plan-${index}`, name: `Plan ${index}` }));
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [...plans, source], next_cursor: null });
    mount();
    expect(await screen.findByRole("link", { name: source.name })).toBeInTheDocument();
  });
  it("keeps planning adjustments reachable when a plan becomes empty on another tab", async () => {
    route.pathname = `/acme/iterations/${targetA.id}`;
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [targetA], next_cursor: null });
    const planned = { workspace_id: ws, iteration: targetA, statistics, snapshot: null };
    vi.mocked(api.getIteration).mockResolvedValue(planned);
    const { user, client } = mount();
    await user.click(await screen.findByRole("tab", { name: "Progress" }));
    vi.mocked(api.getIterationIssues).mockResolvedValue({ ...issuePage([]), iteration_id: targetA.id });
    await act(async () => {
      client.setQueryData(["iterations", ws, "detail", targetA.id], { ...planned, statistics: { ...statistics, current: 0 } });
      await client.invalidateQueries({ queryKey: ["iterations", ws, "issues", targetA.id] });
    });
    expect(screen.getByRole("tab", { name: "Progress" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Planning adjustments" })).toBeVisible();
    await user.click(screen.getByRole("tab", { name: /^Tasks/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Add existing tasks" })).toBeVisible());
    expect(screen.getByRole("button", { name: "New task" })).toBeVisible();
  });
  it("keeps an edit draft while its dialog is closed to inspect progress", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    const { user } = mount();
    await openEditor(user);
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Keep this correction");
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await user.click(screen.getByRole("tab", { name: "Progress" }));
    await openEditor(user);
    expect(screen.getByLabelText("Name")).toHaveValue("Keep this correction");
  });
  it("locks a retained edit when another client disables iteration planning", async () => {
    route.pathname = `/acme/iterations/${source.id}`;
    const { user, client } = mount();
    await openEditor(user);
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Unsent name");
    act(() => client.setQueryData(["iterations", ws, "settings"], { ...settings, enabled: false }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled());
    expect(screen.getByLabelText("Name")).toHaveValue("Unsent name");
  });
});
