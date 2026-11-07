import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enOnboarding from "../../locales/en/onboarding.json";
import { MikaIntro } from "./mika-intro";

vi.mock("@multica/core/api", () => ({
  api: { getBaseUrl: () => "https://api.example.test/" },
}));

describe("MikaIntro", () => {
  it("shows the green Fu avatar before the agent exists", () => {
    render(
      <I18nProvider locale="en" resources={{ en: { onboarding: enOnboarding } }}>
        <MikaIntro />
      </I18nProvider>,
    );

    const avatar = screen.getByRole("img", { name: "小阿孚" });
    expect(avatar).toHaveTextContent("孚");
    expect(avatar.parentElement).toHaveClass("bg-skill-quality/12", "text-skill-quality");
  });
});
