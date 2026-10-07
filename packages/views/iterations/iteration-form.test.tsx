import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError } from "@multica/core/api";
import { IterationForm } from "./iteration-form";
import { source } from "./test-fixtures";
import projects from "../locales/en/projects.json";
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
    getSessionScope: () => "session",
    listIterations: vi.fn(),
    createIteration: vi.fn(),
    updateIteration: vi.fn(),
    getIterationOperation: vi.fn(),
  },
}));
beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  vi.mocked(api.listIterations).mockResolvedValue({ workspace_id: source.workspace_id, items: [], next_cursor: null });
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
