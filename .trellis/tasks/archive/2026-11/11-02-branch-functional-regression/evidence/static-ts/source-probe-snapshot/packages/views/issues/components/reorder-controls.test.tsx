import { useState } from "react";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { ReorderControls } from "./reorder-controls";

function RemountingRow() {
  const [index, setIndex] = useState(1);
  return (
    <div data-reorder-scope="">
      <button type="button">Toolbar</button>
      {/* A virtual window can replace its row when the index moves. */}
      <ReorderControls key={index} id="issue-1" name="MUL-1" index={index} count={3} onMove={(direction) => setIndex(index + direction)} />
    </div>
  );
}

describe("ReorderControls focus ownership", () => {
  it("restores the same action when virtualization remounts the moved row", async () => {
    renderWithI18n(<RemountingRow />);
    const before = screen.getByRole("button", { name: "Move MUL-1 up" });
    before.focus();
    fireEvent.click(before);
    const after = screen.getByRole("button", { name: "Move MUL-1 up" });
    expect(after).not.toBe(before);
    await waitFor(() => expect(after).toHaveFocus());
    expect(after).toHaveAttribute("aria-disabled", "true");
  });

  it("does not reclaim focus after the user moves to the toolbar", async () => {
    const frames: FrameRequestCallback[] = [];
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => { frames.push(callback); return frames.length; });
    renderWithI18n(<RemountingRow />);
    const before = screen.getByRole("button", { name: "Move MUL-1 up" });
    before.focus();
    fireEvent.click(before);
    const toolbar = screen.getByRole("button", { name: "Toolbar" });
    toolbar.focus();
    act(() => frames.forEach((callback) => callback(0)));
    expect(toolbar).toHaveFocus();
    vi.restoreAllMocks();
  });
});
