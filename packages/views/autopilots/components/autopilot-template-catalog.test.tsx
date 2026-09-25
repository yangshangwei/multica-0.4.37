import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { AutopilotTemplate } from "@multica/core/types";
import { ScrollRestorationProvider } from "../../platform/scroll-restoration";
import { renderWithI18n } from "../../test/i18n";
import { AutopilotTemplateCatalog } from "./autopilot-template-catalog";

const templates: AutopilotTemplate[] = [{
  key: "workday-repo-audit", version: 1, category: "maintenance", category_label: "Maintenance",
  title: "Workday repo audit", description: "Check the repository.", cron_expression: "0 9 * * 1-5",
  execution_mode: "run_only", avatar_emoji: "", prompt: "Check the repository.",
}];

describe("template gallery navigation restoration", () => {
  it("captures the browsed offset and restores native-back state after data loads", async () => {
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
    function Page({ loading = false }: { loading?: boolean }) {
      return <ScrollRestorationProvider adapter={adapter}>
        <AutopilotTemplateCatalog templates={loading ? [] : templates} loading={loading} failed={false} onRetry={() => {}} onPick={onPick} />
      </ScrollRestorationProvider>;
    }
    const first = renderWithI18n(<Page />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Repository maintenance 1" }));
    const gallery = screen.getByRole("region", { name: "Start from a template" });
    expect(gallery).toHaveAttribute("data-tab-scroll-root", "autopilot-templates");
    gallery.scrollTop = 320;
    await userEvent.setup().click(screen.getByRole("button", { name: /Workday repo audit/ }));
    expect(onPick).toHaveBeenCalledWith("workday-repo-audit");
    first.unmount();

    const second = renderWithI18n(<Page loading />);
    second.rerender(<Page />);
    await waitFor(() => expect(screen.getByRole("region").scrollTop).toBe(320));
    expect(screen.getByRole("button", { name: "Repository maintenance 1" })).toHaveAttribute("aria-pressed", "true");
  });
});
