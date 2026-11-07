import { render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { configStore } from "@multica/core/config";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));

vi.mock("next/image", () => ({
  default: ({ alt }: { alt: string }) => <span role="img" aria-label={alt} />,
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (auth: { user: null }) => unknown) =>
    selector({ user: null }),
}));

vi.mock("../utils/use-dashboard-cta", () => ({
  useDashboardCtaHref: () => "/login",
}));

import { LocaleProvider } from "../i18n";
import { LandingHero } from "./landing-hero";
import { LandingFooter } from "./landing-footer";

const DOWNLOAD_URL = "http://10.10.10.20:18080/desktop/";

function renderLanding() {
  return render(
    <LocaleProvider>
      <LandingHero />
      <LandingFooter />
    </LocaleProvider>,
  );
}

// The hero and footer link to the operator's download service from
// /api/config; the parsing and store wiring are canonical in
// packages/core/api/schema.test.ts and auth-initializer.test.tsx.
describe("landing desktop download links", () => {
  afterEach(() => configStore.setState(configStore.getInitialState()));

  it("link the hero button and the footer Product column to the configured page", () => {
    configStore.getState().setDesktopDownloadUrl(DOWNLOAD_URL);
    renderLanding();

    const hero = screen.getByRole("link", { name: "Download Desktop" });
    expect(hero).toHaveAttribute("href", DOWNLOAD_URL);
    expect(hero).toHaveAttribute("target", "_blank");

    const product = screen.getByRole("heading", { name: "Product" }).parentElement!;
    const footer = within(product).getByRole("link", { name: "Download" });
    expect(footer).toHaveAttribute("href", DOWNLOAD_URL);
    expect(footer).toHaveAttribute("target", "_blank");
    expect(within(product).getByRole("link", { name: "Changelog" })).toBeInTheDocument();
  });

  it("show neither link when no download page is configured", () => {
    renderLanding();

    expect(screen.queryByRole("link", { name: "Download Desktop" })).not.toBeInTheDocument();
    const product = screen.getByRole("heading", { name: "Product" }).parentElement!;
    expect(within(product).queryByRole("link", { name: "Download" })).not.toBeInTheDocument();
    expect(within(product).getByRole("link", { name: "Changelog" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Resources" })).toBeInTheDocument();
  });
});
