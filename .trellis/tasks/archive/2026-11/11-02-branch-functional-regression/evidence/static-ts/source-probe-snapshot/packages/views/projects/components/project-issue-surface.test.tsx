import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue, Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ProjectIssueSurface } from "./project-issue-surface";

const mocks = vi.hoisted(() => ({ listIssues: vi.fn(), updateIssue: vi.fn(), toastSuccess: vi.fn(), toastError: vi.fn() }));
vi.mock("@multica/core/api", async (original) => ({
  ...(await original<typeof import("@multica/core/api")>()),
  api: { listIssues: mocks.listIssues },
}));
vi.mock("@multica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutateAsync: mocks.updateIssue }),
}));
vi.mock("@multica/core/issue-statuses/hooks", () => ({
  useIssueStatuses: () => ({ entryOf: (key: string) => key === "needs_design" ? { name: "Design review" } : undefined }),
}));
vi.mock("sonner", () => ({ toast: { success: mocks.toastSuccess, error: mocks.toastError } }));
vi.mock("../../issues/surface/issue-surface", () => ({
  IssueSurface: ({ isScopeEmpty, headerActions, renderEmpty }: {
    isScopeEmpty: boolean;
    headerActions: (context: { controller: { openCreateIssue: () => void } }) => ReactNode;
    renderEmpty: (context: { controller: { openCreateIssue: () => void } }) => ReactNode;
  }) => {
    const context = { controller: { openCreateIssue: vi.fn() } };
    return <>{isScopeEmpty ? renderEmpty(context) : headerActions(context)}</>;
  },
}));

const project: Project = {
  id: "project-1", workspace_id: "workspace-1", title: "Launch", description: null,
  icon: null, status: "planned", priority: "none", lead_type: null, lead_id: null,
  start_date: null, due_date: null, created_at: "2026-10-11", updated_at: "2026-10-11",
  issue_count: 0, done_count: 0, resource_count: 0,
};
const issue: Issue = {
  id: "candidate", workspace_id: "workspace-1", number: 12, identifier: "MUL-12",
  title: "Review the launch design", description: null, status: "needs_design", status_category: "in_review",
  priority: "medium", assignee_type: "member", assignee_id: "member-1", creator_type: "member", creator_id: "member-1",
  parent_issue_id: null, project_id: null, position: 0, stage: null, start_date: null, due_date: null,
  metadata: {}, properties: {}, created_at: "2026-10-11", updated_at: "2026-10-11",
};

describe("ProjectIssueSurface candidate browsing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listIssues.mockResolvedValue({ issues: [issue], total: 1 });
    mocks.updateIssue.mockResolvedValue({ ...issue, project_id: project.id });
  });

  // Eligibility and pagination matrices live in core/projects/issue-candidates.test.ts.
  it.each([
    { issueCount: 0, locale: "en" as const, linkLabel: "Link an existing issue" },
    { issueCount: 3, locale: "zh-Hans" as const, linkLabel: "关联已有任务" },
  ])("opens a browsable picker from the $locale project with $issueCount tasks", async ({ issueCount, locale, linkLabel }) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
    renderWithI18n(<QueryClientProvider client={client}>
      <ProjectIssueSurface project={{ ...project, issue_count: issueCount }}
        scope={{ type: "project", projectId: project.id }} />
    </QueryClientProvider>, { locale });
    expect(mocks.listIssues).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: linkLabel }));

    const candidate = await screen.findByRole("option", { name: /Review the launch design/ });
    expect(candidate).toHaveTextContent("MUL-12");
    expect(candidate).toHaveTextContent("Design review");
    expect(screen.getByRole("combobox")).toHaveValue("");
    expect(mocks.listIssues).toHaveBeenCalledWith(expect.objectContaining({ workspace_id: "workspace-1", include_no_project: true, q: "" }));
    fireEvent.click(candidate);

    expect(mocks.updateIssue).toHaveBeenCalledExactlyOnceWith({ id: issue.id, project_id: project.id });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(mocks.toastSuccess).toHaveBeenCalledTimes(1);
    client.clear();
  });
});
