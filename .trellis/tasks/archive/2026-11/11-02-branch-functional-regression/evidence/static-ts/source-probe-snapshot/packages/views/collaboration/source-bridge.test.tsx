import { useState, type ComponentProps } from "react";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { SourceCollaborationActionDialog, SourceCollaborationCreation, SourceCollaborationDetail } from "./source-bridge";

const state = vi.hoisted(() => ({ transfer: true, writeContext: true, error: null as Error | null, refetch: vi.fn() }));
vi.mock("@multica/core/collaboration", async (original) => ({
  ...await original<typeof import("@multica/core/collaboration")>(),
  useCollaborationScope: () => ({ server: "https://server.example", apiScope: "account", userId: "viewer" }),
  useCollaborationIssue: () => ({ isPending: false, error: state.error, refetch: state.refetch, data: { ...collaborationIssueFixture,
    write_context: state.writeContext ? { actor_user_id: "viewer", authority_epoch: "1" } : null,
    capabilities: { ...collaborationIssueFixture.capabilities, transfer: state.transfer } } }),
}));
vi.mock("@multica/core/realtime", () => ({ useCollaborationRealtime: vi.fn() }));
vi.mock("./creation-form", () => ({ CollaborationCreationEntry: ({ sourceWorkspaceId }: { sourceWorkspaceId?: string }) => <button>New collaboration in {sourceWorkspaceId}</button> }));
vi.mock("./collaboration-detail", () => ({ CollaborationDetail: ({ destination }: { destination: { issueId: string; commentId?: string; runId?: string } }) => <p data-testid="shared-detail" data-run-id={destination.runId}>{destination.issueId}:{destination.commentId}</p> }));
vi.mock("./workflow", () => ({ CollaborationWorkflow: ({ initialAction, initialStatus }: { initialAction?: string; initialStatus?: string }) => {
  const [draft, setDraft] = useState("");
  return <form aria-label="shared workflow">{initialAction}:{initialStatus}<input aria-label="Workflow draft" value={draft} onChange={(event) => setDraft(event.target.value)} /></form>;
} }));
vi.mock("../navigation", () => ({ AppLink: (props: ComponentProps<"a">) => <a {...props} /> }));

describe("source collaboration bridge", () => {
  beforeEach(() => { state.transfer = true; state.writeContext = true; state.error = null; state.refetch.mockClear(); });
  afterEach(cleanup);
  it("reuses creation with the explicit source workspace", () => {
    renderWithI18n(<SourceCollaborationCreation workspaceId="source-workspace" />);
    expect(screen.getByRole("button", { name: "New collaboration in source-workspace" })).toBeInTheDocument();
  });
  it("keeps the same UUID and comment anchor in shared source detail", () => {
    const commentId = "00000000-0000-4000-8000-000000000002";
    renderWithI18n(<SourceCollaborationDetail issueId={collaborationIssueFixture.id} commentId={commentId} />);
    expect(screen.getByTestId("shared-detail")).toHaveTextContent(`${collaborationIssueFixture.id}:${commentId}`);
    expect(screen.getByRole("link", { name: "Open in collaboration" })).toHaveAttribute("href", `/account?view=participating&issue=${collaborationIssueFixture.id}#comment-${commentId}`);
  });
  it("passes a selected run through shared detail and its account link", () => {
    const runId = "00000000-0000-4000-8000-000000000010";
    renderWithI18n(<SourceCollaborationDetail issueId={collaborationIssueFixture.id} runId={runId} />);
    expect(screen.getByTestId("shared-detail")).toHaveAttribute("data-run-id", runId);
    expect(screen.getByRole("link", { name: "Open in collaboration" })).toHaveAttribute("href", `/account?view=participating&issue=${collaborationIssueFixture.id}#run-${runId}`);
  });
  it("opens a scoped workflow after capability revalidation and closes accessibly", () => {
    const close = vi.fn();
    renderWithI18n(<SourceCollaborationActionDialog action={{ issueId: collaborationIssueFixture.id, action: "transfer" }} close={close} />);
    expect(screen.getByRole("form", { name: "shared workflow" })).toHaveTextContent("transfer");
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(close).toHaveBeenCalledOnce();
  });
  it("does not expose ordinary assignment when the cached actor is no longer the handler", () => {
    state.transfer = false;
    renderWithI18n(<SourceCollaborationActionDialog action={{ issueId: collaborationIssueFixture.id, action: "transfer" }} close={() => {}} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Only the current handler or creator");
    expect(screen.queryByRole("form")).not.toBeInTheDocument();
  });
  it("does not show write controls without the current write context", () => {
    state.writeContext = false;
    renderWithI18n(<SourceCollaborationActionDialog action={{ issueId: collaborationIssueFixture.id, action: "change_status", status: "in_review" }} close={() => {}} />);
    expect(screen.queryByRole("form")).not.toBeInTheDocument();
  });
  it("preserves a dirty workflow through background failure but removes it on authoritative denial", () => {
    const view = () => <SourceCollaborationActionDialog action={{ issueId: collaborationIssueFixture.id, action: "edit" }} close={() => {}} />;
    const mounted = renderWithI18n(view());
    fireEvent.change(screen.getByRole("textbox", { name: "Workflow draft" }), { target: { value: "Keep my unsent changes" } });
    state.error = new ApiError("Temporarily unavailable", 500, "Internal Server Error");
    mounted.rerender(view());
    expect(screen.getByRole("textbox", { name: "Workflow draft" })).toHaveValue("Keep my unsent changes");
    fireEvent.click(within(screen.getByRole("alert")).getByRole("button"));
    expect(state.refetch).toHaveBeenCalledOnce();
    state.error = new ApiError("Access denied", 403, "Forbidden");
    mounted.rerender(view());
    expect(screen.queryByRole("textbox", { name: "Workflow draft" })).not.toBeInTheDocument();
  });
});
