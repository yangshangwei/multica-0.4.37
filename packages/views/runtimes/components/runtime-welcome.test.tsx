import { afterEach, describe, expect, it, vi } from "vitest";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configStore } from "@multica/core/config";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { RuntimeWelcome } from "./runtime-welcome";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspaceSlug: () => "welcome-workspace" };
});

function renderWelcome(locale: "en" | "zh-Hans" = "en") {
  const onConnectRemote = vi.fn();
  const adapter: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(),
    pathname: "/welcome-workspace/runtimes", searchParams: new URLSearchParams(),
    hash: "", getShareableUrl: (path) => path,
  };
  const view = renderWithI18n(
    <NavigationProvider value={adapter}>
      <RuntimeWelcome onConnectRemote={onConnectRemote} />
    </NavigationProvider>,
    { locale },
  );
  return { ...view, onConnectRemote, adapter };
}

describe("RuntimeWelcome", () => {
  afterEach(() => configStore.setState(configStore.getInitialState()));

  it("connects through the existing action and routes help to the workspace guide", async () => {
    const { onConnectRemote, adapter } = renderWelcome();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Add runtime" }));
    expect(onConnectRemote).toHaveBeenCalledOnce();
    const guide = screen.getByRole("link", { name: "Read the connection guide" });
    expect(guide).toHaveAttribute("href", "/welcome-workspace/docs/daemon-runtimes");
    await user.click(guide);
    expect(adapter.push).toHaveBeenCalledWith("/welcome-workspace/docs/daemon-runtimes");
  });

  it("uses the operator-configured desktop download and reacts when deployment config changes", () => {
    renderWelcome();
    expect(screen.queryByRole("link", { name: "Download Desktop" })).not.toBeInTheDocument();
    act(() => configStore.getState().setDesktopDownloadUrl("http://10.10.10.20:18080/desktop/"));
    const download = screen.getByRole("link", { name: "Download Desktop" });
    expect(download).toHaveAttribute("href", "http://10.10.10.20:18080/desktop/");
    expect(download).toHaveAttribute("target", "_blank");
    expect(download).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.queryByRole("link", { name: "Read the connection guide" })).not.toBeInTheDocument();
    act(() => configStore.getState().setDesktopDownloadUrl());
    expect(screen.queryByRole("link", { name: "Download Desktop" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Read the connection guide" })).toHaveAttribute("href", "/welcome-workspace/docs/daemon-runtimes");
  });

  it("changes the illustrative cards by keyboard and position controls", async () => {
    renderWelcome();
    const user = userEvent.setup();
    const carousel = within(screen.getByRole("region", { name: "Ways to connect a runtime" }));
    const previous = carousel.getByRole("button", { name: "Previous example" });
    const next = carousel.getByRole("button", { name: "Next example" });
    expect(previous).toHaveAttribute("aria-disabled", "true");
    const local = carousel.getByRole("group", { name: "This computer, connected automatically" });
    expect(local).toHaveTextContent("Example");
    expect(local).not.toHaveAttribute("inert");
    expect(carousel.getAllByRole("group", { hidden: true }).filter((slide) => slide.hasAttribute("inert"))).toHaveLength(1);
    expect(carousel.queryByRole("heading", { name: "Another computer, guided setup" })).not.toBeInTheDocument();
    next.focus();
    await user.keyboard("{Enter}");
    expect(carousel.getByRole("heading", { name: "Another computer, guided setup" })).toBeInTheDocument();
    expect(next).toHaveAttribute("aria-disabled", "true");
    expect(carousel.getByRole("status")).toHaveTextContent("Example 2 of 2: Another computer, guided setup");
    expect(carousel.getByText("multica setup")).toBeInTheDocument();
    expect(carousel.queryByRole("textbox")).not.toBeInTheDocument();
    await user.click(previous);
    expect(carousel.getByRole("heading", { name: "This computer, connected automatically" })).toBeInTheDocument();
    await user.click(carousel.getByRole("button", { name: "Show example 2: Another computer, guided setup" }));
    expect(carousel.getByRole("button", { name: "Show example 2: Another computer, guided setup" })).toHaveAttribute("aria-pressed", "true");
    expect(next).toHaveAttribute("aria-disabled", "true");
  });

  it("localizes the reference headline, actions and examples", () => {
    renderWelcome("zh-Hans");
    expect(screen.getByRole("heading", { name: "给智能体一台干活的电脑" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "添加运行时" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看连接指南" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "这台电脑，自动连接" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下一个示例" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "这台电脑，自动连接" })).toHaveTextContent("示例");
  });
});
