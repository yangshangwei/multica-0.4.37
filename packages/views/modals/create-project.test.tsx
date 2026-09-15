import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";

const longRepoUrl =
  "https://github.com/multica-ai/a-very-long-repository-name-that-needs-a-tooltip";
const apiRepoUrl = "https://github.com/multica-ai/api";
const webRepoUrl = "https://github.com/multica-ai/web";

const mocks = vi.hoisted(() => ({
  createProject: vi.fn(),
  push: vi.fn(),
  setDraft: vi.fn(),
  clearDraft: vi.fn(),
  draft: {} as Record<string, unknown>,
  workspaceId: "workspace-1",
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: () => ({ data: [] }),
  // The modal now reads the runtime list to gate worktree mode, and
  // runtimeListOptions builds its descriptor with queryOptions.
  queryOptions: (options: unknown) => options,
}));

vi.mock("@multica/core/projects/mutations", () => ({
  useCreateProject: () => ({ mutateAsync: mocks.createProject }),
}));

vi.mock("@multica/core/projects", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@multica/core/projects"),
  useProjectDraftStore: Object.assign((selector: (state: unknown) => unknown) =>
    selector({
      draft: mocks.draft,
      setDraft: mocks.setDraft,
      clearDraft: mocks.clearDraft,
    }), { getState: () => ({ draft: mocks.draft }) }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => mocks.workspaceId,
}));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({
    id: "workspace-1",
    name: "Test Workspace",
    slug: "test-workspace",
    repos: [{ url: longRepoUrl }, { url: apiRepoUrl }, { url: webRepoUrl }],
  }),
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/test-workspace/projects/${id}`,
  }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"], queryFn: vi.fn() }),
  agentListOptions: () => ({ queryKey: ["agents"], queryFn: vi.fn() }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getActorName: vi.fn() }),
}));

vi.mock("../navigation", () => ({
  useNavigation: () => ({ push: mocks.push }),
}));

vi.mock("../editor", () => {
  const ContentEditor = React.forwardRef<{ getMarkdown: () => string }, { placeholder?: string }>(
    ({ placeholder }, ref) => {
      React.useImperativeHandle(ref, () => ({ getMarkdown: () => "Project context" }));
      return <textarea placeholder={placeholder} />;
    },
  );
  ContentEditor.displayName = "ContentEditor";

  return {
    ContentEditor,
    TitleEditor: ({
      placeholder,
      onChange,
    }: {
      placeholder?: string;
      onChange?: (value: string) => void;
    }) => <input placeholder={placeholder} onChange={(e) => onChange?.(e.target.value)} />,
  };
});

// The picker owns eligibility and runtime selection coverage. These assertions
// pin the modal's prefill, draft, and create-request wiring at its boundary.
vi.mock("../projects/components/project-squad-picker", () => ({
  ProjectSquadPicker: ({ value, onChange }: {
    value: Record<string, unknown>;
    onChange: (value: Record<string, unknown>) => void;
  }) => <div>
    <output aria-label="Execution squad">{JSON.stringify(value)}</output>
    <button onClick={() => onChange({})}>Choose no squad</button>
  </div>,
}));

vi.mock("../issues/components/priority-icon", () => ({
  PriorityIcon: () => <span data-testid="priority-icon" />,
}));

vi.mock("../common/actor-avatar", () => ({
  ActorAvatar: () => <span data-testid="actor-avatar" />,
}));

// Stub the date pickers so this test doesn't pull the real Calendar (and its
// buttonVariants import) into the modal's module graph; the pickers have their
// own test. The stubs render the placeholder label so the pills are assertable.
vi.mock("../projects/components/project-start-date-picker", () => ({
  ProjectStartDatePicker: () => <button type="button">Start date</button>,
}));

vi.mock("../projects/components/project-due-date-picker", () => ({
  ProjectDueDatePicker: () => <button type="button">Due date</button>,
}));

vi.mock("@multica/ui/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div role="tooltip">{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/button", () => ({
  Button: ({
    children,
    disabled,
    onClick,
    type = "button",
  }: {
    children: React.ReactNode;
    disabled?: boolean;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
  }) => (
    <button type={type} disabled={disabled} onClick={onClick}>
      {children}
    </button>
  ),
}));

vi.mock("@multica/ui/components/common/emoji-picker", () => ({
  EmojiPicker: () => null,
}));

vi.mock("@multica/ui/lib/utils", () => ({
  cn: (...values: Array<string | false | null | undefined>) =>
    values.filter(Boolean).join(" "),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { CreateProjectModal } from "./create-project";

beforeEach(() => {
  vi.clearAllMocks();
  mocks.draft = { title: "", description: "", status: "planned", priority: "medium" };
  mocks.workspaceId = "workspace-1";
  mocks.setDraft.mockImplementation((partial) => { mocks.draft = { ...mocks.draft, ...partial }; });
  mocks.createProject.mockResolvedValue({ id: "project-1", execution_squad: { state: "needs_runtime", template_key: "feature-delivery" } });
});

describe("CreateProjectModal", () => {
  it("makes the required project title explicit in Chinese", () => {
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />, { locale: "zh-Hans" });

    expect(screen.getByPlaceholderText("请输入项目标题（必填）")).toBeInTheDocument();
  });

  it("exposes full repository URLs in the repository picker", () => {
    render(<CreateProjectModal onClose={vi.fn()} />);

    // The Tooltip is the single reveal mechanism. A native `title` carrying the
    // same URL would stack a browser tooltip on top of it (MUL-4836).
    expect(screen.getByRole("tooltip", { name: longRepoUrl })).toBeInTheDocument();
    expect(screen.queryByTitle(longRepoUrl)).toBeNull();
  });

  it("reveals the start/due date pickers from the ⋯ overflow menu", async () => {
    const user = userEvent.setup();
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    // Dates are collapsed behind the overflow by default (progressive disclosure).
    expect(screen.queryByRole("button", { name: "Start date" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Due date" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Set start date/ }));
    expect(screen.getByRole("button", { name: "Start date" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Set due date/ }));
    expect(screen.getByRole("button", { name: "Due date" })).toBeInTheDocument();
  });

  it("filters workspace repositories by search text", async () => {
    const user = userEvent.setup();

    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);

    const repoSearchInput = screen.getByRole("textbox", { name: "Search repositories..." });

    await user.type(repoSearchInput, "api");

    expect(
      screen.getByRole("button", { name: (name) => name.includes(apiRepoUrl) }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: (name) => name.includes(webRepoUrl) }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: (name) => name.includes(longRepoUrl) }),
    ).not.toBeInTheDocument();

    await user.clear(repoSearchInput);
    await user.type(repoSearchInput, "no-match");

    expect(screen.getByText("No repositories match your search.")).toBeInTheDocument();
  });

  it("shows resources and the default squad before the property toolbar", () => {
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);
    const resources = screen.getByRole("group", { name: "Resources" });
    const squad = screen.getByLabelText("Execution squad");
    expect(squad).toHaveTextContent('"template_key":"feature-delivery"');
    expect(resources.compareDocumentPosition(squad) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(squad.compareDocumentPosition(screen.getByRole("button", { name: "More options" })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("creates the project with its squad choice even without a runtime", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderWithI18n(<CreateProjectModal onClose={onClose} />);
    await user.type(screen.getByPlaceholderText("Project title"), "Release");
    await user.click(screen.getByRole("button", { name: "Create Project" }));

    await waitFor(() => expect(mocks.createProject).toHaveBeenCalledWith(expect.objectContaining({
      title: "Release", execution_squad: { template_key: "feature-delivery", language: "en" },
    })));
    expect(mocks.push).toHaveBeenCalledWith("/test-workspace/projects/project-1");
    expect(onClose).toHaveBeenCalledOnce();
  });

  it.each([
    [{ squad_template_key: "bug-triage" }, { template_key: "bug-triage" }],
    [{ squad_id: "squad-1" }, { squad_id: "squad-1" }],
  ])("honours a catalog prefill before a retained draft: %j", (data, expected) => {
    mocks.draft.executionSquad = { template_key: "feature-delivery", runtime_id: "old-runtime" };
    renderWithI18n(<CreateProjectModal data={data} onClose={vi.fn()} />);
    expect(JSON.parse(screen.getByLabelText("Execution squad").textContent!)).toEqual(expected);
  });

  it("persists an explicit no-squad choice instead of reinstating the default", async () => {
    const user = userEvent.setup();
    mocks.draft.executionSquad = null;
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);
    expect(screen.getByLabelText("Execution squad")).toHaveTextContent("{}");
    await user.click(screen.getByRole("button", { name: "Choose no squad" }));
    await user.type(screen.getByPlaceholderText("Project title"), "Solo");
    await user.click(screen.getByRole("button", { name: "Create Project" }));
    expect(mocks.setDraft).toHaveBeenCalledWith({ executionSquad: {} });
    expect(mocks.createProject).toHaveBeenCalledWith(expect.objectContaining({ execution_squad: {} }));
  });

  it("opens the created project when preparation failed so it can be retried there", async () => {
    const user = userEvent.setup();
    mocks.createProject.mockResolvedValue({ id: "saved-project", execution_squad: { state: "failed", error_code: "preparation_failed" } });
    renderWithI18n(<CreateProjectModal onClose={vi.fn()} />);
    await user.type(screen.getByPlaceholderText("Project title"), "Retained project");
    await user.click(screen.getByRole("button", { name: "Create Project" }));
    await waitFor(() => expect(mocks.push).toHaveBeenCalledWith("/test-workspace/projects/saved-project"));
    expect(mocks.createProject).toHaveBeenCalledOnce();
    expect(mocks.clearDraft).toHaveBeenCalledOnce();
  });

  it("does not navigate or consume a new draft after the submitting modal unmounts", async () => {
    const user = userEvent.setup();
    let resolve!: (project: { id: string }) => void;
    const pending = new Promise<{ id: string }>((accept) => { resolve = accept; });
    mocks.createProject.mockReturnValue(pending);
    const onClose = vi.fn();
    const { unmount } = renderWithI18n(<CreateProjectModal onClose={onClose} />);
    await user.type(screen.getByPlaceholderText("Project title"), "Original project");
    await user.click(screen.getByRole("button", { name: "Create Project" }));
    unmount();
    mocks.draft = { ...mocks.draft, title: "Reopened project draft" };

    await act(async () => { resolve({ id: "saved-project" }); await pending; });

    expect(mocks.clearDraft).not.toHaveBeenCalled();
    expect(mocks.push).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.draft.title).toBe("Reopened project draft");
  });

  it("ignores late success after the workspace changes while the modal remains mounted", async () => {
    const user = userEvent.setup();
    let resolve!: (project: { id: string }) => void;
    const pending = new Promise<{ id: string }>((accept) => { resolve = accept; });
    mocks.createProject.mockReturnValue(pending);
    const onClose = vi.fn();
    const view = renderWithI18n(<CreateProjectModal onClose={onClose} />);
    await user.type(screen.getByPlaceholderText("Project title"), "Original project");
    await user.click(screen.getByRole("button", { name: "Create Project" }));
    mocks.workspaceId = "workspace-2";
    view.rerender(<CreateProjectModal onClose={onClose} />);

    await act(async () => { resolve({ id: "saved-project" }); await pending; });

    expect(mocks.clearDraft).not.toHaveBeenCalled();
    expect(mocks.push).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("keeps edits made to the shared draft while the request is pending", async () => {
    const user = userEvent.setup();
    let resolve!: (project: { id: string }) => void;
    const pending = new Promise<{ id: string }>((accept) => { resolve = accept; });
    mocks.createProject.mockReturnValue(pending);
    const onClose = vi.fn();
    renderWithI18n(<CreateProjectModal onClose={onClose} />);
    const title = screen.getByPlaceholderText("Project title");
    await user.type(title, "Original project");
    await user.click(screen.getByRole("button", { name: "Create Project" }));
    await user.clear(title);
    await user.type(title, "New project draft");

    await act(async () => { resolve({ id: "saved-project" }); await pending; });

    expect(mocks.clearDraft).not.toHaveBeenCalled();
    expect(mocks.push).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(mocks.draft.title).toBe("New project draft");
  });
});
