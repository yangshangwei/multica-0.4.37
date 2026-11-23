import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { NavigationProvider } from "../navigation";
import { ScrollRestorationProvider } from "../platform/scroll-restoration";
import { renderWithI18n } from "../test/i18n";
import enLayout from "../locales/en/layout.json";
import { MARKETPLACE_CATEGORIES, MarketplaceCompactExample, type MarketplaceCategory } from "./marketplace-welcome";

function setup(state: Map<string, string>, category: MarketplaceCategory = MARKETPLACE_CATEGORIES[3], active = true) {
  const writes = vi.fn((key: string, value: string | undefined) => { if (value !== undefined) state.set(key, value); });
  const content = <WorkspaceSlugProvider slug="acme"><NavigationProvider value={{
    pathname: "/acme/marketplace", searchParams: new URLSearchParams("type=agent-templates"), hash: "",
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path,
  }}><ScrollRestorationProvider adapter={{ get: () => undefined, getViewState: (key) => state.get(key), setViewState: writes }}>
    <MarketplaceCompactExample category={category} active={active} />
  </ScrollRestorationProvider></NavigationProvider></WorkspaceSlugProvider>;
  return { ...renderWithI18n(content), writes };
}

describe("MarketplaceCompactExample restoration", () => {
  it("defaults closed without writing a preference on mount", async () => {
    const view = setup(new Map());
    expect(view.container.querySelector("details")).toHaveProperty("open", false);
    await waitFor(() => expect(view.writes).not.toHaveBeenCalled());
  });

  it.each([true, false])("restores explicit %s before a remounted example is shown", async (open) => {
    const state = new Map<string, string>([["marketplace-example-agent-templates", String(open)]]);
    const view = setup(state);
    const details = view.container.querySelector("details")!;
    expect(details.open).toBe(open);
    fireEvent(details, new Event("toggle"));
    expect(view.writes).not.toHaveBeenCalled();
    view.unmount();
    const remounted = setup(state);
    expect(remounted.container.querySelector("details")).toHaveProperty("open", open);
    expect(screen.getByText(enLayout.marketplace.example.show)).toBeVisible();
  });

  it("records only user changes under the current category", async () => {
    const state = new Map<string, string>();
    const view = setup(state);
    const details = view.container.querySelector("details")!;
    details.open = true;
    fireEvent(details, new Event("toggle"));
    await waitFor(() => expect(state.get("marketplace-example-agent-templates")).toBe("true"));
    details.open = false;
    fireEvent(details, new Event("toggle"));
    await waitFor(() => expect(state.get("marketplace-example-agent-templates")).toBe("false"));
    view.unmount();
    const mcp = setup(state, MARKETPLACE_CATEGORIES[0]);
    expect(mcp.container.querySelector("details")).toHaveProperty("open", false);
    expect(mcp.writes).not.toHaveBeenCalled();
  });

  it("does not let a queued inactive toggle overwrite the saved active choice", () => {
    const state = new Map<string, string>([["marketplace-example-agent-templates", "true"]]);
    const view = setup(state, MARKETPLACE_CATEGORIES[3], false);
    const details = view.container.querySelector("details")!;
    details.open = false;
    fireEvent(details, new Event("toggle"));
    expect(view.writes).not.toHaveBeenCalled();
    expect(state.get("marketplace-example-agent-templates")).toBe("true");
  });
});
