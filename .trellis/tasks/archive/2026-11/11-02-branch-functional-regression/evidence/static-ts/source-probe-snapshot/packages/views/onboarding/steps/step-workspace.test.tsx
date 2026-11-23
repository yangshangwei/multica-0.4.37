import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enOnboarding from "../../locales/en/onboarding.json";
import enWorkspace from "../../locales/en/workspace.json";
import type { Workspace } from "@multica/core/types";
import { WORKSPACE_NAMES } from "@multica/core/workspace/workspace-names";
import { useWorkspaceNamePreferences } from "@multica/core/workspace/workspace-name-preferences";
import zhCommon from "../../locales/zh-Hans/common.json";
import zhOnboarding from "../../locales/zh-Hans/onboarding.json";
import zhWorkspace from "../../locales/zh-Hans/workspace.json";

const TEST_RESOURCES = {
  en: {
    common: enCommon,
    onboarding: enOnboarding,
    workspace: enWorkspace,
  },
  "zh-Hans": {
    common: zhCommon,
    onboarding: zhOnboarding,
    workspace: zhWorkspace,
  },
};

type MockConfigState = {
  workspaceCreationDisabled: boolean;
  daemonAppUrl: string;
};

const mockLogout = vi.hoisted(() => vi.fn());
const mockUseConfigStore = vi.hoisted(() =>
  vi.fn((selector: (state: MockConfigState) => unknown) =>
    selector({ workspaceCreationDisabled: false, daemonAppUrl: "" }),
  ),
);

vi.mock("../../auth", () => ({
  useLogout: () => mockLogout,
}));

vi.mock("@multica/core/config", () => ({
  useConfigStore: (selector: (state: MockConfigState) => unknown) =>
    mockUseConfigStore(selector),
}));

const mockCreateMutate = vi.hoisted(() => vi.fn());
const mockSession = vi.hoisted(() => ({ user: { id: "alice" } as { id: string } | null, pending: false }));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof mockSession) => unknown) => selector(mockSession),
    { getState: () => mockSession },
  ),
}));

vi.mock("@multica/core/workspace/mutations", () => ({
  useCreateWorkspace: () => ({ mutate: mockCreateMutate, isPending: mockSession.pending }),
}));

vi.mock("@multica/core/api", () => ({
  api: { getBaseUrl: () => "http://127.0.0.1:8080" },
}));

import { StepWorkspace } from "./step-workspace";

beforeEach(() => {
  mockCreateMutate.mockClear();
  mockSession.user = { id: "alice" };
  mockSession.pending = false;
  useWorkspaceNamePreferences.setState({ seriesByUser: {} });
});

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

function renderStep({
  existing,
  disabled,
  daemonAppUrl = "",
}: {
  existing: Workspace | null;
  disabled: boolean;
  daemonAppUrl?: string;
}) {
  mockUseConfigStore.mockImplementation(
    (selector: (state: MockConfigState) => unknown) =>
      selector({ workspaceCreationDisabled: disabled, daemonAppUrl }),
  );
  return render(
    <StepWorkspace existing={existing} onCreated={vi.fn()} />,
    { wrapper: I18nWrapper },
  );
}

const EXISTING_WORKSPACE: Workspace = {
  id: "00000000-0000-0000-0000-000000000001",
  name: "Acme",
  slug: "acme",
  description: null,
  context: null,
  settings: {},
  repos: [],
  issue_prefix: "ACM",
  created_at: "2025-01-01T00:00:00Z",
  updated_at: "2025-01-01T00:00:00Z",
} as unknown as Workspace;

// Regression for #3433 (PR feedback): when DISABLE_WORKSPACE_CREATION is on,
// every onboarding entry point must steer the user toward an existing
// workspace or a logout escape — never toward the create form, even
// indirectly (stale CTA copy, "or start another" prose, etc.).
describe("StepWorkspace — DISABLE_WORKSPACE_CREATION gate", () => {
  it("renders the create form when the flag is off and the user has no workspace", () => {
    renderStep({ existing: null, disabled: false });

    expect(
      screen.getByText("Name your workspace.", { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Workspace name")).toBeInTheDocument();
    expect(screen.getByLabelText("URL")).toBeInTheDocument();
  });

  it("hides the create form and shows the disabled notice when the flag is on and there is no workspace", () => {
    renderStep({ existing: null, disabled: true });

    expect(
      screen.getByText("Ask your administrator for an invitation.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Workspace name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("URL")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /log out/i })).toBeInTheDocument();
  });

  it("forces the existing-workspace-only state when the flag is on and the user already has a workspace", () => {
    renderStep({ existing: EXISTING_WORKSPACE, disabled: true });

    // Disabled-specific copy is used in place of the "or start another" prose.
    expect(
      screen.getByText("Continue with Acme.", { exact: false }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/start another/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/create a new one alongside it/i),
    ).not.toBeInTheDocument();

    // Resume picker still shows the existing workspace card (its name
    // appears in both the avatar and the card label — at least one is
    // enough to know the card is rendered), but the "Create a new
    // workspace" radio card is gone entirely.
    expect(screen.getAllByText("Acme").length).toBeGreaterThan(0);
    expect(
      screen.queryByText("Create a new workspace", { exact: false }),
    ).not.toBeInTheDocument();

    // CTA is pre-selected to the existing-only action and immediately
    // enabled, so the user can press it without further interaction.
    const cta = screen.getByRole("button", { name: "Open Acme" });
    expect(cta).toBeEnabled();
  });
});

// #4263: the workspace URL prefix must reflect the deployment's own host on
// self-hosted instances instead of the hardcoded `multica.ai`.
describe("StepWorkspace — workspace URL prefix", () => {
  it("shows the brand host when no app URL is configured", () => {
    renderStep({ existing: null, disabled: false });
    expect(screen.getByText("multica.ai/")).toBeInTheDocument();
  });

  it("shows the deployment host for self-hosted instances", () => {
    renderStep({
      existing: null,
      disabled: false,
      daemonAppUrl: "https://multica.example.com",
    });
    expect(screen.getByText("multica.example.com/")).toBeInTheDocument();
    expect(screen.queryByText("multica.ai/")).not.toBeInTheDocument();
  });
});

describe("StepWorkspace — random workspace identity", () => {
  // Generator selection/exhaustion matrices belong in core/workspace/workspace-names.test.ts.
  it("fills the name and a suffixed English URL", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.click(screen.getByRole("button", { name: "Random" }));

    const name = screen.getByLabelText("Workspace name") as HTMLInputElement;
    const slug = screen.getByLabelText("URL") as HTMLInputElement;
    const expectedSlugPrefix = name.value
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");

    expect(name.value).not.toBe("");
    expect(slug.value).toMatch(
      new RegExp(`^${expectedSlugPrefix}-[a-z0-9]{4}$`),
    );
  });

  it("preserves a manually edited URL and prefix when randomizing, including submission", () => {
    mockCreateMutate.mockClear();
    renderStep({ existing: null, disabled: false });
    fireEvent.change(screen.getByLabelText("URL"), {
      target: { value: "frontend-team" },
    });
    fireEvent.change(screen.getByLabelText("Issue prefix"), {
      target: { value: "FE" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    const name = screen.getByLabelText("Workspace name") as HTMLInputElement;
    expect(name.value).not.toBe("");
    expect(screen.getByLabelText("URL")).toHaveValue("frontend-team");
    expect(screen.getByLabelText("Issue prefix")).toHaveValue("FE");

    fireEvent.click(screen.getByRole("button", { name: `Create ${name.value}` }));
    expect(mockCreateMutate.mock.calls[0]![0]).toEqual({
      name: name.value,
      slug: "frontend-team",
      issue_prefix: "FE",
    });
  });

  it("keeps the generated URL while editing the name and replaces it on the next randomization", () => {
    renderStep({ existing: null, disabled: false });
    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    const slug = screen.getByLabelText("URL") as HTMLInputElement;
    const firstSlug = slug.value;

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Our new team" },
    });
    expect(slug).toHaveValue(firstSlug);

    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    expect(slug.value).not.toBe(firstSlug);
    expect(screen.getByLabelText("Issue prefix")).toHaveValue(
      slug.value.replace(/[^a-z0-9]/g, "").slice(0, 4).toUpperCase(),
    );
  });

  it("changes only the selected series, then uses it for the next random name", async () => {
    const user = userEvent.setup();
    renderStep({ existing: null, disabled: false });
    await user.click(screen.getByRole("button", { name: "Random" }));
    const name = screen.getByLabelText("Workspace name") as HTMLInputElement;
    const slug = screen.getByLabelText("URL") as HTMLInputElement;
    const prefix = screen.getByLabelText("Issue prefix") as HTMLInputElement;
    const initial = { name: name.value, slug: slug.value, prefix: prefix.value };

    await user.click(screen.getByRole("button", { name: "Choose naming series" }));
    await user.click(await screen.findByRole("menuitemradio", { name: /Algorithms & computing/ }));
    expect(name).toHaveValue(initial.name);
    expect(slug).toHaveValue(initial.slug);
    expect(prefix).toHaveValue(initial.prefix);
    expect(useWorkspaceNamePreferences.getState().seriesByUser.alice).toBe("computing");

    await user.click(screen.getByRole("button", { name: "Random" }));
    expect(WORKSPACE_NAMES.computing.map((entry) => entry.en)).toContain(name.value);
    expect(slug.value).not.toBe(initial.slug);
  });

  it("reads the signed-in account's preference and does not reuse another account's series", () => {
    useWorkspaceNamePreferences.setState({ seriesByUser: { alice: "nature", bob: "space" } });
    const view = renderStep({ existing: null, disabled: false });
    expect(screen.getByText("Current: Mountains & nature")).toBeInTheDocument();

    mockSession.user = { id: "bob" };
    view.rerender(<StepWorkspace existing={null} onCreated={vi.fn()} />);
    expect(screen.getByText("Current: Stars & space")).toBeInTheDocument();

    mockSession.user = { id: "charlie" };
    view.rerender(<StepWorkspace existing={null} onCreated={vi.fn()} />);
    expect(screen.getByText("Current: Dev workshop")).toBeInTheDocument();
  });

  it("keeps an anonymous selection in the form without changing saved account preferences", async () => {
    const user = userEvent.setup();
    mockSession.user = null;
    useWorkspaceNamePreferences.setState({ seriesByUser: { alice: "nature" } });
    renderStep({ existing: null, disabled: false });
    await user.click(screen.getByRole("button", { name: "Choose naming series" }));
    await user.click(await screen.findByRole("menuitemradio", { name: /Stars & space/ }));
    expect(screen.getByText("Current: Stars & space")).toBeInTheDocument();
    expect(useWorkspaceNamePreferences.getState().seriesByUser).toEqual({ alice: "nature" });
  });

  it("updates an automatic URL while keeping a manually edited prefix", () => {
    renderStep({ existing: null, disabled: false });
    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    const firstSlug = (screen.getByLabelText("URL") as HTMLInputElement).value;
    fireEvent.change(screen.getByLabelText("Issue prefix"), { target: { value: "DEV" } });
    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    expect((screen.getByLabelText("URL") as HTMLInputElement).value).not.toBe(firstSlug);
    expect(screen.getByLabelText("Issue prefix")).toHaveValue("DEV");
  });

  it("keeps entered values when the language changes and localizes the next random name", async () => {
    const onCreated = vi.fn();
    const view = render(<I18nProvider locale="en" resources={TEST_RESOURCES}>
      <StepWorkspace existing={null} onCreated={onCreated} />
    </I18nProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Random" }));
    const firstName = (screen.getByLabelText("Workspace name") as HTMLInputElement).value;
    const firstSlug = (screen.getByLabelText("URL") as HTMLInputElement).value;
    view.rerender(<I18nProvider locale="zh-Hans" resources={TEST_RESOURCES}>
      <StepWorkspace existing={null} onCreated={onCreated} />
    </I18nProvider>);
    await waitFor(() => expect(screen.getByLabelText("工作空间名称")).toHaveValue(firstName));
    expect(screen.getByLabelText("URL")).toHaveValue(firstSlug);

    fireEvent.click(screen.getByRole("button", { name: "随机" }));
    expect(WORKSPACE_NAMES.workshop.map((entry) => entry.zh)).toContain(
      (screen.getByLabelText("工作空间名称") as HTMLInputElement).value,
    );
    expect((screen.getByLabelText("URL") as HTMLInputElement).value).toMatch(/^[a-z0-9-]+-[a-z0-9]{4}$/);
  });

  it("disables both random-name controls during workspace creation", () => {
    mockSession.pending = true;
    renderStep({ existing: null, disabled: false });
    expect(screen.getByRole("button", { name: "Random" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Choose naming series" })).toBeDisabled();
  });
});

// MUL-6050: the issue prefix used to be a read-only preview derived
// server-side from the workspace NAME, so every workspace named in Chinese
// (or Japanese, Korean, emoji…) was created as "WS" with no way to change it
// in the create flow. It now derives from the slug — which the same form
// already forces the user to pick in ASCII — and is editable here.
describe("StepWorkspace — issue prefix", () => {
  const prefixInput = () =>
    screen.getByLabelText("Issue prefix") as HTMLInputElement;

  it("derives the prefix from the slug, not the name", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Acme Inc" },
    });

    // Slug auto-filled to "acme-inc" → first 4 alphanumerics, uppercased.
    expect(prefixInput().value).toBe("ACME");
    expect(screen.getByText("ACME-123")).toBeInTheDocument();
  });

  // A Chinese name fills the whole form on its own: the URL romanizes from
  // the name, and the prefix follows the URL like any other name would.
  it("fills the URL and prefix from a Chinese name", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "蜘蛛侠" },
    });

    expect(screen.getByLabelText("URL")).toHaveValue("zhizhuxia");
    expect(prefixInput()).toHaveValue("ZHIZ");
    expect(screen.getByText("ZHIZ-123")).toBeInTheDocument();
    expect(screen.queryByText(/WS/)).not.toBeInTheDocument();
  });

  // Romanization only covers Han, so kana / Hangul / emoji names still reach
  // the empty state. Nothing may advertise a prefix there — "WS" in
  // particular is the string this issue exists to remove.
  it("shows no prefix at all when the name romanizes to nothing", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "スパイダーマン" },
    });

    expect(screen.getByLabelText("URL")).toHaveValue("");
    expect(prefixInput()).toHaveValue("");
    expect(prefixInput().placeholder).toBe("");
    expect(screen.queryByText(/WS/)).not.toBeInTheDocument();
    expect(screen.queryByText(/-123/)).not.toBeInTheDocument();
    // The hint takes the example line's place so the field isn't a bare box.
    expect(
      screen.getByText("Set the URL above and issue numbers will follow it", {
        exact: false,
      }),
    ).toBeInTheDocument();

    // …and the moment a URL exists, the prefix follows it.
    fireEvent.change(screen.getByLabelText("URL"), {
      target: { value: "spider" },
    });
    expect(prefixInput()).toHaveValue("SPID");
    expect(screen.getByText("SPID-123")).toBeInTheDocument();
  });

  it("follows a hand-typed URL over the romanized one", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "前端团队" },
    });
    expect(prefixInput().value).toBe("QIAN");

    // Overriding the URL re-derives the prefix from what the user chose.
    fireEvent.change(screen.getByLabelText("URL"), {
      target: { value: "frontend" },
    });

    expect(prefixInput().value).toBe("FRON");
    expect(prefixInput().value).not.toBe("WS");
  });

  it("stops following the slug once the user edits it, and normalizes input", () => {
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Acme Inc" },
    });
    fireEvent.change(prefixInput(), { target: { value: "fe-team!" } });

    // Uppercased, non-alphanumerics dropped — matching the server's
    // `^[A-Z0-9]{1,10}$` rule and the settings tab's guardrail.
    expect(prefixInput().value).toBe("FETEAM");

    // A later slug edit must not clobber the user's choice.
    fireEvent.change(screen.getByLabelText("URL"), {
      target: { value: "acme-corp" },
    });
    expect(prefixInput().value).toBe("FETEAM");
    expect(screen.getByText("FETEAM-123")).toBeInTheDocument();
  });

  it("submits the prefix the user was shown", () => {
    mockCreateMutate.mockClear();
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "前端团队" },
    });
    fireEvent.change(screen.getByLabelText("URL"), {
      target: { value: "frontend" },
    });
    fireEvent.change(prefixInput(), { target: { value: "fe" } });
    fireEvent.click(screen.getByRole("button", { name: /^Create 前端团队$/ }));

    expect(mockCreateMutate).toHaveBeenCalledTimes(1);
    expect(mockCreateMutate.mock.calls[0]![0]).toEqual({
      name: "前端团队",
      slug: "frontend",
      issue_prefix: "FE",
    });
  });

  it("falls back to the slug-derived default when the field is cleared", () => {
    mockCreateMutate.mockClear();
    renderStep({ existing: null, disabled: false });

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Acme Inc" },
    });
    fireEvent.change(prefixInput(), { target: { value: "" } });

    // Empty input doesn't block the CTA: the placeholder already shows the
    // default that will be used, so submitting an empty field can't surprise.
    expect(prefixInput().placeholder).toBe("ACME");
    fireEvent.click(screen.getByRole("button", { name: /^Create Acme Inc$/ }));

    expect(mockCreateMutate.mock.calls[0]![0]).toMatchObject({
      issue_prefix: "ACME",
    });
  });
});
