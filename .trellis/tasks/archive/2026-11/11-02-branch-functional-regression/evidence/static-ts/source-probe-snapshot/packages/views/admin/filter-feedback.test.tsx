import { useState } from "react";
import { expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../navigation";
import en from "../locales/en/admin.json";
import { AdminExecutionFilters } from "./executions/list-controls";
import { ObservationFilters } from "./observability/common";

function Filters({ observation }: { observation: boolean }) {
  const [params, setParams] = useState(new URLSearchParams());
  const navigate = (href: string) => setParams(new URL(href, "https://test.invalid").searchParams);
  return <I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{ pathname: "/admin/tasks", searchParams: params, hash: "", push: navigate, replace: navigate, back: () => navigate("/admin/tasks"), getShareableUrl: path => path }}>
      {observation ? <ObservationFilters params={params} advanced /> : <AdminExecutionFilters />}
      <button onClick={() => navigate("/admin/tasks?timezone=UTC")}>Open saved view</button>
      <button onClick={() => navigate("/admin/tasks")}>Back to initial view</button>
    </NavigationProvider>
  </I18nProvider>;
}

function timezoneField() {
  return screen.getByLabelText(/time\s?zone/i);
}

function invalidateTimezone() {
  const zone = timezoneField();
  fireEvent.change(zone, { target: { value: "Mars/Phobos" } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(zone).toHaveAttribute("aria-invalid", "true");
}

it.each([false, true])("resets drafts and validation even when the URL is already clear (observation=%s)", observation => {
  render(<Filters observation={observation} />);
  invalidateTimezone();
  fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
  expect(timezoneField()).toHaveValue("");
  expect(timezoneField()).not.toHaveAttribute("aria-invalid");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("clears an invalid draft when a quick range restores the applied filters", () => {
  render(<Filters observation />);
  invalidateTimezone();
  fireEvent.click(screen.getByRole("button", { name: en.filters.last24Hours }));
  expect(timezoneField()).toHaveValue("");
  expect(timezoneField()).not.toHaveAttribute("aria-invalid");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("does not restore stale errors when navigating back to an earlier filter URL", () => {
  render(<Filters observation={false} />);
  invalidateTimezone();
  fireEvent.click(screen.getByRole("button", { name: "Open saved view" }));
  expect(timezoneField()).toHaveValue("UTC");
  expect(timezoneField()).not.toHaveAttribute("aria-invalid");
  fireEvent.click(screen.getByRole("button", { name: "Back to initial view" }));
  expect(timezoneField()).toHaveValue("");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
