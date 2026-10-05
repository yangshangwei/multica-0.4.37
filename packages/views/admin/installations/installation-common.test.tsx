import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import zh from "../../locales/zh-Hans/admin.json";
import { InstallationAxis, InstallationRuntimeStatus } from "./installation-common";
import styles from "../admin-visual.module.css";

it("localizes known runtime status while retaining future server values", () => {
  render(<I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { admin: zh } }}>
    <p><InstallationRuntimeStatus status="online" /></p>
    <p><InstallationRuntimeStatus status="offline" /></p>
    <p><InstallationRuntimeStatus status="future-runtime-state" /></p>
  </I18nProvider>);
  expect(screen.getByText("在线")).toBeInTheDocument();
  expect(screen.getByText("离线")).toBeInTheDocument();
  expect(screen.getByText("future-runtime-state")).toBeInTheDocument();
});

it("uses the selected zone for installation evidence timestamps", () => {
  render(<I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { admin: zh } }}>
    <NavigationProvider value={{ pathname: "/admin/installations/install", searchParams: new URLSearchParams("timezone=Asia%2FShanghai"), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
      <InstallationAxis axis={{ state: "active", freshness: "fresh", observedAt: "2026-10-02T01:02:00Z", source: "client", reasonCode: "observed" }} />
    </NavigationProvider>
  </I18nProvider>);
  const timestamp = screen.getByText(/9:02/);
  expect(timestamp).toHaveTextContent("Asia/Shanghai");
  expect(timestamp).toHaveAttribute("datetime", "2026-10-02T01:02:00Z");
});

it.each([
  ["reachable", "fresh", "statusSuccess"],
  ["unreachable", "fresh", "statusDanger"],
  ["stopped", "fresh", "statusWarning"],
  ["reachable", "stale", "statusNeutral"],
  ["unreachable", "stale", "statusNeutral"],
  ["ready", "unknown", "statusNeutral"],
  ["no_permission", "unavailable", "statusNeutral"],
  ["unknown", "fresh", "statusNeutral"],
] as const)("shows %s with %s evidence without overstating current health", (state, freshness, tone) => {
  render(<I18nProvider locale="zh-Hans" resources={{ "zh-Hans": { admin: zh } }}>
    <NavigationProvider value={{ pathname: "/admin/installations", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
      <InstallationAxis axis={{ state, freshness, observedAt: null, source: "client", reasonCode: "observed" }} />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getByText(zh.installations.states[state])).toHaveClass(styles[tone]);
  expect(screen.getByText(zh.installations.freshness[freshness])).toBeInTheDocument();
  expect(screen.getByText("最近上报状态")).toBeInTheDocument();
});
