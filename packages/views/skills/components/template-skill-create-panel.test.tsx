import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { SkillSummary, SkillTemplate } from "@multica/core/types";
import type { SupportedLocale } from "@multica/core/i18n";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { NavigationProvider } from "../../navigation";
import jaCommon from "../../locales/ja/common.json";
import jaSkills from "../../locales/ja/skills.json";
import koCommon from "../../locales/ko/common.json";
import koSkills from "../../locales/ko/skills.json";
import enCommon from "../../locales/en/common.json";
import enSkills from "../../locales/en/skills.json";
import zhCommon from "../../locales/zh-Hans/common.json";
import zhSkills from "../../locales/zh-Hans/skills.json";

// Source grouping is a pure derivation off the template catalog. Drive it
// through the real create dialog so selection/search/keyboard stay unmocked.
const mocks = vi.hoisted(() => ({
  workspaceId: "ws-1",
  listSkillTemplates: vi.fn(),
  listSkills: vi.fn(),
  getSkill: vi.fn(),
  createSkill: vi.fn(),
  updateSkill: vi.fn(),
  importSkillArchive: vi.fn(),
  setAgentSkills: vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    listSkillTemplates: mocks.listSkillTemplates,
    listSkills: mocks.listSkills,
    getSkill: mocks.getSkill,
    createSkill: mocks.createSkill,
    updateSkill: mocks.updateSkill,
    importSkillArchive: mocks.importSkillArchive,
    setAgentSkills: mocks.setAgentSkills,
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mocks.workspaceId,
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

vi.mock("../../rich-content", () => ({
  RichContent: ({ content }: { content: string }) => <div>{content}</div>,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { CreateSkillDialog } from "./create-skill-dialog";

const LOCALES = {
  en: { common: enCommon, skills: enSkills },
  "zh-Hans": { common: zhCommon, skills: zhSkills },
  ja: { common: jaCommon, skills: jaSkills },
  ko: { common: koCommon, skills: koSkills },
};
type TestLocale = Extract<SupportedLocale, keyof typeof LOCALES>;

const BUILTIN_NAME = "multica-code-review";
const MOUNTED_NAME = "team-code-style";
const MOUNTED_DESCRIPTION = "Follow the team's house code style when reviewing.";

function builtinTemplate(): SkillTemplate {
  return {
    name: BUILTIN_NAME,
    version: 1,
    description: enSkills.builtin_role_skills[BUILTIN_NAME].description,
    content: `---\nname: ${BUILTIN_NAME}\ndescription: Review a diff\n---\n# Review\n`,
    files: [],
  };
}

function mountedTemplate(): SkillTemplate {
  return {
    name: MOUNTED_NAME,
    version: 0,
    description: MOUNTED_DESCRIPTION,
    content: `---\nname: ${MOUNTED_NAME}\ndescription: ${MOUNTED_DESCRIPTION}\n---\n# House style\n`,
    files: [],
  };
}

function renderDialog(locale: TestLocale = "en", direct = false, templateName?: string, cachedTemplates?: SkillTemplate[]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  if (cachedTemplates) queryClient.setQueryData(workspaceKeys.skillTemplates(mocks.workspaceId), cachedTemplates);
  render(
    <I18nProvider locale={locale} resources={{ [locale]: LOCALES[locale] }}>
      <QueryClientProvider client={queryClient}>
        <WorkspaceSlugProvider slug="acme">
          <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/skills", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path }}>
            <CreateSkillDialog initialEntry={direct ? { kind: "templates", templateName } : undefined} onClose={vi.fn()} onCreated={vi.fn()} />
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </QueryClientProvider>
    </I18nProvider>,
  );
  return queryClient;
}

async function openTemplates() {
  fireEvent.click(screen.getByRole("button", { name: /Modify from template/ }));
  await screen.findByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) });
}

beforeEach(() => {
  vi.resetAllMocks();
  mocks.workspaceId = "ws-1";
  mocks.listSkills.mockResolvedValue([]);
});

afterEach(() => cleanup());

const builtinTab = () => screen.getByRole("tab", { name: new RegExp(enSkills.create.template.group_builtin) });
const deploymentTab = () => screen.getByRole("tab", { name: new RegExp(enSkills.create.template.group_deployment_label) });

describe("TemplateSkillCreatePanel source tabs", () => {
  it.each([
    ["multica-experience-validation", "验证关键用户路径的浏览器、跨端和可访问性证据，记录结果风险与缺口；缺少证据时标记 unknown。"],
    ["multica-migration-review", "审查 schema、API 和客户端迁移的兼容窗口、校验、幂等、重试与回滚证据；缺失材料时阻塞或标记 unknown。"],
  ] as const)(
    "groups and searches the real source description for %s as built-in",
    async (name, description) => {
      const content = `---\nname: ${name}\ndescription: ${description}\n---\n`;
      mocks.listSkillTemplates.mockResolvedValue([
        builtinTemplate(), { name, version: 1, description, content, files: [] }, mountedTemplate(),
      ]);
      renderDialog();
      await openTemplates();
      expect(builtinTab()).toHaveAccessibleName(/Platform built-in\s*2/);
      expect(deploymentTab()).toHaveAccessibleName(/Provided by this deployment\s*1/);
      expect(screen.getByRole("button", { name: new RegExp(`^${name}`) })).toHaveTextContent(
        enSkills.builtin_role_skills[name].summary,
      );
      fireEvent.change(screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder }), {
        target: { value: enSkills.builtin_role_skills[name].description },
      });
      await waitFor(() => expect(builtinTab()).toHaveAccessibleName(/Platform built-in\s*1/));
      expect(deploymentTab()).toHaveAccessibleName(/Provided by this deployment\s*0/);
      expect(screen.getByRole("button", { name: new RegExp(`^${name}`) })).toBeInTheDocument();
    },
  );

  it("splits built-in and mounted templates into two tabs each showing its count", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    renderDialog();
    await openTemplates();

    // Two tabs, each labelled with its group and a live count.
    expect(builtinTab()).toBeInTheDocument();
    expect(deploymentTab()).toHaveAccessibleName(/Provided by this deployment\s*1/);

    // Built-in tab is active by default: its row shows, the mounted row does not yet.
    expect(screen.getByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).not.toBeInTheDocument();

    // Switching to the deployment tab reveals the mounted row and the inline source guidance.
    fireEvent.click(deploymentTab());
    expect(await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toBeInTheDocument();
    expect(screen.getByText(enSkills.create.template.group_deployment_info)).toBeInTheDocument();
    // Templates exist here, so the empty-state promo must not show.
    expect(screen.queryByText(enSkills.create.template.deployment_empty_hint)).not.toBeInTheDocument();
  });

  it("keeps the deployment tab as an empty-state promo when nothing is mounted", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    renderDialog();
    await openTemplates();

    // Both tabs still render; the deployment tab counts zero.
    expect(builtinTab()).toBeInTheDocument();
    expect(deploymentTab()).toHaveAccessibleName(/Provided by this deployment\s*0/);

    // Its panel shows the promo, not the "these come from ..." provenance line.
    fireEvent.click(deploymentTab());
    expect(await screen.findByText(enSkills.create.template.deployment_empty_hint)).toBeInTheDocument();
    expect(screen.queryByText(enSkills.create.template.group_deployment_info)).not.toBeInTheDocument();
  });

  it("keeps search filtering live per tab and preserves selection", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    renderDialog();
    await openTemplates();

    // Select the mounted row from the deployment tab; it previews and stays pressed.
    fireEvent.click(deploymentTab());
    const mountedRow = await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) });
    fireEvent.click(mountedRow);
    await waitFor(() => expect(mountedRow).toHaveAttribute("aria-pressed", "true"));

    // Searching for the mounted template drops the built-in tab's count to zero
    // while the deployment tab keeps its match.
    fireEvent.change(screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder }), {
      target: { value: "house code style" },
    });
    await waitFor(() => expect(builtinTab()).toHaveAccessibleName(/Platform built-in\s*0/));
    expect(deploymentTab()).toHaveAccessibleName(/Provided by this deployment\s*1/);
    expect(screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toBeInTheDocument();
  });
});


describe("TemplateSkillCreatePanel discovery states", () => {
  it("initializes direct deployment-only browsing in the source containing its visible selection", async () => {
    mocks.listSkillTemplates.mockResolvedValue([mountedTemplate()]);
    renderDialog("en", true);
    const row = await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) });
    expect(deploymentTab()).toHaveAttribute("aria-selected", "true");
    expect(row).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText(/# House style/)).toBeInTheDocument();
    expect(mocks.createSkill).not.toHaveBeenCalled();
  });

  it("selects a named deployment seed but never previews a hidden source after switching tabs", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    renderDialog("en", true, MOUNTED_NAME);
    expect(await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toHaveAttribute("aria-pressed", "true");
    expect(deploymentTab()).toHaveAttribute("aria-selected", "true");
    fireEvent.click(builtinTab());
    expect(screen.getByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByText(/# House style/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Use this template" }));
    expect(await screen.findByRole("textbox", { name: "Name" })).toHaveValue(`${BUILTIN_NAME}-copy`);
  });

  it("uses a short summary only in the row and retains the full purpose for preview and adoption", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    renderDialog("en", true);
    const row = await screen.findByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) });
    expect(row).not.toHaveTextContent(enSkills.builtin_role_skills[BUILTIN_NAME].description);
    expect(screen.getByText(enSkills.builtin_role_skills[BUILTIN_NAME].description)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Use this template" }));
    expect(await screen.findByRole("textbox", { name: "Description" })).toHaveValue(enSkills.builtin_role_skills[BUILTIN_NAME].description);
    expect(screen.getByRole("dialog", { name: "Create a copy" })).toBeInTheDocument();
  });

  it("disables adoption in an empty source or filtered results and offers clear search", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    renderDialog();
    await openTemplates();
    fireEvent.click(deploymentTab());
    expect(screen.getByRole("button", { name: "Use this template" })).toBeDisabled();
    expect(screen.queryByText(/# Review/)).not.toBeInTheDocument();
    fireEvent.click(builtinTab());
    fireEvent.change(screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder }), { target: { value: "no-such-template" } });
    expect(screen.getByRole("button", { name: "Use this template" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Clear search" }));
    expect(screen.getByRole("button", { name: "Use this template" })).toBeEnabled();
  });

  it("keeps cached rows, preview and adoption usable after refresh fails", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    const client = renderDialog();
    await openTemplates();
    mocks.listSkillTemplates.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.skillTemplates("ws-1") }); });
    expect(screen.getByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Use this template" })).toBeEnabled();
    expect(await screen.findByRole("alert")).toHaveTextContent(enSkills.create.template.refresh_failed);
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  });

  it("never claims no related skills while their query is pending or failed", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    let reject!: (reason: Error) => void;
    mocks.listSkills.mockReturnValueOnce(new Promise((_, fail) => { reject = fail; }));
    renderDialog();
    await openTemplates();
    expect(screen.getByRole("status")).toHaveTextContent("Loading related skills");
    expect(screen.queryByText("No related skills in this workspace.")).not.toBeInTheDocument();
    await act(async () => { reject(new Error("offline")); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load related skills");
    expect(screen.queryByText("No related skills in this workspace.")).not.toBeInTheDocument();
    mocks.listSkills.mockResolvedValue([]);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("No related skills in this workspace.")).toBeInTheDocument();
  });

  it("shows every related named link and retains cached links with a refresh failure hint", async () => {
    const related: SkillSummary[] = ["one", "two"].map((name, index) => ({ id: `copy-${index}`, workspace_id: "ws-1", name, description: "", created_by: null, created_at: "2026-09-26T00:00:00Z", updated_at: "2026-09-26T00:00:00Z", config: { template_source: { name: BUILTIN_NAME } } }));
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    mocks.listSkills.mockResolvedValue(related);
    const client = renderDialog();
    await openTemplates();
    expect(screen.getByText("2 related skills in this workspace")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "one" })).toHaveAttribute("href", "/acme/skills/copy-0");
    expect(screen.getByRole("link", { name: "two" })).toHaveAttribute("href", "/acme/skills/copy-1");
    mocks.listSkills.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.skills("ws-1") }); });
    expect(screen.getByRole("link", { name: "two" })).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent(enSkills.create.template.related_refresh_failed);
  });

  it.each(Object.keys(LOCALES) as TestLocale[])("renders preview and copy phases with only %s loaded", async (locale) => {
    mocks.listSkillTemplates.mockResolvedValue([mountedTemplate()]);
    renderDialog(locale, true);
    const copy = LOCALES[locale].skills;
    const dialog = await screen.findByRole("dialog", { name: copy.create.template.preview_label });
    await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) });
    fireEvent.click(within(dialog).getByRole("button", { name: copy.create.template.use_template }));
    expect(await screen.findByRole("textbox", { name: copy.create.manual.description_label })).toHaveValue(MOUNTED_DESCRIPTION);
    expect(screen.getByRole("dialog")).not.toHaveAccessibleName(copy.create.template.preview_label);
  });
});


describe("TemplateSkillCreatePanel catalog lifecycle", () => {
  it("opens a named deployment seed from cached data without selecting the platform source", async () => {
    const cachedTemplates = [builtinTemplate(), mountedTemplate()];
    mocks.listSkillTemplates.mockResolvedValue(cachedTemplates);
    renderDialog("en", true, MOUNTED_NAME, cachedTemplates);

    expect(await screen.findByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toHaveAttribute("aria-pressed", "true");
    expect(deploymentTab()).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder })).toHaveFocus();
    expect(screen.getByText(/# House style/)).toBeInTheDocument();
    expect(screen.queryByText(/# Review/)).not.toBeInTheDocument();
    expect(mocks.createSkill).not.toHaveBeenCalled();
  });

  it("retains the searched deployment selection across cached refresh failure and retry", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    const client = renderDialog("en", true);
    await screen.findByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) });
    fireEvent.click(deploymentTab());
    fireEvent.click(screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) }));
    const search = screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder });
    fireEvent.change(search, { target: { value: "house code style" } });

    mocks.listSkillTemplates.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.skillTemplates("ws-1") }); });
    expect(await screen.findByRole("alert")).toHaveTextContent(enSkills.create.template.refresh_failed);
    expect(deploymentTab()).toHaveAttribute("aria-selected", "true");
    expect(search).toHaveValue("house code style");

    mocks.listSkillTemplates.mockResolvedValue([mountedTemplate(), builtinTemplate()]);
    fireEvent.click(screen.getByRole("button", { name: enSkills.create.template.retry }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(deploymentTab()).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toHaveAttribute("aria-pressed", "true");
    expect(search).toHaveValue("house code style");
    expect(screen.queryByRole("textbox", { name: "Name" })).not.toBeInTheDocument();
    expect(mocks.createSkill).not.toHaveBeenCalled();
  });

  it("falls back from a missing named seed to platform templates regardless of catalog order", async () => {
    mocks.listSkillTemplates.mockResolvedValue([mountedTemplate(), builtinTemplate()]);
    renderDialog("en", true, "missing-template");
    expect(await screen.findByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) })).toHaveAttribute("aria-pressed", "true");
    expect(builtinTab()).toHaveAttribute("aria-selected", "true");
  });

  it("distinguishes cold loading, successful empty data and a cached-empty refresh failure", async () => {
    let resolve!: (templates: SkillTemplate[]) => void;
    mocks.listSkillTemplates.mockReturnValueOnce(new Promise<SkillTemplate[]>((done) => { resolve = done; }));
    const client = renderDialog("en", true);
    expect(screen.getByRole("status")).toHaveTextContent(enSkills.create.template.loading);
    expect(screen.queryByText(enSkills.create.template.empty)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: enSkills.create.template.use_template })).toBeDisabled();
    await act(async () => { resolve([]); });
    expect(await screen.findByText(enSkills.create.template.empty)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: enSkills.create.template.retry })).not.toBeInTheDocument();
    mocks.listSkillTemplates.mockRejectedValueOnce(new Error("offline"));
    await act(async () => { await client.invalidateQueries({ queryKey: workspaceKeys.skillTemplates("ws-1") }); });
    expect(await screen.findByRole("alert")).toHaveTextContent(enSkills.create.template.refresh_failed);
    expect(screen.getByText(enSkills.create.template.empty)).toBeInTheDocument();
  });
});
