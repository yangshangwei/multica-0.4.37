import { StrictMode } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { useModalStore } from "@multica/core/modals";
import { useCreateModeStore } from "@multica/core/issues/stores/create-mode-store";
import { setCurrentWorkspace } from "@multica/core/platform";

const route = vi.hoisted(() => ({
  pathname: "/acme/issues",
  search: "",
  workspace: { id: "ws-acme", slug: "acme" } as { id: string; slug: string } | null,
}));
const viewer = vi.hoisted(() => ({ renderedUserId: "viewer-1", liveUserId: "viewer-1" }));

vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  useCurrentWorkspace: () => route.workspace,
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: { user: { id: string }; status: string }) => unknown) =>
      selector({ user: { id: viewer.renderedUserId }, status: "authenticated" }),
    { getState: () => ({ user: { id: viewer.liveUserId }, status: "authenticated" }) },
  ),
}));
vi.mock("../navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../navigation")>()),
  useNavigation: () => ({ pathname: route.pathname, searchParams: new URLSearchParams(route.search), hash: "" }),
}));
vi.mock("./create-issue-dialog", () => ({
  CreateIssueDialog: ({ initialMode }: { initialMode: string }) => <output aria-label="Issue creation mode">{initialMode}</output>,
}));
vi.mock("./create-project", () => ({ CreateProjectModal: () => null }));
vi.mock("./feedback", () => ({ FeedbackModal: () => null }));
vi.mock("./set-parent-issue", () => ({ SetParentIssueModal: () => null }));
vi.mock("./add-child-issue", () => ({ AddChildIssueModal: () => null }));
vi.mock("./delete-issue-confirm", () => ({ DeleteIssueConfirmModal: () => null }));
vi.mock("./run-confirm", () => ({ RunConfirmModal: () => null }));
vi.mock("./issue-completion-hint", () => ({ IssueCompletionHintModal: () => null }));
vi.mock("./issue-completion-authorization", () => ({ IssueCompletionAuthorizationModal: () => null }));
vi.mock("./issue-limit-upgrade-dialog", () => ({ IssueLimitUpgradeDialog: () => null }));

import { FeedbackModalHost, ModalRegistry } from "./registry";

const api = new ApiClient("https://api.example.test");

function queueCreation() {
  const request = {
    api, sessionScope: api.getSessionScope(), userId: "viewer-1",
    workspaceId: "ws-acme", workspaceSlug: "acme",
    originPath: "/account?view=participating", targetPath: "/acme/issues",
  };
  useModalStore.getState().queueWorkspaceIssueCreation(request);
  return request;
}

beforeEach(() => {
  setApiInstance(api);
  useModalStore.getState().close();
  useCreateModeStore.setState({ lastMode: "manual" });
  route.pathname = "/acme/issues";
  route.search = "";
  route.workspace = { id: "ws-acme", slug: "acme" };
  viewer.renderedUserId = "viewer-1";
  viewer.liveUserId = "viewer-1";
  setCurrentWorkspace(null, null);
});

afterEach(() => {
  act(() => useModalStore.getState().close());
  setCurrentWorkspace(null, null);
  vi.restoreAllMocks();
});

// Core modals/store.test.ts owns the owner/destination cancellation matrix.
it("waits for the actual workspace mirror and opens the preferred form once in StrictMode", async () => {
  const request = queueCreation();
  const open = vi.spyOn(useModalStore.getState(), "open");
  const view = render(<StrictMode><ModalRegistry /></StrictMode>);
  expect(screen.queryByLabelText("Issue creation mode")).not.toBeInTheDocument();
  expect(useModalStore.getState().workspaceIssueCreation).toBe(request);

  await act(async () => setCurrentWorkspace("acme", "ws-acme"));
  await waitFor(() => expect(screen.getByLabelText("Issue creation mode")).toHaveTextContent("manual"));
  expect(open).toHaveBeenCalledExactlyOnceWith("create-issue", null);
  expect(useModalStore.getState().workspaceIssueCreation).toBeNull();

  act(() => useModalStore.getState().close());
  view.rerender(<StrictMode><ModalRegistry /></StrictMode>);
  expect(screen.queryByLabelText("Issue creation mode")).not.toBeInTheDocument();
});

it("keeps the handoff across the account-to-workspace host remount", async () => {
  route.pathname = "/account";
  route.search = "view=participating";
  route.workspace = null;
  const request = queueCreation();
  const account = render(<FeedbackModalHost />);
  expect(useModalStore.getState().workspaceIssueCreation).toBe(request);
  account.unmount();

  route.pathname = "/acme/issues";
  route.search = "";
  route.workspace = { id: "ws-acme", slug: "acme" };
  setCurrentWorkspace("acme", "ws-acme");
  render(<ModalRegistry />);
  await waitFor(() => expect(screen.getByLabelText("Issue creation mode")).toHaveTextContent("manual"));
  expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
});

it("does not grant creation from a target URL while the actual provider still owns another workspace", () => {
  route.workspace = { id: "ws-old", slug: "old" };
  setCurrentWorkspace("acme", "ws-acme");
  const request = queueCreation();
  const view = render(<ModalRegistry />);
  expect(screen.queryByLabelText("Issue creation mode")).not.toBeInTheDocument();
  expect(useModalStore.getState().workspaceIssueCreation).toBe(request);

  route.pathname = "/old/issues";
  view.rerender(<ModalRegistry />);
  expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
});

it("validates the live viewer when an earlier route render still carries the previous identity", () => {
  setCurrentWorkspace("acme", "ws-acme");
  queueCreation();
  viewer.liveUserId = "viewer-2";
  render(<ModalRegistry />);
  expect(screen.queryByLabelText("Issue creation mode")).not.toBeInTheDocument();
  expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
});
