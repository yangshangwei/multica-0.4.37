import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { I18nProvider } from "@multica/core/i18n/react";
import { useSkillsViewStore } from "@multica/core/skills/stores";
import type { SkillSummary, SkillTemplate } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { SkillLibraryCatalog } from "./skill-library-catalog";
import enSkills from "../../locales/en/skills.json";
import zhSkills from "../../locales/zh-Hans/skills.json";

const mocks = vi.hoisted(() => ({ listSkillTemplates: vi.fn() }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: mocks,
}));

const DEPLOYMENT: SkillTemplate = {
  name: "team-review", version: 1, description: "Review the team's release checklist.",
  category: "quality", content: "# Review", files: [],
};
const BUILTIN: SkillTemplate = {
  name: "multica-code-review", version: 1, description: "Review a change.",
  category: "engineering", content: "# Code", files: [],
};
const COPY: SkillSummary = {
  id: "copy-one", workspace_id: "ws-1", name: "billing-review", description: "Review billing changes.",
  config: { template_source: { name: DEPLOYMENT.name, version: 1 } }, created_by: "user-1",
  created_at: "2026-09-28T00:00:00Z", updated_at: "2026-09-28T00:00:00Z",
};
const clients: QueryClient[] = [];

function renderCatalog({
  skills = [COPY], skillsError = false, templates, workspaceId = "ws-1",
}: {
  skills?: readonly SkillSummary[];
  skillsError?: boolean;
  templates?: SkillTemplate[];
  workspaceId?: string;
} = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity, refetchOnMount: false } },
  });
  clients.push(queryClient);
  if (templates) queryClient.setQueryData(workspaceKeys.skillTemplates(workspaceId), templates);
  const navigation: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/skills",
    searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path, openInNewTab: vi.fn(),
  };
  const onPreview = vi.fn();
  const onCreate = vi.fn();
  function content(nextSkills: readonly SkillSummary[] | undefined, error: boolean, wsId: string) {
    return <QueryClientProvider client={queryClient}>
      <WorkspaceSlugProvider slug="acme">
        <NavigationProvider value={navigation}>
          <SkillLibraryCatalog workspaceId={wsId} skills={nextSkills} skillsError={error} onPreview={onPreview} onCreate={onCreate}>
            <div>Workspace collection</div>
          </SkillLibraryCatalog>
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </QueryClientProvider>;
  }
  const result = renderWithI18n(content(skills, skillsError, workspaceId));
  return {
    queryClient, navigation, onPreview, onCreate,
    rerender: (nextSkills: readonly SkillSummary[] | undefined, error = false, wsId = workspaceId) => result.rerender(content(nextSkills, error, wsId)),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  useSkillsViewStore.setState({ libraryView: null, marketSource: "all", marketCategory: null });
  mocks.listSkillTemplates.mockResolvedValue([DEPLOYMENT, BUILTIN]);
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});

describe("SkillLibraryCatalog", () => {
  // Source/category/search matrices live in ../lib/skill-market.test.ts.
  it("keeps deployment templates out of the workspace collection", async () => {
    renderCatalog({ templates: [DEPLOYMENT, BUILTIN] });
    await waitFor(() => expect(mocks.listSkillTemplates).toHaveBeenCalled());
    expect(screen.getByText("Workspace collection")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Preview team-review" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Deployment-provided" })).not.toBeInTheDocument();
  });

  it("opens the template catalog from the workspace action without losing source preferences", async () => {
    const user = userEvent.setup();
    useSkillsViewStore.setState({ marketSource: "deployment", marketCategory: "quality" });
    renderCatalog({ templates: [DEPLOYMENT, BUILTIN] });
    const entry = screen.getByRole("button", { name: "From template" });
    entry.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByRole("tab", { name: "Skill templates" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Skill templates" })).toHaveFocus();
    expect(screen.getByRole("tab", { name: "Deployment-provided" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("button", { name: "Preview team-review" })).toBeVisible();
    expect(screen.getByText("Workspace collection")).not.toBeVisible();
    expect(useSkillsViewStore.getState().marketCategory).toBe("quality");
  });

  it("opens the named template preview with its keyboard trigger", async () => {
    const user = userEvent.setup();
    const { onPreview } = renderCatalog({ skills: [] });
    const preview = await screen.findByRole("button", { name: "Preview team-review" });
    expect(preview).toHaveTextContent("Preview template");
    expect(screen.getAllByText("No workspace copy yet")).toHaveLength(2);
    preview.focus();
    await user.keyboard("{Enter}");
    expect(onPreview).toHaveBeenCalledWith(DEPLOYMENT.name, preview);
  });

  it("defaults only a successfully loaded empty workspace to market and keeps an explicit choice", async () => {
    const { rerender } = renderCatalog({ skills: [], skillsError: true });
    expect(useSkillsViewStore.getState().libraryView).toBeNull();
    expect(screen.getByRole("tab", { name: "Workspace skills" })).toHaveAttribute("aria-selected", "true");
    rerender(undefined);
    expect(useSkillsViewStore.getState().libraryView).toBeNull();
    rerender([]);
    expect(screen.getByRole("tab", { name: "Skill templates" })).toHaveAttribute("aria-selected", "true");
    fireEvent.click(screen.getByRole("tab", { name: "Workspace skills" }));
    rerender([]);
    expect(screen.getByText("Workspace collection")).toBeVisible();
    expect(useSkillsViewStore.getState().libraryView).toBe("workspace");
  });

  it("honors a click on the active workspace tab before its data succeeds", () => {
    const { rerender } = renderCatalog({ skills: [], skillsError: true });
    fireEvent.click(screen.getByRole("tab", { name: "Workspace skills" }));
    rerender([]);
    expect(screen.getByRole("tab", { name: "Workspace skills" })).toHaveAttribute("aria-selected", "true");
    expect(useSkillsViewStore.getState().libraryView).toBe("workspace");
  });

  it("keeps the workspace collection mounted while the market is active", async () => {
    renderCatalog();
    const collection = screen.getByText("Workspace collection");
    fireEvent.click(screen.getByRole("tab", { name: "Skill templates" }));
    expect(collection).toBeInTheDocument();
    expect(collection).not.toBeVisible();
    fireEvent.click(screen.getByRole("tab", { name: "Workspace skills" }));
    expect(screen.getByText("Workspace collection")).toBe(collection);
    expect(collection).toBeVisible();
  });

  it("leaves counts unknown during cold loading", () => {
    mocks.listSkillTemplates.mockReturnValue(new Promise(() => {}));
    renderCatalog({ skills: [] });
    expect(screen.getByRole("tab", { name: "Skill templates" })).not.toHaveTextContent("0");
    expect(screen.getByRole("tab", { name: "Deployment-provided" })).not.toHaveTextContent("0");
    expect(screen.getByRole("status", { name: "Loading templates..." })).toBeVisible();
    expect(screen.queryByText("No templates available yet")).not.toBeInTheDocument();
  });

  it("composes market controls and clears an unavailable category when changing source", async () => {
    renderCatalog({ skills: [] });
    await screen.findByRole("button", { name: "Preview team-review" });
    const search = screen.getByRole("textbox", { name: "Search templates" });
    fireEvent.change(search, { target: { value: "review" } });
    fireEvent.click(screen.getByRole("button", { name: "Testing & quality" }));
    expect(screen.queryByRole("button", { name: "Preview multica-code-review" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Platform built-ins" }));
    expect(search).toHaveValue("review");
    expect(useSkillsViewStore.getState().marketCategory).toBeNull();
    expect(screen.getByRole("button", { name: "Preview multica-code-review" })).toBeVisible();
  });

  it("offers every renamed related skill as a real link without opening the preview", async () => {
    const user = userEvent.setup();
    const { onPreview, navigation, rerender } = renderCatalog({ skills: [COPY, { ...COPY, id: "copy-two", name: "security-review" }] });
    fireEvent.click(screen.getByRole("button", { name: "From template" }));
    await screen.findByRole("button", { name: "Preview team-review" });
    expect(screen.getByText("2 related skills in this workspace")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "View skills" }));
    const first = await screen.findByRole("menuitem", { name: "billing-review" });
    const second = screen.getByRole("menuitem", { name: "security-review" });
    expect(first).toHaveAttribute("href", "/acme/skills/copy-one");
    expect(second).toHaveAttribute("href", "/acme/skills/copy-two");
    await user.click(second);
    expect(navigation.push).toHaveBeenCalledWith("/acme/skills/copy-two");
    expect(onPreview).not.toHaveBeenCalled();
    rerender([COPY], true);
    expect(screen.getAllByText("Workspace skill status unavailable")).toHaveLength(2);
    expect(screen.queryByRole("link", { name: "View skill" })).not.toBeInTheDocument();
  });

  it("keeps cached results and the search while a failed refresh exposes retry", async () => {
    mocks.listSkillTemplates.mockRejectedValue(new Error("Catalog unavailable"));
    renderCatalog({ skills: [], templates: [DEPLOYMENT, BUILTIN] });
    fireEvent.change(screen.getByRole("textbox", { name: "Search templates" }), { target: { value: "team" } });
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not refresh templates");
    expect(screen.getByRole("button", { name: "Preview team-review" })).toBeVisible();
    mocks.listSkillTemplates.mockResolvedValue([DEPLOYMENT, BUILTIN]);
    fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByRole("textbox", { name: "Search templates" })).toHaveValue("team");
  });

  it("distinguishes catalog failure from a successful empty catalog", async () => {
    mocks.listSkillTemplates.mockRejectedValue(new Error("Catalog unavailable"));
    const { queryClient, onCreate } = renderCatalog({ skills: [] });
    await screen.findByRole("alert");
    expect(screen.queryByText("No templates available yet")).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Skill templates" })).not.toHaveTextContent("0");
    await act(async () => { queryClient.setQueryData(workspaceKeys.skillTemplates("ws-1"), []); });
    expect(await screen.findByText("No templates available yet")).toBeVisible();
    const create = screen.getByRole("button", { name: "New skill" });
    fireEvent.click(create);
    expect(onCreate).toHaveBeenCalledWith(create);
  });

  it("keeps empty deployment guidance in the catalog and distinguishes search misses", async () => {
    mocks.listSkillTemplates.mockResolvedValue([BUILTIN]);
    const { rerender } = renderCatalog();
    await waitFor(() => expect(screen.getByRole("tab", { name: "Skill templates" })).toHaveTextContent("1"));
    expect(screen.queryByText("This deployment has no shared templates yet.")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "From template" }));
    fireEvent.click(screen.getByRole("tab", { name: "Deployment-provided" }));
    expect(screen.getByText("This deployment has no shared templates yet.")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Browse platform templates" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Search templates" }), { target: { value: "no-match" } });
    expect(screen.getByText('No templates match "no-match".')).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(screen.getByRole("button", { name: "Preview multica-code-review" })).toBeVisible();
    fireEvent.change(screen.getByRole("textbox", { name: "Search templates" }), { target: { value: "old search" } });
    rerender([COPY], false, "ws-2");
    expect(screen.getByRole("textbox", { name: "Search templates" })).toHaveValue("");
  });
});

describe("catalog active-locale rendering", () => {
  it.each([
    { locale: "en" as const, skills: enSkills, preview: "Preview multica-code-review" },
    { locale: "zh-Hans" as const, skills: zhSkills, preview: `预览 ${zhSkills.builtin_role_skills["multica-code-review"].name}` },
  ])("renders the catalog with only $locale resources", async ({ locale, skills, preview }) => {
    mocks.listSkillTemplates.mockResolvedValue([BUILTIN]);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    clients.push(queryClient);
    render(
      <I18nProvider locale={locale} resources={{ [locale]: { skills } }}>
        <QueryClientProvider client={queryClient}>
          <WorkspaceSlugProvider slug="acme">
            <SkillLibraryCatalog workspaceId="ws-1" skills={[]} skillsError={false} onPreview={vi.fn()} onCreate={vi.fn()}>
              <div>Workspace collection</div>
            </SkillLibraryCatalog>
          </WorkspaceSlugProvider>
        </QueryClientProvider>
      </I18nProvider>,
    );
    expect(await screen.findByRole("button", { name: preview })).toBeVisible();
    expect(screen.getByRole("tab", { name: skills.market.title })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("textbox", { name: skills.market.search })).toBeVisible();
  });
});
