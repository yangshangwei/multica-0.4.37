import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { IssueCompletionHintModal } from "./issue-completion-hint";

vi.mock("../i18n", () => ({
  useT: () => ({
    t: (select: (messages: unknown) => string) => select({
      detail: { completion_hint: {
        unknown_title: "Could not check",
        unknown_description: "Retry the read",
        cancel: "Cancel",
        retry: "Retry",
      } },
    }),
  }),
}));

describe("IssueCompletionHintModal", () => {
  it("closes the old result before retry can display its replacement", () => {
    const events: string[] = [];
    render(<IssueCompletionHintModal
      onClose={() => events.push("close")}
      data={{ state: "unknown", onRetry: () => { events.push("retry"); } }}
    />);
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(events).toEqual(["close", "retry"]);
  });
});
