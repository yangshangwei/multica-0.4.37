import userEvent from "@testing-library/user-event";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import zh from "../locales/zh-Hans/triage.json";
import { TriageHistoryFilterSummary } from "./triage-history-filter-summary";
import { triageHistoryFilters } from "./triage-ui";

const state = vi.hoisted(() => ({ locale: "en" }));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      selector: (value: typeof en) => string,
      values?: Record<string, string>,
    ) => selector(state.locale === "en" ? en : zh).replace(
      /{{(\w+)}}/g,
      (_, key: string) => values?.[key] ?? key,
    ),
  }),
}));

beforeEach(() => {
  state.locale = "en";
});

it.each(["en", "zh-Hans"])("names the applied conditions in %s and exposes one clear action", async (locale) => {
  state.locale = locale;
  const messages = locale === "en" ? en : zh;
  const onClear = vi.fn();
  const user = userEvent.setup();
  render(
    <TriageHistoryFilterSummary
      filters={triageHistoryFilters(new URLSearchParams({
        source: "manual",
        result: "accept_and_execute",
        processed_by: "member-1",
      }))}
      members={[{ user_id: "member-1", name: "", email: "reviewer@example.test" }]}
      onClear={onClear}
    />,
  );
  const summary = screen.getByRole("list", { name: messages.history_filters.label });
  const values = within(summary).getAllByRole("listitem");
  expect(values[0]).toHaveTextContent(messages.source);
  expect(values[0]).toHaveTextContent(messages.manual);
  expect(values[1]).toHaveTextContent(messages.result);
  expect(values[1]).toHaveTextContent(messages.accept_and_execute);
  expect(values[2]).toHaveTextContent(messages.processed_by);
  expect(values[2]).toHaveTextContent("reviewer@example.test");
  await user.click(screen.getByRole("button", { name: messages.clear_filters }));
  expect(onClear).toHaveBeenCalledTimes(1);
});

it("keeps unknown filter values and unavailable processor identities readable", () => {
  render(
    <TriageHistoryFilterSummary
      filters={triageHistoryFilters(new URLSearchParams({
        source: "future-source",
        result: "future-action",
        processed_by: "unavailable-member-id",
      }))}
      members={[]}
      onClear={vi.fn()}
    />,
  );
  expect(screen.getByText("Source: future-source")).toBeVisible();
  expect(screen.getByText("Result: future-action")).toBeVisible();
  expect(screen.getByText("Processed by: unavailable-member-id")).toBeVisible();
});
