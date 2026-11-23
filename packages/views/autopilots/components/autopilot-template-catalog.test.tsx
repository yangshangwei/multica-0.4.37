import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AutopilotTemplate } from "@multica/core/types";
import { ScrollRestorationProvider } from "../../platform/scroll-restoration";
import { renderWithI18n } from "../../test/i18n";
import { AutopilotTemplateCatalog } from "./autopilot-template-catalog";
import { NavigationProvider } from "../../navigation";

const templates: AutopilotTemplate[] = [{
  key: "workday-repo-audit", version: 1, category: "maintenance", category_label: "Maintenance",
  title: "Workday repo audit", description: "Check the repository.", cron_expression: "0 9 * * 1-5",
  execution_mode: "run_only", avatar_emoji: "", prompt: "Check the repository.",
}];

function createGallery() {
  const onPick = vi.fn();
  const viewState = new Map<string, string>();
  const adapter = {
    // Desktop clears plain scroll entries when leaving the configure step.
    get: () => undefined,
    getViewState: (key: string) => viewState.get(key),
    setViewState: (key: string, value: string | undefined) => {
      if (value === undefined) viewState.delete(key);
      else viewState.set(key, value);
    },
  };
  function Page({ loading = false, items = templates, welcome = false }: { loading?: boolean; items?: AutopilotTemplate[]; welcome?: boolean }) {
    return <NavigationProvider value={{
      push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/autopilots",
      searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
    }}><ScrollRestorationProvider adapter={adapter}>
      <AutopilotTemplateCatalog
        templates={loading ? [] : items}
        loading={loading}
        failed={false}
        onRetry={() => {}}
        onPick={onPick}
        onStartBlank={() => {}}
        welcome={welcome ? { docsHref: "/acme/docs/autopilots", templatesHref: "/acme/autopilots/new/template" } : undefined}
      />
    </ScrollRestorationProvider></NavigationProvider>;
  }
  return { Page, onPick, viewState };
}

afterEach(() => vi.restoreAllMocks());

describe("template card", () => {
  it("names the card by its template and keeps its facts as the description", () => {
    const { Page } = createGallery();
    renderWithI18n(<Page />);
    const card = screen.getByRole("button", { name: "View template: Workday repo audit" });
    // Each fact is its own reference, so schedule and outcome never run together.
    expect(card).toHaveAccessibleDescription(
      /^Recommended starting point Check the repository\. At 09:00.*Mon–Fri Creates or updates issues as needed$/,
    );
  });

  it("falls back to the key and drops facts a card does not render", () => {
    const { Page } = createGallery();
    renderWithI18n(<Page items={[{ ...templates[0]!, key: "future-template", title: "", execution_mode: "future_mode" }]} />);
    const card = screen.getByRole("button", { name: "View template: future-template" });
    expect(card).toHaveTextContent("future-template");
    expect(card).toHaveAccessibleDescription(/^Check the repository\. At 09:00.*Mon–Fri$/);
  });
});

describe("template gallery navigation restoration", () => {
  it.each([
    { filter: "Repository maintenance 1", scrollTop: 320 },
    { filter: "All 1", scrollTop: 0 },
  ])("restores $filter, scroll $scrollTop and card focus after delayed data", async ({ filter, scrollTop }) => {
    const { Page, onPick, viewState } = createGallery();
    viewState.set("autopilot-template-group", "maintenance");
    viewState.set("autopilot-template-scroll", "320");
    const first = renderWithI18n(<Page />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: filter }));
    const gallery = screen.getByRole("region", { name: "Start from a template" });
    expect(gallery).toHaveAttribute("data-tab-scroll-root", "autopilot-templates");
    gallery.scrollTop = scrollTop;
    screen.getByRole("button", { name: /Workday repo audit/ }).focus();
    await user.keyboard("{Enter}");
    expect(onPick).toHaveBeenCalledWith("workday-repo-audit");
    first.unmount();

    const second = renderWithI18n(<Page loading />);
    expect(document.body).toHaveFocus();
    const focus = vi.spyOn(HTMLElement.prototype, "focus");
    second.rerender(<Page />);
    await waitFor(() => expect(screen.getByRole("button", { name: /Workday repo audit/ })).toHaveFocus());
    expect(screen.getByRole("region").scrollTop).toBe(scrollTop);
    expect(screen.getByRole("button", { name: filter })).toHaveAttribute("aria-pressed", "true");
    expect(focus).toHaveBeenCalledWith({ preventScroll: true });
  });

  it.each([
    { key: "hourly-queue-check", filter: "Repository maintenance 1" },
    { key: "daily-change-review", filter: "All 1" },
  ])("returns to $filter when the selected template is no longer available", async ({ key, filter }) => {
    const { Page } = createGallery();
    const first = renderWithI18n(<Page />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Repository maintenance 1" }));
    await user.click(screen.getByRole("button", { name: /Workday repo audit/ }));
    first.unmount();

    renderWithI18n(<Page items={[{ ...templates[0]!, key, title: "Replacement template" }]} />);
    expect(screen.getByRole("button", { name: filter })).toHaveFocus();
    expect(screen.getByRole("button", { name: filter })).toHaveAttribute("aria-pressed", "true");
  });

  it.each([false, true])("keeps a newer focus owner while data loads (dialog: %s)", async (inDialog) => {
    const { Page } = createGallery();
    const first = renderWithI18n(<Page />);
    await userEvent.setup().click(screen.getByRole("button", { name: /Workday repo audit/ }));
    first.unmount();

    const page = (loading: boolean) => <>
      <Page loading={loading} />
      <div role={inDialog ? "dialog" : undefined} aria-label={inDialog ? "Custom automation" : undefined}>
        <button type="button">Another action</button>
      </div>
    </>;
    const second = renderWithI18n(page(true));
    const otherAction = screen.getByRole("button", { name: "Another action" });
    await userEvent.setup().click(otherAction);
    second.rerender(page(false));
    expect(otherAction).toHaveFocus();

    second.unmount();
    renderWithI18n(<Page />);
    expect(document.body).toHaveFocus();
  });

  it("returns to blank creation when the registry is empty and consumes that handoff", async () => {
    const { Page } = createGallery();
    const first = renderWithI18n(<Page />);
    await userEvent.setup().click(screen.getByRole("button", { name: /Workday repo audit/ }));
    first.unmount();

    const second = renderWithI18n(<Page items={[]} />);
    expect(screen.getByRole("button", { name: "Start from scratch" })).toHaveFocus();

    second.unmount();
    renderWithI18n(<Page items={[]} />);
    expect(document.body).toHaveFocus();
  });

  it("consumes return focus so refetches and later visits cannot reclaim it", async () => {
    const { Page } = createGallery();
    const first = renderWithI18n(<Page />);
    await userEvent.setup().click(screen.getByRole("button", { name: /Workday repo audit/ }));
    first.unmount();

    const second = renderWithI18n(<Page />);
    expect(screen.getByRole("button", { name: /Workday repo audit/ })).toHaveFocus();
    const blankAction = screen.getByRole("button", { name: "Start from scratch" });
    await userEvent.setup().click(blankAction);
    second.rerender(<Page items={[...templates]} />);
    expect(blankAction).toHaveFocus();

    second.unmount();
    renderWithI18n(<Page />);
    expect(document.body).toHaveFocus();
  });
});

describe("welcome template preview", () => {
  it("previews six real templates in server order, independently of saved category filters", () => {
    const { Page, viewState } = createGallery();
    viewState.set("autopilot-template-group", "maintenance");
    const items = [
      "workday-repo-audit", "daily-change-review", "stale-pr-reminder", "bug-triage",
      "weekly-progress-report", "dependency-audit", "documentation-check", "future-template",
    ].map((key) => ({ ...templates[0]!, key, title: key }));
    renderWithI18n(<Page welcome items={items} />);
    expect(screen.getAllByRole("button", { name: /^View template:/ }).map((card) => card.getAttribute("aria-label"))).toEqual([
      "View template: daily-change-review", "View template: stale-pr-reminder", "View template: bug-triage",
      "View template: weekly-progress-report", "View template: dependency-audit", "View template: documentation-check",
    ]);
    expect(screen.queryByRole("group", { name: "Filter templates by scenario" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Browse all templates" })).toBeInTheDocument();
  });

  it("fills a partial registry with available templates and keeps the server schedule", () => {
    const { Page } = createGallery();
    renderWithI18n(<Page welcome items={[
      { ...templates[0]!, key: "future-template", title: "Future template", cron_expression: "17 14 * * 1" },
      { ...templates[0]!, key: "daily-change-review", title: "Daily change review" },
    ]} />);
    const cards = screen.getAllByRole("button", { name: /^View template:/ });
    expect(cards).toHaveLength(2);
    expect(cards[0]).toHaveAccessibleName("View template: Future template");
    expect(cards[0]).toHaveAccessibleDescription(/^Check the repository\. At 14:17.*Mon Run only$/);
  });

  it("restores welcome scroll and focus after delayed template loading", async () => {
    const { Page, viewState, onPick } = createGallery();
    viewState.set("autopilot-template-group", "reporting");
    const first = renderWithI18n(<Page welcome />);
    const gallery = screen.getByRole("region", { name: "Let your agents get to work on schedule" });
    gallery.scrollTop = 240;
    await userEvent.setup().click(screen.getByRole("button", { name: /Workday repo audit/ }));
    expect(onPick).toHaveBeenCalledWith("workday-repo-audit");
    first.unmount();
    const second = renderWithI18n(<Page welcome loading />);
    second.rerender(<Page welcome />);
    expect(screen.getByRole("region", { name: "Let your agents get to work on schedule" }).scrollTop).toBe(240);
    expect(screen.getByRole("button", { name: /Workday repo audit/ })).toHaveFocus();
    expect(viewState.has("autopilot-template-focus")).toBe(false);
  });

  it("returns focus to new automation when a welcome template disappears", () => {
    const { Page, viewState } = createGallery();
    viewState.set("autopilot-template-focus", "removed-template");
    renderWithI18n(<Page welcome />);
    expect(screen.getByRole("button", { name: "New autopilot" })).toHaveFocus();
  });
});
