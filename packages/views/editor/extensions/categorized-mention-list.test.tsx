import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { createRef } from "react";
import { QueryClient } from "@tanstack/react-query";
import { Editor, Extension } from "@tiptap/core";
import { EditorContent } from "@tiptap/react";
import { Suggestion } from "@tiptap/suggestion";
import { createSuggestionPopupRender } from "./suggestion-popup";
import {
  SuggestionTriggerArmingExtension,
  isTriggerArmedAt,
} from "./suggestion-trigger-arming";
import StarterKit from "@tiptap/starter-kit";
import { PluginKey } from "@tiptap/pm/state";
import { afterEach, describe, expect, it, vi } from "vitest";
import { workspaceKeys } from "@multica/core/workspace/queries";
import en from "../../locales/en/editor.json";
import { configureShortcutPlatform } from "@multica/core/shortcuts";
import { CategorizedMentionList } from "./categorized-mention-list";
import {
  createMentionSuggestion,
  type MentionItem,
  type MentionListRef,
} from "./mention-suggestion";

const state = vi.hoisted(() => ({
  ws: "ws",
  user: "user",
  exit: vi.fn(),
  search: vi.fn().mockResolvedValue({ issues: [] }),
  listeners: new Set<() => void>(),
  favorites: [{ type: "agent", id: "a0" }],
  ready: true,
}));
vi.mock("@multica/core/platform", () => ({
  getCurrentWsId: () => state.ws,
  subscribeToCurrentSlug: (fn: () => void) => {
    state.listeners.add(fn);
    return () => state.listeners.delete(fn);
  },
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(() => "authenticated", {
    getState: () => ({ user: { id: state.user } }),
    subscribe: (fn: () => void) => {
      state.listeners.add(fn);
      return () => state.listeners.delete(fn);
    },
  }),
}));
vi.mock("@multica/core/issues/stores/quick-create-store", () => ({
  captureQuickCreateScope: () => null,
  isQuickCreateStoreReady: () => state.ready,
  useQuickCreateStore: Object.assign(
    () => ({ favoriteActors: state.favorites, resetGeneration: 0 }),
    { getState: () => ({}), persist: { rehydrate: vi.fn() } },
  ),
}));
vi.mock("@multica/core/api", () => ({ api: { searchIssues: state.search } }));
vi.mock("@tiptap/suggestion", async (original) => ({
  ...(await original<object>()),
  exitSuggestion: state.exit,
}));
vi.mock("../../common/actor-avatar", () => ({
  ActorAvatar: ({ profileLink }: { profileLink?: boolean }) => {
    expect(profileLink).toBe(false);
    return null;
  },
}));
vi.mock("../../i18n", () => ({
  useT: () => ({
    t: (
      selector: (resource: typeof en) => string,
      values?: Record<string, string | number>,
    ) => {
      let text = selector(en);
      for (const [key, value] of Object.entries(values ?? {}))
        text = text.replace(`{{${key}}}`, String(value));
      return text;
    },
    i18n: { resolvedLanguage: "en" },
  }),
}));

const editors: Editor[] = [];
afterEach(() => {
  editors.splice(0).forEach((editor) => editor.destroy());
  state.ws = "ws";
  state.user = "user";
  state.ready = true;
  state.exit.mockClear();
  configureShortcutPlatform(null);
  localStorage.clear();
});
function setup(count = 550) {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  const agents = Array.from({ length: count }, (_, index) => ({
    id: `a${index}`,
    name: `Agent ${String(index).padStart(3, "0")}`,
    description:
      index === count - 1 ? `${"x".repeat(350)} rare responsibility` : "Saved role",
  }));
  qc.setQueryData(workspaceKeys.agents("ws"), agents);
  qc.setQueryData(workspaceKeys.members("ws"), []);
  qc.setQueryData(workspaceKeys.squads("ws"), [
    { id: "team", name: "Team", leader_id: "a0" },
  ]);
  const buildItems = (query: string): MentionItem[] => [
    ...(qc.getQueryData<typeof agents>(workspaceKeys.agents("ws")) ?? [])
      .filter((agent) => agent.name.toLowerCase().includes(query.toLowerCase()))
      .map((agent) => ({
        id: agent.id,
        label: agent.name,
        type: "agent" as const,
      })),
    { id: "team", label: "Team", type: "squad" },
  ];
  const editor = new Editor({ extensions: [StarterKit], content: "<p>@</p>" });
  editors.push(editor);
  const ref = createRef<MentionListRef>();
  const props = {
    query: "",
    command: vi.fn(),
    editor,
    qc,
    pluginKey: new PluginKey("testMention"),
    buildItems,
  };
  const view = render(<CategorizedMentionList {...props} ref={ref} />);
  return { ...view, props, ref, qc };
}

describe("categorized mention discovery", () => {
  it("pages the full directory and searches responsibilities beyond previews", () => {
    const { rerender, props, container } = setup();
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(51);
    fireEvent.click(screen.getByRole("button", { name: /Show more/ }));
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(101);
    for (let pageIndex = 0; pageIndex < 9; pageIndex++) {
      fireEvent.click(screen.getByRole("button", { name: /Show more/ }));
    }
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(551);
    expect(screen.queryByRole("button", { name: /Show more/ })).toBeNull();
    expect(container.querySelector('[data-mention-key="agent:a549"]')).toBeTruthy();
    rerender(<CategorizedMentionList {...props} query="rare responsibility" />);
    expect(
      container.querySelector('[data-mention-key="agent:a549"]'),
    ).toBeTruthy();
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(1);
  });
  it("keeps leaders in Agents and exposes an explicit leader subset", () => {
    const { container } = setup(4);
    fireEvent.click(screen.getByRole("button", { name: "Agents" }));
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(4);
    fireEvent.click(screen.getByRole("button", { name: "Squad leaders" }));
    expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(1);
    expect(
      container.querySelector('[data-mention-key="agent:a0"]'),
    ).toBeTruthy();
  });
  it("keeps the selected identity across appended pages and records only mention recency", () => {
    const { container, props, ref } = setup();
    const row = container.querySelector('[data-mention-key="agent:a12"]')!;
    fireEvent.mouseMove(row);
    fireEvent.click(screen.getByRole("button", { name: /Show more/ }));
    act(() => {
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "Enter" }),
      });
    });
    expect(props.command).toHaveBeenCalledWith(
      expect.objectContaining({ id: "a12", type: "agent" }),
    );
    expect(state.favorites).toEqual([{ type: "agent", id: "a0" }]);
    expect(localStorage.getItem("multica:mention-recency:ws")).toContain(
      "agent:a12",
    );
  });
  it("routes Alt+Down into controls and ignores IME-owned keys", () => {
    const { props, ref } = setup(4);
    act(() => {
      ref.current?.onKeyDown({
        event: new KeyboardEvent("keydown", { key: "ArrowDown", altKey: true }),
      });
    });
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "All" }),
    );
    fireEvent.keyDown(document.activeElement!, {
      key: "Escape",
      isComposing: true,
    });
    expect(state.exit).not.toHaveBeenCalled();
    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(state.exit).toHaveBeenCalledWith(props.editor.view, props.pluginKey);
    expect(props.command).not.toHaveBeenCalled();
  });
  it("rejects account changes even when the query is identical", () => {
    const { container, props } = setup(4);
    state.user = "other";
    act(() => state.listeners.forEach((listener) => listener()));
    fireEvent.click(container.querySelector('[data-mention-key="agent:a0"]')!);
    expect(state.exit).toHaveBeenCalled();
    expect(props.command).not.toHaveBeenCalled();
  });
  it("reacts to Query directory arrivals without a new editor query", async () => {
    const { qc, container } = setup(1);
    act(() =>
      qc.setQueryData(workspaceKeys.agents("ws"), [
        { id: "late", name: "Late arrival" },
      ]),
    );
    await waitFor(() =>
      expect(
        container.querySelector('[data-mention-key="agent:late"]'),
      ).toBeTruthy(),
    );
    expect(
      container.querySelector('[data-mention-key="agent:a0"]'),
    ).toBeFalsy();
  });
});

it("preserves typed trigger and selection through real-editor filter controls", async () => {
  const fixture = setup(4);
  fixture.unmount();
  const pluginKey = new PluginKey("categorizedIntegration");
  const accepted = vi.fn();
  const extension = Extension.create({
    name: "categorizedIntegration",
    addProseMirrorPlugins() {
      return [
        Suggestion<MentionItem>({
          editor: this.editor,
          pluginKey,
          shouldShow: ({ editor, range }) =>
            isTriggerArmedAt(editor, range.from),
          items: () => [],
          command: ({ editor, range, props }) => {
            accepted(props);
            editor.commands.insertContentAt(
              range,
              `[${props.type}:${props.id}]`,
            );
          },
          render: createSuggestionPopupRender({
            pluginKey,
            component: CategorizedMentionList,
            guardComposition: true,
            getProps: (props) => ({
              ...fixture.props,
              query: props.query,
              command: props.command,
              editor: props.editor,
              pluginKey,
            }),
            onKeyDown: (ref: MentionListRef | null | undefined, props) =>
              ref?.onKeyDown(props) ?? false,
          }),
        }),
      ];
    },
  });
  const editor = new Editor({
    extensions: [StarterKit, SuggestionTriggerArmingExtension, extension],
  });
  editors.push(editor);
  render(<EditorContent editor={editor} />);
  const type = (text: string) => {
    for (const character of text) {
      const { from, to } = editor.state.selection;
      const handled = editor.view.someProp("handleTextInput", (handler) =>
        handler(editor.view, from, to, character, () =>
          editor.state.tr.insertText(character, from, to),
        ),
      );
      if (!handled)
        editor.view.dispatch(editor.state.tr.insertText(character, from, to));
    }
  };
  await act(async () => {
    editor.commands.focus();
    type("@");
  });
  await screen.findByRole("button", { name: "Agents" });
  const selection = editor.state.selection.from;
  fireEvent.mouseDown(screen.getByRole("button", { name: "Agents" }));
  fireEvent.click(screen.getByRole("button", { name: "Agents" }));
  expect(editor.state.selection.from).toBe(selection);
  expect(pluginKey.getState(editor.state).active).toBe(true);
  await act(async () => type("Agent"));
  expect(pluginKey.getState(editor.state).query).toBe("Agent");
  fireEvent.keyDown(editor.view.dom, { key: "ArrowDown", altKey: true });
  expect(document.activeElement).toBe(
    screen.getByRole("button", { name: "Agents" }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Return to typing" }));
  fireEvent.keyDown(editor.view.dom, { key: "Enter" });
  expect(accepted).toHaveBeenCalledWith(
    expect.objectContaining({ type: "agent", id: "a0" }),
  );
  expect(editor.getText()).toContain("[agent:a0]");
});

it("resets paging and removed selection on filter round trips", () => {
  const { container, ref, props } = setup();
  fireEvent.mouseMove(
    container.querySelector('[data-mention-key="agent:a12"]')!,
  );
  fireEvent.click(screen.getByRole("button", { name: /Show more/ }));
  fireEvent.click(screen.getByRole("button", { name: "AI squads" }));
  fireEvent.click(screen.getByRole("button", { name: "All" }));
  expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(51);
  act(() => {
    ref.current?.onKeyDown({
      event: new KeyboardEvent("keydown", { key: "Enter" }),
    });
  });
  expect(props.command).toHaveBeenCalledWith(
    expect.objectContaining({ id: "a0" }),
  );
  expect(props.editor.view.dom.getAttribute("aria-activedescendant")).toBe(
    container.querySelector('[aria-selected="true"]')?.id,
  );
});

it("does not revive an asynchronously removed selection", async () => {
  const { qc, container, props, ref } = setup(4);
  const original = qc.getQueryData<Array<{ id: string; name: string }>>(
    workspaceKeys.agents("ws"),
  )!;
  fireEvent.mouseMove(
    container.querySelector('[data-mention-key="agent:a2"]')!,
  );
  act(() =>
    qc.setQueryData(
      workspaceKeys.agents("ws"),
      original.filter((item) => item.id !== "a2"),
    ),
  );
  await waitFor(() =>
    expect(
      container.querySelector('[data-mention-key="agent:a2"]'),
    ).toBeFalsy(),
  );
  act(() => qc.setQueryData(workspaceKeys.agents("ws"), original));
  await waitFor(() =>
    expect(
      container.querySelector('[data-mention-key="agent:a2"]'),
    ).toBeTruthy(),
  );
  act(() => {
    ref.current?.onKeyDown({
      event: new KeyboardEvent("keydown", { key: "Enter" }),
    });
  });
  expect(props.command).toHaveBeenCalledWith(
    expect.objectContaining({ id: "a0" }),
  );
});

it("keeps server-only issues during unrelated directory retry", async () => {
  state.search.mockClear();
  state.search.mockResolvedValueOnce({
    issues: [
      {
        id: "remote",
        identifier: "TEST-42",
        title: "Remote result",
        status: "todo",
      },
    ],
  });
  const { qc, props, rerender, container } = setup(4);
  rerender(<CategorizedMentionList {...props} query="Remote" />);
  await waitFor(() =>
    expect(
      container.querySelector('[data-mention-key="issue:remote"]'),
    ).toBeTruthy(),
  );
  act(() =>
    qc
      .getQueryCache()
      .find({ queryKey: workspaceKeys.agents("ws"), exact: true })!
      .setState({ status: "error", error: new Error("offline") }),
  );
  const retry = await screen.findByRole("button", { name: /Retry/ });
  fireEvent.mouseMove(
    container.querySelector('[data-mention-key="issue:remote"]')!,
  );
  fireEvent.click(retry);
  expect(
    container.querySelector('[data-mention-key="issue:remote"]'),
  ).toHaveAttribute("aria-selected", "true");
  await new Promise((resolve) => setTimeout(resolve, 200));
  expect(state.search).toHaveBeenCalledTimes(1);
  expect(
    container.querySelector('[data-mention-key="issue:remote"]'),
  ).toBeTruthy();
});

it("withholds favorites until scoped readiness and keeps All members outside paging", () => {
  state.ready = false;
  const { props, rerender, container } = setup();
  expect(screen.queryByText("My frequent choices")).toBeNull();
  rerender(
    <CategorizedMentionList
      {...props}
      buildItems={(query) => [
        ...props.buildItems(query),
        { id: "all", type: "all", label: "All members" },
      ]}
    />,
  );
  expect(container.querySelector('[data-mention-key="all:all"]')).toBeTruthy();
  expect(container.querySelectorAll("[data-mention-key]")).toHaveLength(51);
});

it("refreshes settled member rows and role-dependent eligibility through the mention factory", async () => {
  const { qc, props, rerender, container } = setup(1);
  qc.setQueryData(workspaceKeys.members("ws"), [
    { user_id: "user", name: "Current member", role: "member" },
    { user_id: "old", name: "Old teammate", role: "member" },
  ]);
  qc.setQueryData(workspaceKeys.agents("ws"), [
    {
      id: "restricted",
      name: "Workspace agent",
      archived_at: null,
      owner_id: "someone-else",
      permission_mode: "public_to",
      runtime_bound: true,
      runtime_id: "runtime",
      invocation_targets: [{ target_type: "workspace", target_id: "ws" }],
    },
  ]);
  const suggestion = createMentionSuggestion(qc);
  const buildItems = (query: string) => {
    const items = suggestion.items!({
      query,
      editor: props.editor,
      signal: new AbortController().signal,
    });
    if (!Array.isArray(items))
      throw new Error("Expected synchronous mention directory");
    return items;
  };
  rerender(<CategorizedMentionList {...props} buildItems={buildItems} />);
  expect(
    container.querySelector('[data-mention-key="member:old"]'),
  ).toBeTruthy();
  expect(
    container.querySelector('[data-mention-key="agent:restricted"]'),
  ).toBeTruthy();
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 30));
  });
  act(() =>
    qc.setQueryData(workspaceKeys.members("ws"), [
      { user_id: "new", name: "New teammate", role: "member" },
    ]),
  );
  expect(qc.getQueryState(workspaceKeys.members("ws"))?.status).toBe("success");
  await waitFor(() =>
    expect(
      container.querySelector('[data-mention-key="member:new"]'),
    ).toBeTruthy(),
  );
  expect(container.querySelector('[data-mention-key="member:old"]')).toBeNull();
  expect(
    container.querySelector('[data-mention-key="agent:restricted"]'),
  ).toBeNull();
});

it.each([
  ["macos", "⌥↓"],
  ["windows", "Alt+↓"],
] as const)("formats the filter shortcut for %s", (platform, shortcut) => {
  configureShortcutPlatform(platform);
  setup(1);
  expect(
    screen.getByText(
      `Type after @ to search names or responsibilities · ${shortcut}: filters`,
    ),
  ).toBeTruthy();
});
