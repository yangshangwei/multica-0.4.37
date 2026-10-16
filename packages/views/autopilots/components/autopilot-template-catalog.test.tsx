import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AutopilotTemplate } from "@multica/core/types";
import { ScrollRestorationProvider } from "../../platform/scroll-restoration";
import { renderWithI18n } from "../../test/i18n";
import { AutopilotTemplateCatalog } from "./autopilot-template-catalog";

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
  function Page({ loading = false, items = templates }: { loading?: boolean; items?: AutopilotTemplate[] }) {
    return <ScrollRestorationProvider adapter={adapter}>
      <AutopilotTemplateCatalog
        templates={loading ? [] : items}
        loading={loading}
        failed={false}
        onRetry={() => {}}
        onPick={onPick}
        onStartBlank={() => {}}
      />
    </ScrollRestorationProvider>;
  }
  return { Page, onPick, viewState };
}

afterEach(() => vi.restoreAllMocks());

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
