// @vitest-environment jsdom

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import type { AgentApproval } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import enAgents from "../../locales/en/agents.json";

const mockListApprovals = vi.hoisted(() => vi.fn());
const mockDecide = vi.hoisted(() => vi.fn());
const mockCancel = vi.hoisted(() => vi.fn());
const mockListAgents = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    listAgentApprovals: mockListApprovals,
    decideAgentApproval: mockDecide,
    cancelAgentApproval: mockCancel,
    listAgents: mockListAgents,
  },
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: mockToastError },
}));

import { AgentApprovalsPage } from "./agent-approvals-page";

function approval(overrides: Partial<AgentApproval> = {}): AgentApproval {
  return {
    id: "ap-1",
    workspace_id: "ws-1",
    agent_id: "agent-release",
    task_id: null,
    issue_id: null,
    risk_class: "production_release",
    summary: "Deploy 0.4.41 to production",
    plan: "1. git tag v0.4.41\n2. push the tag\nRollback: re-tag the previous commit",
    status: "pending",
    decided_by: null,
    decided_at: null,
    decision_note: "",
    executed_at: null,
    execution_note: "",
    created_at: "2026-09-05T10:00:00Z",
    updated_at: "2026-09-05T10:00:00Z",
    ...overrides,
  };
}

function navAdapter(): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/agents/approvals",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path: string) => path,
  };
}

function renderQueue() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  // The queue is a workspace-scoped route, and its "learn more" link is now an
  // in-app AppLink into the bundled docs rather than an external URL. That needs
  // both a slug (useWorkspacePaths() throws without one) and navigation context,
  // so this mounts the page the way the router does.
  return render(
    <I18nProvider locale="en" resources={{ en: { agents: enAgents } }}>
      <WorkspaceSlugProvider slug="acme">
        <NavigationProvider value={navAdapter()}>
          <QueryClientProvider client={queryClient}>
            <AgentApprovalsPage />
          </QueryClientProvider>
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockListApprovals.mockResolvedValue([approval()]);
  mockListAgents.mockResolvedValue([
    { id: "agent-release", name: "Release Engineer" },
  ]);
});

describe("AgentApprovalsPage", () => {
  it("shows what is waiting, who asked, and which class of action it is", async () => {
    renderQueue();

    expect(
      await screen.findByText("Deploy 0.4.41 to production"),
    ).toBeTruthy();
    // The agent's name, resolved from the workspace list: the payload carries
    // only agent_id, and "Release Engineer wants to deploy" is the sentence.
    expect(screen.getByText("Release Engineer")).toBeTruthy();
    expect(screen.getByText("Production release")).toBeTruthy();
    // "Waiting" is deliberately the same word on the filter and on the badge, so
    // pin the badge rather than the first match.
    expect(
      screen
        .getAllByText("Waiting")
        .some((node) => node.getAttribute("data-slot") === "badge"),
    ).toBe(true);
  });

  it("defaults to the waiting filter and asks the server for that status", async () => {
    renderQueue();
    await screen.findByText("Deploy 0.4.41 to production");

    expect(mockListApprovals).toHaveBeenCalledWith({
      status: "pending",
      limit: undefined,
    });
  });

  it("switches to the audit view without a status filter", async () => {
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "All" }));

    await waitFor(() =>
      expect(mockListApprovals).toHaveBeenCalledWith({
        status: undefined,
        limit: undefined,
      }),
    );
  });

  // The design's boundary rule, at the surface a person reads: a status this build
  // has no copy for still renders, as itself.
  it("renders a status and a risk class this client does not know", async () => {
    mockListApprovals.mockResolvedValue([
      approval({ status: "escalated", risk_class: "data_export" }),
    ]);
    renderQueue();

    expect(await screen.findByText("escalated")).toBeTruthy();
    expect(screen.getByText("data_export")).toBeTruthy();
  });

  it("offers no decision on a request that is no longer pending", async () => {
    mockListApprovals.mockResolvedValue([
      approval({ status: "approved", decided_at: "2026-09-05T11:00:00Z" }),
    ]);
    renderQueue();
    await screen.findByText("Deploy 0.4.41 to production");

    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Reject" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Withdraw" })).toBeNull();
  });

  it("shows the plan on expand", async () => {
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    expect(screen.queryByText(/git tag v0.4.41/)).toBeNull();
    await user.click(screen.getByRole("button", { name: /Show plan/ }));
    expect(screen.getByText(/git tag v0.4.41/)).toBeTruthy();
  });

  it("says so when a request carries no plan", async () => {
    mockListApprovals.mockResolvedValue([approval({ plan: "   " })]);
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: /Show plan/ }));
    expect(
      screen.getByText(/carries no plan/),
    ).toBeTruthy();
  });

  it("reports a load failure and retries", async () => {
    mockListApprovals.mockRejectedValue(new Error("nope"));
    renderQueue();
    const user = userEvent.setup();

    expect(
      await screen.findByText("Could not load the approval queue."),
    ).toBeTruthy();
    mockListApprovals.mockResolvedValue([approval()]);
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByText("Deploy 0.4.41 to production"),
    ).toBeTruthy();
  });

  it("explains an empty queue rather than showing a blank page", async () => {
    mockListApprovals.mockResolvedValue([]);
    renderQueue();

    expect(await screen.findByText("Nothing is waiting for you")).toBeTruthy();
  });
});

// Deciding is the point of the page, and the part that must not be one click.
describe("AgentApprovalsPage decisions", () => {
  it("confirms before approving, and sends the note with the decision", async () => {
    mockDecide.mockResolvedValue(approval({ status: "approved" }));
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "Approve" }));

    // Nothing is sent by opening the dialog: the click that matters is the one
    // inside it.
    expect(mockDecide).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(/Approve this one action\?/),
    ).toBeTruthy();
    // The dialog restates the exact action being authorized.
    expect(
      within(dialog).getByText("Deploy 0.4.41 to production"),
    ).toBeTruthy();
    // And says out loud what the approval does not cover.
    expect(within(dialog).getByText(/does not extend to a retry/)).toBeTruthy();

    await user.type(
      within(dialog).getByLabelText(/Note/),
      "Release notes reviewed.",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Approve this action" }),
    );

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("ap-1", {
        decision: "approve",
        note: "Release notes reviewed.",
      }),
    );
  });

  it("closes the confirmation without deciding", async () => {
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Keep it open",
      }),
    );

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(mockDecide).not.toHaveBeenCalled();
  });

  it("sends reject as its own decision, and omits an empty note", async () => {
    mockDecide.mockResolvedValue(approval({ status: "rejected" }));
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "Reject" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Reject",
      }),
    );

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("ap-1", {
        decision: "reject",
        note: undefined,
      }),
    );
  });

  it("withdraws through the cancel endpoint, not the decision one", async () => {
    mockCancel.mockResolvedValue(approval({ status: "cancelled" }));
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "Withdraw" }));
    const dialog = screen.getByRole("dialog");
    // Nothing is being decided, so there is no note to annotate.
    expect(within(dialog).queryByLabelText(/Note/)).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Withdraw" }));

    await waitFor(() => expect(mockCancel).toHaveBeenCalledWith("ap-1"));
    expect(mockDecide).not.toHaveBeenCalled();
  });

  // The server refuses a second decision with 409 and a sentence saying why. That
  // sentence is more useful than a generic failure, so it reaches the person.
  it("surfaces the server's own message when someone decided first", async () => {
    mockDecide.mockRejectedValue(new Error("this request is no longer pending"));
    renderQueue();
    const user = userEvent.setup();
    await screen.findByText("Deploy 0.4.41 to production");

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Approve this action",
      }),
    );

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "this request is no longer pending",
      ),
    );
  });
});
