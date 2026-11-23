import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { SkillWelcome } from "./skill-welcome";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("welcome-workspace") };
});

function renderWelcome(locale: "en" | "zh-Hans" = "en") {
  const adapter: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/welcome-workspace/skills",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path) => path,
  };
  const onCreate = vi.fn();
  const buttonRef = createRef<HTMLButtonElement>();
  const view = renderWithI18n(
    <NavigationProvider value={adapter}>
      <SkillWelcome onCreate={onCreate} buttonRef={buttonRef} />
    </NavigationProvider>,
    { locale },
  );
  return { ...view, adapter, onCreate, buttonRef };
}

describe("SkillWelcome", () => {
  it("opens the existing creation callback and routes docs under the workspace", async () => {
    const { adapter, onCreate, buttonRef } = renderWelcome();
    const user = userEvent.setup();
    const create = screen.getByRole("button", { name: "New skill" });
    expect(buttonRef.current).toBe(create);
    await user.click(create);
    expect(onCreate).toHaveBeenCalledOnce();
    const docs = screen.getByRole("link", { name: "Read the docs" });
    expect(docs).toHaveAttribute("href", "/welcome-workspace/docs/skills");
    await user.click(docs);
    expect(adapter.push).toHaveBeenCalledExactlyOnceWith("/welcome-workspace/docs/skills");
  });

  it("shows all five real methods as illustrative content rather than a second form", () => {
    renderWelcome();
    const scene = within(screen.getByRole("group", { name: "Five ways to add a skill" }));
    for (const method of ["Create manually", "Modify from template", "Import from local", "Import from URL", "Copy from runtime"]) {
      expect(scene.getByText(method)).toBeVisible();
    }
    expect(scene.getByText("Example")).toBeVisible();
    expect(scene.queryByRole("button")).not.toBeInTheDocument();
    expect(scene.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("selects every scene by keyboard or position and hides inactive scenes", async () => {
    renderWelcome();
    const user = userEvent.setup();
    const carousel = within(screen.getByRole("region", { name: "How skills work" }));
    const previous = carousel.getByRole("button", { name: "Previous example" });
    const next = carousel.getByRole("button", { name: "Next example" });
    expect(previous).toBeDisabled();
    expect(carousel.getByRole("group", { name: "Five ways to add a skill" })).not.toHaveAttribute("inert");
    expect(carousel.queryByRole("heading", { name: "Assign it to your agents" })).not.toBeInTheDocument();
    expect(carousel.getAllByRole("group", { hidden: true }).filter((slide) => slide.hasAttribute("inert"))).toHaveLength(2);

    next.focus();
    await user.keyboard("{Enter}");
    expect(carousel.getByRole("group", { name: "Assign it to your agents" })).not.toHaveAttribute("inert");
    expect(carousel.getByRole("status")).toHaveTextContent("Example 2 of 3: Assign it to your agents");
    expect(next).toHaveFocus();
    await user.click(next);
    expect(carousel.getByRole("group", { name: "Keep one source for future work" })).not.toHaveAttribute("inert");
    expect(next).toBeDisabled();
    expect(carousel.getByText(/future runs/)).toBeVisible();
    await user.click(previous);
    expect(carousel.getByRole("heading", { name: "Assign it to your agents" })).toBeVisible();

    const first = carousel.getByRole("button", { name: "Show example 1: Five ways to add a skill" });
    await user.click(first);
    expect(first).toHaveAttribute("aria-pressed", "true");
    expect(previous).toBeDisabled();
    expect(carousel.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });

  it("localizes the reference title, methods and carousel controls", () => {
    renderWelcome("zh-Hans");
    expect(screen.getByRole("heading", { name: "把团队的做事方法写成 skill" })).toBeVisible();
    expect(screen.getByRole("button", { name: "新建 skill" })).toBeVisible();
    expect(screen.getByRole("link", { name: "阅读文档" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "五种方式添加" })).toBeVisible();
    expect(screen.getByText("从模板中修改")).toBeVisible();
    expect(screen.getByRole("button", { name: "下一个示例" })).toBeVisible();
    expect(screen.getByRole("button", { name: "查看示例 1：五种方式添加" })).toHaveAttribute("aria-pressed", "true");
  });
});
