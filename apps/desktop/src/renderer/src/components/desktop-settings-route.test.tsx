import type { ComponentProps } from "react";
import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import type { SettingsPage } from "@multica/views/settings";

const settingsPage = vi.fn<(props: ComponentProps<typeof SettingsPage>) => null>(() => null);

vi.mock("@multica/views/settings", () => ({
  SettingsPage: (props: ComponentProps<typeof SettingsPage>) => settingsPage(props),
}));
vi.mock("./daemon-settings-tab", () => ({ DaemonSettingsTab: () => null }));
vi.mock("./updates-settings-tab", () => ({ UpdatesSettingsTab: () => null }));
vi.mock("./runtime-config-settings-tab", () => ({ RuntimeConfigSettingsTab: () => null }));
vi.mock("./desktop-behavior-settings-tab", () => ({ DesktopBehaviorSettingsTab: () => null }));

import { DesktopSettingsRoute } from "./desktop-settings-route";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  settingsPage.mockClear();
});

describe("DesktopSettingsRoute", () => {
  it.each([
    ["macos", ["daemon", "server", "updates"]],
    ["windows", ["daemon", "server", "updates", "behavior"]],
    ["linux", ["daemon", "server", "updates", "behavior"]],
    ["unknown", ["daemon", "server", "updates"]],
  ])("registers only the settings supported by %s", (os, expectedTabs) => {
    vi.stubGlobal("desktopAPI", { appInfo: { os, version: "0.1.0" } });

    render(
      <I18nProvider locale="en" resources={RESOURCES}>
        <DesktopSettingsRoute />
      </I18nProvider>,
    );

    const props = settingsPage.mock.lastCall?.[0];
    expect(props?.extraDesktopTabs?.map((tab) => tab.value)).toEqual(expectedTabs);
  });

  it.each([
    ["en", "Server connection"],
    ["zh-Hans", "服务器连接"],
  ] as const)("names the server destination clearly in %s", (locale, label) => {
    vi.stubGlobal("desktopAPI", { appInfo: { os: "macos", version: "0.1.0" } });

    render(
      <I18nProvider locale={locale} resources={RESOURCES}>
        <DesktopSettingsRoute />
      </I18nProvider>,
    );

    const props = settingsPage.mock.lastCall?.[0];
    expect(props?.extraDesktopTabs?.find((tab) => tab.value === "server")?.label).toBe(label);
  });
});
