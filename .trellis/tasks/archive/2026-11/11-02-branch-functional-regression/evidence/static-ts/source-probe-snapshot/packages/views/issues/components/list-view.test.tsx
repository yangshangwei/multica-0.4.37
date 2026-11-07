import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Issue, IssueStatus, IssueStatusCategory } from "@multica/core/types";
import { useIssueSelectionStore } from "@multica/core/issues/stores/selection-store";
import { ListView } from "./list-view";
import { IssueContextMenuProvider } from "../actions";
import { ScrollRestorationProvider } from "../../platform";
import type { IssueStatusPagination } from "../surface/use-issue-status-branches";
import enCommon from "../../locales/en/common.json";
import enIssues from "../../locales/en/issues.json";

const TEST_RESOURCES = { en: { common: enCommon, issues: enIssues } };

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

const mockGetAgentTaskSnapshot = vi.hoisted(() => vi.fn().mockResolvedValue([]));
vi.mock("@multica/core/api", () => ({
  api: { getAgentTaskSnapshot: mockGetAgentTaskSnapshot },
  getApi: () => ({ getAgentTaskSnapshot: mockGetAgentTaskSnapshot }),
  setApiInstance: vi.fn(),
}));

vi.mock("@multica/core/paths", async () => {
  const actual =
    await vi.importActual<typeof import("@multica/core/paths")>("@multica/core/paths");
  return {
    ...actual,
    useWorkspaceSlug: () => "acme",
    useRequiredWorkspaceSlug: () => "acme",
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});

vi.mock("../../navigation", () => ({
  AppLink: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
  useNavigation: () => ({ push: vi.fn(), pathname: "/issues" }),
  resolveClickIntent: () => "push",
  useIntentNavigate: () => () => {},
  NavigationProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const mockAuthUser = { id: "user-1", email: "test@test.com", name: "Test User" };
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: any) => {
      const state = { user: mockAuthUser, isAuthenticated: true };
      return selector ? selector(state) : state;
    },
    { getState: () => ({ user: mockAuthUser, isAuthenticated: true }) },
  ),
  registerAuthStore: vi.fn(),
  createAuthStore: vi.fn(),
}));

// View store. `listCollapsedStatuses` is mutable so `toggleListCollapsed`
// models the real store: the accordion's expanded set is derived from it.
const mockViewState: {
  sortBy: string;
  sortDirection: string;
  cardProperties: Record<string, boolean>;
  cardPropertyIds: string[];
  listCollapsedStatuses: IssueStatus[];
  toggleListCollapsed: (status: IssueStatus) => void;
} = {
  sortBy: "position",
  sortDirection: "asc",
  cardProperties: {},
  cardPropertyIds: [],
  listCollapsedStatuses: [],
  toggleListCollapsed: vi.fn((status: IssueStatus) => {
    mockViewState.listCollapsedStatuses =
      mockViewState.listCollapsedStatuses.includes(status)
        ? mockViewState.listCollapsedStatuses.filter((s) => s !== status)
        : [...mockViewState.listCollapsedStatuses, status];
  }),
};

vi.mock("@multica/core/issues/stores/view-store-context", () => ({
  ViewStoreProvider: ({ children }: { children: React.ReactNode }) => children,
  useViewStore: (selector?: any) => (selector ? selector(mockViewState) : mockViewState),
  useViewStoreApi: () => ({
    getState: () => mockViewState,
    setState: vi.fn(),
    subscribe: vi.fn(),
  }),
}));

vi.mock("@multica/core/modals", () => ({
  useModalStore: Object.assign(
    () => ({ open: vi.fn() }),
    { getState: () => ({ open: vi.fn() }) },
  ),
}));

// Capture the DndContext callbacks so a test can drive dnd-kit's real
// lifecycle — including the cancel path, which never calls onDragEnd.
let lastOnDragStart: any = null;
let lastOnDragCancel: any = null;
type RecordedDrag = (event: { active: { id: string }; over: { id: string } }) => void;
let lastOnDragOver: RecordedDrag | null = null;
let lastOnDragEnd: RecordedDrag | null = null;
const stableSetNodeRef = () => {};

vi.mock("@dnd-kit/core", () => ({
  DndContext: ({ children, onDragStart, onDragCancel, onDragOver, onDragEnd }: any) => {
    lastOnDragStart = onDragStart;
    lastOnDragCancel = onDragCancel;
    lastOnDragOver = onDragOver;
    lastOnDragEnd = onDragEnd;
    return children;
  },
  DragOverlay: () => null,
  PointerSensor: class {},
  useSensor: () => ({}),
  useSensors: () => [],
  useDroppable: () => ({ setNodeRef: stableSetNodeRef, isOver: false }),
  pointerWithin: vi.fn(),
  closestCenter: vi.fn(),
}));

vi.mock("@dnd-kit/sortable", () => ({
  SortableContext: ({ children }: any) => children,
  verticalListSortingStrategy: {},
  arrayMove: <T,>(arr: T[]) => arr,
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    transition: null,
    isDragging: false,
  }),
}));

vi.mock("@dnd-kit/utilities", () => ({
  CSS: { Transform: { toString: () => undefined } },
}));

// jsdom has no layout, so the real Virtuoso measures a 0-height viewport and
// renders nothing. Render rows inline so the panel's contents are assertable.
vi.mock("react-virtuoso", () => ({
  Virtuoso: ({ data, itemContent, computeItemKey }: any) => (
    <div data-testid="virtuoso-mock">
      {(data ?? []).map((item: any, i: number) => (
        <div key={computeItemKey?.(i, item) ?? i}>{itemContent(i, item)}</div>
      ))}
    </div>
  ),
}));

const ISSUES: Issue[] = [
  {
    id: "issue-1",
    identifier: "MUL-1",
    title: "First todo issue",
    status: "todo",
    priority: "none",
    position: 1,
    workspace_id: "ws-1",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  } as Issue,
  {
    id: "issue-2",
    identifier: "MUL-2",
    title: "Second todo issue",
    status: "todo",
    priority: "none",
    position: 2,
    workspace_id: "ws-1",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  } as Issue,
];

const emptyPage = {
  hasMore: false,
  isLoading: false,
  isFetching: false,
  isError: false,
  loadMore: vi.fn(),
  retry: vi.fn(),
};

const PAGINATION = {
  todo: { ...emptyPage, total: 2 },
  in_review: { ...emptyPage, total: 1 },
} as unknown as IssueStatusPagination;

function renderListView(
  issues: Issue[] = ISSUES,
  visibleStatuses: IssueStatusCategory[] = ["todo"],
  onMoveIssue = vi.fn(),
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider resources={TEST_RESOURCES} locale="en">
        <IssueContextMenuProvider>
          <ScrollRestorationProvider adapter={{ get: () => undefined }}>
            <ListView
              issues={issues}
              visibleStatuses={visibleStatuses}
              statusPagination={PAGINATION}
              onMoveIssue={onMoveIssue}
            />
          </ScrollRestorationProvider>
        </IssueContextMenuProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe("ListView status header collapse", () => {
  beforeEach(() => {
    mockViewState.listCollapsedStatuses = [];
    lastOnDragStart = null;
    lastOnDragCancel = null;
  });

  it("collapses a status group when its header is clicked", async () => {
    const user = userEvent.setup();
    renderListView();

    const trigger = screen.getByRole("button", { expanded: true });
    await user.click(trigger);

    expect(mockViewState.listCollapsedStatuses).toEqual(["todo"]);
  });

  it("still collapses after a drag is cancelled instead of dropped", async () => {
    const user = userEvent.setup();
    renderListView();

    // dnd-kit fires onDragCancel — never onDragEnd — when the browser aborts
    // an active pointer drag. On touch that happens on every scroll gesture
    // that starts on a row: the drag activates past the 5px threshold, then
    // the browser takes the gesture over and fires pointercancel. A cancel
    // that leaves the drag lock engaged makes the header's collapse toggle a
    // permanent no-op (MUL-6240).
    expect(lastOnDragCancel).toBeTypeOf("function");
    act(() => {
      lastOnDragStart({ active: { id: "issue-1" } });
      lastOnDragCancel();
    });

    const trigger = screen.getByRole("button", { expanded: true });
    await user.click(trigger);

    expect(mockViewState.listCollapsedStatuses).toEqual(["todo"]);
  });
});

// Sections are CATEGORIES, cards carry concrete status KEYS. Bucketing a card
// by its key gave a custom status a section id no section has, so the card was
// dropped: filtering the surface down to that status left the section rendering
// "no issues" beside a non-zero header count (MUL-6409). The category mapping
// itself is covered in utils/drag-utils.test.ts.
describe("ListView custom statuses", () => {
  it("renders a custom-status issue in its category's section", () => {
    const custom = {
      ...ISSUES[0]!,
      id: "issue-custom",
      identifier: "MUL-3",
      title: "Waiting on the reporter",
      status: "awaiting_response",
      status_category: "in_review",
    } as Issue;

    renderListView([custom], ["in_review"]);

    expect(screen.getByText("Waiting on the reporter")).toBeInTheDocument();
  });
});

describe("ListView keyboard selection", () => {
  beforeEach(() => {
    mockViewState.listCollapsedStatuses = [];
    useIssueSelectionStore.getState().clear();
  });

  it("reaches named group and row checkboxes by Tab and selects with Space", async () => {
    const user = userEvent.setup();
    renderListView();

    const group = screen.getByRole("checkbox", {
      name: "Select all loaded issues in Todo",
    });
    const first = screen.getByRole("checkbox", { name: "Select MUL-1" });
    const second = screen.getByRole("checkbox", { name: "Select MUL-2" });

    await user.tab();
    expect(group).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { expanded: true })).toHaveFocus();
    await user.tab();
    expect(first).toHaveFocus();
    await user.keyboard(" ");

    expect(first).toBeChecked();
    expect(second).not.toBeChecked();
    expect(group).toBePartiallyChecked();
    expect(mockViewState.listCollapsedStatuses).toEqual([]);
    expect(first).toHaveFocus();

    await user.tab({ shift: true });
    await user.tab({ shift: true });
    expect(group).toHaveFocus();
    await user.keyboard(" ");
    expect(group).toBeChecked();
    expect(first).toBeChecked();
    expect(second).toBeChecked();
    await user.keyboard(" ");
    expect(group).not.toBeChecked();
    expect(first).not.toBeChecked();
    expect(second).not.toBeChecked();
  });
});


describe("ListView non-drag ordering", () => {
  beforeEach(() => {
    mockViewState.sortBy = "position";
    mockViewState.sortDirection = "asc";
    mockViewState.listCollapsedStatuses = [];
  });

  it("moves a row by keyboard, sends sibling anchors and retains focus on failure", async () => {
    const user = userEvent.setup();
    const move = vi.fn();
    renderListView(ISSUES, ["todo"], move);
    const up = screen.getByRole("button", { name: "Move MUL-2 up" });
    up.focus();
    await user.keyboard("{Enter}");
    expect(move).toHaveBeenCalledWith("issue-2", {
      position: 0, before_id: null, after_id: "issue-1",
    }, expect.any(Function));
    await act(async () => move.mock.calls[0]![2](new Error("offline")));
    expect(screen.getByRole("status")).toHaveTextContent(enIssues.page.move_failed);
    expect(screen.getByRole("button", { name: "Move MUL-2 up" })).toHaveFocus();
  });

  it("uses ascending rank anchors while moving upward in a descending manual view", async () => {
    const user = userEvent.setup();
    mockViewState.sortDirection = "desc";
    const move = vi.fn();
    renderListView([...ISSUES].reverse(), ["todo"], move);
    await user.click(screen.getByRole("button", { name: "Move MUL-1 up" }));
    expect(move).toHaveBeenCalledWith("issue-1", {
      position: 3, before_id: "issue-2", after_id: null,
    }, expect.any(Function));
  });

  it("does not expose manual order actions in a sorted view", () => {
    mockViewState.sortBy = "priority";
    renderListView();
    expect(screen.queryByRole("button", { name: /Move MUL-/ })).toBeNull();
  });
});

describe("ListView collaboration drag", () => {
  it("retains the source row while opening a status workflow on drop", () => {
    mockViewState.sortBy = "position";
    mockViewState.listCollapsedStatuses = [];
    const account = { user_id: "external", name: "External handler", avatar_url: "", external: true };
    const shared = { ...ISSUES[0]!, collaboration: { creator: account, handler: account, has_external_collaborators: true } };
    const move = vi.fn();
    const { container } = renderListView([shared, ISSUES[1]!], ["todo", "in_review"], move);
    act(() => lastOnDragStart({ active: { id: shared.id } }));
    const before = container.innerHTML;
    act(() => lastOnDragOver?.({ active: { id: shared.id }, over: { id: "status:in_review" } }));
    expect(container.innerHTML).toBe(before);
    expect(move).not.toHaveBeenCalled();
    act(() => lastOnDragEnd?.({ active: { id: shared.id }, over: { id: "status:in_review" } }));
    expect(move).toHaveBeenCalledWith(shared.id, { status: "in_review", position: shared.position, before_id: null, after_id: null });
  });
});
