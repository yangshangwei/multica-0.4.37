import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError } from "@multica/core/api";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { IssueCompletionAuthorizationModal } from "./issue-completion-authorization";

const mocks = vi.hoisted(() => ({
  candidates: vi.fn(), preview: vi.fn(), authorize: vi.fn(), submit: vi.fn(), recover: vi.fn(),
  revoke: vi.fn(), readDrafts: vi.fn(), retire: vi.fn(), reconcile: vi.fn(), current: true,
}));

vi.mock("@multica/core/issues", () => ({
  completionCandidatesOptions: (_scope: unknown, input: { cursor?: string }) => ({
    queryKey: ["candidates", input.cursor], queryFn: () => mocks.candidates(input), retry: false,
  }),
  completionPreviewOptions: (_scope: unknown, input: { approval_id: string }) => ({
    queryKey: ["preview", input.approval_id], queryFn: () => mocks.preview(input), retry: false,
  }),
  readCompletionDrafts: mocks.readDrafts,
  authorizeIssueCompletion: mocks.authorize,
  submitIssueCompletion: mocks.submit,
  recoverIssueCompletion: mocks.recover,
  revokeIssueCompletion: mocks.revoke,
  retireCompletionDraft: mocks.retire,
  reconcileCompletionResult: mocks.reconcile,
  isCompletionScopeCurrent: () => mocks.current,
  completionDraftScope: (scope: unknown) => scope,
  subscribeCompletionDrafts: () => () => {},
  completionDraftRevision: () => 0,
  issueStatusCategory: (issue: Issue) => issue.status_category ?? issue.status,
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign((selector: (s: { user: { id: string } }) => unknown) =>
    selector({ user: { id: "user-1" } }), { getState: () => ({ user: { id: "user-1" } }) }),
}));
vi.mock("../i18n", async () => {
  const { default: messages } = await import("../locales/en/issues.json");
  return { useT: () => ({ t: (selector: (m: typeof messages) => string) => selector(messages) }) };
});
vi.mock("../issues/utils/status-label", () => ({ useStatusLabel: () => (status: string) => status }));

const issue: Issue = {
  id: "issue-1", workspace_id: "ws-1", number: 1, identifier: "MUL-1", title: "Review this delivery",
  description: null, status: "in_review", status_category: "in_review", priority: "medium",
  assignee_type: null, assignee_id: null, creator_type: "member", creator_id: "user-1",
  parent_issue_id: null, project_id: "project-1", position: 1, stage: null, start_date: null,
  due_date: null, metadata: {}, properties: {}, created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z", revision: 4,
};
const scope = { server: "test", userId: "user-1", workspaceId: "ws-1", issueId: "issue-1", apiScope: {} };
const candidate = { approval_id: "approval-1", execution_id: "execution-1", summary: "Reviewed artifact",
  source_revision: "a".repeat(40), spec_digest: "b".repeat(64), artifact_digest: "c".repeat(64),
  receipt_digest: "d".repeat(64), created_at: "2026-11-12T00:00:00Z", expires_at: "2099-11-12T12:00:00Z",
  status: "executed", result_code: "ok", selectable: true };
const preview = { protocol_version: 1, issue_id: issue.id, issue_revision: 4, current_status: "in_review",
  current_category: "in_review", target_status: "done", target_category: "done", project_id: "project-1",
  project_revision: 3, policy_mode: "enforce", policy_revision: 2, checked_at: "2099-11-12T10:00:00Z",
  reason: "authorization_available", observation: { verdict: "pass", reason: "matching", observed_at: "2099-11-12T10:00:00Z",
    valid_until: "2099-11-12T12:00:00Z", source_revision: candidate.source_revision, spec_digest: candidate.spec_digest,
    artifact_digest: candidate.artifact_digest, observed_head_sha: candidate.source_revision, revision_provenance: "database_observed_pr_head_not_remote_verified" },
  authorization_binding: { consumer: { kind: "human", user_id: "user-1" } }, max_expires_at: "2099-11-12T12:00:00Z",
  enforced: false, usable_for_completion: false, skip_supported: false };
const draft = { version: 1, scope, requestKey: "request-1", createdAt: 1, creationDispatched: true,
  authorizationId: "authorization-1", submissionDispatched: false, revocation: null,
  request: { operation_id: "operation-1", target: { issue_id: issue.id, expected_issue_revision: 4, target_status: "done" }, reason: "Reviewed", expires_at: "2099-11-12T10:15:00Z" } };
const result = (outcome: string, overrides = {}) => ({ outcome, draft,
  authorization: { authorization_id: "authorization-1", state: "issued", applicability: "not_checked", expires_at: "2099-11-12T10:15:00Z" },
  issue: null, currentIssueRead: "not_requested", reason: null, submittedNow: false, ...overrides });

function setup(overrides = {}) {
  const data = { issue, scope, targetStatus: "done", ownerSignal: new AbortController().signal,
    onCompleted: vi.fn(), onCancel: vi.fn(), onRefresh: vi.fn(), ...overrides };
  const onClose = vi.fn();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const view = render(<QueryClientProvider client={client}><IssueCompletionAuthorizationModal data={data} onClose={onClose} /></QueryClientProvider>);
  return { ...view, data, onClose };
}

beforeEach(() => {
  Object.values(mocks).forEach((value) => { if (typeof value === "function" && "mockReset" in value) value.mockReset(); });
  mocks.current = true;
  mocks.readDrafts.mockReturnValue([]);
  mocks.candidates.mockResolvedValue({ items: [candidate], issue_revision: 4, next_cursor: null, has_more: false });
  mocks.preview.mockResolvedValue(preview);
  mocks.authorize.mockResolvedValue(result("authorized"));
  mocks.submit.mockResolvedValue(result("consumed", { issue: { ...issue, status: "done", status_category: "done", revision: 5 }, currentIssueRead: "ok", submittedNow: true }));
});

describe("IssueCompletionAuthorizationModal", () => {
  // Core owns the complete outcome/state matrix. These tests exercise the
  // shared view's explicit actions, original-intent wiring and lifecycle.
  it("requires evidence selection and separate authorization and submission", async () => {
    const { data } = setup();
    await screen.findByRole("radio", { name: /Reviewed artifact/ });
    expect(screen.getByText("executed · ok")).toBeInTheDocument();
    expect(mocks.authorize).not.toHaveBeenCalled();
    expect(mocks.preview).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("radio", { name: /Reviewed artifact/ }));
    await screen.findByLabelText("Acceptance reason");
    fireEvent.change(screen.getByLabelText("Acceptance reason"), { target: { value: "I reviewed this exact artifact" } });
    fireEvent.click(screen.getByRole("button", { name: "Authorize this version" }));
    await screen.findByRole("button", { name: "Submit completion" });
    expect(mocks.authorize).toHaveBeenCalledWith(scope, preview, expect.objectContaining({ reason: "I reviewed this exact artifact" }), expect.anything());
    expect(mocks.submit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Submit completion" }));
    await waitFor(() => expect(data.onCompleted).toHaveBeenCalledTimes(1));
    expect(mocks.submit).toHaveBeenCalledWith(scope, draft, expect.anything());
    expect(mocks.reconcile).toHaveBeenCalled();
  });

  it("distinguishes denied evidence reads from ordinary read failures", async () => {
    mocks.candidates.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    setup();
    await screen.findByRole("alert");
    expect(screen.getByRole("alert")).toHaveTextContent("Access to current delivery evidence is unavailable.");
  });

  it("keeps expired evidence visible but unavailable for authorization", async () => {
    mocks.candidates.mockResolvedValueOnce({ items: [{ ...candidate, selectable: false, expires_at: "2026-11-11T00:00:00Z" }], issue_revision: 4, next_cursor: null, has_more: false });
    setup();
    const choice = await screen.findByRole("radio", { name: /Reviewed artifact/ });
    expect(choice).toBeDisabled();
    expect(screen.getByText("Expired")).toBeInTheDocument();
    expect(screen.queryByText("No accessible delivery evidence is available.")).not.toBeInTheDocument();
  });

  it("closes before authorization without creating or revoking a grant", async () => {
    const { onClose, data } = setup();
    fireEvent.click(screen.getAllByRole("button", { name: "Close" })[0]!);
    expect(data.onCancel).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mocks.authorize).not.toHaveBeenCalled();
    expect(mocks.revoke).not.toHaveBeenCalled();
  });

  it("restores an original submitted intent for lookup only", async () => {
    const submitted = { ...draft, submissionDispatched: true };
    mocks.readDrafts.mockReturnValue([submitted]);
    mocks.recover.mockResolvedValue(result("unknown", { draft: submitted }));
    const { data } = setup();
    fireEvent.click(await screen.findByRole("button", { name: "Query original result" }));
    await waitFor(() => expect(mocks.recover).toHaveBeenCalledWith(scope, submitted, expect.anything()));
    expect(screen.queryByRole("button", { name: "Submit completion" })).not.toBeInTheDocument();
    expect(mocks.submit).not.toHaveBeenCalled();
    expect(mocks.authorize).not.toHaveBeenCalled();
    expect(data.onCompleted).not.toHaveBeenCalled();
  });

  it("does not navigate from a recovered historical completion after reopen", async () => {
    mocks.readDrafts.mockReturnValue([{ ...draft, submissionDispatched: true }]);
    mocks.recover.mockResolvedValue(result("consumed", { issue: { ...issue, status: "todo", status_category: "todo", revision: 6 }, currentIssueRead: "ok" }));
    const { data } = setup();
    fireEvent.click(await screen.findByRole("button", { name: "Query original result" }));
    await screen.findByText("Completion was recorded for this operation.");
    expect(screen.getByText("todo")).toBeInTheDocument();
    expect(data.onCompleted).not.toHaveBeenCalled();
    expect(mocks.submit).not.toHaveBeenCalled();
  });

  it("does not continue a late write response after unmount", async () => {
    let finish!: (value: ReturnType<typeof result>) => void;
    mocks.authorize.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    const { unmount, data } = setup();
    fireEvent.click(await screen.findByRole("radio", { name: /Reviewed artifact/ }));
    fireEvent.change(await screen.findByLabelText("Acceptance reason"), { target: { value: "Reviewed" } });
    fireEvent.click(screen.getByRole("button", { name: "Authorize this version" }));
    await waitFor(() => expect(mocks.authorize).toHaveBeenCalledTimes(1));
    unmount();
    await act(async () => { finish(result("authorized")); });
    expect(mocks.submit).not.toHaveBeenCalled();
    expect(data.onCompleted).not.toHaveBeenCalled();
  });

  it("requires an explicit revocation reason and does not revoke on close", async () => {
    mocks.readDrafts.mockReturnValue([draft]);
    mocks.recover.mockResolvedValue(result("authorized"));
    mocks.revoke.mockResolvedValue(result("revoked"));
    setup();
    fireEvent.click(await screen.findByRole("button", { name: "Query original result" }));
    await screen.findByRole("button", { name: "Revoke authorization" });
    expect(screen.getByRole("button", { name: "Revoke authorization" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Revocation reason"), { target: { value: "Acceptance withdrawn" } });
    fireEvent.click(screen.getByRole("button", { name: "Revoke authorization" }));
    await waitFor(() => expect(mocks.revoke).toHaveBeenCalledWith(scope, draft, "Acceptance withdrawn", expect.anything()));
    expect(mocks.submit).not.toHaveBeenCalled();
  });
});
