import type { ReactNode } from "react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";

const mockUpdateWorkspace = vi.hoisted(() => vi.fn());
const mockDeleteInstallation = vi.hoisted(() => vi.fn());
const mockGetConnectURL = vi.hoisted(() => vi.fn());
const mockInvalidate = vi.hoisted(() => vi.fn());
const mockNavPush = vi.hoisted(() => vi.fn());
const mockSetQueryData = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());
const navigationRef = vi.hoisted(() => ({
  search: "tab=integrations&integration=github",
  hash: "",
}));

const workspaceRef = vi.hoisted(() => ({
  current: {
    id: "workspace-1",
    name: "Acme",
    slug: "acme",
    settings: {} as Record<string, unknown>,
    repos: [{ url: "https://github.com/acme/api" }] as { url: string }[],
  },
}));
type MemberRole = "owner" | "admin" | "member" | "guest";
const membersRef = vi.hoisted(() => ({
  current: [{ user_id: "user-1", role: "owner" as MemberRole }],
}));
const installationsRef = vi.hoisted(() => ({
  current: {
    installations: [] as {
      id: string;
      account_login: string;
      installation_id?: number;
      connected_by?: string;
    }[],
    configured: true,
    can_manage: true as boolean,
  },
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (opts: { queryKey: unknown[] }) => {
    const key = JSON.stringify(opts.queryKey);
    if (key.includes("members")) return { data: membersRef.current };
    if (key.includes("installations")) return { data: installationsRef.current };
    return { data: undefined };
  },
  useQueryClient: () => ({
    setQueryData: mockSetQueryData,
    invalidateQueries: mockInvalidate,
  }),
  queryOptions: <T,>(opts: T) => opts,
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => workspaceRef.current,
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: vi.fn() }),
  workspaceKeys: { list: () => ["workspaces"] },
}));

vi.mock("@multica/core/github", async () => {
  const actual =
    await vi.importActual<typeof import("@multica/core/github")>("@multica/core/github");
  return {
    ...actual,
    githubInstallationsOptions: () => ({
      queryKey: ["github", "installations"],
      queryFn: vi.fn(),
    }),
  };
});

vi.mock("@multica/core/api", () => ({
  api: {
    updateWorkspace: mockUpdateWorkspace,
    deleteGitHubInstallation: mockDeleteInstallation,
    getGitHubConnectURL: mockGetConnectURL,
  },
}));

vi.mock("@multica/core/auth", () => {
  const useAuthStore = Object.assign(
    (sel?: (s: { user: { id: string } }) => unknown) =>
      sel ? sel({ user: { id: "user-1" } }) : { user: { id: "user-1" } },
    { getState: () => ({ user: { id: "user-1" } }) },
  );
  return { useAuthStore };
});

// Mocked at the context module rather than the barrel so <AppLink> stays the
// real component and its click contract is what the test exercises.
vi.mock("../../navigation/context", () => ({
  useNavigation: () => ({
    push: mockNavPush,
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/settings",
    searchParams: new URLSearchParams(navigationRef.search),
    hash: navigationRef.hash,
    getShareableUrl: (p: string) => `https://app.example${p}`,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}));

import { GitHubTab } from "./github-tab";

const TEST_RESOURCES = {
  en: { common: enCommon, settings: enSettings },
};

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

function resetFixtures() {
  vi.resetAllMocks();
  navigationRef.search = "tab=integrations&integration=github";
  navigationRef.hash = "";
  workspaceRef.current = {
    id: "workspace-1",
    name: "Acme",
    slug: "acme",
    settings: {},
    repos: [{ url: "https://github.com/acme/api" }],
  };
  membersRef.current = [{ user_id: "user-1", role: "owner" }];
  installationsRef.current = { installations: [], configured: true, can_manage: true };
}

describe("GitHubTab", () => {
  beforeEach(resetFixtures);
  afterEach(() => vi.restoreAllMocks());

  it("uses h3 section headings beneath the integration detail h2", () => {
    render(<GitHubTab />, { wrapper: I18nWrapper });

    expect(screen.getAllByRole("heading", { level: 3 }).map((heading) => heading.textContent))
      .toEqual(["Connection", "Features", "Repositories"]);
  });

  it("folds the non-dev hint into the master switch description (no separate callout)", () => {
    render(<GitHubTab />, { wrapper: I18nWrapper });
    expect(screen.getByText(/Not a development team\? Just turn it off here\./)).toBeTruthy();
    // The old standalone callout (title + dedicated "Turn GitHub off" button) is gone.
    expect(screen.queryByRole("button", { name: /^Turn GitHub off$/ })).toBeNull();
  });

  it("does not show the hint once the master switch is off", () => {
    workspaceRef.current.settings = { github_enabled: false };
    render(<GitHubTab />, { wrapper: I18nWrapper });
    expect(screen.queryByText(/Not a development team\?/)).toBeNull();
  });

  it("disables every feature switch when the master switch is off", () => {
    workspaceRef.current.settings = { github_enabled: false };
    render(<GitHubTab />, { wrapper: I18nWrapper });

    const master = screen.getByRole("switch", { name: /enable github features/i });
    expect(master.getAttribute("aria-checked")).toBe("false");

    const switches = screen.getAllByRole("switch");
    // First switch is master; remaining must be disabled (aria-disabled or disabled attr)
    const features = switches.slice(1);
    expect(features.length).toBeGreaterThan(0);
    for (const sw of features) {
      const ariaDisabled = sw.getAttribute("aria-disabled");
      const disabled = sw.hasAttribute("disabled");
      expect(ariaDisabled === "true" || disabled).toBe(true);
    }
  });

  // The full flag-derivation matrix belongs to packages/core/github/settings.test.ts.
  it("retains feature preferences and the connection across a master off/on cycle", async () => {
    const user = userEvent.setup();
    const preferences = {
      github_pr_sidebar_enabled: true,
      co_authored_by_enabled: false,
      github_auto_link_prs_enabled: true,
      unrelated_setting: "preserve me",
    };
    workspaceRef.current.settings = preferences;
    installationsRef.current.installations = [{ id: "inst-42", account_login: "acme" }];
    mockUpdateWorkspace.mockImplementation(async (_id, patch) => ({
      ...workspaceRef.current, ...patch,
    }));
    mockSetQueryData.mockImplementation((_key, update) => {
      [workspaceRef.current] = update([workspaceRef.current]);
    });

    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("switch", { name: /enable github features/i }));

    await waitFor(() => {
      expect(mockUpdateWorkspace).toHaveBeenCalledWith("workspace-1", {
        settings: { ...preferences, github_enabled: false },
      });
      expect(mockToastSuccess).toHaveBeenCalledWith("Changes saved", {
        id: "settings-auto-save",
      });
    });

    for (const feature of screen.getAllByRole("switch").slice(1)) {
      expect(feature).toHaveAttribute("aria-disabled", "true");
      expect(feature).toHaveAttribute("aria-checked", "false");
    }
    expect(screen.getByText(/Connected to acme/i)).toBeVisible();
    expect(screen.getByRole("button", { name: /^Disconnect$/ })).toBeEnabled();
    expect(mockDeleteInstallation).not.toHaveBeenCalled();

    await user.click(screen.getByRole("switch", { name: /enable github features/i }));
    await waitFor(() => expect(mockUpdateWorkspace).toHaveBeenLastCalledWith("workspace-1", {
      settings: { ...preferences, github_enabled: true },
    }));
    expect(screen.getByRole("switch", { name: "Pull Request sidebar" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("switch", { name: "Co-authored-by trailer" })).toHaveAttribute("aria-checked", "false");
    expect(screen.getByRole("switch", { name: "Auto-link issues and PRs" })).toHaveAttribute("aria-checked", "true");
  });

  it("keeps the saved state and reports an error when a preference update fails", async () => {
    const user = userEvent.setup();
    mockUpdateWorkspace.mockRejectedValue(new Error("Workspace update failed"));
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("switch", { name: /enable github features/i }));

    await waitFor(() => expect(mockToastError).toHaveBeenCalledWith("Workspace update failed"));
    expect(mockSetQueryData).not.toHaveBeenCalled();
    expect(screen.getByRole("switch", { name: /enable github features/i })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("switch", { name: /enable github features/i })).not.toHaveAttribute("aria-disabled", "true");
  });

  it("opens the configured GitHub installation URL from Connect", async () => {
    const user = userEvent.setup();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    mockGetConnectURL.mockResolvedValue({ configured: true, url: "https://github.com/apps/multica/installations/new" });
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /^Connect GitHub$/ }));

    expect(mockGetConnectURL).toHaveBeenCalledWith("workspace-1");
    await waitFor(() => expect(open).toHaveBeenCalledWith(
      "https://github.com/apps/multica/installations/new", "_blank", "noopener",
    ));
    expect(screen.getByRole("button", { name: /^Connect GitHub$/ })).toBeEnabled();
  });

  it("reports a failed Connect request and allows retrying", async () => {
    const user = userEvent.setup();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    mockGetConnectURL.mockRejectedValue(new Error("Connection unavailable"));
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /^Connect GitHub$/ }));

    await waitFor(() => expect(mockToastError).toHaveBeenCalledWith("Connection unavailable"));
    expect(open).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /^Connect GitHub$/ })).toBeEnabled();
  });

  it("clicking Disconnect opens the confirmation and only fires on confirm", async () => {
    const user = userEvent.setup();
    installationsRef.current = {
      configured: true,
      can_manage: true,
      installations: [{ id: "inst-42", account_login: "acme", installation_id: 42 }],
    };
    mockDeleteInstallation.mockResolvedValue(undefined);

    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /^Disconnect$/ }));
    expect(screen.getByText(/Multica will stop receiving webhooks/i)).toBeTruthy();
    expect(mockDeleteInstallation).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: /^Disconnect$/ }));

    await waitFor(() => {
      expect(mockDeleteInstallation).toHaveBeenCalledWith("workspace-1", "inst-42");
      expect(mockInvalidate).toHaveBeenCalledWith({ queryKey: ["github", "workspace-1"] });
      expect(mockToastSuccess).toHaveBeenCalledWith("GitHub App disconnected");
    });
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
  });

  it("canceling Disconnect keeps the installation connected", async () => {
    const user = userEvent.setup();
    installationsRef.current.installations = [{ id: "inst-42", account_login: "acme" }];
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /^Disconnect$/ }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(mockDeleteInstallation).not.toHaveBeenCalled();
    expect(screen.getByText(/Connected to acme/i)).toBeVisible();
  });

  it("keeps the confirmation open for retry when Disconnect fails", async () => {
    const user = userEvent.setup();
    installationsRef.current.installations = [{ id: "inst-42", account_login: "acme" }];
    mockDeleteInstallation.mockRejectedValue(new Error("Disconnect failed"));
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /^Disconnect$/ }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: /^Disconnect$/ }));

    await waitFor(() => expect(mockToastError).toHaveBeenCalledWith("Disconnect failed"));
    expect(within(screen.getByRole("alertdialog")).getByRole("button", { name: /^Disconnect$/ })).toBeEnabled();
    expect(mockInvalidate).not.toHaveBeenCalled();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("Disconnect button is still visible when the master switch is off", () => {
    workspaceRef.current.settings = { github_enabled: false };
    installationsRef.current = {
      configured: true,
      can_manage: true,
      installations: [{ id: "inst-1", account_login: "acme", installation_id: 1 }],
    };
    render(<GitHubTab />, { wrapper: I18nWrapper });
    expect(screen.getByRole("button", { name: /^Disconnect$/ })).toBeTruthy();
  });

  it("non-managers see the existing connection with all settings read-only", async () => {
    const user = userEvent.setup();
    membersRef.current = [{ user_id: "user-1", role: "member" }];
    installationsRef.current = {
      configured: true,
      can_manage: false,
      installations: [{ id: "inst-1", account_login: "acme" }],
    };
    render(<GitHubTab />, { wrapper: I18nWrapper });

    expect(screen.getByText(/Connected to acme/i)).toBeTruthy();
    expect(screen.getByText(/Read-only view\./i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Connect GitHub$/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Disconnect$/ })).toBeNull();
    for (const setting of screen.getAllByRole("switch")) {
      expect(setting).toHaveAttribute("aria-disabled", "true");
      await user.click(setting);
    }
    expect(mockUpdateWorkspace).not.toHaveBeenCalled();
    expect(mockGetConnectURL).not.toHaveBeenCalled();
    expect(mockDeleteInstallation).not.toHaveBeenCalled();
  });

  it("non-admin with no connection sees the contact-admin hint", () => {
    membersRef.current = [{ user_id: "user-1", role: "member" }];
    installationsRef.current = {
      configured: true,
      can_manage: false,
      installations: [],
    };
    render(<GitHubTab />, { wrapper: I18nWrapper });

    expect(screen.getByText(/Ask an admin or owner/i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^Connect GitHub$/ })).toBeNull();
  });

  it("renders the connected_by line when the backend provides it", () => {
    installationsRef.current = {
      configured: true,
      can_manage: true,
      installations: [
        {
          id: "inst-7",
          account_login: "acme",
          installation_id: 7,
          connected_by: "Jiayuan",
        },
      ],
    };
    render(<GitHubTab />, { wrapper: I18nWrapper });
    expect(screen.getByText(/Connected by Jiayuan/)).toBeTruthy();
  });

  it("repositories shortcut navigates to the repositories tab", async () => {
    const user = userEvent.setup();
    render(<GitHubTab />, { wrapper: I18nWrapper });
    await user.click(screen.getByRole("button", { name: /Manage repositories/ }));
    expect(mockNavPush).toHaveBeenCalledWith("/acme/settings?tab=repositories");
  });

  it("repositories shortcut clears the selected integration and preserves unrelated URL context", async () => {
    const user = userEvent.setup();
    navigationRef.search = "tab=integrations&integration=github&context=workspace";
    navigationRef.hash = "#settings";
    render(<GitHubTab />, { wrapper: I18nWrapper });

    await user.click(screen.getByRole("button", { name: /Manage repositories/ }));

    expect(mockNavPush).toHaveBeenCalledWith("/acme/settings?tab=repositories&context=workspace#settings");
  });
});
