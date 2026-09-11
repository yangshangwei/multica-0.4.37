import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import { EMPTY_CREATE_AUTOPILOT_FROM_TEMPLATE_RESPONSE } from "@multica/core/api/schemas";
import type { AutopilotTemplate } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import enAutopilots from "../../locales/en/autopilots.json";

// The two steps of the built-in autopilot template flow. The contract this
// file exists to pin is the last one: the create request carries only what a
// PERSON chose. Title, prompt, cron and execution mode are the server's to take
// from the template — a client that could send them could claim a template's
// provenance while supplying its own brief.

const mockListTemplates = vi.hoisted(() => vi.fn());
const mockCreateFromTemplate = vi.hoisted(() => vi.fn());
const mockPush = vi.hoisted(() => vi.fn());
const searchParams = vi.hoisted(() => ({ value: new URLSearchParams() }));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));

vi.mock("@multica/core/paths", () => ({
  useCurrentWorkspace: () => ({ name: "Acme" }),
  useWorkspacePaths: () => ({
    autopilots: () => "/acme/autopilots",
    autopilotDetail: (id: string) => `/acme/autopilots/${id}`,
    newAutopilotTemplate: () => "/acme/autopilots/new/template",
  }),
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    push: mockPush,
    replace: vi.fn(),
    searchParams: searchParams.value,
  }),
  useBackOrReplace: () => vi.fn(),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: { listAutopilotTemplates: mockListTemplates },
}));

vi.mock("@multica/core/autopilots/mutations", () => ({
  useCreateAutopilotFromTemplate: () => ({
    mutateAsync: mockCreateFromTemplate,
    isPending: false,
    isError: false,
    error: null,
  }),
  // Pulled in by the blank-create dialog this page keeps reachable.
  useCreateAutopilot: () => ({ mutateAsync: vi.fn() }),
  useCreateAutopilotTrigger: () => ({ mutateAsync: vi.fn() }),
  useUpdateAutopilot: () => ({ mutateAsync: vi.fn() }),
  useUpdateAutopilotTrigger: () => ({ mutateAsync: vi.fn() }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: (wsId: string) => ({
    queryKey: ["agents", wsId],
    queryFn: async () => [
      {
        id: "agent-1",
        name: "Scout",
        description: "Researches things",
        archived_at: null,
        runtime_id: "runtime-1",
      },
    ],
  }),
  squadListOptions: (wsId: string) => ({
    queryKey: ["squads", wsId],
    queryFn: async () => [],
  }),
}));

vi.mock("@multica/core/projects/queries", () => ({
  projectListOptions: (wsId: string) => ({
    queryKey: ["projects", wsId],
    queryFn: async () => [],
  }),
}));

// Reached only through the blank-create dialog's schedule editor.
vi.mock("@multica/core/autopilots/queries", () => ({
  cronPreviewOptions: (wsId: string, expr: string, tz: string) => ({
    queryKey: ["cron-preview", wsId, expr, tz],
    queryFn: async () => ({ next_runs: ["2126-07-14T01:00:00Z"] }),
    retry: false,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

// Tiptap in jsdom is neither cheap nor the subject here; the blank-create
// dialog only has to be mountable.
vi.mock("../../editor", () => ({
  TitleEditor: () => <input aria-label="title" />,
  ContentEditor: () => <textarea aria-label="runbook" />,
}));

vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ actorId }: { actorId: string }) => (
    <span data-testid="actor-avatar">{actorId}</span>
  ),
}));

vi.mock("../../projects/components/project-picker", () => ({
  ProjectPicker: ({ triggerRender }: { triggerRender: React.ReactElement }) =>
    triggerRender,
}));

vi.mock("./subscriber-multi-select", () => ({
  SubscriberMultiSelect: () => <div data-testid="subscriber-multi-select" />,
}));

vi.mock("./pickers/timezone-picker", () => ({
  TimezonePicker: ({ value }: { value: string }) => (
    <div data-testid="timezone-picker">{value}</div>
  ),
}));

import { TemplateCreateAutopilotPage } from "./template-create-autopilot-page";

const TEMPLATES: AutopilotTemplate[] = [
  {
    key: "workday-repo-audit",
    version: 1,
    category: "repo-health",
    category_label: "Repo health",
    title: "Workday repo audit",
    description: "Checks dependency health and risky open changes.",
    cron_expression: "0 9 * * 1-5",
    execution_mode: "run_only",
    avatar_emoji: "🩺",
    prompt: "# Workday repo audit\nAudit the repository.",
  },
  {
    key: "release-readiness",
    version: 2,
    category: "release",
    category_label: "Release prep",
    title: "Release readiness",
    description: "Weekly release risk summary.",
    cron_expression: "0 17 * * 1",
    execution_mode: "create_issue",
    avatar_emoji: "🚦",
    prompt: "# Release readiness\nSummarize release risk.",
  },
  {
    key: "daily-change-review",
    version: 1,
    category: "review",
    category_label: "Review",
    title: "Daily change review",
    description: "Flags correctness, UX and coverage risk in recent work.",
    cron_expression: "0 18 * * *",
    execution_mode: "create_issue",
    avatar_emoji: "🔍",
    prompt: "# Daily change review\nReview today's changes.",
  },
  {
    key: "hourly-queue-check",
    version: 1,
    category: "maintenance",
    category_label: "Maintenance",
    title: "Hourly queue check",
    description: "Looks for stuck work and stale artifacts.",
    cron_expression: "0 * * * *",
    execution_mode: "run_only",
    avatar_emoji: "⏱️",
    prompt: "# Hourly queue check\nCheck the queue.",
  },
];

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderWithI18n(
    <QueryClientProvider client={qc}>
      <TemplateCreateAutopilotPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  searchParams.value = new URLSearchParams();
  mockListTemplates.mockResolvedValue(TEMPLATES);
});

describe("autopilot template picker", () => {
  it("lists every template the server ships, with its category and description", async () => {
    renderPage();

    const card = await screen.findByRole("button", {
      name: /Workday repo audit/,
    });
    expect(card.textContent).toContain("Repo health");
    expect(card.textContent).toContain(
      "Checks dependency health and risky open changes.",
    );
    // The cron reaches the card as a sentence, not as five fields.
    expect(card.textContent).toContain("At 09:00");
    expect(card.textContent).toContain("Mon–Fri");

    for (const template of TEMPLATES) {
      expect(
        screen.getByRole("button", { name: new RegExp(template.title) }),
      ).toBeTruthy();
    }
  });

  it("navigates to the same route with the picked template in the query", async () => {
    renderPage();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Release readiness/ }),
    );

    // Same route, `?template=` added: going back to the list is this route
    // without the param, which is why the template is not a path segment —
    // and why there is no persistent selected card to keep legible on hover.
    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith(
        "/acme/autopilots/new/template?template=release-readiness",
      ),
    );
  });

  it("shows placeholders rather than an empty grid while templates load", () => {
    mockListTemplates.mockReturnValue(new Promise(() => {}));
    const { container } = renderPage();

    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBe(4);
    expect(
      screen.queryByText(enAutopilots.template_picker.empty),
    ).not.toBeInTheDocument();
  });

  it("says so when the server ships no templates", async () => {
    mockListTemplates.mockResolvedValue([]);
    renderPage();

    expect(
      await screen.findByText(enAutopilots.template_picker.empty),
    ).toBeInTheDocument();
  });

  it("reports a failed load instead of claiming there are none", async () => {
    mockListTemplates.mockRejectedValue(new Error("offline"));
    renderPage();

    expect(
      await screen.findByText(enAutopilots.template_picker.load_failed),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(enAutopilots.template_picker.empty),
    ).not.toBeInTheDocument();
  });

  it("keeps the blank creation flow reachable from the picker", async () => {
    renderPage();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: enAutopilots.page.start_blank }),
    );

    expect(
      await screen.findByRole("button", { name: enAutopilots.dialog.create }),
    ).toBeInTheDocument();
  });
});

describe("autopilot template configure step", () => {
  beforeEach(() => {
    searchParams.value = new URLSearchParams("template=release-readiness");
  });

  it("previews what the template decides, read-only", async () => {
    renderPage();

    expect(
      await screen.findByText("# Release readiness Summarize release risk."),
    ).toBeInTheDocument();
    // Cron and mode are stated, not offered as controls.
    expect(screen.getByText("At 17:00 · Mon")).toBeInTheDocument();
    expect(
      screen.getByText(enAutopilots.dialog.output_modes.create_issue.label),
    ).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("blocks the create until an assignee is picked", async () => {
    renderPage();
    const user = userEvent.setup();

    const createButton = await screen.findByRole("button", {
      name: enAutopilots.dialog.create,
    });
    expect(createButton).toBeDisabled();
    // The disabled control is not a dead end: the reason is on the field.
    expect(
      screen.getByText(enAutopilots.template_picker.assignee_hint),
    ).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /Select agent or squad/ }),
    );
    await user.click(await screen.findByRole("button", { name: /Scout/ }));

    await waitFor(() => expect(createButton).not.toBeDisabled());
    expect(mockCreateFromTemplate).not.toHaveBeenCalled();
  });

  it("sends only what the person chose, never the template's own content", async () => {
    mockCreateFromTemplate.mockResolvedValue({
      autopilot: { id: "ap-1", title: "Release readiness" },
      trigger: { id: "tr-1" },
    });
    renderPage();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Select agent or squad/ }),
    );
    await user.click(await screen.findByRole("button", { name: /Scout/ }));
    await user.click(
      screen.getByRole("button", { name: enAutopilots.dialog.create }),
    );

    await waitFor(() =>
      expect(mockCreateFromTemplate).toHaveBeenCalledTimes(1),
    );
    const body = mockCreateFromTemplate.mock.calls[0]?.[0] as Record<
      string,
      unknown
    >;
    expect(body).toMatchObject({
      template_key: "release-readiness",
      assignee_id: "agent-1",
      assignee_type: "agent",
      project_id: null,
      language: "en",
    });
    // The provenance contract: the server takes these from the template, so
    // the request must not carry a client's version of any of them.
    expect(Object.keys(body).sort()).toEqual([
      "assignee_id",
      "assignee_type",
      "language",
      "project_id",
      "template_key",
      "timezone",
    ]);
    // One round trip — the autopilot and its schedule land in one transaction.
    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith("/acme/autopilots/ap-1"),
    );
  });

  it("reports a failure instead of navigating when the response did not parse", async () => {
    // `parseWithFallback` does not throw: a payload the schema cannot read
    // comes back as the conservative empty shape, so the mutation RESOLVES and
    // `isError` stays false. Treating that as success would push
    // `/acme/autopilots/` — the list, under a toast claiming an automation was
    // created that nobody can confirm exists.
    mockCreateFromTemplate.mockResolvedValue(
      EMPTY_CREATE_AUTOPILOT_FROM_TEMPLATE_RESPONSE,
    );
    renderPage();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Select agent or squad/ }),
    );
    await user.click(await screen.findByRole("button", { name: /Scout/ }));
    await user.click(
      screen.getByRole("button", { name: enAutopilots.dialog.create }),
    );

    expect(
      await screen.findByRole("alert"),
    ).toHaveTextContent(enAutopilots.dialog.toast_create_failed);
    expect(mockPush).not.toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });
});
