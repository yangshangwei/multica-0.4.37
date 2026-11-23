import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { configStore } from "@multica/core/config";
import { renderWithI18n } from "../../test/i18n";
import { StepWelcome } from "./step-welcome";

// The welcome illustration's mock cards carry issue statuses, and the status
// label used to come from `STATUS_CONFIG.label` — the server's English seed —
// while every other string on the screen went through `useT` (MUL-6850). A
// non-English member met the product on a line that read half in each
// language. These assert against the REAL locale bundles, so a regression that
// re-introduces the English seed fails here rather than in a screenshot.
describe("StepWelcome status labels", () => {
  it("renders the built-in status labels in English", () => {
    renderWithI18n(<StepWelcome onNext={vi.fn()} />);

    expect(screen.getAllByRole("listitem")).toHaveLength(5);
    for (const actor of ["You", "Dev Agent", "Architect Agent", "Security Review Agent", "QA Agent"]) {
      expect(screen.getByText(actor)).toBeInTheDocument();
    }
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.getByText("In Review")).toBeInTheDocument();
    expect(screen.getAllByText("Done")).toHaveLength(2);
  });

  it("localizes them rather than rendering the English seed", () => {
    renderWithI18n(<StepWelcome onNext={vi.fn()} />, { locale: "zh-Hans" });

    expect(screen.getByText("进行中")).toBeInTheDocument();
    expect(screen.getByText("审核中")).toBeInTheDocument();
    expect(screen.getAllByText("已完成")).toHaveLength(2);
    expect(screen.queryByText("In Progress")).not.toBeInTheDocument();
  });
});

describe("StepWelcome desktop download", () => {
  afterEach(() => configStore.setState(configStore.getInitialState()));

  it("links web users to the configured download page in a new tab", () => {
    configStore.getState().setDesktopDownloadUrl("http://10.10.10.20:18080/desktop/");
    renderWithI18n(<StepWelcome onNext={vi.fn()} isWeb />);

    const link = screen.getByRole("link", { name: "Download Desktop" });
    expect(link).toHaveAttribute("href", "http://10.10.10.20:18080/desktop/");
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getByRole("button", { name: "Continue on web" })).toBeInTheDocument();
    expect(screen.getByText(/desktop bundles the runtime/i)).toBeInTheDocument();
  });

  it("leaves only Continue on web when no download page is configured", () => {
    renderWithI18n(<StepWelcome onNext={vi.fn()} isWeb />);

    expect(screen.queryByRole("link", { name: "Download Desktop" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue on web" })).toBeInTheDocument();
    // The lede must not promise a Desktop option the screen does not offer.
    expect(screen.queryByText(/desktop bundles the runtime/i)).not.toBeInTheDocument();
    expect(screen.getByText(/connect this computer from the terminal/i)).toBeInTheDocument();
  });
});
