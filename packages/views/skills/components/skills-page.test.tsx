// @vitest-environment jsdom

import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import type { Label, SkillSummary, SkillTemplate } from "@multica/core/types";
import type { SupportedLocale } from "@multica/core/i18n";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";

// Regression rig for the Source cell's anchor living inside a `useRowLink`
// row: the row handles BOTH click and auxclick, so the anchor must stop both
// from bubbling. Stopping only click let a middle click reach the row, whose
// preventDefault() cancelled the anchor's native open — the skill detail page
// opened in a background tab instead of the source page.

const mocks = vi.hoisted(() => ({
  skills: [] as SkillSummary[],
  templates: [] as SkillTemplate[],
  templatesError: false,
  refetchTemplates: vi.fn(),
  viewState: {
    // The row-anchor regression rig below lives in the LIST view.
    viewMode: "list" as string,
    sortField: "name",
    sortDirection: "asc" as string,
    hiddenColumns: [] as string[],
    filters: {
      usage: [] as string[],
      categories: [] as string[],
      origins: [] as string[],
      agents: [] as string[],
      creators: [] as string[],
      labels: [] as string[],
    },
    toggleSort: vi.fn(),
    setSortField: vi.fn(),
    setSortDirection: vi.fn(),
    toggleColumn: vi.fn(),
    toggleFilter: vi.fn(),
    clearFilters: vi.fn(),
    selectCategory: vi.fn(),
    setViewMode: vi.fn(),
  },
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    if (options.queryKey?.[0] === "skill-templates") {
      return { data: mocks.templates, isLoading: false, isPending: false, isError: mocks.templatesError, refetch: mocks.refetchTemplates };
    }
    if (options.queryKey?.[0] === "skills") {
      return {
        data: mocks.skills,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      };
    }
    return { data: [], isLoading: false, error: null, refetch: vi.fn() };
  },
}));

// Render every row so the anchor under test is always mounted.
vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () =>
      Array.from({ length: count }, (_, index) => ({
        index,
        key: index,
        start: index * 48,
        end: (index + 1) * 48,
        size: 48,
      })),
    getTotalSize: () => count * 48,
  }),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ user: { id: "user-1" } }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// Partial mock: SkillIcon resolves its icon from the real WORKSPACE_PAGES, and
// the paths come from the real factory so a route this page starts linking to
// later cannot be missing from a hand-written stub.
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});

vi.mock("@multica/core/workspace/queries", () => ({
  skillListOptions: () => ({ queryKey: ["skills"] }),
  skillTemplateListOptions: () => ({ queryKey: ["skill-templates"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
  memberListOptions: () => ({ queryKey: ["members"] }),
  selectSkillAssignments: () => new Map(),
}));

vi.mock("@multica/core/runtimes", () => ({
  runtimeListOptions: () => ({ queryKey: ["runtimes"] }),
  runtimeDisplayLabel: () => "runtime",
}));

vi.mock("@multica/core/workspace/avatar-url", () => ({
  resolvePublicFileUrl: (u: string | null) => u,
}));

vi.mock("@multica/core/skills/stores", () => ({
  useSkillsViewStore: (selector: (state: unknown) => unknown) =>
    selector(mocks.viewState),
  DEFAULT_HIDDEN_COLUMNS: [],
}));

// View-layer children with heavy / portal deps — stubbed to keep the test on
// the row/anchor event wiring.
vi.mock("@multica/ui/components/common/actor-avatar", () => ({
  ActorAvatar: () => null,
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: () => null,
}));
vi.mock("./create-skill-dialog", () => ({
  CreateSkillDialog: ({
    initialTemplateName,
    initialPresentation,
  }: {
    initialTemplateName?: string;
    initialPresentation?: { category?: string };
  }) => (
    <div
      role="dialog"
      aria-label="Create skill"
      data-template-name={initialTemplateName}
      data-category={initialPresentation?.category}
    />
  ),
}));
// The toolbar keeps its real exports (originIcon & co. feed the sidebar and
// the card); only the component is replaced with a search box plus the view
// toggle so the page-level wiring stays observable.
vi.mock("./skill-list-toolbar", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./skill-list-toolbar")>();
  return {
    ...actual,
    SkillListToolbar: ({
      search,
      onSearchChange,
      viewMode,
      onViewModeChange,
    }: {
      search: string;
      onSearchChange: (value: string) => void;
      viewMode: string;
      onViewModeChange: (mode: "card" | "list") => void;
    }) => (
      <>
        <input
          aria-label="Search skills"
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
        />
        <button
          type="button"
          aria-label="Card view"
          aria-pressed={viewMode === "card"}
          onClick={() => onViewModeChange("card")}
        />
      </>
    ),
  };
});
vi.mock("./skill-list-actions", () => ({
  SkillBatchToolbar: () => null,
  SkillRowActions: () => null,
}));

import SkillsPage from "./skills-page";

const SOURCE_URL = "https://github.com/anthropics/skills/tree/main/animations";

const importedSkill: SkillSummary = {
  id: "skill-1",
  workspace_id: "ws-1",
  name: "animations",
  description: "",
  config: { origin: { type: "github", source_url: SOURCE_URL } },
  created_by: "user-1",
  created_at: "2026-07-28T18:11:37Z",
  updated_at: "2026-07-28T18:14:40Z",
};

function makeAdapter(
  overrides: Partial<NavigationAdapter> = {},
): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/skills",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
    openInNewTab: vi.fn(),
    ...overrides,
  };
}

function renderPage(adapter: NavigationAdapter, locale: SupportedLocale = "en") {
  return renderWithI18n(
    <NavigationProvider value={adapter}>
      <SkillsPage />
    </NavigationProvider>,
    { locale },
  );
}

function middleClick(target: Element): MouseEvent {
  const event = new MouseEvent("auxclick", {
    bubbles: true,
    button: 1,
    cancelable: true,
  });
  target.dispatchEvent(event);
  return event;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.skills = [importedSkill];
  mocks.templates = [];
  mocks.templatesError = false;
  mocks.viewState.viewMode = "list";
  mocks.viewState.filters.categories = [];
  mocks.viewState.filters.labels = [];
});

const REVIEW_TEMPLATE: SkillTemplate = {
  name: "multica-code-review", version: 1,
  description: "Review changes and report actionable findings.",
  content: "---\nname: multica-code-review\n---\nReview changes.", files: [],
};

describe("SkillsPage built-in catalog", () => {
  it("keeps templates collapsed in an empty workspace and opens a preselected copy on request", () => {
    mocks.skills = [];
    mocks.templates = [REVIEW_TEMPLATE];
    renderPage(makeAdapter());

    const catalog = screen.getByRole("region", { name: "Built-in skills" });
    const toggle = within(catalog).getByRole("button", { name: "Built-in skills 1" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(within(catalog).queryByRole("button", { name: "View template" })).not.toBeInTheDocument();
    fireEvent.click(toggle);
    expect(within(catalog).getByText(REVIEW_TEMPLATE.description)).toBeInTheDocument();
    expect(within(screen.getByRole("heading", { level: 1 }).parentElement!).queryByText("1")).not.toBeInTheDocument();
    fireEvent.click(within(catalog).getByRole("button", { name: "View template" }));
    expect(screen.getByRole("dialog", { name: "Create skill" })).toHaveAttribute("data-template-name", REVIEW_TEMPLATE.name);
  });

  it("opens a provenance-backed copy and keeps the catalog outside instance search", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    mocks.skills = [
      { ...importedSkill, id: "same-name", name: REVIEW_TEMPLATE.name, config: {} },
      { ...importedSkill, id: "copy-id", name: "Payments checks", config: { template_source: { name: REVIEW_TEMPLATE.name, version: 1 } } },
    ];
    const adapter = makeAdapter();
    renderPage(adapter);
    fireEvent.change(screen.getByRole("textbox", { name: "Search skills" }), { target: { value: "nothing matches" } });

    const catalog = screen.getByRole("region", { name: "Built-in skills" });
    fireEvent.click(within(catalog).getByRole("button", { name: /^Built-in skills/ }));
    expect(within(catalog).getByText(REVIEW_TEMPLATE.name)).toBeInTheDocument();
    fireEvent.click(within(catalog).getByRole("button", { name: "Open skill" }));
    expect(adapter.push).toHaveBeenCalledWith("/acme/skills/copy-id");
  });

  it("does not infer an instance from its name and offers catalog retry without hiding the custom list", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    mocks.skills = [{ ...importedSkill, name: REVIEW_TEMPLATE.name, config: {} }];
    const adapter = makeAdapter();
    const view = renderPage(adapter);
    fireEvent.click(screen.getByRole("button", { name: /^Built-in skills/ }));
    expect(within(screen.getByRole("region", { name: "Built-in skills" })).queryByRole("button", { name: "Open skill" })).not.toBeInTheDocument();

    mocks.templatesError = true;
    view.rerender(<NavigationProvider value={adapter}><SkillsPage /></NavigationProvider>);
    fireEvent.click(within(screen.getByRole("region", { name: "Built-in skills" })).getByRole("button", { name: "Retry" }));
    expect(mocks.refetchTemplates).toHaveBeenCalledOnce();
    expect(screen.getByRole("textbox", { name: "Search skills" })).toBeInTheDocument();
  });
});

describe("SkillsPage source link vs row navigation", () => {
  it("middle click on the source link keeps its native open (row must not hijack)", async () => {
    const adapter = makeAdapter();
    renderPage(adapter);

    const link = await screen.findByRole("link", { name: "From GitHub" });
    expect(link.getAttribute("href")).toBe(SOURCE_URL);

    const event = middleClick(link);

    // The anchor's native middle-click open must survive: the row's auxclick
    // handler (which calls preventDefault) must never see the event.
    expect(event.defaultPrevented).toBe(false);
    expect(adapter.openInNewTab).not.toHaveBeenCalled();
    expect(adapter.push).not.toHaveBeenCalled();
  });

  it("left click on the source link does not trigger row navigation", async () => {
    const adapter = makeAdapter();
    renderPage(adapter);

    fireEvent.click(
      await screen.findByRole("link", { name: "From GitHub" }),
    );

    expect(adapter.push).not.toHaveBeenCalled();
    expect(adapter.openInNewTab).not.toHaveBeenCalled();
  });

  // Sanity check that the rig exercises the row's auxclick handler at all —
  // without this, the assertions above could pass vacuously.
  it("middle click elsewhere on the row still background-tabs the skill", async () => {
    const adapter = makeAdapter();
    renderPage(adapter);

    const event = middleClick(await screen.findByText("animations"));

    expect(event.defaultPrevented).toBe(true);
    expect(adapter.openInNewTab).toHaveBeenCalledWith(
      "/acme/skills/skill-1",
      "animations",
    );
  });
});

describe("SkillsPage built-in skill presentation", () => {
  const builtInSkill: SkillSummary = {
    ...importedSkill,
    name: "multica-code-review",
    description:
      "Use when reviewing a diff: what to look for, how to state a finding so it is actionable, and what not to report.",
    config: {
      origin: { type: "builtin_role_skill", name: "multica-code-review", version: 1 },
    },
  };

  it("displays and searches the two historical defaults from existing workspace copies", async () => {
    mocks.skills = [
      {
        ...importedSkill,
        name: "multica-release-check",
        description: "Use before any release or production-affecting action: the gate, the rollback, and the approval request that must precede execution.",
        config: { origin: { type: "builtin_role_skill", name: "multica-release-check", version: 1 } },
      },
      {
        ...importedSkill,
        id: "skill-2",
        name: "multica-architecture-decision-record",
        description: "Use when a technical decision will constrain later work: writes an ADR with context, the decision, the rejected alternatives and the consequences.",
        config: { origin: { type: "builtin_role_skill", name: "multica-architecture-decision-record", version: 1 } },
      },
    ];
    renderPage(makeAdapter(), "zh-Hans");
    expect(await screen.findByText("发布检查")).toBeInTheDocument();
    expect(screen.getByText(/完成必要检查、准备回滚方案/)).toBeInTheDocument();
    expect(screen.getByText(/编写 ADR，记录背景、决策/)).toBeInTheDocument();

    const search = screen.getByRole("textbox", { name: "Search skills" });
    fireEvent.change(search, { target: { value: "申请审批" } });
    expect(screen.getByText("发布检查")).toBeInTheDocument();
    expect(screen.queryByText("架构决策记录")).not.toBeInTheDocument();
    fireEvent.change(search, { target: { value: "rejected alternatives" } });
    expect(screen.getByText("架构决策记录")).toBeInTheDocument();
    expect(screen.queryByText("发布检查")).not.toBeInTheDocument();
  });

  it("shows the Chinese name and purpose while navigation keeps the skill ID", async () => {
    mocks.skills = [builtInSkill];
    const adapter = makeAdapter();
    renderPage(adapter, "zh-Hans");

    const name = await screen.findByText("代码审查");
    expect(screen.getByText(/围绕正确性、兼容性和失败场景/)).toBeInTheDocument();
    middleClick(name);
    expect(adapter.openInNewTab).toHaveBeenCalledWith(
      "/acme/skills/skill-1",
      "代码审查",
    );
  });

  it("finds the same row by Chinese purpose and English identifier", async () => {
    mocks.skills = [builtInSkill, { ...importedSkill, id: "skill-2" }];
    renderPage(makeAdapter(), "zh-Hans");
    const search = screen.getByRole("textbox", { name: "Search skills" });

    fireEvent.change(search, { target: { value: "兼容性" } });
    expect(await screen.findByText("代码审查")).toBeInTheDocument();
    expect(screen.queryByText("animations")).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: "MULTICA-CODE-REVIEW" } });
    expect(screen.getByText("代码审查")).toBeInTheDocument();
    expect(screen.queryByText("animations")).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: "unmatched phrase" } });
    expect(screen.queryByText("代码审查")).not.toBeInTheDocument();
  });

  it("supports Chinese search while reading the English interface", async () => {
    mocks.skills = [builtInSkill];
    renderPage(makeAdapter());
    fireEvent.change(screen.getByRole("textbox", { name: "Search skills" }), {
      target: { value: "代码审查" },
    });
    expect(await screen.findByText("multica-code-review")).toBeInTheDocument();
  });
});

// Category / view-mode wiring. Filter predicate and count matrices are covered
// in @multica/core (presentation.test.ts, view-store.test.ts) and
// use-skill-list-facets.test.ts; this suite keeps the page-level wiring.
describe("SkillsPage categories and view mode", () => {
  const qualityLabel: Label = {
    id: "lbl-quality",
    workspace_id: "ws-1",
    resource_type: "skill",
    name: "quality",
    color: "#22c55e",
    created_at: "2026-07-28T18:11:37Z",
    updated_at: "2026-07-28T18:11:37Z",
  };
  const engineeringSkill: SkillSummary = {
    ...importedSkill,
    id: "skill-eng",
    name: "lint-fixer",
    config: { presentation: { category: "engineering" } },
    labels: [qualityLabel],
  };

  it("narrows the list to the selected category and counts it in the sidebar", () => {
    mocks.skills = [importedSkill, engineeringSkill];
    mocks.viewState.filters.categories = ["engineering"];
    renderPage(makeAdapter());

    expect(screen.getByText("lint-fixer")).toBeInTheDocument();
    expect(screen.queryByText("animations")).not.toBeInTheDocument();

    const sidebar = screen.getByRole("navigation", { name: "Categories" });
    const engineering = within(sidebar).getByRole("button", { name: /Engineering/ });
    expect(engineering).toHaveAttribute("data-active");
    expect(engineering).toHaveTextContent("1");
    expect(within(sidebar).getByRole("button", { name: /^All/ })).toHaveTextContent("2");

    fireEvent.click(within(sidebar).getByRole("button", { name: /Writing/ }));
    expect(mocks.viewState.selectCategory).toHaveBeenCalledWith("writing");
  });

  it("renders the category and labels columns with LabelChip in list view", () => {
    mocks.skills = [importedSkill, engineeringSkill];
    renderPage(makeAdapter());
    expect(screen.getByRole("columnheader", { name: /Category/ })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: /Labels/ })).toBeInTheDocument();
    // The label renders as a colored chip carrying the workspace label color.
    const chip = screen.getByTitle("quality");
    expect(chip).toHaveStyle({ backgroundColor: "#22c55e" });
    // A skill without labels (older server shape) renders no chip.
    expect(screen.getAllByTitle("quality")).toHaveLength(1);
  });

  it("matches the search against label names", () => {
    mocks.skills = [importedSkill, engineeringSkill];
    renderPage(makeAdapter());
    fireEvent.change(screen.getByRole("textbox", { name: "Search skills" }), {
      target: { value: "quality" },
    });
    expect(screen.getByText("lint-fixer")).toBeInTheDocument();
    expect(screen.queryByText("animations")).not.toBeInTheDocument();
  });

  it("keeps only skills carrying a selected label filter", () => {
    mocks.skills = [importedSkill, engineeringSkill];
    mocks.viewState.filters.labels = ["lbl-quality"];
    renderPage(makeAdapter());
    expect(screen.getByText("lint-fixer")).toBeInTheDocument();
    expect(screen.queryByText("animations")).not.toBeInTheDocument();
  });

  it("routes the view toggle to the store and renders cards in card mode", () => {
    renderPage(makeAdapter());
    fireEvent.click(screen.getByRole("button", { name: "Card view" }));
    expect(mocks.viewState.setViewMode).toHaveBeenCalledWith("card");

    mocks.viewState.viewMode = "card";
    mocks.skills = [importedSkill];
    renderPage(makeAdapter());
    expect(screen.getAllByTestId("skill-card").length).toBeGreaterThan(0);
  });

  it("shows the category empty state and pre-fills the create dialog", () => {
    mocks.skills = [importedSkill];
    mocks.viewState.filters.categories = ["data"];
    renderPage(makeAdapter());

    const empty = screen.getByText(/No skills in "Data" yet/).closest("[data-slot=empty]")!;
    fireEvent.click(within(empty as HTMLElement).getByRole("button", { name: "New skill" }));
    expect(screen.getByRole("dialog", { name: "Create skill" })).toHaveAttribute(
      "data-category",
      "data",
    );
  });

  it("shows plain no-matches, not the category empty state, when a search narrows a populated category", () => {
    mocks.skills = [importedSkill, engineeringSkill];
    mocks.viewState.filters.categories = ["engineering"];
    renderPage(makeAdapter());
    fireEvent.change(screen.getByRole("textbox", { name: "Search skills" }), {
      target: { value: "nothing matches" },
    });
    expect(screen.getByText("No matches")).toBeInTheDocument();
    expect(screen.queryByText(/No skills in "Engineering" yet/)).not.toBeInTheDocument();
  });

  it("renders the no-matches state inside the card grid too", () => {
    mocks.viewState.viewMode = "card";
    mocks.skills = [importedSkill];
    renderPage(makeAdapter());
    fireEvent.change(screen.getByRole("textbox", { name: "Search skills" }), {
      target: { value: "nothing matches" },
    });
    expect(screen.queryAllByTestId("skill-card")).toHaveLength(0);
    expect(screen.getByText("No matches")).toBeInTheDocument();
  });
});
