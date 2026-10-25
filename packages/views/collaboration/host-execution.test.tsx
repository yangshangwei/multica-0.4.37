import { afterEach, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, type CollaborationExecutionRun } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider } from "../navigation";
import { CollaborationRunDisclosure } from "./run-records";

// Core execution-host.test.ts owns the mode, cleanup and malformed-evidence matrix.
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); vi.unstubAllGlobals(); });
const issue = parseCollaborationIssue(collaborationExecutionIssueFixture)!;
const source = issue.execution!.runs[0]!;
const ended: CollaborationExecutionRun = { ...source, execution_mode: "host_process_supervision_v1", proof_state: "host_exit_observed",
  launch_count: "2", descendant_cleanup: { state: "best_effort", scope: "process_group", attempted: true },
  progress_state: "settled", settled_at: "2026-10-25T01:00:00Z", forced_disposition: null, business_outcome: "failure" };

function mount(run: CollaborationExecutionRun) {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(issue.creator.user_id);
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}>
    <ul><CollaborationRunDisclosure scope={scope} issue={issue} run={run} group={issue.execution!.groups[0]!} /></ul>
  </NavigationProvider></QueryClientProvider>);
}

it("shows limited cleanup separately from a host execution's real end", () => {
  mount(ended);
  expect(screen.getByText("Standard host execution")).toBeInTheDocument();
  expect(screen.getByText("Managed execution ended")).toBeInTheDocument();
  expect(screen.getByText("Managed execution ended and cleanup was attempted; background processes outside the managed scope were not verified")).toBeInTheDocument();
  expect(screen.queryByText("Termination confirmed")).not.toBeInTheDocument();
});

it("keeps an unknown managed execution primary even after cleanup was attempted", () => {
  mount({ ...ended, proof_state: "unknown", progress_state: "ending", settled_at: null, forced_disposition: "stopped" });
  expect(screen.getByText("Execution state unconfirmed; cannot resume yet")).toBeInTheDocument();
  expect(screen.queryByText("Managed execution ended")).not.toBeInTheDocument();
  expect(screen.queryByText(/Managed execution ended and cleanup/)).not.toBeInTheDocument();
});

it("preserves legacy namespace proof wording", () => {
  mount(issue.execution!.runs[1]!);
  expect(screen.getByText("Termination confirmed")).toBeInTheDocument();
  expect(screen.queryByText("Standard host execution")).not.toBeInTheDocument();
});
