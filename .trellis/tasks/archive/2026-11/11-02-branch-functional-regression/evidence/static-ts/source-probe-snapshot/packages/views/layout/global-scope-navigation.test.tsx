import { expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";
import { GlobalScopeNavigation } from "./global-scope-navigation";

it("exposes one current scope and resumes only the other scope by keyboard", async () => {
  const user = userEvent.setup();
  const workspace = vi.fn();
  const collaboration = vi.fn();
  const view = renderWithI18n(<GlobalScopeNavigation activeScope="workspace" onWorkspace={workspace} onCollaboration={collaboration} />);
  expect(screen.getByRole("navigation", { name: "Application navigation" })).toBeVisible();
  const workspaceButton = screen.getByRole("button", { name: "Workspaces" });
  expect(workspaceButton).toHaveAttribute("aria-current", "page");
  await user.tab();
  expect(workspaceButton).toHaveFocus();
  await user.keyboard("{Enter}");
  expect(workspace).not.toHaveBeenCalled();
  await user.tab();
  await user.keyboard("{Enter}");
  expect(collaboration).toHaveBeenCalledOnce();
  view.rerender(<GlobalScopeNavigation activeScope="account" onWorkspace={workspace} onCollaboration={collaboration} />);
  expect(screen.getByRole("button", { name: "Collaboration" })).toHaveAttribute("aria-current", "page");
  await user.click(screen.getByRole("button", { name: "Collaboration" }));
  expect(collaboration).toHaveBeenCalledOnce();
  await user.click(workspaceButton);
  expect(workspace).toHaveBeenCalledOnce();
});
