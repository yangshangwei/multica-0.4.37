import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import { DesktopBehaviorSettingsTab } from "./desktop-behavior-settings-tab";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("DesktopBehaviorSettingsTab", () => {
  it("keeps the saved minimize label translated when the tray becomes unavailable", async () => {
    const set = vi.fn();
    vi.stubGlobal("closeBehaviorAPI", {
      get: vi.fn(async () => "minimize"),
      isTraySupported: vi.fn(async () => false),
      set,
    });

    render(
      <I18nProvider locale="zh-Hans" resources={RESOURCES}>
        <DesktopBehaviorSettingsTab />
      </I18nProvider>,
    );

    const trigger = await screen.findByRole("combobox");
    expect(trigger).toHaveTextContent("最小化到系统托盘");
    expect(screen.getByText("当前系统不支持系统托盘，此选项不可用。")).toBeInTheDocument();

    fireEvent.click(trigger);
    expect(await screen.findByRole("option", { name: "退出 Multica" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "每次询问" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "最小化到系统托盘" })).not.toBeInTheDocument();
    expect(set).not.toHaveBeenCalled();
  });
});
