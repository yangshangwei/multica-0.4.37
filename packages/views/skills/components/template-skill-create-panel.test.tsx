import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { SkillTemplate } from "@multica/core/types";
import type { SupportedLocale } from "@multica/core/i18n";
import { I18nProvider } from "@multica/core/i18n/react";
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

function renderDialog(locale: TestLocale = "en") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  render(
    <I18nProvider locale={locale} resources={{ [locale]: LOCALES[locale] }}>
      <QueryClientProvider client={queryClient}>
        <CreateSkillDialog onClose={vi.fn()} onCreated={vi.fn()} />
      </QueryClientProvider>
    </I18nProvider>,
  );
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
        enSkills.builtin_role_skills[name].description,
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
