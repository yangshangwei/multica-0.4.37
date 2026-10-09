import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IterationPreview } from "@multica/core/iterations";
import { api, ApiError } from "@multica/core/api";
import { IterationOperation } from "./iteration-operation";
import {
  alpha,
  beta,
  issuePage,
  previewFor,
  receipt,
  settings,
  source,
  statistics,
  targetA,
  targetB,
  ws,
} from "./test-fixtures";
import projects from "../locales/en/projects.json";
import issues from "../locales/en/issues.json";
import zhProjects from "../locales/zh-Hans/projects.json";
import zhIssues from "../locales/zh-Hans/issues.json";
const locale = vi.hoisted(() => ({ value: "en" }));
vi.mock("../i18n", () => ({
  useLocale: () => locale.value === "zh" ? "zh-Hans" : "en",
  useT: (namespace?: string) => ({
    t: (fn: (x: unknown) => string, variables?: Record<string, string>) =>
      fn(
        namespace === "issues"
          ? locale.value === "zh"
            ? zhIssues
            : issues
          : locale.value === "zh"
            ? zhProjects
            : projects,
      ).replace(/\{\{(\w+)\}\}/g, (_match, key: string) => variables?.[key] ?? ""),
  }),
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: { user: { id: string } }) => unknown) =>
      select({ user: { id: ws } }),
    { getState: () => ({ user: { id: ws } }) },
  ),
}));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    getBaseUrl: () => "test",
    getSessionScope: () => "session",
    listIterations: vi.fn(),
    getIteration: vi.fn(),
    getIterationSettings: vi.fn(),
    getIterationIssues: vi.fn(),
    getIssue: vi.fn(),
    previewIteration: vi.fn(),
    applyIterationOperation: vi.fn(),
    getIterationOperation: vi.fn(),
  },
}));
beforeEach(() => {
  locale.value = "en";
  vi.resetAllMocks();
  window.localStorage.clear();
  vi.mocked(api.listIterations).mockResolvedValue({
    workspace_id: ws,
    items: [targetA, targetB],
    next_cursor: null,
  });
  vi.mocked(api.getIterationSettings).mockResolvedValue(settings);
  vi.mocked(api.getIteration).mockResolvedValue({
    workspace_id: ws,
    iteration: source,
    statistics,
    snapshot: null,
  });
  vi.mocked(api.getIterationIssues).mockResolvedValue(issuePage([alpha, beta]));
  vi.mocked(api.getIssue).mockImplementation(async (id) =>
    id === alpha.id ? alpha : beta,
  );
  vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
    previewFor(draft),
  );
  vi.mocked(api.applyIterationOperation).mockResolvedValue(receipt);
});
function mount(operation: "end" | "start" | "handoff" = "end") {
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <IterationOperation
        wsId={ws}
        iteration={source}
        settingsRevision={1}
        operation={operation}
      />
    </QueryClientProvider>,
  );
  return Object.assign(userEvent.setup(), { client });
}
async function openPreview(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "End iteration" }));
  await user.type(screen.getByLabelText("Reason"), "End this period");
  await user.click(screen.getByRole("combobox", { name: "Move remaining work to" }));
  await user.click(await screen.findByRole("option", { name: targetA.name }));
  await user.click(screen.getByRole("button", { name: "Preview changes" }));
  await screen.findByRole("button", { name: "End iteration" });
}
describe("iteration operation interaction", () => {
  it("explains the required reason while the preview action is unavailable", async () => {
    const user = mount();
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    expect(screen.getByLabelText("Reason")).toBeRequired();
    expect(screen.getByRole("button", { name: "Preview changes" })).toHaveAccessibleDescription(
      "Enter a reason before previewing this change.",
    );
    expect(screen.getByRole("button", { name: "Preview changes" })).toBeDisabled();
  });
  it("labels the start-date choice and includes the selected mode in the complete preview", async () => {
    const user = mount("start");
    await user.click(screen.getByRole("button", { name: "Start iteration" }));
    await user.click(screen.getByRole("combobox", { name: "Start dates" }));
    await user.click(await screen.findByRole("option", { name: "Start today" }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await waitFor(() => expect(api.previewIteration).toHaveBeenCalled());
    expect(vi.mocked(api.previewIteration).mock.calls.at(-1)![1].start?.mode).toBe("today");
  });
  it("names the confirmation action and separates complete task facts from the summary", async () => {
    const user = mount();
    await openPreview(user);
    const dialog = screen.getByRole("dialog");
    const summary = within(dialog).getByRole("region", { name: "Change summary" });
    expect(summary).toHaveTextContent("Affected tasks");
    expect(summary).toHaveTextContent("Notification recipients");
    const tasks = within(dialog).getByRole("region", { name: "Task changes" });
    expect(within(tasks).getAllByRole("listitem")).toHaveLength(2);
    expect(tasks).toHaveTextContent(alpha.title);
    expect(tasks).toHaveTextContent(beta.title);
    expect(tasks).toHaveTextContent(targetA.name);
    expect(within(dialog).getByRole("button", { name: "End iteration" })).toBeEnabled();
  });
  it("keeps already planned next-iteration work at its actual destination in the handoff summary", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => {
      const preview = previewFor(draft);
      return {
        ...preview,
        total_affected: 3,
        issues: [...preview.issues, { ...preview.issues[0]!, issue_id: "30000000-0000-4000-8000-000000000003", identifier: "ITR-3", title: "Task already in next plan", source_id: targetA.id }],
      };
    });
    const user = mount("handoff");
    await user.click(screen.getByRole("button", { name: "End and start next iteration" }));
    await user.type(screen.getByLabelText("Reason"), "Finish this period and start the next");
    await user.click(screen.getByRole("combobox", { name: "Move remaining work to" }));
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    const tasks = await screen.findByRole("region", { name: "Task changes" });
    const nextTask = within(tasks).getByText("Task already in next plan").closest("li")!;
    expect(nextTask).toHaveTextContent("Destination iteration");
    expect(nextTask).not.toHaveTextContent("Remove from iteration");
    expect(within(nextTask).getAllByText(targetA.name)).toHaveLength(2);
  });
  it("keeps individual destination labels and choices across a stale-preview refresh", async () => {
    const user = mount();
    await openPreview(user);
    await user.click(screen.getByRole("combobox", { name: "Destination for Alpha task" }));
    await user.click(await screen.findByRole("option", { name: targetB.name }));
    expect(screen.getByLabelText("Destination for Beta task")).toHaveTextContent(targetA.name);
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "End iteration" });
    vi.mocked(api.applyIterationOperation).mockRejectedValueOnce(
      new ApiError("Changed", 409, "Conflict"),
    );
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    await screen.findByRole("alert");
    expect(screen.getByLabelText("Reason")).toHaveValue("End this period");
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "End iteration" });
    expect(
      vi
        .mocked(api.previewIteration)
        .mock.calls.at(-1)![1]
        .moves.map((move) => move.target_id),
    ).toEqual([targetB.id, targetA.id]);
  });
  it("looks up an unknown result then retries the exact original request", async () => {
    const user = mount();
    await openPreview(user);
    vi.mocked(api.applyIterationOperation).mockRejectedValueOnce(
      new TypeError("Response lost"),
    );
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    await screen.findByText(projects.iterations.unknownResult);
    expect(screen.getByLabelText("Reason")).toBeDisabled();
    const original = vi.mocked(api.applyIterationOperation).mock.calls[0]![1];
    vi.mocked(api.getIterationOperation).mockRejectedValueOnce(
      new ApiError("Not found", 404, "Not Found"),
    );
    await user.click(
      screen.getByRole("button", { name: "Check original request" }),
    );
    await waitFor(() =>
      expect(api.applyIterationOperation).toHaveBeenCalledTimes(2),
    );
    expect(api.getIterationOperation).toHaveBeenCalledWith(
      ws,
      original.request_id,
    );
    expect(vi.mocked(api.applyIterationOperation).mock.calls[1]![1]).toEqual(
      original,
    );
  });
  it("preserves the reason and removes protected preview titles after 403", async () => {
    const user = mount();
    await openPreview(user);
    vi.mocked(api.applyIterationOperation).mockRejectedValueOnce(
      new ApiError("Revoked", 403, "Forbidden"),
    );
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      projects.iterations.permission,
    );
    expect(screen.getByLabelText("Reason")).toHaveValue("End this period");
    expect(screen.queryByText("Alpha task")).not.toBeInTheDocument();
    expect(window.localStorage.length).toBe(0);
  });
  it.each([403, 404])(
    "hides every preview field if a subsequent read revokes access with %s",
    async (status) => {
      const user = mount();
      await openPreview(user);
      vi.mocked(api.getIterationSettings).mockRejectedValueOnce(
        new ApiError("Revoked", status, "Forbidden", {
          code: "workspace_access_denied",
        }),
      );
      await user.click(screen.getByRole("button", { name: "Preview changes" }));
      await screen.findByRole("alert");
      const dialog = screen.getByRole("dialog");
      for (const secret of [
        alpha.title,
        alpha.id,
        source.name,
        targetA.name,
        "Affected tasks",
      ])
        expect(dialog).not.toHaveTextContent(secret);
      expect(screen.getByLabelText("Reason")).toHaveValue("End this period");
      expect(
        JSON.stringify(
          user.client.getQueriesData({ queryKey: ["iterations", ws] }),
        ),
      ).not.toContain(targetA.name);
    },
  );
  it("does not offer a ignored destination when cancelling a planned iteration", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <IterationOperation
          wsId={ws}
          iteration={{ ...source, status: "planned" }}
          settingsRevision={1}
          operation="cancel"
        />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Cancel iteration" }));
    expect(
      screen.queryByLabelText("Move remaining work to"),
    ).not.toBeInTheDocument();
  });
  it("lets terminal items be retained independently before start confirmation", async () => {
    const planned = { ...source, status: "planned", started_at: null };
    vi.mocked(api.getIteration).mockResolvedValue({
      workspace_id: ws,
      iteration: planned,
      statistics,
      snapshot: null,
    });
    vi.mocked(api.getIterationIssues).mockResolvedValue(
      issuePage([
        { ...alpha, status: "done" },
        { ...beta, status: "cancelled" },
      ]),
    );
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
      previewFor(draft, [
        { ...alpha, status: "done" },
        { ...beta, status: "cancelled" },
      ]),
    );
    const user = mount("start");
    await user.click(screen.getByRole("button", { name: "Start iteration" }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Start iteration" });
    await user.click(screen.getByRole("checkbox", { name: `Retain in starting scope: ${alpha.identifier} · ${alpha.title}` }));
    expect(screen.getByRole("checkbox", { name: `Retain in starting scope: ${beta.identifier} · ${beta.title}` })).not.toBeChecked();
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: "Start iteration" });
    expect(
      vi.mocked(api.previewIteration).mock.calls.at(-1)![1].start
        ?.terminal_choices,
    ).toEqual([
      { issue_id: alpha.id, retain: true },
      { issue_id: beta.id, retain: false },
    ]);
    vi.mocked(api.getIterationSettings).mockRejectedValueOnce(new ApiError("Revoked", 403, "Forbidden"));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("alert");
    expect(screen.getByRole("dialog")).not.toHaveTextContent(alpha.title);
    expect(screen.getByRole("dialog")).not.toHaveTextContent(alpha.id);
  });
  it.each([
    { timezone: "Asia/Shanghai", time: "02:05 AM" },
    { timezone: "UTC", time: "06:05 PM" },
  ])("formats an empty workspace preview in its effective planning timezone: $timezone", async ({ timezone, time }) => {
    vi.mocked(api.getIterationSettings).mockResolvedValue({ ...settings, effective_timezone: timezone });
    vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: ws, items: [], next_cursor: null });
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => ({
      ...previewFor(draft),
      previewed_at: "2026-10-06T18:05:00Z",
      iterations: [],
      issues: [],
    }));
    render(<QueryClientProvider client={new QueryClient()}>
      <IterationOperation wsId={ws} iteration={null} settingsRevision={1} operation="disable" />
    </QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: projects.iterations.disable }));
    await user.type(screen.getByLabelText("Reason"), "Disable empty workspace iterations");
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await screen.findByRole("button", { name: projects.iterations.disable });
    const timestamp = screen.getByRole("dialog").querySelector("time");
    expect(timestamp).toHaveTextContent(time);
    expect(timestamp?.parentElement).toHaveTextContent(timezone);
  });
  it("formats preview time in the saved source timezone instead of the reader clock", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => ({
      ...previewFor(draft),
      previewed_at: "2026-10-06T18:05:00.123456Z",
      iterations: [{ ...source, timezone: "Asia/Shanghai" }, targetA],
    }));
    const user = mount();
    await openPreview(user);
    const time = screen.getByRole("dialog").querySelector("time");
    expect(time).toHaveAttribute("dateTime", "2026-10-06T18:05:00.123456Z");
    expect(time).toHaveTextContent("02:05");
    expect(time?.parentElement).toHaveTextContent("Asia/Shanghai");
    expect(time).not.toHaveTextContent("123456");
  });
  it("renders frozen categories and actionable preview validation in Chinese", async () => {
    locale.value = "zh";
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => ({
      ...previewFor(draft, [{ ...alpha, status: "in_review" }]),
      invalid_items: [
        { issue_id: alpha.id, code: "iteration_reason_required" },
      ],
    }));
    const user = mount();
    await user.click(screen.getByRole("button", { name: "结束迭代" }));
    await user.type(screen.getByLabelText("原因"), "结束本期");
    await user.click(screen.getByRole("button", { name: "预览变更" }));
    await screen.findByText(zhProjects.iterations.validationReason);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveTextContent(zhIssues.status.in_review);
    expect(dialog).not.toHaveTextContent("in_review");
    expect(dialog).not.toHaveTextContent("iteration_reason_required");
  });
  it("uses safe localized defaults for unknown status categories and validation codes", async () => {
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) => ({
      ...previewFor(draft),
      issues: previewFor(draft).issues.map((issue) => ({
        ...issue,
        status_category: "future_category",
      })),
      invalid_items: [{ issue_id: null, code: "future_private_code" }],
    }));
    const user = mount();
    await openPreview(user);
    expect(screen.getByRole("dialog")).toHaveTextContent(
      projects.iterations.unknownTaskStatus,
    );
    expect(screen.getByRole("dialog")).toHaveTextContent(
      projects.iterations.validationUnknown,
    );
    expect(screen.getByRole("dialog")).not.toHaveTextContent("future_category");
    expect(screen.getByRole("dialog")).not.toHaveTextContent(
      "future_private_code",
    );
  });
  it("locks destination input while a deferred final preview is pending", async () => {
    let finish!: () => void;
    vi.mocked(api.previewIteration).mockImplementation(async (_ws, draft) =>
      draft.moves.length === 0
        ? previewFor(draft)
        : new Promise<IterationPreview>((resolve) => {
            finish = () => resolve(previewFor(draft));
          }),
    );
    const user = mount();
    await user.click(screen.getByRole("button", { name: "End iteration" }));
    await user.type(screen.getByLabelText("Reason"), "Close");
    await user.click(screen.getByRole("combobox", { name: "Move remaining work to" }));
    await user.click(await screen.findByRole("option", { name: targetA.name }));
    await user.click(screen.getByRole("button", { name: "Preview changes" }));
    await waitFor(() => expect(api.previewIteration).toHaveBeenCalledTimes(2));
    expect(screen.getByLabelText("Move remaining work to")).toBeDisabled();
    expect(screen.getByLabelText("Reason")).toBeDisabled();
    await user.click(screen.getByRole("combobox", { name: "Move remaining work to" }));
    expect(screen.queryByRole("option", { name: targetB.name })).not.toBeInTheDocument();
    await act(async () => finish());
    expect(screen.getByLabelText("Move remaining work to")).toHaveTextContent(targetA.name);
    await user.click(
      await screen.findByRole("button", { name: "End iteration" }),
    );
    expect(
      vi
        .mocked(api.applyIterationOperation)
        .mock.calls[0]![1].draft.moves.every(
          (move) => move.target_id === targetA.id,
        ),
    ).toBe(true);
  });
});
