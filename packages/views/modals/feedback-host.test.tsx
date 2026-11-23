import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useModalStore } from "@multica/core/modals";
import { FeedbackModalHost } from "./registry";

vi.mock("./feedback", () => ({
  FeedbackModal: ({ data, onClose }: { data?: Record<string, unknown> | null; onClose: () => void }) => (
    <div data-testid="feedback-modal" data-message={String(data?.initialMessage ?? "")}>
      <button type="button" onClick={onClose}>Close feedback</button>
    </div>
  ),
}));

beforeEach(() => useModalStore.getState().close());
afterEach(() => {
  cleanup();
  useModalStore.getState().close();
});

describe("FeedbackModalHost", () => {
  it("renders the existing feedback modal only for feedback state", () => {
    useModalStore.getState().open("create-issue");
    const { rerender } = render(<FeedbackModalHost />);
    expect(screen.queryByTestId("feedback-modal")).not.toBeInTheDocument();

    useModalStore.getState().open("feedback", { initialMessage: "Account page" });
    rerender(<FeedbackModalHost />);
    expect(screen.getByTestId("feedback-modal")).toHaveAttribute("data-message", "Account page");

    screen.getByRole("button", { name: "Close feedback" }).click();
    expect(useModalStore.getState().modal).toBeNull();
  });
});
