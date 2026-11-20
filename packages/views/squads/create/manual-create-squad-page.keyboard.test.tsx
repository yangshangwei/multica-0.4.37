import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { NavigationProvider } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { ManualCreateSquadPage } from "./manual-create-squad-page";

vi.mock("@multica/core/api", () => ({ api: {
  listAgents: vi.fn().mockResolvedValue([]),
  listMembers: vi.fn().mockResolvedValue([{ id: "membership-ada", user_id: "ada", name: "Ada", role: "member" }]),
} }));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "viewer" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("../../common/avatar-upload-control", () => ({ AvatarUploadControl: () => null }));

describe("manual squad member selection with real primitives", () => {
  it("announces the keyboard candidate and preserves search focus through repeated selection", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{
      push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads/new/manual", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
    }}><ManualCreateSquadPage /></NavigationProvider></QueryClientProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Additional Members" }));
    const search = screen.getByPlaceholderText("Search agents and members...");
    await user.type(search, "Ada");
    const option = await screen.findByRole("option", { name: "Ada" });
    await waitFor(() => expect(search).toHaveAttribute("aria-activedescendant", option.id));
    await user.keyboard("{Enter}");
    expect(option).toHaveAttribute("aria-selected", "true");
    expect(search).toHaveFocus();
    expect(search).toHaveAttribute("aria-activedescendant", option.id);
    await user.keyboard("{Enter}");
    expect(option).toHaveAttribute("aria-selected", "false");
    await user.keyboard("{Escape}");
    expect(screen.getByRole("button", { name: "Additional Members" })).toHaveFocus();
  });
});
