import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import type { TriageItem, TriageSettings } from "@multica/core/triage";
import en from "../locales/en/triage.json";
import { TriageActionDialog } from "./triage-action-dialog";

const { mutate, close, success } = vi.hoisted(() => ({
  mutate: vi.fn(),
  close: vi.fn(),
  success: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (selector: (value: typeof en) => string) => selector(en),
    i18n: { language: "en" },
  }),
}));
vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ slug: "acme" }),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({ refetch: vi.fn() }),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  useTriageAction: () => ({ mutateAsync: mutate, isPending: false }),
}));
vi.mock("./triage-execution-preview", () => ({
  useTriageExecutionPrerequisites: () => ({ canExecute: false }),
  TriageExecutionSummary: () => null,
}));
vi.mock("../modals/issue-picker-modal", () => ({
  IssuePickerModal: () => null,
}));
vi.mock("./triage-fields", () => ({
  TriageField: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ReviewerSelect: () => null,
  TriageFieldsEditor: () => <div>Acceptance fields</div>,
}));

const item = {
  issue: {
    id: "issue-1",
    identifier: "MUL-1",
    title: "Pending task",
    revision: 7,
    admission_status: "pending",
    priority: "none",
  },
  candidate_project_id: null,
  candidate_assignee_id: null,
  candidate_assignee_type: null,
  reviewer_id: null,
} as TriageItem;
const settings: TriageSettings = {
  supported: true,
  enabled: true,
  acceptance_status: "backlog",
  require_priority: true,
  responsibility_mode: "none",
  responsibility_member_id: null,
  revision: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("triage decision dialog", () => {
  it("keeps the reason and request identity after a failed decision, then closes only on success", async () => {
    mutate
      .mockRejectedValueOnce(new Error("Network unavailable"))
      .mockResolvedValueOnce({ item, action: { id: "action-1" } });
    render(
      <TriageActionDialog
        wsId="ws-1"
        item={item}
        action="reject"
        settings={settings}
        onClose={close}
        onSuccess={success}
      />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Reason" }), {
      target: { value: "Needs more context" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    await screen.findByText("Network unavailable");
    expect(screen.queryByText(en.refresh_conflict)).not.toBeInTheDocument();
    expect(close).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Reason" })).toHaveValue(
      "Needs more context",
    );
    const first = mutate.mock.calls[0]![0];
    expect(first.input).toMatchObject({
      action: "reject",
      expected_revision: 7,
      reason: "Needs more context",
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    await waitFor(() => expect(close).toHaveBeenCalledOnce());
    expect(mutate.mock.calls[1]![0].input.request_id).toBe(
      first.input.request_id,
    );
    expect(success).toHaveBeenCalledOnce();
  });
  it("does not accept without the configured priority, and never hides the form", async () => {
    render(
      <TriageActionDialog
        wsId="ws-1"
        item={item}
        action="accept"
        settings={settings}
        onClose={close}
        onSuccess={success}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Accept" }));
    expect(await screen.findByText(en.priority_required)).toBeVisible();
    expect(screen.getByText("Acceptance fields")).toBeVisible();
    expect(mutate).not.toHaveBeenCalled();
  });
  it("requires a nonblank rejection reason", () => {
    render(
      <TriageActionDialog
        wsId="ws-1"
        item={item}
        action="reject"
        settings={settings}
        onClose={close}
        onSuccess={success}
      />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Reason" }), {
      target: { value: "   " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    expect(screen.getByText(en.reason_required)).toBeVisible();
    expect(screen.queryByText(en.refresh_conflict)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: en.refresh }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Reason" })).toHaveFocus();
    expect(mutate).not.toHaveBeenCalled();
  });
});

it("offers refresh only for an actual task revision conflict and preserves the reason", async () => {
  mutate.mockRejectedValueOnce(
    new ApiError(
      "triage item changed; refresh before deciding",
      409,
      "Conflict",
    ),
  );
  render(
    <TriageActionDialog
      wsId="ws-1"
      item={item}
      action="reject"
      settings={settings}
      onClose={close}
      onSuccess={success}
    />,
  );
  fireEvent.change(screen.getByRole("textbox", { name: "Reason" }), {
    target: { value: "Keep this reason" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Reject" }));
  expect(await screen.findByText(en.refresh_conflict)).toBeVisible();
  expect(screen.getByRole("button", { name: en.refresh })).toBeVisible();
  expect(screen.getByRole("textbox", { name: "Reason" })).toHaveValue(
    "Keep this reason",
  );
});
