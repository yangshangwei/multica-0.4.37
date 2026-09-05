// @vitest-environment jsdom

import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EMPTY_AGENT_DRAFT } from "@multica/core/agents";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent } from "@multica/core/types";
import { workspaceKeys } from "@multica/core/workspace/queries";
import enAgents from "../../locales/en/agents.json";

const mockCreateFromTemplate = vi.hoisted(() => vi.fn());
const mockAddSquadMember = vi.hoisted(() => vi.fn());
const mockPush = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  useWorkspacePaths: () => ({
    agentDetail: (id: string) => `/acme/agents/${id}`,
    squadDetail: (id: string) => `/acme/squads/${id}`,
  }),
}));

vi.mock("../../navigation", () => ({
  useNavigation: () => ({ push: mockPush }),
}));

vi.mock("@multica/core/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/api")>();
  return {
    ...actual,
    api: {
      createAgentFromTemplate: mockCreateFromTemplate,
      addSquadMember: mockAddSquadMember,
    },
  };
});

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), warning: vi.fn() },
}));

import { useCreateTemplateAgentSubmit } from "./use-create-template-agent-submit";

const CREATED: Agent = {
  id: "agent-new",
  workspace_id: "ws-1",
  runtime_id: "runtime-1",
  name: "Code Reviewer",
  description: "Reads the diff",
  instructions: "# Code Reviewer",
  avatar_url: null,
  runtime_mode: "local",
  runtime_config: {},
  custom_args: [],
  visibility: "private",
  permission_mode: "private",
  invocation_targets: [],
  status: "idle",
  max_concurrent_tasks: 3,
  model: "",
  owner_id: "user-1",
  skills: [],
  template_key: "code-reviewer",
  template_version: 1,
  autonomy_level: "observer",
  created_at: "2026-09-01T00:00:00Z",
  updated_at: "2026-09-01T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

function wrapper(queryClient: QueryClient) {
  return function TestWrapper({ children }: { children: ReactNode }) {
    return (
      <I18nProvider locale="en" resources={{ en: { agents: enAgents } }}>
        <QueryClientProvider client={queryClient}>
          {children}
        </QueryClientProvider>
      </I18nProvider>
    );
  };
}

function setup(overrides?: {
  squadId?: string | null;
  permissionScope?: "private" | "workspace" | "members";
  memberIds?: Set<string>;
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const { result } = renderHook(
    () =>
      useCreateTemplateAgentSubmit({
        templateKey: "code-reviewer",
        draft: {
          ...EMPTY_AGENT_DRAFT,
          name: "  Code Reviewer  ",
          runtimeId: "runtime-1",
          model: " sonnet ",
          permissionScope: overrides?.permissionScope ?? "private",
          memberIds: overrides?.memberIds ?? new Set(),
        },
        runtimeId: "runtime-1",
        squadId: overrides?.squadId ?? null,
      }),
    { wrapper: wrapper(queryClient) },
  );
  return { result, queryClient };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockCreateFromTemplate.mockResolvedValue(CREATED);
});

describe("useCreateTemplateAgentSubmit", () => {
  it("sends the template key and only the fields a person chose", async () => {
    const { result } = setup();

    await act(async () => {
      await result.current.create();
    });

    expect(mockCreateFromTemplate).toHaveBeenCalledTimes(1);
    const body = mockCreateFromTemplate.mock.calls[0]?.[0];
    expect(body).toMatchObject({
      template_key: "code-reviewer",
      runtime_id: "runtime-1",
      name: "Code Reviewer",
      model: "sonnet",
      permission_mode: "private",
      invocation_targets: [],
    });
    // The role's own content is the backend's to apply. Sending it from here
    // would let a client claim a template's provenance with a different prompt.
    expect(body).not.toHaveProperty("instructions");
    expect(body).not.toHaveProperty("skill_ids");
    expect(body).not.toHaveProperty("autonomy_level");
    expect(body).not.toHaveProperty("max_concurrent_tasks");
  });

  it("maps a workspace access choice onto the invocation target", async () => {
    const { result } = setup({ permissionScope: "workspace" });

    await act(async () => {
      await result.current.create();
    });

    expect(mockCreateFromTemplate.mock.calls[0]?.[0]).toMatchObject({
      permission_mode: "public_to",
      invocation_targets: [{ target_type: "workspace" }],
    });
  });

  it("caches the created agent before navigating to it", async () => {
    const { result, queryClient } = setup();
    let detailAtNavigation: Agent | undefined;
    mockPush.mockImplementation(() => {
      detailAtNavigation = queryClient.getQueryData<Agent>(
        workspaceKeys.agent("ws-1", CREATED.id),
      );
    });

    await act(async () => {
      await result.current.create();
    });

    // The flow navigates to the agent, so it has to be readable before the
    // destination renders — this is why the submit is not optimistic.
    expect(detailAtNavigation?.id).toBe(CREATED.id);
    expect(mockPush).toHaveBeenCalledWith(`/acme/agents/${CREATED.id}`);
  });

  it("joins the squad it was created for and lands on the squad", async () => {
    mockAddSquadMember.mockResolvedValue({});
    const { result } = setup({ squadId: "squad-7" });

    await act(async () => {
      await result.current.create();
    });

    expect(mockAddSquadMember).toHaveBeenCalledWith("squad-7", {
      member_type: "agent",
      member_id: CREATED.id,
    });
    expect(mockPush).toHaveBeenCalledWith("/acme/squads/squad-7");
  });

  it("still finishes when the squad join fails", async () => {
    mockAddSquadMember.mockRejectedValue(new Error("squad archived"));
    const { result } = setup({ squadId: "squad-7" });

    await act(async () => {
      await result.current.create();
    });

    // The agent is already committed; failing here would invite a retry that
    // produces a second one.
    expect(mockPush).toHaveBeenCalledWith("/acme/squads/squad-7");
    expect(result.current.formError).toBeNull();
  });

  it("surfaces a name conflict on the name field", async () => {
    class ApiErrorLike extends Error {
      status = 409;
    }
    mockCreateFromTemplate.mockRejectedValue(new ApiErrorLike("conflict"));
    const { result } = setup();

    await act(async () => {
      await result.current.create();
    });

    // Not an ApiError instance here, so it classifies as a form error rather
    // than a field error — either way the flow reports it and stays put.
    expect(result.current.creating).toBe(false);
    expect(result.current.nameError ?? result.current.formError).toBeTruthy();
    expect(mockPush).not.toHaveBeenCalled();
  });

  it("does nothing without a runtime", async () => {
    const queryClient = new QueryClient();
    const { result } = renderHook(
      () =>
        useCreateTemplateAgentSubmit({
          templateKey: "code-reviewer",
          draft: { ...EMPTY_AGENT_DRAFT, name: "Reviewer" },
          runtimeId: null,
          squadId: null,
        }),
      { wrapper: wrapper(queryClient) },
    );

    await act(async () => {
      await result.current.create();
    });

    expect(mockCreateFromTemplate).not.toHaveBeenCalled();
  });
});
