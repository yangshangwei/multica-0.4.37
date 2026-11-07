import { act, fireEvent, render } from "@testing-library/react";
import { createRef, type ReactNode } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { createI18n, I18nProvider } from "@multica/core/i18n/react";
import { api } from "@multica/core/api";
import { skillListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import type { Agent, MemberWithUser, SkillSummary } from "@multica/core/types";
import { onlineManager, QueryClient } from "@tanstack/react-query";
import enEditor from "../../locales/en/editor.json";
import enSkills from "../../locales/en/skills.json";
import zhEditor from "../../locales/zh-Hans/editor.json";
import zhSkills from "../../locales/zh-Hans/skills.json";

const TEST_RESOURCES = {
  en: { editor: enEditor, skills: enSkills },
  "zh-Hans": { editor: zhEditor, skills: zhSkills },
};

function I18nWrapper({ children }: { children: ReactNode }) {
  return (
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      {children}
    </I18nProvider>
  );
}

beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

beforeEach(() => {
  createI18n("en", TEST_RESOURCES);
});

vi.mock("@multica/core/platform", () => ({
  getCurrentWsId: () => "ws-1",
}));

vi.mock("@multica/core/api", () => ({
  api: { listSkills: vi.fn() },
}));

const authState = { user: { id: "u1" } as { id: string } | null };
vi.mock("@multica/core/auth", () => ({
  useAuthStore: { getState: () => authState },
}));

const chatState = { selectedAgentId: "agent-1" as string | null };
vi.mock("@multica/core/chat", () => ({
  useChatStore: { getState: () => chatState },
}));

import {
  SlashCommandList,
  type SlashCommandListRef,
  createSlashCommandSuggestion,
  type SlashCommandItem,
  buildBuiltinCommandItems,
  BUILTIN_COMMANDS,
  createBuiltinCommandSuggestion,
  QUICK_ACTION_ITEM_PREFIX,
} from "./slash-command-suggestion";

function agent(overrides: Partial<Agent>): Agent {
  return {
    id: "agent-1",
    workspace_id: "ws-1",
    runtime_id: "runtime-1",
    name: "Agent",
    description: "",
    instructions: "",
    avatar_url: null,
    runtime_mode: "local",
    runtime_config: {},
    custom_args: [],
    visibility: "workspace",
    permission_mode: "public_to",
    invocation_targets: [{ target_type: "workspace", target_id: null }],
    status: "idle",
    max_concurrent_tasks: 1,
    model: "",
    owner_id: null,
    skills: [],
    created_at: "",
    updated_at: "",
    archived_at: null,
    archived_by: null,
    ...overrides,
  };
}

function fakeQc(data: {
  members?: Array<Pick<MemberWithUser, "user_id" | "name" | "role">>;
  agents?: Agent[];
  skills?: SkillSummary[];
  fetchSkills?: () => Promise<SkillSummary[]>;
}): QueryClient {
  const map = new Map<string, unknown>();
  map.set(JSON.stringify(workspaceKeys.members("ws-1")), data.members ?? []);
  map.set(JSON.stringify(workspaceKeys.agents("ws-1")), data.agents ?? []);
  if (data.skills !== undefined) {
    map.set(JSON.stringify(workspaceKeys.skills("ws-1")), data.skills);
  }
  return {
    getQueryData: (key: readonly unknown[]) => map.get(JSON.stringify(key)),
    getQueryState: () => undefined,
    fetchQuery: data.fetchSkills ?? (() => Promise.resolve(data.skills ?? [])),
  } as unknown as QueryClient;
}

async function items(qc: QueryClient, query = ""): Promise<SlashCommandItem[]> {
  const config = createSlashCommandSuggestion(qc);
  return await config.items!({
    query,
    editor: {} as never,
    signal: new AbortController().signal,
  });
}

// An existing suggestion must settle before a network response is available.
function immediateItems(qc: QueryClient, query = "") {
  return Promise.race([
    items(qc, query),
    new Promise<"still waiting">((resolve) => setTimeout(() => resolve("still waiting"), 0)),
  ]);
}

describe("slash command suggestion items", () => {
  it("returns all active agent skills when query is empty", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "deploy", description: "Ship changes" },
            { id: "s2", name: "review", description: "Review code" },
          ],
        }),
      ],
    });

    expect((await items(qc)).map((i) => i.label)).toEqual(["deploy", "review"]);
  });

  it("filters skills by name case-insensitively", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "Deploy", description: "" },
            { id: "s2", name: "Review", description: "" },
          ],
        }),
      ],
    });

    expect((await items(qc, "dep")).map((i) => i.id)).toEqual(["s1"]);
  });

  it("filters skills by description", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "deploy", description: "Ship changes" },
            { id: "s2", name: "review", description: "Read a pull request" },
          ],
        }),
      ],
    });

    expect((await items(qc, "pull")).map((i) => i.id)).toEqual(["s2"]);
  });

  it("ranks name prefix matches above description-only matches", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "grilling", description: "Grill the user about a plan" },
            { id: "s2", name: "prototype", description: "Build a throwaway prototype" },
            { id: "s3", name: "wayfinder", description: "Plan a huge chunk of work" },
          ],
        }),
      ],
    });

    expect((await items(qc, "wa")).map((i) => i.id)).toEqual(["s3", "s2"]);
  });

  it("ranks an exact name match ahead of a longer prefix match", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "reviewer", description: "" },
            { id: "s2", name: "review", description: "" },
          ],
        }),
      ],
    });

    expect((await items(qc, "review")).map((i) => i.id)).toEqual(["s2", "s1"]);
  });

  it("ranks a name prefix above a mid-name match", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "pr-review", description: "" },
            { id: "s2", name: "review", description: "" },
          ],
        }),
      ],
    });

    expect((await items(qc, "rev")).map((i) => i.id)).toEqual(["s2", "s1"]);
  });

  it("keeps the configured skill order within a match tier", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "deploy-web", description: "" },
            { id: "s2", name: "deploy-api", description: "" },
          ],
        }),
      ],
    });

    expect((await items(qc, "deploy")).map((i) => i.id)).toEqual(["s1", "s2"]);
  });

  it("keeps a name match inside the 20-item cap when description hits fill it", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            ...Array.from({ length: 25 }, (_, i) => ({
              id: `d${i}`,
              name: `skill-${i}`,
              description: "Build a throwaway prototype",
            })),
            { id: "s-named", name: "wayfinder", description: "" },
          ],
        }),
      ],
    });

    const result = await items(qc, "wa");
    expect(result).toHaveLength(20);
    expect(result[0]?.id).toBe("s-named");
  });

  it("tolerates skills with missing descriptions from cached API data", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [
            { id: "s1", name: "deploy" } as Agent["skills"][number],
          ],
        }),
      ],
    });

    expect(await items(qc, "dep")).toEqual([
      { id: "s1", label: "deploy", description: "" },
    ]);
  });

  it("returns empty when the active agent has no skills", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ id: "agent-1", skills: [] })],
    });

    expect(await items(qc)).toEqual([]);
  });

  it("caps results at 20", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: Array.from({ length: 25 }, (_, i) => ({
            id: `s${i}`,
            name: `skill-${i}`,
            description: "",
          })),
        }),
      ],
    });

    expect(await items(qc)).toHaveLength(20);
  });

  it("falls back to the first available agent when selectedAgentId is stale", async () => {
    chatState.selectedAgentId = "missing";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [
        agent({
          id: "agent-1",
          skills: [{ id: "s1", name: "deploy", description: "" }],
        }),
      ],
    });

    expect((await items(qc)).map((i) => i.id)).toEqual(["s1"]);
  });

  it("returns empty when no agents exist", async () => {
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [],
    });

    expect(await items(qc)).toEqual([]);
  });

  it("excludes skills from private agents the user cannot access", async () => {
    chatState.selectedAgentId = "private-agent";
    const qc = fakeQc({
      members: [
        { user_id: "u1", name: "Alice", role: "member" },
        { user_id: "u2", name: "Bob", role: "member" },
      ],
      agents: [
        agent({
          id: "private-agent",
          visibility: "private",
          permission_mode: "private",
          invocation_targets: [],
          owner_id: "u2",
          skills: [{ id: "private-skill", name: "secret", description: "" }],
        }),
      ],
    });

    expect(await items(qc)).toEqual([]);
  });
});

// Canonical copy/provenance cases live in skills/lib/skill-presentation.test.ts.
describe("built-in role skill slash suggestions", () => {
  const canonicalName = "multica-code-review";
  const skill: SkillSummary = {
    id: "review-skill",
    workspace_id: "ws-1",
    name: canonicalName,
    description: enSkills.builtin_role_skills[canonicalName].description,
    config: { origin: { type: "builtin_role_skill", name: canonicalName } },
    created_by: "u1",
    created_at: "",
    updated_at: "",
  };

  function configuredClient(
    fetchSkills?: () => Promise<SkillSummary[]>,
    workspaceSkills = [skill],
  ) {
    chatState.selectedAgentId = "agent-1";
    return fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ skills: [{ id: skill.id, name: skill.name, description: skill.description }] })],
      skills: workspaceSkills,
      fetchSkills,
    });
  }

  it("finds an assigned skill by Chinese purpose and English identifier in either locale", async () => {
    const qc = configuredClient();

    for (const locale of ["en", "zh-Hans"] as const) {
      createI18n(locale, TEST_RESOURCES);
      for (const query of ["代码审查", "MULTICA-CODE-REVIEW", "actionable"]) {
        expect((await items(qc, query)).map((item) => item.id)).toEqual([skill.id]);
      }
      const purpose = zhSkills.builtin_role_skills[canonicalName].description;
      expect((await items(qc, purpose)).map((item) => item.id)).toEqual([skill.id]);
    }
  });

  it("uses assigned UUIDs rather than matching unrelated workspace skills by name", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ skills: [{ id: "custom-copy", name: canonicalName, description: "Custom review" }] })],
      skills: [skill],
    });

    expect(await items(qc, "代码审查")).toEqual([]);
    expect(await items(qc, canonicalName)).toEqual([
      { id: "custom-copy", label: canonicalName, description: "Custom review" },
    ]);
  });

  it("keeps raw assigned suggestions available when metadata loading fails", async () => {
    const qc = configuredClient(() => Promise.reject(new Error("offline")), []);

    expect((await items(qc, canonicalName)).map(({ id, label }) => ({ id, label })))
      .toEqual([{ id: skill.id, label: canonicalName }]);
  });

  it("uses cached metadata immediately while its refresh is still unresolved", async () => {
    const refresh = vi.fn(() => new Promise<SkillSummary[]>(() => {}));
    const qc = configuredClient(refresh);

    expect(await immediateItems(qc, "代码审查")).toEqual([
      expect.objectContaining({ id: skill.id, label: canonicalName, skill }),
    ]);
    expect(refresh).toHaveBeenCalledOnce();
  });

  it("keeps cold-cache raw matches immediate while metadata loads", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ skills: [{ id: skill.id, name: skill.name, description: skill.description }] })],
      fetchSkills: () => new Promise<SkillSummary[]>(() => {}),
    });

    for (const query of ["", canonicalName, "actionable"]) {
      expect(await immediateItems(qc, query)).toEqual([
        { id: skill.id, label: canonicalName, description: skill.description },
      ]);
    }
  });

  it("loads provenance for a cold-cache query that has no raw match", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ skills: [{ id: skill.id, name: skill.name, description: skill.description }] })],
      fetchSkills: () => Promise.resolve([skill]),
    });

    expect((await items(qc, "代码审查")).map((item) => item.id)).toEqual([skill.id]);
  });

  it("never waits on offline-paused metadata for cached or cold suggestions", async () => {
    chatState.selectedAgentId = "agent-1";
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(workspaceKeys.members("ws-1"), [{ user_id: "u1", name: "Alice", role: "member" }]);
    qc.setQueryData(workspaceKeys.agents("ws-1"), [
      agent({ skills: [{ id: skill.id, name: skill.name, description: skill.description }] }),
    ]);
    qc.setQueryData(workspaceKeys.skills("ws-1"), [skill], { updatedAt: 1 });
    const fetchSkills = vi.mocked(api.listSkills).mockResolvedValue([skill]);
    onlineManager.setOnline(false);
    try {
      expect(await immediateItems(qc)).toEqual([
        expect.objectContaining({ id: skill.id, label: canonicalName }),
      ]);
      qc.removeQueries({ queryKey: workspaceKeys.skills("ws-1"), exact: true });
      expect(await immediateItems(qc, canonicalName)).toEqual([
        { id: skill.id, label: canonicalName, description: skill.description },
      ]);
      expect(await immediateItems(qc, "代码审查")).toEqual([]);
      void qc.fetchQuery({ ...skillListOptions("ws-1"), retry: false }).catch(() => []);
      expect(qc.getQueryState(workspaceKeys.skills("ws-1"))?.fetchStatus).toBe("paused");
      // A previously paused request may not resume as soon as connectivity
      // changes (for example while its QueryClient has no mounted observer).
      onlineManager.setOnline(true);
      expect(await immediateItems(qc, "代码审查")).toEqual([]);
      expect(fetchSkills).not.toHaveBeenCalled();
    } finally {
      qc.clear();
      onlineManager.setOnline(true);
      fetchSkills.mockReset();
    }
  });

  it("ranks an exact Chinese name before description hits in the English UI", async () => {
    createI18n("en", { en: TEST_RESOURCES.en });
    chatState.selectedAgentId = "agent-1";
    const qc = fakeQc({
      members: [{ user_id: "u1", name: "Alice", role: "member" }],
      agents: [agent({ skills: [
        ...Array.from({ length: 20 }, (_, index) => ({
          id: `description-${index}`,
          name: `custom-${index}`,
          description: "与代码审查有关的自定义说明",
        })),
        { id: skill.id, name: skill.name, description: skill.description },
      ] })],
      skills: [skill],
    });

    const results = await items(qc, "代码审查");
    expect(results).toHaveLength(20);
    expect(results[0]?.id).toBe(skill.id);
  });

  it("updates the visible language while inserting the unchanged slash command identity", async () => {
    const qc = configuredClient();
    const skillItems = await items(qc);
    const insertContentAt = vi.fn().mockReturnThis();
    const chain = { focus: vi.fn().mockReturnThis(), insertContentAt, run: vi.fn() };
    const editor = {
      chain: () => chain,
      view: { state: { selection: { $to: { nodeAfter: null } } } },
    };
    const suggestion = createSlashCommandSuggestion(qc);
    const command = (props: SlashCommandItem) => suggestion.command!({
      editor,
      range: { from: 1, to: 2 },
      props,
    } as never);
    const list = <SlashCommandList items={skillItems} query="" command={command} />;
    const view = render(
      <I18nProvider locale="zh-Hans" resources={TEST_RESOURCES}>
        {list}
      </I18nProvider>,
    );

    expect(view.getByText("/代码审查")).toBeInTheDocument();
    expect(view.getByText(zhSkills.builtin_role_skills[canonicalName].description)).toBeInTheDocument();
    window.getSelection()?.collapse(view.container, 0);
    fireEvent.click(view.getByRole("button"));
    expect(insertContentAt).toHaveBeenCalledWith({ from: 1, to: 2 }, [
      { type: "slashCommand", attrs: { id: skill.id, label: canonicalName, mentionSuggestionChar: "/" } },
      { type: "text", text: " " },
    ]);

    view.rerender(
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        {list}
      </I18nProvider>,
    );
    expect(view.getByText(`/${canonicalName}`)).toBeInTheDocument();
    expect(view.queryByText("/代码审查")).not.toBeInTheDocument();
  });
});

describe("SlashCommandList keyboard handling", () => {
  it("lets Enter and arrow keys fall through when there are no selectable items", () => {
    const ref = createRef<SlashCommandListRef>();

    render(
      <I18nWrapper>
        <SlashCommandList ref={ref} items={[]} query="" command={vi.fn()} />
      </I18nWrapper>,
    );

    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "Enter" }),
      }),
    ).toBe(false);
    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "Enter", metaKey: true }),
      }),
    ).toBe(false);
    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "ArrowUp" }),
      }),
    ).toBe(false);
    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "ArrowDown" }),
      }),
    ).toBe(false);
  });

  it("handles Enter and arrow keys when selectable items exist", () => {
    const ref = createRef<SlashCommandListRef>();
    const command = vi.fn();
    const selectableItems: SlashCommandItem[] = [
      { id: "s1", label: "deploy", description: "Ship changes" },
      { id: "s2", label: "review", description: "Review code" },
    ];

    render(
      <I18nWrapper>
        <SlashCommandList
          ref={ref}
          items={selectableItems}
          query=""
          command={command}
        />
      </I18nWrapper>,
    );

    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "ArrowUp" }),
      }),
    ).toBe(true);
    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "ArrowDown" }),
      }),
    ).toBe(true);
    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "Enter" }),
      }),
    ).toBe(true);
    expect(command).toHaveBeenCalledWith(selectableItems[0]);
  });

  // MUL-5495: same Ctrl aliases the command bar (cmdk) accepts, so the slash
  // picker navigates like every other list in the product.
  it("navigates with Ctrl+N/J and Ctrl+P/K, and leaves the bare letters alone", () => {
    const ref = createRef<SlashCommandListRef>();
    const command = vi.fn();
    const selectableItems: SlashCommandItem[] = [
      { id: "s1", label: "deploy", description: "Ship changes" },
      { id: "s2", label: "review", description: "Review code" },
      { id: "s3", label: "note", description: "Leave a note" },
    ];

    render(
      <I18nWrapper>
        <SlashCommandList
          ref={ref}
          items={selectableItems}
          query=""
          command={command}
        />
      </I18nWrapper>,
    );

    const highlightedLabel = () => {
      const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>("button"));
      return buttons.find((b) => b.classList.contains("bg-accent"))?.textContent ?? "";
    };
    let handled: boolean | undefined;
    const press = (init: KeyboardEventInit) =>
      act(() => {
        handled = ref.current?.onKeyDown({ event: new KeyboardEvent("keydown", init) });
      });

    press({ key: "n", ctrlKey: true });
    expect(handled).toBe(true);
    expect(highlightedLabel()).toContain("/review");

    press({ key: "j", ctrlKey: true });
    expect(highlightedLabel()).toContain("/note");

    press({ key: "p", ctrlKey: true });
    expect(highlightedLabel()).toContain("/review");

    press({ key: "k", ctrlKey: true });
    expect(highlightedLabel()).toContain("/deploy");

    // Bare letters stay query characters — "/note" must remain typeable.
    press({ key: "n" });
    expect(handled).toBe(false);
    expect(highlightedLabel()).toContain("/deploy");

    press({ key: "Enter" });
    expect(command).toHaveBeenCalledWith(selectableItems[0]);
  });

  // MUL-3685: plain Tab accepts the highlighted item like Enter; Shift+Tab and
  // modifier+Tab fall through so reverse focus / OS switching are preserved.
  it("accepts the highlighted item on plain Tab, ignoring Shift/modifier+Tab", () => {
    const ref = createRef<SlashCommandListRef>();
    const command = vi.fn();
    const selectableItems: SlashCommandItem[] = [
      { id: "s1", label: "deploy", description: "Ship changes" },
      { id: "s2", label: "review", description: "Review code" },
    ];

    render(
      <I18nWrapper>
        <SlashCommandList
          ref={ref}
          items={selectableItems}
          query=""
          command={command}
        />
      </I18nWrapper>,
    );

    const press = (init: KeyboardEventInit) =>
      ref.current?.onKeyDown({ event: new KeyboardEvent("keydown", init) });

    expect(press({ key: "Tab", shiftKey: true })).toBe(false);
    expect(press({ key: "Tab", metaKey: true })).toBe(false);
    expect(command).not.toHaveBeenCalled();

    expect(press({ key: "Tab" })).toBe(true);
    expect(command).toHaveBeenCalledWith(selectableItems[0]);
  });

  it("lets Tab fall through when there are no selectable items, like Enter", () => {
    const ref = createRef<SlashCommandListRef>();

    render(
      <I18nWrapper>
        <SlashCommandList ref={ref} items={[]} query="" command={vi.fn()} />
      </I18nWrapper>,
    );

    expect(
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "Tab" }),
      }),
    ).toBe(false);
  });
});

describe("SlashCommandList empty states", () => {
  it("shows a configured-skills empty state before search text is entered", () => {
    const { getByText } = render(
      <I18nWrapper>
        <SlashCommandList items={[]} query="" command={vi.fn()} />
      </I18nWrapper>,
    );

    expect(getByText("No skills configured")).toBeInTheDocument();
  });

  it("shows a no-results empty state when search text has no matches", () => {
    const { getByText } = render(
      <I18nWrapper>
        <SlashCommandList items={[]} query="deploy" command={vi.fn()} />
      </I18nWrapper>,
    );

    expect(getByText("No matching skills")).toBeInTheDocument();
  });

  it("renders nothing on empty items when hideOnEmpty is set (command menu)", () => {
    const { container } = render(
      <I18nWrapper>
        <SlashCommandList items={[]} query="6" command={vi.fn()} hideOnEmpty />
      </I18nWrapper>,
    );

    // No popup box on a non-matching `/` (e.g. typing a date like 6/8).
    expect(container).toBeEmptyDOMElement();
  });
});

describe("buildBuiltinCommandItems", () => {
  it("returns the full built-in command set for an empty query", () => {
    expect(buildBuiltinCommandItems("")).toEqual(BUILTIN_COMMANDS);
  });

  it("includes /note while the query is a prefix of the label", () => {
    expect(buildBuiltinCommandItems("no").map((c) => c.id)).toEqual(["note"]);
    expect(buildBuiltinCommandItems("NOTE").map((c) => c.id)).toEqual(["note"]);
  });

  it("matches the label as a prefix only — not the description", () => {
    // "agent" appears in the description but is not a label prefix.
    expect(buildBuiltinCommandItems("agent")).toEqual([]);
    // A non-prefix substring of the label does not match either.
    expect(buildBuiltinCommandItems("ote")).toEqual([]);
  });

  it("returns nothing for a query that matches no command", () => {
    expect(buildBuiltinCommandItems("deploy")).toEqual([]);
  });
});

describe("SlashCommandList built-in command rendering", () => {
  it("renders the localized description for a built-in command", () => {
    const { getByText } = render(
      <I18nWrapper>
        <SlashCommandList
          items={buildBuiltinCommandItems("")}
          query=""
          command={vi.fn()}
          hideOnEmpty
        />
      </I18nWrapper>,
    );

    expect(getByText("/note")).toBeInTheDocument();
    expect(
      getByText("Add a note — won't trigger any agents"),
    ).toBeInTheDocument();
  });
});


// Async quick-action rendering in the `/` menu (MUL-5465, review finding #4).
//
// The render request resolves after an arbitrary delay, during which the user
// keeps typing. Three behaviours have to hold, and each one was a real bug at
// some point in this PR:
//   - a rejection must not destroy what the user typed
//   - a success must replace the ORIGINAL command, not wherever the caret is
//   - a command edited mid-flight must be left alone, not overwritten
describe("builtin `/` menu — async quick action rendering", () => {
  // Minimal editor stand-in: enough ProseMirror surface for the command to
  // read the range text and issue its chain.
  function fakeEditor(text: string) {
    const calls: { from: number; to: number; content: string; contentType?: string }[] = [];
    let docText = text;
    const chain = {
      focus: () => chain,
      insertContentAt: (
        range: { from: number; to: number },
        content: string,
        opts?: { contentType?: string },
      ) => {
        calls.push({ from: range.from, to: range.to, content, contentType: opts?.contentType });
        return chain;
      },
      insertContent: (content: string) => {
        calls.push({ from: -1, to: -1, content });
        return chain;
      },
      deleteRange: () => chain,
      run: () => true,
    };
    return {
      calls,
      setText: (next: string) => {
        docText = next;
      },
      editor: {
        chain: () => chain,
        state: {
          doc: {
            get content() {
              return { size: docText.length + 1 };
            },
            textBetween: (from: number, to: number) => docText.slice(from, to),
          },
        },
        view: { state: { selection: { $to: { nodeAfter: null } } } },
      },
    };
  }

  const range = { from: 0, to: 7 };
  const item = { id: `${QUICK_ACTION_ITEM_PREFIX}qa-1`, label: "review" };

  it("leaves the typed command intact and reports the failure when render rejects", async () => {
    const { editor, calls } = fakeEditor("/review");
    const onRenderError = vi.fn();
    const suggestion = createBuiltinCommandSuggestion({
      renderQuickAction: () => Promise.reject(new Error("boom")),
      onRenderError,
    });

    suggestion.command!({ editor, range, props: item } as never);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(calls).toHaveLength(0);
    expect(onRenderError).toHaveBeenCalledTimes(1);
  });

  it("replaces the original range once a delayed render resolves", async () => {
    const { editor, calls } = fakeEditor("/review");
    let resolve!: (v: string) => void;
    const suggestion = createBuiltinCommandSuggestion({
      renderQuickAction: () => new Promise<string>((r) => { resolve = r; }),
    });

    suggestion.command!({ editor, range, props: item } as never);
    expect(calls).toHaveLength(0); // nothing destroyed while in flight

    await act(async () => {
      resolve("rendered body");
      await Promise.resolve();
      await Promise.resolve();
    });

    // contentType must be "markdown": inserted as a plain string, the
    // server-rendered `[@Name](mention://…)` lands as literal text and
    // serialises back out with escaped brackets, so the mention never becomes
    // a node and renders as raw markup in the thread.
    expect(calls).toEqual([
      { from: 0, to: 7, content: "rendered body", contentType: "markdown" },
    ]);
  });

  it("abandons the insert when the command was edited while the request was open", async () => {
    const { editor, calls, setText } = fakeEditor("/review");
    let resolve!: (v: string) => void;
    const suggestion = createBuiltinCommandSuggestion({
      renderQuickAction: () => new Promise<string>((r) => { resolve = r; }),
    });

    suggestion.command!({ editor, range, props: item } as never);
    // The user rewrites the command; a prefix-only check would still see a
    // leading "/" here and clobber it.
    setText("/fixnow");

    await act(async () => {
      resolve("rendered body");
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(calls).toHaveLength(0);
  });
});
