import userEvent from "@testing-library/user-event";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { RuntimeDevice, SquadTemplate, StaffedSquad } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { TemplateCreateSquadPage } from "./template-create-squad-page";

const mocks = vi.hoisted(() => ({
  listTemplates: vi.fn(), listRuntimes: vi.fn(), listMembers: vi.fn(),
  create: vi.fn(), success: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/api")>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listSquadTemplates: mocks.listTemplates,
      listRuntimes: mocks.listRuntimes,
      listMembers: mocks.listMembers,
      createSquadFromTemplate: mocks.create,
    },
  };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
// The badge's appearance belongs to the agent template suite; avoid importing
// the unrelated agent configuration form through this display-only export.
vi.mock("../../agents/create/template-create-agent-page", () => ({ AutonomyBadge: () => null }));
vi.mock("sonner", () => ({ toast: { success: mocks.success } }));

const TEMPLATE: SquadTemplate = {
  key: "feature-delivery", version: 1, name: "Feature delivery", title: "Feature delivery",
  description: "Design, build, review, and verify a feature.", avatar_emoji: "", instructions: "Coordinate the work.",
  leader: { template_key: "tech-lead", title: "Tech lead", name: "Lead", role: "Coordinate", autonomy_level: "coordinator", avatar_emoji: "" },
  members: [{ template_key: "reviewer", title: "Reviewer", name: "Reviewer", role: "Review the implementation", autonomy_level: "contributor", avatar_emoji: "" }],
};
const RUNTIME: RuntimeDevice = {
  id: "runtime-1", workspace_id: "ws-1", daemon_id: null, name: "Test Runtime",
  runtime_mode: "local", provider: "claude", launch_header: "", status: "online",
  device_info: "host.local", metadata: {}, owner_id: "user-1", visibility: "private",
  last_seen_at: null, created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
};
const STAFFED: StaffedSquad = {
  squad: {
    id: "squad-new", workspace_id: "ws-1", name: "Delivery team", description: "",
    instructions: "Coordinate the work.", avatar_url: null, leader_id: "leader-1",
    creator_id: "user-1", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
    archived_at: null, archived_by: null,
  },
  created_agent_ids: ["agent-new"], reused_agent_ids: ["leader-1"],
};

function renderPage(canGoBack = false, locale: "en" | "zh-Hans" = "en") {
  const navigation: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), canGoBack: () => canGoBack,
    pathname: "/acme/squads/new/template", searchParams: new URLSearchParams(), hash: "",
    getShareableUrl: (path) => path,
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const invalidate = vi.spyOn(client, "invalidateQueries");
  renderWithI18n(
    <QueryClientProvider client={client}>
      <NavigationProvider value={navigation}>
        <TemplateCreateSquadPage />
      </NavigationProvider>
    </QueryClientProvider>,
    { locale },
  );
  return { navigation, invalidate };
}

async function pickTemplate() {
  await userEvent.click(await screen.findByRole("button", { name: /Feature delivery/ }));
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listTemplates.mockResolvedValue([TEMPLATE]);
  mocks.listRuntimes.mockResolvedValue([RUNTIME]);
  mocks.listMembers.mockResolvedValue([]);
  mocks.create.mockResolvedValue(STAFFED);
});

describe("TemplateCreateSquadPage", () => {
  it("creates with the selected template, name and access, invalidates caches, and navigates once", async () => {
    const user = userEvent.setup();
    const { navigation, invalidate } = renderPage();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create squad" })).toBeDisabled();
    await pickTemplate();
    expect(screen.getByText("Tech lead")).toBeVisible();
    expect(screen.getByText("Reviewer", { exact: true })).toBeVisible();
    const name = screen.getByRole("textbox", { name: "Squad name" });
    expect(name).toHaveValue(TEMPLATE.title);
    await user.clear(name);
    await user.type(name, " Delivery team ");
    await user.click(screen.getByRole("button", { name: "Everyone in the workspace" }));
    await user.click(screen.getByRole("button", { name: "Create squad" }));
    await waitFor(() => expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/squad-new"));
    expect(mocks.create).toHaveBeenCalledExactlyOnceWith({
      template_key: TEMPLATE.key, runtime_id: RUNTIME.id, name: "Delivery team",
      permission_mode: "public_to", invocation_targets: [{ target_type: "workspace" }], language: "en",
    });
    for (const key of [workspaceKeys.squads("ws-1"), workspaceKeys.agents("ws-1"), workspaceKeys.skills("ws-1")]) {
      expect(invalidate).toHaveBeenCalledWith({ queryKey: key });
    }
    expect(mocks.success).toHaveBeenCalledExactlyOnceWith("Delivery team is ready — 1 created, 1 reused");
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.back).not.toHaveBeenCalled();
  });

  it("preserves the draft on failure and allows retry", async () => {
    mocks.create.mockRejectedValueOnce(new Error("Server unavailable"));
    const { navigation } = renderPage();
    await pickTemplate();
    fireEvent.change(screen.getByRole("textbox", { name: "Squad name" }), { target: { value: "Keep my name" } });
    await userEvent.click(screen.getByRole("button", { name: "Create squad" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Server unavailable");
    expect(screen.getByRole("textbox", { name: "Squad name" })).toHaveValue("Keep my name");
    expect(screen.getByRole("button", { name: "Create squad" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Go back" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
    expect(navigation.push).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Create squad" }));
    await waitFor(() => expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/squad-new"));
    expect(mocks.create).toHaveBeenLastCalledWith(expect.objectContaining({ name: "Keep my name", permission_mode: "private", invocation_targets: [] }));
  });

  it.each([
    ["missing", []],
    ["offline", [{ ...RUNTIME, status: "offline" }]],
    ["private to another member", [{ ...RUNTIME, owner_id: "someone-else" }]],
  ] satisfies [string, RuntimeDevice[]][])("does not create with a runtime that is %s", async (_state, runtimes) => {
    mocks.listRuntimes.mockResolvedValue(runtimes);
    renderPage();
    await pickTemplate();
    expect(await screen.findByText(/Connect a runtime first/)).toBeVisible();
    expect(screen.getByRole("button", { name: "Create squad" })).toBeDisabled();
    expect(mocks.create).not.toHaveBeenCalled();
  });

  it("awaits creation with both return actions disabled", async () => {
    let complete!: (result: StaffedSquad) => void;
    mocks.create.mockReturnValueOnce(new Promise<StaffedSquad>((resolve) => { complete = resolve; }));
    const { navigation } = renderPage();
    await pickTemplate();
    await userEvent.click(screen.getByRole("button", { name: "Create squad" }));
    expect(screen.getByRole("button", { name: "Creating…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Go back" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(navigation.push).not.toHaveBeenCalled();
    await act(async () => { complete(STAFFED); });
    await waitFor(() => expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/squad-new"));
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.back).not.toHaveBeenCalled();
  });

  it.each(["Cancel", "Go back"])("%s returns a direct-link visit to the chooser", async (name) => {
    const { navigation } = renderPage();
    await userEvent.click(screen.getByRole("button", { name }));
    expect(navigation.replace).toHaveBeenCalledExactlyOnceWith("/acme/squads/new");
    expect(navigation.push).not.toHaveBeenCalled();
    expect(navigation.back).not.toHaveBeenCalled();
    expect(mocks.create).not.toHaveBeenCalled();
  });

  it("uses existing history for the return action", async () => {
    const { navigation } = renderPage(true);
    await userEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(navigation.back).toHaveBeenCalledTimes(1);
    expect(navigation.replace).not.toHaveBeenCalled();
  });
});
