// @vitest-environment jsdom

import { cleanup, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../../test/i18n";
import { PillButton } from "../../../common/pill-button";
import { AssigneePicker } from "./assignee-picker";
import { PriorityPicker } from "./priority-picker";

const { query } = vi.hoisted(() => ({ query: vi.fn(() => ({ data: [] })) }));

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQuery: query,
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: { user: null }) => unknown) =>
    selector({ user: null }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: () => "Unknown" }),
}));

afterEach(cleanup);

describe("deferred picker triggers", () => {
  it.each(["content", "render"] as const)("retains explicit names and values before and after opening custom %s triggers", async (kind) => {
    const user = userEvent.setup();
    query.mockClear();
    renderWithI18n(<>
      <PriorityPicker priority="high" onUpdate={() => {}} triggerAriaLabel="Task priority"
        {...(kind === "content" ? { trigger: "High" } : { triggerRender: <PillButton>High</PillButton> })} />
      <AssigneePicker assigneeType={null} assigneeId={null} onUpdate={() => {}} triggerAriaLabel="Task assignee"
        {...(kind === "content" ? { trigger: "Unassigned" } : { triggerRender: <PillButton>Unassigned</PillButton> })} />
    </>);

    expect(screen.getByRole("button", { name: "Task priority" })).toHaveAccessibleDescription("High");
    expect(screen.getByRole("button", { name: "Task assignee" })).toHaveAccessibleDescription("Unassigned");
    expect(query).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Task priority" }));
    expect(screen.getByRole("button", { name: "Task priority" })).toHaveAccessibleDescription("High");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Task assignee" }));
    expect(screen.getByRole("button", { name: "Task assignee" })).toHaveAccessibleDescription("Unassigned");
    expect(query).toHaveBeenCalled();
  });

  it("renders generated content when triggerRender only supplies an empty shell", () => {
    renderWithI18n(
      <>
        <PriorityPicker
          priority="none"
          onUpdate={() => {}}
          triggerRender={<PillButton />}
        />
        <AssigneePicker
          assigneeType={null}
          assigneeId={null}
          onUpdate={() => {}}
          triggerRender={<PillButton />}
        />
      </>,
    );

    expect(
      screen.getByRole("button", { name: "No priority" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Unassigned" }),
    ).toBeInTheDocument();
  });
});
