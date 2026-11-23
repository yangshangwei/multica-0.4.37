import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import { useTabStore } from "@/stores/tab-store";
import { TabBar } from "./tab-bar";

vi.mock("@multica/views/layout", () => ({
  useTabPresentation: (_url: string, title: string) => ({
    visual: { kind: "icon", icon: "ListTodo" },
    title,
  }),
  ResourceLeadingVisual: () => null,
}));

beforeEach(() => {
  localStorage.clear();
  useTabStore.getState().reset();
  vi.stubGlobal("ResizeObserver", class {
    observe() {}
    unobserve() {}
    disconnect() {}
  });
});

afterEach(() => vi.unstubAllGlobals());

function renderTabs(locale: "en" | "zh-Hans" = "en") {
  const store = useTabStore.getState();
  store.switchWorkspace("acme");
  const issuesId = useTabStore.getState().byWorkspace.acme.tabs[0].id;
  const projectsId = store.addTab("/acme/projects", "Projects")!;
  const agentsId = store.addTab("/acme/agents", "Agents")!;
  store.setActiveTab(issuesId);

  const view = render(
    <I18nProvider locale={locale} resources={RESOURCES}>
      <TabBar />
    </I18nProvider>,
  );
  return { ...view, issuesId, projectsId, agentsId };
}

function tabOrder() {
  return useTabStore.getState().byWorkspace.acme.tabs.map((tab) => tab.title);
}

describe("TabBar accessible navigation", () => {
  it("exposes exactly the current destination to assistive technology", () => {
    renderTabs();
    expect(screen.getByRole("button", { name: "Issues" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Projects" })).not.toHaveAttribute("aria-current");

    fireEvent.click(screen.getByRole("button", { name: "Projects" }));

    expect(screen.getByRole("button", { name: "Projects" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "Issues" })).not.toHaveAttribute("aria-current");
  });

  // The clamp matrix belongs to tab-store.test.ts. This covers the real
  // keyboard/menu wiring, focus and persistence through that existing store.
  it("moves by keyboard within the unpinned segment and retains focus and active destination", () => {
    const { issuesId } = renderTabs();
    act(() => useTabStore.getState().togglePin(issuesId));
    const projects = screen.getByRole("button", { name: "Projects" });
    projects.focus();

    fireEvent.keyDown(projects, { key: "ArrowLeft", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Issues", "Projects", "Agents"]);

    fireEvent.keyDown(projects, { key: "ArrowRight", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Issues", "Agents", "Projects"]);
    expect(projects).toHaveFocus();
    expect(useTabStore.getState().byWorkspace.acme.activeTabId).toBe(issuesId);

    fireEvent.keyDown(projects, { key: "ArrowRight", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Issues", "Agents", "Projects"]);
    expect(projects).toHaveFocus();

    fireEvent.keyDown(projects, { key: "ArrowLeft", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Issues", "Projects", "Agents"]);
  });

  it("offers pointer commands with disabled boundaries and saves the resulting order", async () => {
    const { issuesId } = renderTabs();
    act(() => useTabStore.getState().togglePin(issuesId));
    const projects = screen.getByRole("button", { name: "Projects" });
    projects.focus();
    fireEvent.contextMenu(projects);

    expect(await screen.findByRole("menuitem", { name: "Move left" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(screen.getByRole("menuitem", { name: "Move right" }));

    expect(tabOrder()).toEqual(["Issues", "Agents", "Projects"]);
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
    await waitFor(() => expect(projects).toHaveFocus());
    await act(async () => { await useTabStore.persist.rehydrate(); });
    expect(tabOrder()).toEqual(["Issues", "Agents", "Projects"]);
    expect(useTabStore.getState().byWorkspace.acme.activeTabId).toBe(issuesId);
  });

  it("moves pinned tabs by keyboard without crossing into the unpinned segment", async () => {
    const { issuesId, projectsId } = renderTabs();
    act(() => {
      useTabStore.getState().togglePin(issuesId);
      useTabStore.getState().togglePin(projectsId);
    });
    const issues = screen.getByRole("button", { name: "Issues (pinned)" });
    issues.focus();
    fireEvent.keyDown(issues, { key: "ArrowRight", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Projects", "Issues", "Agents"]);
    expect(issues).toHaveFocus();

    fireEvent.contextMenu(issues);
    expect(await screen.findByRole("menuitem", { name: "Move right" })).toHaveAttribute("aria-disabled", "true");
    fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    await waitFor(() => expect(issues).toHaveFocus());
    fireEvent.keyDown(issues, { key: "ArrowRight", altKey: true, shiftKey: true });
    expect(tabOrder()).toEqual(["Projects", "Issues", "Agents"]);
  });

  it("labels the new commands in Chinese", async () => {
    renderTabs("zh-Hans");
    fireEvent.contextMenu(screen.getByRole("button", { name: "Projects" }));
    expect(await screen.findByRole("menuitem", { name: "向左移动" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "向右移动" })).toBeInTheDocument();
  });
});
