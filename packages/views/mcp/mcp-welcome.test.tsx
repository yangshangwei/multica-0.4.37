import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";
import { McpWelcome } from "./mcp-welcome";

function renderWelcome(locale: "en" | "zh-Hans" = "en", canCreate = true) {
  const onBrowse = vi.fn();
  const onCreate = vi.fn();
  const createButtonRef = createRef<HTMLButtonElement>();
  const view = renderWithI18n(
    <McpWelcome onBrowse={onBrowse} onCreate={canCreate ? onCreate : undefined} createButtonRef={createButtonRef} />,
    { locale },
  );
  return { ...view, onBrowse, onCreate, createButtonRef };
}

describe("McpWelcome", () => {
  it("uses existing action callbacks and exposes the connected creation opener", async () => {
    const user = userEvent.setup();
    const { onBrowse, onCreate, createButtonRef } = renderWelcome();
    await user.click(screen.getByRole("button", { name: "Browse capability market" }));
    expect(onBrowse).toHaveBeenCalledOnce();
    const create = screen.getByRole("button", { name: "Add custom server" });
    expect(createButtonRef.current).toBe(create);
    await user.click(create);
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("makes the three workflow scenes illustrative and manually operable", async () => {
    const user = userEvent.setup();
    const { onBrowse, onCreate } = renderWelcome();
    const carousel = within(screen.getByRole("region", { name: "How MCP works" }));
    const previous = carousel.getByRole("button", { name: "Previous example" });
    const next = carousel.getByRole("button", { name: "Next example" });
    expect(previous).toBeDisabled();
    const firstScene = within(carousel.getByRole("group", { name: "Choose a service" }));
    for (const service of ["Playwright", "Postgres MCP", "MarkItDown"]) {
      expect(firstScene.getByText(service)).toBeVisible();
    }
    expect(firstScene.getByText("Example")).toBeVisible();
    expect(firstScene.queryByRole("button")).not.toBeInTheDocument();
    expect(firstScene.queryByRole("textbox")).not.toBeInTheDocument();
    expect(carousel.getAllByRole("group", { hidden: true }).filter((scene) => scene.hasAttribute("inert"))).toHaveLength(2);

    next.focus();
    await user.keyboard("{Enter}");
    const configuration = carousel.getByRole("group", { name: "Configure the connection" });
    expect(configuration).not.toHaveAttribute("inert");
    expect(within(configuration).getByText("npx")).toBeVisible();
    expect(within(configuration).queryByRole("textbox")).not.toBeInTheDocument();
    expect(carousel.getByRole("status")).toHaveTextContent("Example 2 of 3: Configure the connection");
    expect(next).toHaveFocus();
    await user.click(next);
    expect(carousel.getByRole("group", { name: "Assign it to your agents" })).not.toHaveAttribute("inert");
    expect(next).toBeDisabled();
    expect(carousel.getByText(/never assigns it automatically/)).toBeVisible();
    await user.click(previous);
    expect(carousel.getByRole("heading", { name: "Configure the connection" })).toBeVisible();
    const first = carousel.getByRole("button", { name: "Show example 1: Choose a service" });
    await user.click(first);
    expect(first).toHaveAttribute("aria-pressed", "true");
    expect(previous).toBeDisabled();
    expect(first).toHaveAttribute("aria-controls");
    expect(carousel.getByRole("status")).toHaveAttribute("aria-live", "polite");
    expect(onBrowse).not.toHaveBeenCalled();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("localizes the introduction, scenes and carousel controls", async () => {
    const user = userEvent.setup();
    renderWelcome("zh-Hans");
    expect(screen.getByRole("heading", { name: "让智能体连接你的工具" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "选择需要的服务" })).toBeVisible();
    expect(screen.getByRole("button", { name: "查看示例 1：选择需要的服务" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "下一个示例" }));
    expect(screen.getByRole("heading", { name: "填写连接配置" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "下一个示例" }));
    expect(screen.getByRole("heading", { name: "分配给智能体" })).toBeVisible();
    expect(screen.getByText("添加配置后不会自动分配。", { exact: false })).toBeVisible();
  });

  it("explains member permissions without exposing a custom configuration action", () => {
    const { createButtonRef } = renderWelcome("en", false);
    expect(screen.getByRole("button", { name: "Browse capability market" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Add custom server" })).not.toBeInTheDocument();
    expect(createButtonRef.current).toBeNull();
    expect(screen.getByText("Ask a workspace owner or admin to add and assign MCP servers.")).toBeVisible();
  });
});
