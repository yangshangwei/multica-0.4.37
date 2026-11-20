import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { AgentWelcome } from "./agent-welcome";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("welcome-workspace") };
});

function renderWelcome(locale: "en" | "zh-Hans" = "en") {
  const adapter: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/welcome-workspace/agents",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path) => path,
  };
  const view = renderWithI18n(
    <NavigationProvider value={adapter}><AgentWelcome runtimeConnected={false} /></NavigationProvider>,
    { locale },
  );
  return { ...view, adapter };
}

describe("AgentWelcome", () => {
  it("routes the three entry points through the current workspace", async () => {
    const { adapter } = renderWelcome();
    const user = userEvent.setup();
    const entries = [
      ["New agent", "/welcome-workspace/agents/new"],
      ["Read the docs", "/welcome-workspace/docs/agents"],
      ["Connect runtime", "/welcome-workspace/runtimes"],
    ] as const;
    for (const [name, href] of entries) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      await user.click(link);
      expect(adapter.push).toHaveBeenLastCalledWith(href);
    }
  });

  it("supports keyboard and position controls while exposing only the current illustrative slide", async () => {
    renderWelcome();
    const user = userEvent.setup();
    const carousel = within(screen.getByRole("region", { name: "How agents work" }));
    const previous = carousel.getByRole("button", { name: "Previous example" });
    const next = carousel.getByRole("button", { name: "Next example" });

    expect(previous).toBeDisabled();
    expect(carousel.getByRole("group", { name: "Describe what it should do" })).not.toHaveAttribute("inert");
    expect(carousel.queryByRole("heading", { name: "Assign work like a teammate" })).not.toBeInTheDocument();
    expect(carousel.getAllByRole("group", { hidden: true }).filter((slide) => slide.hasAttribute("inert"))).toHaveLength(2);

    next.focus();
    await user.keyboard("{Enter}");
    expect(carousel.getByRole("heading", { name: "Assign work like a teammate" })).toBeInTheDocument();
    expect(carousel.getByRole("status")).toHaveTextContent("Example 2 of 3: Assign work like a teammate");
    expect(next).toHaveFocus();
    await user.click(next);
    expect(carousel.getByRole("heading", { name: "Find the result in the comments" })).toBeInTheDocument();
    expect(next).toBeDisabled();
    await user.click(previous);
    expect(carousel.getByRole("heading", { name: "Assign work like a teammate" })).toBeInTheDocument();

    const first = carousel.getByRole("button", { name: "Show example 1: Describe what it should do" });
    await user.click(first);
    expect(first).toHaveAttribute("aria-pressed", "true");
    expect(previous).toBeDisabled();
    expect(carousel.getByRole("group", { name: "Describe what it should do" })).toHaveTextContent("Example");
    expect(carousel.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("localizes the reference headline, checklist and controls in Chinese", () => {
    renderWelcome("zh-Hans");
    expect(screen.getByRole("heading", { name: "招第一位 AI 队友" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "新建智能体" })).toBeInTheDocument();
    expect(screen.getByText("接入运行时")).toBeInTheDocument();
    expect(screen.getByText("创建智能体")).toBeInTheDocument();
    expect(screen.getByText("给它分配一个任务")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下一个示例" })).toBeInTheDocument();
  });
});
