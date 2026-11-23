import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue, IssueAssigneeType } from "@multica/core/types";
import { AppLink, NavigationProvider, type NavigationAdapter } from "../../navigation";
import {
  IssueSurfaceActionsProvider,
  type IssueSurfaceActions,
} from "../surface/actions-context";
import { BoardCardContent } from "./board-card";
import enIssues from "../../locales/en/issues.json";

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({ data: [] }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/properties", () => ({
  propertyListOptions: () => ({ queryKey: ["properties"] }),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: { user: { id: string } }) => unknown) =>
    selector({ user: { id: "viewer-1" } }),
}));

vi.mock("@multica/core/agents", () => ({
  isAgentRuntimeBound: () => true,
  useAgentPresenceDetail: () => ({ availability: "offline", workload: null }),
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ id: "ws-1", slug: "acme" }),
  useWorkspacePaths: () => ({
    memberDetail: (id: string) => `/acme/members/${id}`,
    agentDetail: (id: string) => `/acme/agents/${id}`,
    squadDetail: (id: string) => `/acme/squads/${id}`,
  }),
}));

const viewState = vi.hoisted(() => ({
  cardProperties: {
    priority: false,
    description: false,
    assignee: true,
    startDate: true,
    dueDate: true,
    project: false,
    childProgress: false,
    labels: false,
  },
  cardPropertyIds: [],
}));

vi.mock("@multica/core/issues/stores/view-store-context", () => ({
  useViewStore: (selector: (state: typeof viewState) => unknown) => selector(viewState),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: (type: string) => `Assigned ${type}`,
    getActorInitials: () => "AA",
    getActorAvatarUrl: () => null,
    getSquadTemplateKey: () => null,
  }),
}));

vi.mock("../../i18n", () => ({
  useT: () => ({ t: (selector: (value: typeof enIssues) => unknown) => selector(enIssues) }),
  useTimeAgo: () => () => "now",
}));

vi.mock("../../agents/components/agent-profile-card", () => ({
  AgentProfileCard: () => null,
}));

vi.mock("../../agents/components/agent-live-peek-card", () => ({
  AgentLivePeekCard: () => null,
}));

vi.mock("../../members/member-profile-card", () => ({
  MemberProfileCard: () => null,
}));

vi.mock("../../squads/components/squad-profile-card", () => ({
  SquadProfileCard: () => null,
}));

vi.mock("./issue-agent-activity-indicator", () => ({
  IssueAgentActivityIndicator: () => null,
}));

const navigation: NavigationAdapter = {
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  pathname: "/acme/issues",
  searchParams: new URLSearchParams(),
  hash: "",
  getShareableUrl: (path) => `https://app.example${path}`,
};

const actions: IssueSurfaceActions = {
  isPending: false,
  createIssue: vi.fn(),
  updateIssue: vi.fn(),
  moveIssue: vi.fn(),
  batchUpdate: vi.fn().mockResolvedValue(undefined),
  batchDelete: vi.fn().mockResolvedValue(undefined),
};

function makeIssue(assigneeType: IssueAssigneeType): Issue {
  return {
    id: `issue-${assigneeType}`,
    workspace_id: "ws-1",
    number: 6082,
    identifier: "MUL-6082",
    title: "Fix Board assignee interaction",
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: assigneeType,
    assignee_id: `${assigneeType}-1`,
    creator_type: "member",
    creator_id: "member-1",
    parent_issue_id: null,
    project_id: null,
    position: 1,
    stage: null,
    start_date: "2026-08-12",
    due_date: "2026-08-13",
    metadata: {},
    properties: {},
    labels: [],
    created_at: "2026-08-12T00:00:00Z",
    updated_at: "2026-08-12T00:00:00Z",
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  viewState.cardProperties.priority = false;
});

describe("BoardCardContent assignee picker", () => {
  // Core field-revision.test.ts owns numeric validity and compatibility.
  it("passes the card observation through real priority and assignee pickers after a revision refresh", async () => {
    viewState.cardProperties.priority = true;
    const issue = { ...makeIssue("member"), revision: 17 };
    const content = (value: Issue) => (
      <NavigationProvider value={navigation}>
        <IssueSurfaceActionsProvider actions={actions}>
          <BoardCardContent issue={value} editable />
        </IssueSurfaceActionsProvider>
      </NavigationProvider>
    );
    const view = render(content(issue));
    fireEvent.click(screen.getByRole("button", { name: "No priority" }));
    fireEvent.click(screen.getByRole("option", { name: "High" }));
    await waitFor(() => expect(actions.updateIssue).toHaveBeenLastCalledWith(
      issue.id, { priority: "high", expected_revision: 17 }, expect.any(Object),
    ));

    fireEvent.click(view.container.querySelector('[data-slot="avatar"]')!);
    fireEvent.click(screen.getByRole("option", { name: "Unassigned" }));
    expect(actions.updateIssue).toHaveBeenLastCalledWith(
      issue.id, { assignee_type: null, assignee_id: null, expected_revision: 17 }, expect.any(Object),
    );

    view.rerender(content({ ...issue, revision: 18 }));
    fireEvent.click(screen.getByRole("button", { name: "No priority" }));
    fireEvent.click(screen.getByRole("option", { name: "Low" }));
    expect(actions.updateIssue).toHaveBeenLastCalledWith(
      issue.id, { priority: "low", expected_revision: 18 }, expect.any(Object),
    );
    expect(navigation.push).not.toHaveBeenCalled();
  });

  it.each<IssueAssigneeType>(["member", "agent", "squad"])(
    "opens the picker from an avatar-only %s assignee without navigating the card",
    (assigneeType) => {
      const issue = makeIssue(assigneeType);
      const { container } = render(
        <NavigationProvider value={navigation}>
          <IssueSurfaceActionsProvider actions={actions}>
            <AppLink href={`/acme/issues/${issue.id}`}>
              <BoardCardContent issue={issue} editable />
            </AppLink>
          </IssueSurfaceActionsProvider>
        </NavigationProvider>,
      );
      const avatar = container.querySelector('[data-slot="avatar"]');

      expect(avatar).not.toBeNull();
      expect(avatar!.closest('[role="link"]')).toBeNull();
      expect(fireEvent.click(avatar!)).toBe(false);
      expect(screen.getByRole("textbox")).toBeInTheDocument();
      expect(navigation.push).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole("option", { name: "Unassigned" }));
      expect(actions.updateIssue).toHaveBeenCalledWith(
        issue.id, { assignee_type: null, assignee_id: null }, expect.any(Object),
      );
    },
  );
});
