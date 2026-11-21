// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import enLayout from "../locales/en/layout.json";
import zhLayout from "../locales/zh-Hans/layout.json";
import { NavigationProvider } from "../navigation";
import { MarketplacePage } from "./marketplace-page";

const state = vi.hoisted(() => ({
  workspaceId: "workspace-1",
  pathname: "/acme/marketplace",
  searchParams: new URLSearchParams(),
  hash: "",
  push: vi.fn(),
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => state.workspaceId }));
vi.mock("../mcp", () => ({
  McpMarketPanel: ({ workspaceId, active }: { workspaceId: string; active: boolean }) => (
    <div data-testid="mcp-panel" data-workspace={workspaceId} data-active={active}>
      <input aria-label="MCP draft" defaultValue="" />
    </div>
  ),
}));
vi.mock("../skills", () => ({
  SkillMarketPanel: ({ workspaceId, active }: { workspaceId: string; active: boolean }) => (
    <div data-testid="skills-panel" data-workspace={workspaceId} data-active={active}>
      <input aria-label="Skill draft" defaultValue="" />
    </div>
  ),
}));
vi.mock("./squad-templates-market-panel", () => ({
  SquadTemplatesMarketPanel: ({ workspaceId, active }: { workspaceId: string; active: boolean }) => (
    <div data-testid="squad-templates-panel" data-workspace={workspaceId} data-active={active} />
  ),
}));
vi.mock("./agent-templates-market-panel", () => ({
  AgentTemplatesMarketPanel: ({ workspaceId, active }: { workspaceId: string; active: boolean }) => (
    <div data-testid="agent-templates-panel" data-workspace={workspaceId} data-active={active} />
  ),
}));

function Page({ locale = "en" }: { locale?: "en" | "zh-Hans" }) {
  const slug = state.pathname.split("/")[1]!;
  return (
    <I18nProvider locale={locale} resources={{ en: { layout: enLayout }, "zh-Hans": { layout: zhLayout } }}>
      <WorkspaceSlugProvider slug={slug}>
        <NavigationProvider value={{ ...state, replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path }}>
          <MarketplacePage />
        </NavigationProvider>
      </WorkspaceSlugProvider>
    </I18nProvider>
  );
}

beforeEach(() => {
  state.workspaceId = "workspace-1";
  state.pathname = "/acme/marketplace";
  state.searchParams = new URLSearchParams();
  state.hash = "";
  state.push.mockClear();
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("MarketplacePage", () => {
  it.each([
    ["", "MCP"],
    ["type=unknown", "MCP"],
    ["type=mcp", "MCP"],
    ["type=skills", "Skills"],
    ["type=squad-templates", "Squad templates"],
    ["type=agent-templates", "Agent templates"],
  ])("restores the active catalog from %s", (search, label) => {
    state.searchParams = new URLSearchParams(search);
    render(<Page />);
    expect(screen.getByRole("heading", { name: "Capability market" })).toBeVisible();
    expect(screen.getByRole("tab", { name: label })).toHaveAttribute("aria-selected", "true");
    expect(screen.getAllByRole("tabpanel")).toHaveLength(1);
    expect(state.push).not.toHaveBeenCalled();
  });

  // Domain suites own real draft, portal, polling, permissions and API recovery.
  // This test owns their retained mount/active contract across URL history changes.
  it("preserves visited drafts and deactivates hidden catalogs through type switches and history", async () => {
    state.searchParams = new URLSearchParams("source=deployment");
    state.hash = "#catalog";
    const user = userEvent.setup();
    const { rerender } = render(<Page />);
    const mcpDraft = screen.getByRole("textbox", { name: "MCP draft" });
    expect(screen.queryByTestId("skills-panel")).toBeNull();
    await user.type(mcpDraft, "private input");
    await user.click(screen.getByRole("tab", { name: "Skills" }));
    expect(state.push).toHaveBeenLastCalledWith("/acme/marketplace?source=deployment&type=skills#catalog");
    // Selection follows the committed URL, not the click's optimistic state.
    expect(screen.getByRole("tab", { name: "MCP" })).toHaveAttribute("aria-selected", "true");
    state.searchParams = new URLSearchParams("source=deployment&type=skills");
    rerender(<Page />);
    expect(mcpDraft).not.toBeVisible();
    expect(screen.getByTestId("mcp-panel")).toHaveAttribute("data-active", "false");
    expect(mcpDraft.closest('[role="tabpanel"]')).toHaveAttribute("inert");
    expect(screen.getByTestId("skills-panel")).toHaveAttribute("data-active", "true");
    const skillDraft = screen.getByRole("textbox", { name: "Skill draft" });
    await user.type(skillDraft, "skill instructions");

    state.searchParams = new URLSearchParams("source=deployment");
    rerender(<Page />);
    expect(screen.getByRole("textbox", { name: "MCP draft" })).toBe(mcpDraft);
    expect(mcpDraft).toHaveValue("private input");
    expect(skillDraft).not.toBeVisible();
    expect(screen.getByTestId("skills-panel")).toHaveAttribute("data-active", "false");

    state.searchParams = new URLSearchParams("type=skills");
    rerender(<Page />);
    expect(screen.getByRole("textbox", { name: "Skill draft" })).toBe(skillDraft);
    expect(skillDraft).toHaveValue("skill instructions");
  });

  it("retains the squad-template panel while switching and preserves location context", async () => {
    state.searchParams = new URLSearchParams("source=deployment");
    state.hash = "#catalog";
    const user = userEvent.setup();
    const { rerender } = render(<Page />);

    expect(screen.queryByTestId("squad-templates-panel")).toBeNull();
    await user.click(screen.getByRole("tab", { name: "Squad templates" }));
    expect(state.push).toHaveBeenLastCalledWith("/acme/marketplace?source=deployment&type=squad-templates#catalog");

    state.searchParams = new URLSearchParams("source=deployment&type=squad-templates");
    rerender(<Page />);
    const panel = screen.getByTestId("squad-templates-panel");
    expect(panel).toHaveAttribute("data-active", "true");

    state.searchParams = new URLSearchParams("source=deployment&type=skills");
    rerender(<Page />);
    expect(panel).toBeInTheDocument();
    expect(panel).toHaveAttribute("data-active", "false");
    expect(panel.closest('[role="tabpanel"]')).toHaveAttribute("inert");

    state.searchParams = new URLSearchParams("source=deployment&type=squad-templates");
    rerender(<Page />);
    expect(screen.getByTestId("squad-templates-panel")).toBe(panel);
    expect(panel).toHaveAttribute("data-active", "true");
  });

  it("activates a focused type with the keyboard", async () => {
    const user = userEvent.setup();
    render(<Page />);
    screen.getByRole("tab", { name: "MCP" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "Skills" })).toHaveFocus();
    expect(state.push).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(state.push).toHaveBeenCalledWith("/acme/marketplace?type=skills");
  });

  it("keeps the selected type in view on URL changes and strip resize, and releases its observer", () => {
    const scroll = vi.spyOn(HTMLElement.prototype, "scrollIntoView");
    const observers: Array<{ notify: () => void; targets: Element[]; disconnect: ReturnType<typeof vi.fn> }> = [];
    vi.stubGlobal("ResizeObserver", class {
      private entry;
      constructor(callback: () => void) {
        this.entry = { notify: callback, targets: [] as Element[], disconnect: vi.fn() };
        observers.push(this.entry);
      }
      observe = (target: Element) => { this.entry.targets.push(target); };
      disconnect = () => { this.entry.disconnect(); };
    });
    const { rerender, unmount } = render(<Page />);
    const strip = screen.getByRole("tablist").parentElement;
    const initialObserver = observers.find((observer) => observer.targets.includes(strip!));
    expect(initialObserver).toBeDefined();
    expect(scroll.mock.contexts.at(-1)).toBe(screen.getByRole("tab", { name: "MCP" }));
    expect(scroll).toHaveBeenLastCalledWith({ block: "nearest", inline: "nearest" });

    state.searchParams = new URLSearchParams("type=agent-templates");
    rerender(<Page />);
    const selected = screen.getByRole("tab", { name: "Agent templates" });
    expect(scroll.mock.contexts.at(-1)).toBe(selected);
    expect(initialObserver?.disconnect).toHaveBeenCalledOnce();
    scroll.mockClear();
    const activeObserver = observers.find((observer) => observer.targets.includes(strip!) && observer.disconnect.mock.calls.length === 0);
    expect(activeObserver).toBeDefined();
    act(() => activeObserver?.notify());
    expect(scroll.mock.contexts.at(-1)).toBe(selected);
    expect(scroll).toHaveBeenLastCalledWith({ block: "nearest", inline: "nearest" });
    unmount();
    expect(activeObserver?.disconnect).toHaveBeenCalledOnce();
  });

  it("lazily retains agent templates through history and resets them with the workspace", async () => {
    state.searchParams = new URLSearchParams("context=keep");
    state.hash = "#catalog";
    const { rerender } = render(<Page />);
    expect(screen.queryByTestId("agent-templates-panel")).not.toBeInTheDocument();
    const squadTab = screen.getByRole("tab", { name: "Squad templates" });
    squadTab.focus();
    await userEvent.setup().keyboard("{ArrowRight}{Enter}");
    expect(state.push).toHaveBeenLastCalledWith("/acme/marketplace?context=keep&type=agent-templates#catalog");

    state.searchParams = new URLSearchParams("context=keep&type=agent-templates");
    rerender(<Page />);
    const panel = screen.getByTestId("agent-templates-panel");
    expect(panel).toHaveAttribute("data-active", "true");
    state.searchParams = new URLSearchParams("type=skills");
    rerender(<Page />);
    expect(panel).toBeInTheDocument();
    expect(panel).not.toBeVisible();
    expect(panel).toHaveAttribute("data-active", "false");
    expect(panel.closest('[role="tabpanel"]')).toHaveAttribute("inert");
    state.searchParams = new URLSearchParams("type=agent-templates");
    rerender(<Page />);
    expect(screen.getByTestId("agent-templates-panel")).toBe(panel);

    state.workspaceId = "workspace-2";
    state.pathname = "/other/marketplace";
    rerender(<Page />);
    expect(panel).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-templates-panel")).toHaveAttribute("data-workspace", "workspace-2");
  });

  it("reaches the squad-template category with tab keyboard navigation", async () => {
    state.searchParams = new URLSearchParams("type=skills");
    const user = userEvent.setup();
    render(<Page />);
    const skills = screen.getByRole("tab", { name: "Skills" });
    skills.focus();

    await user.keyboard("{ArrowRight}");
    const squadTemplates = screen.getByRole("tab", { name: "Squad templates" });
    expect(squadTemplates).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(state.push).toHaveBeenCalledWith("/acme/marketplace?type=squad-templates");
  });

  it("resets retained panels when workspace identity changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Page />);
    const oldDraft = screen.getByRole("textbox", { name: "MCP draft" });
    await user.type(oldDraft, "workspace one input");
    state.workspaceId = "workspace-2";
    state.pathname = "/other/marketplace";
    rerender(<Page />);
    expect(screen.getByTestId("mcp-panel")).toHaveAttribute("data-workspace", "workspace-2");
    expect(screen.getByRole("textbox", { name: "MCP draft" })).toHaveValue("");
    expect(oldDraft).not.toBeInTheDocument();
  });

  it("uses the Chinese market name while preserving MCP and Skills terminology", () => {
    render(<Page locale="zh-Hans" />);
    expect(screen.getByRole("heading", { name: "能力市场" })).toBeVisible();
    expect(screen.getByRole("tablist", { name: "能力类型" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "MCP" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "Skills" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "AI小队模板" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "智能体模板" })).toBeVisible();
  });
});
