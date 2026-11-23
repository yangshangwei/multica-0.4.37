// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enLayout from "../locales/en/layout.json";
import zhLayout from "../locales/zh-Hans/layout.json";
import { MarketplacePage } from "./marketplace-page";

const state = vi.hoisted(() => ({
  workspaceId: "workspace-1",
  pathname: "/acme/marketplace",
  searchParams: new URLSearchParams(),
  hash: "",
  push: vi.fn(),
}));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => state.workspaceId }));
vi.mock("../navigation", async (original) => ({
  ...await original<typeof import("../navigation")>(),
  useNavigation: () => state,
}));
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

function Page({ locale = "en" }: { locale?: "en" | "zh-Hans" }) {
  return (
    <I18nProvider locale={locale} resources={{ en: { layout: enLayout }, "zh-Hans": { layout: zhLayout } }}>
      <MarketplacePage />
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

describe("MarketplacePage", () => {
  it.each([
    ["", "MCP"],
    ["type=unknown", "MCP"],
    ["type=mcp", "MCP"],
    ["type=skills", "Skills"],
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
  });
});
