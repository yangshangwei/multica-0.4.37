import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import en from "../locales/en/triage.json";
import { TriagePage } from "./triage-page";

const state = vi.hoisted(() => ({
  enabled: true,
  replace: vi.fn(),
  params: new URLSearchParams("issue=a"),
  rows: ["a", "b"],
}));
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (selector: (value: typeof en) => string) => selector(en),
    i18n: { language: "en" },
  }),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    triage: () => "/acme/triage",
    settings: () => "/acme/settings",
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "member" }),
}));
vi.mock("../navigation", () => ({
  useNavigation: () => ({ searchParams: state.params, replace: state.replace }),
  AppLink: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
vi.mock("../layout/page-header", () => ({
  PageHeader: ({ children }: { children: React.ReactNode }) => (
    <header>{children}</header>
  ),
}));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsCompact: () => false }));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizablePanel: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizableHandle: () => null,
}));
vi.mock("../issues/components/issue-detail", () => ({
  IssueDetail: () => <input aria-label="Description" />,
}));
vi.mock("./triage-fields", () => ({ TriageSelect: () => null }));
vi.mock("./triage-filters", () => ({ TriageFilters: () => null }));
vi.mock("./triage-history", () => ({
  TriageItemHistory: () => null,
  TriageHistoryRecord: () => null,
  TriageExecutionStatus: () => null,
  TriageDuplicateTarget: () => null,
}));
vi.mock("./triage-create-dialog", () => ({
  TriageCreateDialog: () => <div role="dialog">Create form</div>,
}));
vi.mock("./triage-import-dialog", () => ({
  TriageImportDialog: ({
    batchId,
    onClose,
  }: {
    batchId?: string;
    onClose: () => void;
  }) => (
    <div role="dialog">
      <span>{batchId}</span>
      <button onClick={onClose}>Close import</button>
    </div>
  ),
}));
vi.mock("./triage-batch-dialog", () => ({ TriageBatchDialog: () => null }));
vi.mock("./triage-action-dialog", () => ({
  TriageActionDialog: ({
    action,
    onSuccess,
    onClose,
  }: {
    action: string;
    onSuccess: (value: unknown) => void;
    onClose: () => void;
  }) => (
    <div role="dialog">
      <span>{`Action ${action}`}</span>
      <button
        onClick={() => {
          onSuccess({
            item: { issue: { id: "a" } },
            action: { action, execution_status: "not_requested" },
          });
          onClose();
        }}
      >
        Confirm decision
      </button>
    </div>
  ),
}));
vi.mock("@tanstack/react-query", async (original) => ({
  ...(await original<typeof import("@tanstack/react-query")>()),
  useQuery: ({ queryKey }: { queryKey: string[] }) => {
    const kind = queryKey[2];
    const item = (id: string) => ({
      issue: {
        id,
        identifier: `MUL-${id}`,
        title: `Task ${id}`,
        admission_status: "pending",
        revision: 1,
      },
      entered_at: "2026-10-04T10:00:00Z",
      first_entered_at: "2026-10-04T10:00:00Z",
      source: "manual",
      round: 1,
    });
    if (kind === "settings")
      return {
        data: {
          supported: true,
          enabled: state.enabled,
          acceptance_status: "backlog",
        },
      };
    if (kind === "counts")
      return {
        data: {
          pending: state.rows.length,
          ready: state.rows.length,
          snoozed: 0,
        },
      };
    if (kind === "list")
      return {
        data: { items: state.rows.map(item), total: state.rows.length },
      };
    if (kind === "detail") return { data: item("a") };
    if (kind === "history") return { data: { entries: [], total: 0 } };
    return { data: [] };
  },
}));

beforeEach(() => {
  state.enabled = true;
  state.rows = ["a", "b"];
  state.params = new URLSearchParams("issue=a");
  state.replace.mockClear();
});
it("keeps history available but hides intake controls when disabled", () => {
  state.enabled = false;
  render(<TriagePage />);
  expect(
    screen.queryByRole("button", { name: "Create pending task" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "History" })).toBeVisible();
  expect(screen.getByText(en.disabled)).toBeVisible();
});
it("runs queue shortcuts only from queue focus and advances only after a confirmed decision", () => {
  render(<TriagePage />);
  const editor = screen.getByRole("textbox", { name: "Description" });
  editor.focus();
  fireEvent.keyDown(editor, { key: "3" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  const row = screen.getByRole("button", { name: /MUL-a Task a/ });
  row.focus();
  fireEvent.keyDown(row, { key: "3" });
  expect(screen.getByText("Action reject")).toBeVisible();
  expect(state.replace).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  expect(state.replace).toHaveBeenCalledWith("/acme/triage?issue=b");
});

it("opens a notification batch deep link and clears it on close", () => {
  state.params = new URLSearchParams("view=history&batch=batch-1");
  render(<TriagePage />);
  expect(screen.getByRole("dialog")).toHaveTextContent("batch-1");
  fireEvent.click(screen.getByRole("button", { name: "Close import" }));
  expect(state.replace).toHaveBeenCalledWith("/acme/triage?view=history");
});

it("keeps a cleared search as the latest intent through delayed route acknowledgements and a review decision", () => {
  const { rerender } = render(<TriagePage />);
  const search = screen.getByRole("textbox", { name: en.search });
  fireEvent.change(search, { target: { value: "jk123" } });
  fireEvent.change(search, { target: { value: "jk1231" } });
  fireEvent.change(search, { target: { value: "" } });
  // The first platform replacement is still pending while later input accumulates.
  expect(state.replace).toHaveBeenCalledTimes(1);
  state.params = new URLSearchParams("issue=a&q=jk123");
  rerender(<TriagePage />);
  expect(search).toHaveValue("");
  expect(state.replace).toHaveBeenLastCalledWith("/acme/triage?issue=a");
  state.params = new URLSearchParams("issue=a");
  rerender(<TriagePage />);
  fireEvent.click(screen.getByRole("button", { name: "Accept" }));
  fireEvent.click(screen.getByRole("button", { name: "Confirm decision" }));
  const target = state.replace.mock.calls.at(-1)?.[0] as string;
  expect(new URL(target, "https://example.test").searchParams.has("q")).toBe(
    false,
  );
  expect(target).toBe("/acme/triage?issue=b");
});

it("adopts external route changes instead of pinning the original filters", () => {
  const { rerender } = render(<TriagePage />);
  state.params = new URLSearchParams("view=snoozed&q=external");
  rerender(<TriagePage />);
  expect(screen.getByRole("textbox", { name: en.search })).toHaveValue(
    "external",
  );
});
