import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../locales/en/common.json";
import enOnboarding from "../locales/en/onboarding.json";
import enWorkspace from "../locales/en/workspace.json";

const TEST_RESOURCES = {
  en: { common: enCommon, onboarding: enOnboarding, workspace: enWorkspace },
};

const mocks = vi.hoisted(() => ({
  questionnaire: {} as Record<string, unknown>,
  saveQuestionnaire: vi.fn(),
}));

vi.mock("../auth", () => ({ useLogout: () => vi.fn() }));

vi.mock("@multica/core/config", () => ({
  useConfigStore: (
    selector: (s: { workspaceCreationDisabled: boolean; daemonAppUrl: string }) => unknown,
  ) => selector({ workspaceCreationDisabled: false, daemonAppUrl: "" }),
}));

vi.mock("@multica/core/api", () => ({
  api: { getBaseUrl: () => "https://multica.ai" },
}));

vi.mock("@multica/core/workspace/mutations", () => ({
  useCreateWorkspace: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (s: { user: unknown }) => unknown) =>
      selector({ user: { id: "u-1", onboarding_questionnaire: mocks.questionnaire } }),
    { getState: () => ({ user: { id: "u-1" } }) },
  ),
}));

// Returning one workspace proves new-workspace mode does not offer to
// continue with it.
vi.mock("@multica/core/workspace", () => {
  return {
    useWorkspaceList: () => ({
      workspaces: [{ id: "ws-1", name: "Existing", slug: "existing" }],
      ready: true,
    }),
  };
});

vi.mock("@multica/core/onboarding", async () => {
  const actual = await vi.importActual<Record<string, unknown>>(
    "@multica/core/onboarding",
  );
  return {
    ...actual,
    useBootstrapMika: () => ({ mutateAsync: vi.fn() }),
    saveQuestionnaire: mocks.saveQuestionnaire,
  };
});

import { OnboardingFlow } from "./onboarding-flow";

function renderFlow(props: Record<string, unknown>) {
  return render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <OnboardingFlow onComplete={vi.fn()} {...props} />
    </I18nProvider>,
  );
}

describe("OnboardingFlow — new-workspace mode", () => {
  beforeEach(() => {
    mocks.questionnaire = {};
    mocks.saveQuestionnaire.mockReset().mockResolvedValue(undefined);
  });

  it("starts at About you with the first progress step active", () => {
    const { container } = renderFlow({ mode: "new_workspace", onCancel: vi.fn() });

    expect(screen.getByRole("radiogroup", {
      name: "What's your role in the development process?",
    })).toBeInTheDocument();
    expect(container.querySelector('[aria-current="step"]')).toHaveTextContent("About you");
    expect(
      screen.queryByRole("heading", { name: /Name your workspace/i }),
    ).not.toBeInTheDocument();
  });

  it("pre-fills saved answers and continues to workspace creation", async () => {
    const user = userEvent.setup();
    mocks.questionnaire = { role: "engineer", use_case: ["ship_code"] };
    renderFlow({ mode: "new_workspace", onCancel: vi.fn() });

    expect(screen.getByRole("radio", { name: "Engineer" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /code & test with agents/i })).toBeChecked();
    await user.click(screen.getByRole("button", { name: /^Continue$/i }));

    expect(screen.getByLabelText("Workspace name")).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /Existing/i })).not.toBeInTheDocument();
  });

  it("allows explicitly skipping About you before creating a workspace", async () => {
    const user = userEvent.setup();
    renderFlow({ mode: "new_workspace", onCancel: vi.fn() });

    await user.click(screen.getByRole("button", { name: /^Skip$/i }));

    expect(screen.getByLabelText("Workspace name")).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /Existing/i })).not.toBeInTheDocument();
    expect(mocks.saveQuestionnaire).toHaveBeenCalledWith(expect.objectContaining({
      role_skipped: true,
      use_case_skipped: true,
    }));
  });

  it("cancels from About you", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderFlow({ mode: "new_workspace", onCancel });

    await user.click(screen.getAllByRole("button", { name: "Back" })[0]!);

    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it.each(["back", "rail"])("returns from workspace to About you via %s and keeps answers", async (navigation) => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderFlow({ mode: "new_workspace", onCancel });
    await user.click(screen.getByRole("radio", { name: "Engineer" }));
    await user.click(screen.getByRole("button", { name: /^Continue$/i }));

    await user.click(navigation === "back"
      ? screen.getAllByRole("button", { name: "Back" })[0]!
      : screen.getByRole("button", { name: /About you/i }));

    expect(screen.getByRole("radio", { name: "Engineer" })).toBeChecked();
    expect(onCancel).not.toHaveBeenCalled();
  });

  it("still opens on the product intro and returns there in first-run mode", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderFlow({ onCancel });

    await user.click(screen.getByRole("button", { name: "Start exploring" }));
    expect(screen.getByRole("radio", { name: "Engineer" })).toBeInTheDocument();
    await user.click(screen.getAllByRole("button", { name: "Back" })[0]!);

    expect(screen.getByRole("button", { name: "Start exploring" })).toBeInTheDocument();
    expect(onCancel).not.toHaveBeenCalled();
  });
});
