import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import issues from "../locales/en/issues.json";
import { TriageFilters } from "./triage-filters";
vi.mock("../i18n", () => ({
  useT: (namespace: string) => ({
    t: (
      selector: (value: typeof en | typeof issues) => string,
      values?: Record<string, string>,
    ) =>
      selector(namespace === "issues" ? issues : en).replace(
        /{{(\w+)}}/g,
        (_, key: string) => values?.[key] ?? key,
      ),
  }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({ queryKey }: { queryKey: string[] }) => ({
    data:
      queryKey.at(-1) === "members"
        ? [{ user_id: "member-1", name: "Human intake" }]
        : queryKey.at(-1) === "agents"
          ? [{ id: "agent-1", name: "Robot intake" }]
          : [],
  }),
}));
it("offers human and agent creators with distinct labels and preserves the selected actor UUID", async () => {
  const user = userEvent.setup();
  const onChange = vi.fn();
  render(
    <TriageFilters
      wsId="ws"
      params={new URLSearchParams()}
      history={false}
      onChange={onChange}
    />,
  );
  await user.click(screen.getByRole("combobox", { name: "Submitted by" }));
  expect(
    await screen.findByRole("option", { name: "Human intake (Member)" }),
  ).toBeVisible();
  await user.click(
    await screen.findByRole("option", { name: "Robot intake (Agent)" }),
  );
  expect(onChange).toHaveBeenCalledWith("creator_id", "agent-1");
});
