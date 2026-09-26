import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { EMPTY_SKILL_FILTERS, useSkillsViewStore } from "@multica/core/skills/stores";
import type { SkillSummary, SkillTemplate, Workspace } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { toast } from "sonner";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import enSkills from "../../locales/en/skills.json";

const mocks = vi.hoisted(() => ({
  listWorkspaces: vi.fn(),
  listSkillTemplates: vi.fn(),
  listSkills: vi.fn(),
  listAgents: vi.fn(),
  listMembers: vi.fn(),
  listRuntimes: vi.fn(),
  createSkill: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: mocks,
}));

vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return {
    useAuthStore: Object.assign(
      (selector?: (value: typeof state) => unknown) => selector ? selector(state) : state,
      { getState: () => state },
    ),
  };
});

// Markdown is a heavy leaf. The page, queries, dialog, session and primitives
// stay real so an ancestor remount cannot be hidden by the test harness.
vi.mock("../../rich-content", () => ({
  RichContent: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import SkillsPage from "./skills-page";

const WORKSPACE: Workspace = {
  id: "12345678-1234-4234-8234-123456789abc",
  slug: "acme",
  name: "Acme",
  description: null,
  context: null,
  settings: {},
  repos: [],
  issue_prefix: "ACME",
  avatar_url: null,
  created_at: "2026-09-26T00:00:00Z",
  updated_at: "2026-09-26T00:00:00Z",
};
const TEMPLATE: SkillTemplate = {
  name: "multica-code-review",
  version: 1,
  description: enSkills.builtin_role_skills["multica-code-review"].description,
  content: "---\nname: multica-code-review\ndescription: Review a diff\n---\n# Review\n\nFind actionable regressions.\n",
  files: [],
};
const RELATED_SKILL: SkillSummary = {
  id: "87654321-4321-4321-8321-cba987654321",
  workspace_id: WORKSPACE.id,
  name: "team-code-review",
  description: "The team's saved review instructions.",
  config: { template_source: { name: TEMPLATE.name, version: 1 } },
  created_by: "user-1",
  created_at: "2026-09-26T00:00:00Z",
  updated_at: "2026-09-26T00:00:00Z",
};
const EDITED_DESCRIPTION = "Check billing changes and their regression evidence.";
const EDITED_BODY = "# Billing review\n\nInspect payment handling.\n";
const clients: QueryClient[] = [];

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: Infinity, refetchOnMount: false },
    },
  });
  clients.push(queryClient);
  queryClient.setQueryData(workspaceKeys.list(), [WORKSPACE]);
  queryClient.setQueryData(workspaceKeys.skills(WORKSPACE.id), [RELATED_SKILL]);
  const navigation: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/skills",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path) => path,
    openInNewTab: vi.fn(),
  };
  renderWithI18n(
    <QueryClientProvider client={queryClient}>
      <WorkspaceSlugProvider slug={WORKSPACE.slug}>
        <NavigationProvider value={navigation}>
          <SkillsPage />
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </QueryClientProvider>,
  );
  return { queryClient, navigation };
}

async function openEditor() {
  fireEvent.click(screen.getByRole("button", { name: "Browse templates" }));
  fireEvent.click(await screen.findByRole("button", { name: /^multica-code-review/ }));
  const adopt = screen.getByRole("button", { name: "Use this template" });
  await waitFor(() => expect(adopt).toBeEnabled());
  fireEvent.click(adopt);
  await screen.findByRole("textbox", { name: "Name" });
}

function editDraft() {
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
    target: { value: "billing-review" },
  });
  fireEvent.change(screen.getByRole("textbox", { name: "Description" }), {
    target: { value: EDITED_DESCRIPTION },
  });
  fireEvent.change(screen.getByRole("textbox", { name: "Instructions" }), {
    target: { value: EDITED_BODY },
  });
}

function expectEditedDraft() {
  expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("billing-review");
  expect(screen.getByRole("textbox", { name: "Description" })).toHaveValue(EDITED_DESCRIPTION);
  expect(screen.getByRole("textbox", { name: "Instructions" })).toHaveValue(EDITED_BODY);
}

async function failWorkspaceRefresh(queryClient: QueryClient) {
  const refresh = deferred<SkillSummary[]>();
  mocks.listSkills.mockReturnValueOnce(refresh.promise);
  let completed!: Promise<void>;
  act(() => {
    completed = queryClient.invalidateQueries({ queryKey: workspaceKeys.skills(WORKSPACE.id), exact: true });
  });
  await waitFor(() => expect(queryClient.getQueryState(workspaceKeys.skills(WORKSPACE.id))?.fetchStatus).toBe("fetching"));
  await act(async () => {
    refresh.reject(new Error("Skills refresh unavailable"));
    await completed;
  });
  await screen.findByText("Skills refresh unavailable");
  expect(queryClient.getQueryData(workspaceKeys.skills(WORKSPACE.id))).toEqual([RELATED_SKILL]);
}

async function retryWorkspaceRefresh(queryClient: QueryClient, skills = [RELATED_SKILL]) {
  const refresh = deferred<SkillSummary[]>();
  mocks.listSkills.mockReturnValueOnce(refresh.promise);
  let completed!: Promise<void>;
  act(() => {
    completed = queryClient.refetchQueries({ queryKey: workspaceKeys.skills(WORKSPACE.id), exact: true });
  });
  await act(async () => {
    refresh.resolve(skills);
    await completed;
  });
  await waitFor(() => expect(screen.queryByText("Skills refresh unavailable")).not.toBeInTheDocument());
}

beforeEach(() => {
  vi.resetAllMocks();
  useSkillsViewStore.setState({ viewMode: "card", filters: EMPTY_SKILL_FILTERS });
  mocks.listWorkspaces.mockResolvedValue([WORKSPACE]);
  mocks.listSkillTemplates.mockResolvedValue([TEMPLATE]);
  mocks.listSkills.mockResolvedValue([RELATED_SKILL]);
  mocks.listAgents.mockResolvedValue([]);
  mocks.listMembers.mockResolvedValue([]);
  mocks.listRuntimes.mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});

describe("SkillsPage template-session lifetime", () => {
  it.each(["Browse templates", "New skill"])("returns focus to %s after closing the template browser", async (entryName) => {
    const user = userEvent.setup();
    renderPage();
    const entry = screen.getByRole("button", { name: entryName });
    await user.click(entry);
    if (entryName === "New skill") await user.click(screen.getByRole("button", { name: /Modify from template/ }));
    const search = await screen.findByRole("textbox", { name: enSkills.create.template.search_placeholder });
    await waitFor(() => expect(search).toHaveFocus());

    await user.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(entry).toHaveFocus());
    expect(mocks.createSkill).not.toHaveBeenCalled();
  });

  it("returns focus to the current browse button after cached list failure and retry", async () => {
    const user = userEvent.setup();
    const { queryClient } = renderPage();
    await user.click(screen.getByRole("button", { name: "Browse templates" }));
    await screen.findByRole("textbox", { name: enSkills.create.template.search_placeholder });
    await failWorkspaceRefresh(queryClient);
    await retryWorkspaceRefresh(queryClient);

    await user.click(screen.getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Browse templates" })).toHaveFocus();
  });

  it("retains an edited draft and its dirty guard across cached list failure and retry", async () => {
    const { queryClient, navigation } = renderPage();
    await openEditor();
    editDraft();

    await failWorkspaceRefresh(queryClient);
    expectEditedDraft();
    await retryWorkspaceRefresh(queryClient);
    expectEditedDraft();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(await screen.findByRole("alertdialog")).toHaveTextContent("Your changes to this copy have not been saved.");
    fireEvent.click(screen.getByRole("button", { name: "Keep editing" }));
    expectEditedDraft();
    expect(mocks.createSkill).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("retains unknown-result recovery and close protection across cached list failure and retry", async () => {
    mocks.createSkill.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    const { queryClient, navigation } = renderPage();
    await openEditor();
    fireEvent.click(screen.getByRole("button", { name: "Create skill" }));
    await screen.findByRole("button", { name: "Check creation result" });

    await failWorkspaceRefresh(queryClient);
    expect(screen.getByText(enSkills.create.template.result_unknown)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Check creation result" })).toBeEnabled();
    await retryWorkspaceRefresh(queryClient);
    expect(screen.getByText(enSkills.create.template.result_unknown)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Check creation result" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Continue editing" }));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(enSkills.create.template.discard_unknown_description);
    expect(mocks.createSkill).toHaveBeenCalledTimes(1);
    expect(navigation.push).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("keeps a pending related-skill discard cancellable through list failure and retry", async () => {
    const user = userEvent.setup();
    const { queryClient, navigation } = renderPage();
    await openEditor();
    editDraft();
    await user.click(screen.getByRole("button", { name: "Choose another template" }));
    const link = await screen.findByRole("link", { name: RELATED_SKILL.name });
    await user.click(link);
    const alert = await screen.findByRole("alertdialog");

    await failWorkspaceRefresh(queryClient);
    expect(screen.getByRole("alertdialog")).toBe(alert);
    await retryWorkspaceRefresh(queryClient);
    expect(screen.getByRole("alertdialog")).toBe(alert);
    await user.click(screen.getByRole("button", { name: "Keep editing" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    await waitFor(() => expect(link).toHaveFocus());
    expect(navigation.push).not.toHaveBeenCalled();
    expect(navigation.openInNewTab).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Use this template" }));
    expectEditedDraft();
    expect(mocks.createSkill).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("confirms the original pending destination and tab intent once after list failure and retry", async () => {
    const user = userEvent.setup();
    const { queryClient, navigation } = renderPage();
    await openEditor();
    editDraft();
    await user.click(screen.getByRole("button", { name: "Choose another template" }));
    const link = await screen.findByRole("link", { name: RELATED_SKILL.name });
    await user.keyboard("{Control>}");
    await user.click(link);
    await user.keyboard("{/Control}");
    const alert = await screen.findByRole("alertdialog");

    await failWorkspaceRefresh(queryClient);
    expect(screen.getByRole("alertdialog")).toBe(alert);
    await retryWorkspaceRefresh(queryClient, [{
      ...RELATED_SKILL,
      id: "99999999-4321-4321-8321-cba987654321",
      name: "another-related-review",
    }]);
    expect(screen.getByRole("alertdialog")).toBe(alert);
    expect(navigation.push).not.toHaveBeenCalled();
    expect(navigation.openInNewTab).not.toHaveBeenCalled();

    // The confirmation gesture must not replace the original background intent.
    fireEvent.click(screen.getByRole("button", { name: "Discard changes" }), { ctrlKey: true, shiftKey: true });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(navigation.openInNewTab).toHaveBeenCalledExactlyOnceWith(
      `/acme/skills/${RELATED_SKILL.id}`,
      RELATED_SKILL.name,
    );
    expect(navigation.push).not.toHaveBeenCalled();
    expect(mocks.createSkill).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });
});
