import userEvent from "@testing-library/user-event";
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { ChooseCreateSquadMethodPage } from "./choose-create-squad-method-page";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});

function renderChooser(canGoBack = false) {
  const navigation: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), canGoBack: () => canGoBack,
    pathname: "/acme/squads/new", searchParams: new URLSearchParams(), hash: "",
    getShareableUrl: (path) => path,
  };
  renderWithI18n(
    <NavigationProvider value={navigation}>
      <ChooseCreateSquadMethodPage />
    </NavigationProvider>,
  );
  return navigation;
}

describe("ChooseCreateSquadMethodPage", () => {
  it("offers native links to both creation pages without a modal", async () => {
    const user = userEvent.setup();
    const navigation = renderChooser();
    expect(screen.getByRole("heading", { name: "New Squad", level: 1 })).toBeVisible();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    for (const [name, href] of [
      [/Create from template/, "/acme/squads/new/template"],
      [/Create custom squad/, "/acme/squads/new/manual"],
    ] as const) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      link.focus();
      await user.keyboard("{Enter}");
      expect(navigation.push).toHaveBeenCalledExactlyOnceWith(href);
      vi.mocked(navigation.push).mockClear();
    }
  });

  it("returns a direct link to the squad directory", async () => {
    const navigation = renderChooser();
    await userEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(navigation.replace).toHaveBeenCalledExactlyOnceWith("/acme/squads");
    expect(navigation.back).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
  });

  it("returns through the existing history when available", async () => {
    const navigation = renderChooser(true);
    await userEvent.click(screen.getByRole("button", { name: "Go back" }));
    expect(navigation.back).toHaveBeenCalledTimes(1);
    expect(navigation.replace).not.toHaveBeenCalled();
  });
});
