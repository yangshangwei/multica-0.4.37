import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriageExecutionStatus } from "./triage-history";

const state = vi.hoisted(() => ({
  userId: "actor-1",
  retry: vi.fn(),
  accept: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof en) => string) => selector(en) }),
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ userId: state.userId }),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useRetryTriageExecution: () => ({ mutate: state.retry, isPending: false }),
  useTriageAction: () => ({ mutate: state.accept }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({
    data: {
      events: [
        {
          id: "accepted-action-1",
          actor_id: "actor-1",
          action: "accept_and_execute",
          execution_status: "failed",
          execution_error: "The runtime disconnected before startup",
          after: { issue: { admission_status: "accepted" } },
        },
      ],
    },
  }),
}));
beforeEach(() => {
  state.userId = "actor-1";
  vi.clearAllMocks();
});
it("keeps accepted-with-failed-start visible and retries the original action without accepting again", () => {
  render(<TriageExecutionStatus wsId="ws" issueId="issue-1" />);
  expect(screen.getByRole("status")).toHaveTextContent(en.execution_failed);
  expect(
    screen.getByText("The runtime disconnected before startup"),
  ).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: en.retry_execution }));
  expect(state.retry).toHaveBeenCalledExactlyOnceWith("accepted-action-1");
  expect(state.accept).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: en.accept }),
  ).not.toBeInTheDocument();
});
it("shows the accepted result and failure reason to another member without offering an unauthorized retry", () => {
  state.userId = "other-member";
  render(<TriageExecutionStatus wsId="ws" issueId="issue-1" />);
  expect(screen.getByRole("status")).toHaveTextContent(en.execution_failed);
  expect(
    screen.getByText("The runtime disconnected before startup"),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: en.retry_execution }),
  ).not.toBeInTheDocument();
  expect(state.retry).not.toHaveBeenCalled();
  expect(state.accept).not.toHaveBeenCalled();
});
