import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { useSidebarPreferences } from "@multica/core/navigation";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { NavigationGroup } from "./sidebar-navigation-group";

beforeEach(() => {
  useSidebarPreferences.setState({ groups: { work: true, ai: false, resources: false } });
});

function content(pathname: string, active = false) {
  const navigation: NavigationAdapter = { pathname, searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (value) => value };
  return <NavigationProvider value={navigation}><NavigationGroup group="ai" label="AI Team" active={active}><a href="/acme/agents">Agents</a></NavigationGroup></NavigationProvider>;
}

it("offers a real keyboard disclosure and remembers manual choices after remount", async () => {
  const view = renderWithI18n(content("/acme/issues"));
  const toggle = screen.getByRole("button", { name: "AI Team" });
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  toggle.focus();
  await userEvent.keyboard("{Enter}");
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByRole("link", { name: "Agents" })).toBeVisible();
  view.unmount();
  renderWithI18n(content("/acme/issues"));
  expect(screen.getByRole("button", { name: "AI Team" })).toHaveAttribute("aria-expanded", "true");
});

it("reveals a newly entered active child without persisting an automatic preference", async () => {
  const view = renderWithI18n(content("/acme/issues"));
  view.rerender(content("/acme/agents", true));
  const toggle = screen.getByRole("button", { name: "AI Team" });
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  expect(useSidebarPreferences.getState().groups.ai).toBe(false);
  await userEvent.click(toggle);
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  view.rerender(content("/acme/squads", true));
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  view.rerender(content("/acme/agents", true));
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  expect(useSidebarPreferences.getState().groups.ai).toBe(false);
  act(() => useSidebarPreferences.setState({ groups: { work: true, ai: false, resources: false } }));
});
