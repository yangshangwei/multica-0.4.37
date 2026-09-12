import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithI18n } from "../test/i18n";

const mocks = vi.hoisted(() => ({
  workspace: { id: "new-workspace", slug: "new-workspace", name: "New workspace" },
  runtime: { id: "runtime-1" },
  bootstrapMika: vi.fn(),
  completeOnboarding: vi.fn(),
  saveQuestionnaire: vi.fn(),
  welcome: vi.fn(),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (select: (state: unknown) => unknown) => select({ user: { id: "user-1", onboarding_questionnaire: {} } }),
    { getState: () => ({ user: { id: "user-1" } }) },
  ),
}));
vi.mock("@multica/core/workspace", () => ({
  useWorkspaceList: () => ({ workspaces: [], ready: true }),
}));
vi.mock("@multica/core/onboarding", async () => ({
  ...await vi.importActual<Record<string, unknown>>("@multica/core/onboarding"),
  useBootstrapMika: () => ({ mutateAsync: mocks.bootstrapMika }),
  completeOnboarding: mocks.completeOnboarding,
  saveQuestionnaire: mocks.saveQuestionnaire,
  useWelcomeStore: { getState: () => ({ set: mocks.welcome }) },
}));
vi.mock("./components/step-shell", () => ({ StepShell: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }));
vi.mock("./components/onboarding-logout-button", () => ({ OnboardingLogoutButton: () => null }));
vi.mock("./steps/step-workspace", () => ({
  StepWorkspace: ({ onCreated }: { onCreated: (workspace: unknown) => void }) => (
    <button onClick={() => onCreated(mocks.workspace)}>Create workspace</button>
  ),
}));
vi.mock("./steps/step-runtime-connect", () => ({
  StepRuntimeConnect: ({ onNext }: { onNext: (runtime: unknown) => void }) => (
    <>
      <button onClick={() => onNext(mocks.runtime)}>Use runtime</button>
      <button onClick={() => onNext(null)}>Skip runtime</button>
    </>
  ),
}));

import { OnboardingFlow } from "./onboarding-flow";

beforeEach(() => {
  vi.clearAllMocks();
  mocks.bootstrapMika.mockResolvedValue({ chatSession: { id: "mika-chat" } });
  mocks.completeOnboarding.mockResolvedValue(undefined);
  mocks.saveQuestionnaire.mockResolvedValue(undefined);
});

describe("new workspace completion", () => {
  it("keeps Mika available and lands on Projects after connecting a runtime", async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();
    renderWithI18n(<OnboardingFlow mode="new_workspace" onComplete={onComplete} />);

    await user.click(screen.getByRole("button", { name: "Create workspace" }));
    await user.click(screen.getByRole("button", { name: "Use runtime" }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledWith(mocks.workspace, { kind: "projects" }));
    expect(mocks.bootstrapMika).toHaveBeenCalledWith(expect.objectContaining({ runtimeId: "runtime-1", workspaceSlug: "new-workspace" }));
    expect(mocks.completeOnboarding).toHaveBeenCalledWith("full", "new-workspace");
  });

  it("lands on Projects when the runtime is skipped", async () => {
    const user = userEvent.setup();
    const onComplete = vi.fn();
    renderWithI18n(<OnboardingFlow mode="new_workspace" onComplete={onComplete} />);

    await user.click(screen.getByRole("button", { name: "Create workspace" }));
    await user.click(screen.getByRole("button", { name: "Skip runtime" }));

    await waitFor(() => expect(onComplete).toHaveBeenCalledWith(mocks.workspace, { kind: "projects" }));
    expect(mocks.bootstrapMika).not.toHaveBeenCalled();
    expect(mocks.completeOnboarding).toHaveBeenCalledWith("runtime_skipped", "new-workspace");
    expect(mocks.welcome).toHaveBeenCalledWith({ workspaceId: "new-workspace", choice: "skip" });
  });
});
