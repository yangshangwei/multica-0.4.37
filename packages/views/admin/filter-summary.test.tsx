import { expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import en from "../locales/en/admin.json";
import { AdminFilterSummary } from "./filter-summary";

it("describes submitted filters using known labels and preserves unknown values", () => {
  const params = new URLSearchParams("status=running&q=Example&custom=future-value&cursor=private-cursor");
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><AdminFilterSummary params={params} fields={[
    { name: "status", label: "Status", options: [{ value: "running", label: "Running" }] },
    { name: "q", label: "Search" }, { name: "custom", label: "Custom", displayValue: "" },
  ]} /></I18nProvider>);
  const summary = screen.getByRole("status", { name: "Applied filters" });
  expect(within(summary).getByText("Running")).toBeVisible();
  expect(within(summary).getByText("Example")).toBeVisible();
  expect(within(summary).getByText("future-value")).toBeVisible();
  expect(screen.queryByText("private-cursor")).not.toBeInTheDocument();
  expect(params.get("cursor")).toBe("private-cursor");
});

it("does not label cursors or empty drafts as applied filters", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><AdminFilterSummary params={new URLSearchParams("cursor=next&q=%20")} fields={[{ name: "q", label: "Search" }]} /></I18nProvider>);
  expect(screen.queryByRole("status")).not.toBeInTheDocument();
});
