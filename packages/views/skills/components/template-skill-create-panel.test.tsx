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

describe("TemplateSkillCreatePanel source grouping", () => {
  it("splits built-in and mounted templates into two labelled groups with a count", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    renderDialog();
    await openTemplates();

    expect(screen.getByText(enSkills.create.template.group_builtin)).toBeInTheDocument();
    expect(screen.getByText("Provided by this deployment (1)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toBeInTheDocument();
    // Inline source guidance, queryable by the info trigger's label.
    expect(
      screen.getByRole("button", { name: enSkills.create.template.group_deployment_info }),
    ).toBeInTheDocument();
    // No mounted templates missing => no empty-state promo.
    expect(screen.queryByText(enSkills.create.template.deployment_empty_hint)).not.toBeInTheDocument();
  });

  it("shows only the built-in group and a persistent promo when nothing is mounted", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate()]);
    renderDialog();
    await openTemplates();

    expect(screen.getByText(enSkills.create.template.group_builtin)).toBeInTheDocument();
    // The deployment section (and its info trigger) must be absent, not just its label text.
    expect(
      screen.queryByRole("button", { name: enSkills.create.template.group_deployment_info }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(enSkills.create.template.deployment_empty_hint)).toBeInTheDocument();
  });

  it("keeps search filtering across groups and preserves selection", async () => {
    mocks.listSkillTemplates.mockResolvedValue([builtinTemplate(), mountedTemplate()]);
    renderDialog();
    await openTemplates();

    // Selecting the mounted row still previews it and marks it pressed.
    const mountedRow = screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) });
    fireEvent.click(mountedRow);
    await waitFor(() => expect(mountedRow).toHaveAttribute("aria-pressed", "true"));

    // Searching for the mounted template hides the built-in group entirely.
    fireEvent.change(screen.getByRole("textbox", { name: enSkills.create.template.search_placeholder }), {
      target: { value: "house code style" },
    });
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: new RegExp(`^${BUILTIN_NAME}`) })).not.toBeInTheDocument(),
    );
    expect(screen.queryByText(enSkills.create.template.group_builtin)).not.toBeInTheDocument();
    expect(screen.getByText("Provided by this deployment (1)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: new RegExp(`^${MOUNTED_NAME}`) })).toBeInTheDocument();
  });
});
