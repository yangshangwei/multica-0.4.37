import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import type { TriageItem } from "@multica/core/triage";
import en from "../locales/en/triage.json";
import { TriageBatchDialog } from "./triage-batch-dialog";
const { preview, commit } = vi.hoisted(() => ({
  preview: vi.fn(),
  commit: vi.fn(),
}));
vi.mock("../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof en) => string) => selector(en) }),
}));
vi.mock("@multica/core/triage", async (original) => ({
  ...(await original<typeof import("@multica/core/triage")>()),
  usePreviewTriageBatch: () => ({ mutateAsync: preview, isPending: false }),
  useCommitTriageBatch: () => ({ mutateAsync: commit, isPending: false }),
}));
vi.mock("./triage-fields", () => ({
  TriageSelect: () => null,
  ReviewerSelect: () => null,
}));
vi.mock("./triage-action-dialog", () => ({ SnoozeInput: () => null }));
const items = ["a", "b", "c"].map((id) => ({
  issue: { id, title: `Task ${id}`, identifier: id, revision: 2 },
})) as TriageItem[];
beforeEach(() => vi.clearAllMocks());
it("commits selected valid rows only and retries only the unfinished row with its request ID", async () => {
  preview.mockResolvedValue({
    items: [
      { issue_id: "a", expected_revision: 2, valid: true },
      { issue_id: "b", expected_revision: 2, valid: true },
      {
        issue_id: "c",
        expected_revision: 2,
        valid: false,
        error: "No longer pending",
      },
    ],
    valid_count: 2,
  });
  commit
    .mockResolvedValueOnce({
      results: [
        { issue_id: "a", status: "success" },
        { issue_id: "b", status: "failed", error: "Try again" },
      ],
      success_count: 1,
    })
    .mockResolvedValueOnce({
      results: [{ issue_id: "b", status: "success" }],
      success_count: 1,
    });
  const onSuccess = vi.fn();
  render(
    <TriageBatchDialog
      wsId="ws"
      items={items}
      onClose={vi.fn()}
      onSuccess={onSuccess}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Preview" }));
  await screen.findByText("No longer pending");
  fireEvent.click(
    screen.getByRole("button", { name: "Confirm selected valid tasks" }),
  );
  await screen.findByText("Try again");
  const submitted = commit.mock.calls[0]![0].items;
  expect(submitted.map((row: { issue_id: string }) => row.issue_id)).toEqual([
    "a",
    "b",
  ]);
  expect(onSuccess).toHaveBeenCalledWith(["a"]);
  fireEvent.click(
    screen.getByRole("button", { name: "Retry unfinished tasks" }),
  );
  await waitFor(() => expect(commit).toHaveBeenCalledTimes(2));
  expect(commit.mock.calls[1]![0].items).toEqual([submitted[1]]);
});
