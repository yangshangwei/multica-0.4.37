import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { BuiltinTemplateCatalog, BuiltinTemplateRow } from "./builtin-template-catalog";

describe("BuiltinTemplateCatalog disclosure", () => {
  it("starts compact and reveals existing actions with pointer and keyboard", async () => {
    const user = userEvent.setup();
    const onView = vi.fn();
    render(
      <BuiltinTemplateCatalog
        copy={{
          title: "Built-in agents", description: "Choose a role to get started.",
          loading: "Loading roles", error: "Could not load roles", empty: "No roles", retry: "Retry",
        }}
        count={1} loading={false} failed={false} empty={false} onRetry={vi.fn()}
      >
        <BuiltinTemplateRow
          title="Code reviewer" description="Review changes."
          actions={<button onClick={onView}>View template</button>}
        />
      </BuiltinTemplateCatalog>,
    );

    const toggle = screen.getByRole("button", { name: "Built-in agents 1" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Choose a role to get started.")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "View template" })).not.toBeInTheDocument();

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Choose a role to get started.")).toBeVisible();
    expect(screen.getByRole("button", { name: "View template" })).toBeVisible();
    expect(onView).not.toHaveBeenCalled();

    await user.keyboard("{Enter}");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await waitFor(() => expect(screen.queryByRole("button", { name: "View template" })).not.toBeInTheDocument());
    expect(toggle).toHaveFocus();

    await user.keyboard(" ");
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    await user.click(screen.getByRole("button", { name: "View template" }));
    expect(onView).toHaveBeenCalledOnce();
  });
});
