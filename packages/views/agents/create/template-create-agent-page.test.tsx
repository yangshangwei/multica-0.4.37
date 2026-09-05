// @vitest-environment jsdom

import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AgentRoleTemplate } from "@multica/core/types";
import enAgents from "../../locales/en/agents.json";

const mockListTemplates = vi.hoisted(() => vi.fn());
const mockPush = vi.hoisted(() => vi.fn());
const searchParams = vi.hoisted(() => ({ value: new URLSearchParams() }));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// Partial mock: the module also exports WORKSPACE_PAGES, which the skill picker
// reads at import time through the configuration panel's dependency graph.
vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  useWorkspacePaths: () => ({
    agents: () => "/acme/agents",
    newAgent: () => "/acme/agents/new",
    newAgentTemplate: () => "/acme/agents/new/template",
    agentDetail: (id: string) => `/acme/agents/${id}`,
    squadDetail: (id: string) => `/acme/squads/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    push: mockPush,
    replace: vi.fn(),
    searchParams: searchParams.value,
  }),
  useBackOrReplace: () => vi.fn(),
  AppLink: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: { listAgentRoleTemplates: mockListTemplates },
}));

import { TemplateCreateAgentPage } from "./template-create-agent-page";

const TEMPLATES: AgentRoleTemplate[] = [
  {
    key: "product-analyst",
    version: 1,
    name: "Product Analyst",
    title: "Product Analyst",
    description: "Turns a vague request into a decidable one.",
    autonomy_level: "observer",
    avatar_emoji: "🔍",
    max_concurrent_tasks: 3,
    skill_names: ["multica-requirement-clarification"],
    instructions: "# Product Analyst",
  },
  {
    key: "release-engineer",
    version: 2,
    name: "Release Engineer",
    title: "Release Engineer",
    description: "Prepares the release and the rollback.",
    autonomy_level: "operator",
    avatar_emoji: "🚦",
    max_concurrent_tasks: 1,
    skill_names: ["multica-release-check"],
    instructions: "# Release Engineer",
  },
  {
    // A level this client has never heard of: the card must still render.
    key: "future-role",
    version: 1,
    name: "Future Role",
    title: "Future Role",
    description: "Ships with a newer server.",
    autonomy_level: "supervisor",
    avatar_emoji: "🛰️",
    max_concurrent_tasks: 1,
    skill_names: [],
    instructions: "# Future",
  },
];

function renderPicker() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nProvider locale="en" resources={{ en: { agents: enAgents } }}>
      <QueryClientProvider client={queryClient}>
        <TemplateCreateAgentPage />
      </QueryClientProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  searchParams.value = new URLSearchParams();
  mockListTemplates.mockResolvedValue(TEMPLATES);
});

describe("TemplateCreateAgentPage role picker", () => {
  it("lists the roles the server ships, with their autonomy levels", async () => {
    renderPicker();

    expect(
      await screen.findByRole("button", { name: /Product Analyst/ }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: /Release Engineer/ })).toBeTruthy();
    // The badge is the level's label, not the raw wire value.
    expect(screen.getByText("Observer")).toBeTruthy();
    expect(screen.getByText("Operator")).toBeTruthy();
  });

  it("renders a role whose autonomy level this client does not know", async () => {
    renderPicker();

    // The card must be pickable. Only the badge is dropped — showing an
    // untranslated level, or claiming a limit we cannot describe, would be worse.
    expect(
      await screen.findByRole("button", { name: /Future Role/ }),
    ).toBeTruthy();
    expect(screen.queryByText("supervisor")).toBeNull();
  });

  it("navigates to the same route with the picked role in the query", async () => {
    renderPicker();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Release Engineer/ }),
    );

    // Same route, `?template=` added: going back to the role list is the route
    // without the param, which is why the role is not a path segment.
    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith(
        "/acme/agents/new/template?template=release-engineer",
      ),
    );
  });

  it("keeps the squad context across the hop", async () => {
    searchParams.value = new URLSearchParams("squad=squad-7");
    renderPicker();
    const user = userEvent.setup();

    await user.click(
      await screen.findByRole("button", { name: /Product Analyst/ }),
    );

    await waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith(
        "/acme/agents/new/template?squad=squad-7&template=product-analyst",
      ),
    );
  });

  it("says so when the server ships no templates", async () => {
    mockListTemplates.mockResolvedValue([]);
    renderPicker();

    expect(
      await screen.findByText(enAgents.role_templates.empty),
    ).toBeTruthy();
  });

  it("reports a failed load instead of rendering an empty grid", async () => {
    mockListTemplates.mockRejectedValue(new Error("offline"));
    renderPicker();

    expect(
      await screen.findByText(enAgents.role_templates.load_failed),
    ).toBeTruthy();
  });
});
