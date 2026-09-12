import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "@multica/views/navigation";
import type { SupportedLocale } from "@multica/core/i18n";
import {
  UpdateNotification,
  UpdateNotificationNavigationBridge,
  UpdateNotificationNavigationProvider,
} from "./update-notification";

type UpdateDownloadedListener = (info: { version: string; releaseNotes?: string }) => void;

describe("UpdateNotification", () => {
  let updateDownloaded: UpdateDownloadedListener;
  let navigation: NavigationAdapter;
  const installUpdate = vi.fn();
  const openExternal = vi.fn();
  const unsubscribe = vi.fn();

  beforeEach(() => {
    installUpdate.mockReset().mockResolvedValue(undefined);
    openExternal.mockReset().mockResolvedValue(undefined);
    unsubscribe.mockReset();
    navigation = {
      push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/issues",
      searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
    };
    Object.defineProperty(window, "desktopAPI", { configurable: true, value: { openExternal } });
    Object.defineProperty(window, "updater", {
      configurable: true,
      value: {
        onUpdateDownloaded: (listener: UpdateDownloadedListener) => { updateDownloaded = listener; return unsubscribe; },
        installUpdate,
      },
    });
  });

  function tree(shell = true, slug: string | null = "acme", locale: SupportedLocale = "en") {
    return (
      <UpdateNotificationNavigationProvider>
        <UpdateNotification locale={locale} />
        {shell ? (
          <NavigationProvider value={navigation}>
            <WorkspaceSlugProvider slug={slug}>
              <UpdateNotificationNavigationBridge workspaceSlug={slug} />
            </WorkspaceSlugProvider>
          </NavigationProvider>
        ) : null}
      </UpdateNotificationNavigationProvider>
    );
  }

  it("opens the downloaded version in the in-app reader via the navigation adapter", () => {
    render(tree());
    act(() => updateDownloaded({ version: "0.4.27" }));
    fireEvent.click(screen.getByRole("button", { name: "See changelog" }));
    expect(navigation.push).toHaveBeenCalledWith("/acme/changelog?version=0.4.27");
    expect(openExternal).not.toHaveBeenCalled();
  });

  it("retains an early event before shell/providers mount and enables its existing action later", () => {
    const { rerender } = render(tree(false));
    act(() => updateDownloaded({ version: "0.4.27" }));
    expect(screen.getByRole("button", { name: "See changelog" })).toBeDisabled();
    expect(screen.getByText("Open a workspace to read release notes.")).toBeInTheDocument();
    rerender(tree(true));
    fireEvent.click(screen.getByRole("button", { name: "See changelog" }));
    expect(navigation.push).toHaveBeenCalledWith("/acme/changelog?version=0.4.27");
    expect(unsubscribe).not.toHaveBeenCalled();
  });

  it("keeps restart available without a workspace or navigation provider", () => {
    const { rerender } = render(tree(false));
    act(() => updateDownloaded({ version: "0.4.27" }));
    fireEvent.click(screen.getByRole("button", { name: "Restart now" }));
    expect(installUpdate).toHaveBeenCalledOnce();
    rerender(tree(true, null));
    expect(screen.getByRole("button", { name: "See changelog" })).toBeDisabled();
    expect(navigation.push).not.toHaveBeenCalled();
  });

  it("uses the current workspace and removes the action when its shell unmounts", () => {
    const { rerender } = render(tree(true, "other"));
    act(() => updateDownloaded({ version: "0.4.27" }));
    fireEvent.click(screen.getByRole("button", { name: "See changelog" }));
    expect(navigation.push).toHaveBeenCalledWith("/other/changelog?version=0.4.27");
    rerender(tree(false));
    expect(screen.getByRole("button", { name: "See changelog" })).toBeDisabled();
  });

  it("localizes controls without requiring the core i18n provider and allows dismissing", () => {
    render(tree(false, null, "zh-Hans"));
    act(() => updateDownloaded({ version: "v0.4.27" }));
    expect(screen.getByText("下次启动时将更新至 v0.4.27。")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "关闭更新通知" }));
    expect(screen.queryByRole("button", { name: "立即重启" })).not.toBeInTheDocument();
    act(() => updateDownloaded({ version: "0.4.28" }));
    expect(screen.getByRole("button", { name: "立即重启" })).toBeInTheDocument();
  });

  it("unsubscribes from updater events when the app root unmounts", () => {
    const { unmount } = render(tree(false));
    unmount();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });
});
