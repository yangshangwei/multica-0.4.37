import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { parseAdminExecution } from "@multica/core/admin";
import en from "../../locales/en/admin.json";
import { AdminExecutionControls } from "./control-panel";
import type { AdminControlTarget } from "./control-form";
const state = vi.hoisted(() => ({ role: "super_admin", refresh: vi.fn() }));
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  useAdminAccess: () => ({ status: "ready", identity: { userId: "actor", organizationId: "organization", role: state.role } }),
  adminApiScope: () => "scope",
  useAdminControlDraft: () => ({ loading: false, error: false, draft: null, reload: vi.fn() }),
}));
vi.mock("./control-form", () => ({ AdminControlForm: ({ target, onClose }: { target: AdminControlTarget; onClose(): void }) => <section><output aria-label="Snapshot">{JSON.stringify(target)}</output><button onClick={onClose}>Close confirmation</button></section> }));
const id = "11111111-1111-4111-8111-111111111111";
const execution = parseAdminExecution({ id, workspace_id: id, agent_id: id, status: "queued", source: "chat", attempt: 1, created_at: "2026-10-01T00:00:00Z", state_version: "1", execution_fence: { runtime_id: null, dispatched_at: null, target_version: "1" }, allowed_actions: ["cancel"] })!;
function content(version = "1") {
  return <I18nProvider locale="en" resources={{ en: { admin: en } }}><AdminExecutionControls execution={{ ...execution, executionFence: { runtimeId: null, dispatchedAt: null, targetVersion: version } }} onRefresh={state.refresh} /></I18nProvider>;
}
beforeEach(() => { state.role = "super_admin"; vi.clearAllMocks(); });
it("freezes the opened target and returns keyboard focus when the confirmation closes", async () => {
  const view = render(content());
  fireEvent.click(screen.getByRole("button", { name: "Cancel this execution" }));
  view.rerender(content("2"));
  expect(screen.getByLabelText("Snapshot")).toHaveTextContent('"targetVersion":"1"');
  fireEvent.click(screen.getByRole("button", { name: "Close confirmation" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Cancel this execution" })).toHaveFocus());
});
it("removes write controls immediately when the current identity becomes an observer", () => {
  const view = render(content());
  fireEvent.click(screen.getByRole("button", { name: "Cancel this execution" }));
  state.role = "platform_observer";
  view.rerender(content());
  expect(screen.queryByLabelText("Snapshot")).not.toBeInTheDocument();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});
