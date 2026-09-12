// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { configStore } from "@multica/core/config";
import { I18nProvider } from "@multica/core/i18n/react";
import type { Agent } from "@multica/core/types";
import enCommon from "../../../locales/en/common.json";
import enAgents from "../../../locales/en/agents.json";
// The conversation-starter editor previews the chat empty state, so it reads the
// chat namespace for the built-in defaults it renders when nothing is set.
import enChat from "../../../locales/en/chat.json";
import zhCommon from "../../../locales/zh-Hans/common.json";
import zhAgents from "../../../locales/zh-Hans/agents.json";
import zhChat from "../../../locales/zh-Hans/chat.json";
import { NavigationProvider } from "../../../navigation";
import type { NavigationAdapter } from "../../../navigation";
import { InstructionsTab } from "./instructions-tab";

const TEST_RESOURCES = {
  en: { common: enCommon, agents: enAgents, chat: enChat },
  "zh-Hans": { common: zhCommon, agents: zhAgents, chat: zhChat },
};
const persistedPrompt = {
  label: "Review a PR",
  prompt: "Review the open pull request.",
};
const baseAgent: Agent = {
  id: "agent-1",
  workspace_id: "ws-1",
  runtime_id: "runtime-1",
  name: "Reviewer",
  description: "",
  instructions: "Review carefully.",
  conversation_starters: [persistedPrompt],
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
  owner_id: "user-1",
  skills: [],
  created_at: "2026-08-24T00:00:00Z",
  updated_at: "2026-08-24T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

function tab(
  agent: Agent,
  onSave = vi.fn().mockResolvedValue(undefined),
  {
    locale = "en",
    onDirtyChange,
  }: {
    locale?: "en" | "zh-Hans";
    onDirtyChange?: (dirty: boolean) => void;
  } = {},
) {
  return (
    <I18nProvider locale={locale} resources={{ [locale]: TEST_RESOURCES[locale] }}>
      <InstructionsTab
        agent={agent}
        onSave={onSave}
        onDirtyChange={onDirtyChange}
      />
    </I18nProvider>
  );
}

describe("InstructionsTab persisted-state synchronization", () => {
  beforeEach(() => {
    configStore.getState().setAgentConversationStartersSupported(true);
  });

  afterEach(() => {
    act(() => {
      configStore.getState().setAgentConversationStartersSupported(false);
    });
  });

  it("preserves an unsaved prompt across an equivalent agent-list refetch", async () => {
    const user = userEvent.setup();
    const { rerender } = render(tab(baseAgent));
    const label = screen.getByLabelText("Suggestion 1 label");
    await user.clear(label);
    await user.type(label, "Inspect the patch");

    rerender(
      tab({
        ...baseAgent,
        conversation_starters: [{ ...persistedPrompt }],
      }),
    );

    expect(screen.getByLabelText("Suggestion 1 label")).toHaveValue(
      "Inspect the patch",
    );
  });

  it("preserves dirty local state when persisted contents change", async () => {
    const user = userEvent.setup();
    const { rerender } = render(tab(baseAgent));
    const label = screen.getByLabelText("Suggestion 1 label");
    await user.clear(label);
    await user.type(label, "Inspect the patch");

    rerender(
      tab({
        ...baseAgent,
        conversation_starters: [
          { label: "Server-side change", prompt: "A different prompt." },
        ],
      }),
    );

    expect(screen.getByLabelText("Suggestion 1 label")).toHaveValue(
      "Inspect the patch",
    );
  });

  it("preserves submitted prompt edits when an optimistic update rolls back", async () => {
    let rejectSave!: (reason?: unknown) => void;
    const onSave = vi.fn(
      () =>
        new Promise<void>((_, reject) => {
          rejectSave = reject;
        }),
    );
    const user = userEvent.setup();
    const { rerender } = render(tab(baseAgent, onSave));
    const label = screen.getByLabelText("Suggestion 1 label");
    const prompt = screen.getByLabelText("Suggestion 1 prompt");
    await user.clear(label);
    await user.type(label, "Inspect the patch");
    await user.clear(prompt);
    await user.type(prompt, "Inspect the patch for correctness.");
    await user.click(screen.getByRole("button", { name: "Save" }));

    const optimisticPrompt = {
      label: "Inspect the patch",
      prompt: "Inspect the patch for correctness.",
    };
    rerender(
      tab(
        {
          ...baseAgent,
          conversation_starters: [optimisticPrompt],
        },
        onSave,
      ),
    );
    rerender(
      tab(
        {
          ...baseAgent,
          conversation_starters: [{ ...persistedPrompt }],
        },
        onSave,
      ),
    );
    await act(async () => rejectSave(new Error("Update failed")));

    expect(screen.getByLabelText("Suggestion 1 label")).toHaveValue(
      "Inspect the patch",
    );
    expect(screen.getByLabelText("Suggestion 1 prompt")).toHaveValue(
      "Inspect the patch for correctness.",
    );
  });

  it("omits conversation starters from settings writes to an older backend", async () => {
    configStore.getState().setAgentConversationStartersSupported(false);
    const onSave = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(tab(baseAgent, onSave));

    expect(
      screen.queryByText("Conversation starters"),
    ).not.toBeInTheDocument();
    const instructions = screen.getByLabelText("System prompt");
    await user.clear(instructions);
    await user.type(instructions, "Updated instructions.");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith({
      instructions: "Updated instructions.",
    });
  });
});

describe("InstructionsTab canonical instruction content", () => {
  beforeEach(() => {
    configStore.getState().setAgentConversationStartersSupported(false);
  });

  it("saves Chinese Markdown verbatim and reloads it in an English UI", async () => {
    const agent = { ...baseAgent, instructions: "# 职责\n\n审查提交的代码。" };
    const edited =
      "# 职责\n\n逐项复核中文任务。\n- 保留 `task_id` 和 `in_review`。\n- 智能体名称：{{AGENT_NAME}}。\n";
    const onSave = vi.fn<(updates: { instructions: string }) => Promise<void>>()
      .mockResolvedValue(undefined);
    const user = userEvent.setup();
    const { unmount } = render(tab(agent, onSave, { locale: "zh-Hans" }));
    const editor = screen.getByRole("textbox", {
      name: zhAgents.tab_body.instructions.system_prompt_label,
    });
    expect(editor).toHaveValue(agent.instructions);

    await user.clear(editor);
    await user.paste(edited);
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(onSave).toHaveBeenCalledExactlyOnceWith({ instructions: edited });
    const [saved] = onSave.mock.calls[0]!;
    unmount();
    render(tab({ ...agent, instructions: saved.instructions }, onSave));

    expect(screen.getByRole("textbox", { name: "System prompt" })).toHaveValue(edited);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("keeps a pristine Chinese instruction body clean across locale changes", async () => {
    const agent = { ...baseAgent, instructions: "# 职责\n\n只审查任务指定的变更。" };
    const onSave = vi.fn().mockResolvedValue(undefined);
    const onDirtyChange = vi.fn();
    const { rerender } = render(
      tab(agent, onSave, { locale: "zh-Hans", onDirtyChange }),
    );

    rerender(tab(agent, onSave, { locale: "en", onDirtyChange }));

    expect(await screen.findByRole("textbox", { name: "System prompt" })).toHaveValue(
      agent.instructions,
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    rerender(tab(agent, onSave, { locale: "zh-Hans", onDirtyChange }));

    expect(await screen.findByRole("textbox", {
      name: zhAgents.tab_body.instructions.system_prompt_label,
    })).toHaveValue(agent.instructions);
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    expect(onDirtyChange).not.toHaveBeenCalledWith(true);
    expect(onSave).not.toHaveBeenCalled();
  });

  it("keeps an unsaved Chinese draft across locale changes and saves that draft", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    const onDirtyChange = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(
      tab(baseAgent, onSave, { locale: "zh-Hans", onDirtyChange }),
    );
    const edited = "# 团队规则\n\n只审查本次任务的变更，保留 `multica issue` 命令。\n";
    const editor = screen.getByRole("textbox", {
      name: zhAgents.tab_body.instructions.system_prompt_label,
    });
    await user.clear(editor);
    await user.paste(edited);

    rerender(tab(baseAgent, onSave, { locale: "en", onDirtyChange }));

    expect(await screen.findByRole("textbox", { name: "System prompt" })).toHaveValue(edited);
    expect(screen.getByRole("button", { name: "Save" })).toBeEnabled();

    rerender(tab(baseAgent, onSave, { locale: "zh-Hans", onDirtyChange }));

    expect(await screen.findByRole("textbox", {
      name: zhAgents.tab_body.instructions.system_prompt_label,
    })).toHaveValue(edited);
    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
    expect(onSave).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "保存" }));
    expect(onSave).toHaveBeenCalledExactlyOnceWith({ instructions: edited });
  });

  it("keeps Mika's Chinese system layer read-only while saving only workspace notes", async () => {
    const agent = {
      ...baseAgent,
      name: "Mika",
      instructions: "# 工作区补充\n\n主要仓库是 github.com/acme/platform。",
      system_instructions: "# Mika\n\n你是工作区系统助手，遵循任务的语言回复。",
    };
    const onSave = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    const { rerender } = render(tab(agent, onSave, { locale: "zh-Hans" }));
    await user.click(screen.getByRole("button", { name: "查看" }));

    const systemLayer = screen.getByText(agent.system_instructions, {
      normalizer: (text) => text,
    });
    expect(systemLayer.tagName).toBe("PRE");
    expect(screen.getAllByRole("textbox")).toHaveLength(1);
    const editor = screen.getByRole("textbox", { name: "工作区补充" });
    expect(editor).toHaveValue(agent.instructions);

    const edited = "# 工作区补充\n\n团队偏好中文沟通。周五不部署。\n";
    await user.clear(editor);
    await user.paste(edited);
    const updatedSystem = "# Mika\n\n你是工作区系统助手，先检查任务上下文再回复。";
    rerender(tab({ ...agent, system_instructions: updatedSystem }, onSave, {
      locale: "zh-Hans",
    }));

    expect(screen.getByText(updatedSystem, { normalizer: (text) => text }).tagName).toBe("PRE");
    expect(screen.getByRole("textbox", { name: "工作区补充" })).toHaveValue(edited);
    await user.click(screen.getByRole("button", { name: "保存" }));
    expect(onSave).toHaveBeenCalledExactlyOnceWith({ instructions: edited });
  });
});

// The "customize" link in a chat's empty state lands here with ?focus=. The
// tab must bring the editor into view and then clear the param, so a refresh
// or a later tab switch does not replay the flash.
describe("InstructionsTab conversation-starters deep link", () => {
  // Mirrors FOCUS_FLASH_MS in instructions-tab.tsx.
  const FLASH_MS = 1600;

  beforeEach(() => {
    configStore.getState().setAgentConversationStartersSupported(true);
  });

  afterEach(() => {
    act(() => {
      configStore.getState().setAgentConversationStartersSupported(false);
    });
  });

  // A fresh adapter object per render — what the web platform actually hands
  // down, and the condition the flash-timer regression below depends on.
  const adapter = (search: string, replace = vi.fn()): NavigationAdapter => ({
    push: vi.fn(),
    replace,
    back: vi.fn(),
    pathname: "/acme/agents/agent-1",
    searchParams: new URLSearchParams(search),
    hash: "",
    getShareableUrl: (path: string) => `https://app.test${path}`,
  });

  function renderWithSearch(search: string) {
    const replace = vi.fn();
    const scrollIntoView = vi.fn();
    // jsdom has no layout, so the method does not exist at all.
    Element.prototype.scrollIntoView = scrollIntoView;
    const tree = (value: NavigationAdapter, agent: Agent) => (
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <NavigationProvider value={value}>
          <InstructionsTab agent={agent} onSave={vi.fn()} />
        </NavigationProvider>
      </I18nProvider>
    );
    const { rerender } = render(tree(adapter(search, replace), baseAgent));
    return {
      replace,
      scrollIntoView,
      /**
       * Re-render as the platform does: a new adapter object every time, and
       * optionally a different agent for an in-place navigation between two
       * agent pages on the same route.
       */
      settleUrl: (next: string, agent: Agent = baseAgent) =>
        rerender(tree(adapter(next, replace), agent)),
    };
  }

  const ringed = () =>
    document.querySelector<HTMLElement>("[class*='ring-brand']");

  it("scrolls to the editor and drops the focus param, keeping the view", () => {
    const { replace, scrollIntoView } = renderWithSearch(
      "view=instructions&focus=conversation_starters",
    );

    expect(scrollIntoView).toHaveBeenCalledWith({ block: "nearest" });
    expect(replace).toHaveBeenCalledWith(
      "/acme/agents/agent-1?view=instructions",
    );
  });

  // Regression: the flash timer must not hang off the effect's cleanup. The
  // navigation adapter is not referentially stable, so React would run that
  // cleanup on the very next render and cancel the timeout, leaving the ring
  // on the editor permanently.
  it("ends the flash instead of ringing the editor forever", () => {
    vi.useFakeTimers();
    try {
      const { settleUrl } = renderWithSearch(
        "view=instructions&focus=conversation_starters",
      );
      expect(ringed()).not.toBeNull();

      // The stripped URL arrives on a new adapter object, re-running the
      // effect. A flash timer owned by that effect's cleanup dies here.
      act(() => settleUrl("view=instructions"));
      act(() => {
        vi.advanceTimersByTime(FLASH_MS + 400);
      });

      expect(ringed()).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  // Regression: the guard against re-firing before the stripped URL lands must
  // not become a one-shot-per-mount latch. A chat window left open beside this
  // page can send the very same link again, and it has to land again.
  it("focuses again when the link is clicked a second time", () => {
    const { replace, scrollIntoView, settleUrl } = renderWithSearch(
      "view=instructions&focus=conversation_starters",
    );
    expect(scrollIntoView).toHaveBeenCalledTimes(1);

    act(() => settleUrl("view=instructions"));
    act(() => settleUrl("view=instructions&focus=conversation_starters"));

    expect(scrollIntoView).toHaveBeenCalledTimes(2);
    expect(replace).toHaveBeenCalledTimes(2);
    expect(ringed()).not.toBeNull();
  });

  // Regression: the same guard is keyed by agent, so navigating between two
  // agent pages on this route does not swallow the second agent's link.
  it("focuses a link aimed at a different agent", () => {
    const other: Agent = { ...baseAgent, id: "agent-2" };
    const { scrollIntoView, settleUrl } = renderWithSearch(
      "view=instructions&focus=conversation_starters",
    );
    expect(scrollIntoView).toHaveBeenCalledTimes(1);

    // The URL still carries focus — it is the NEXT agent's deep link.
    act(() => settleUrl("view=instructions&focus=conversation_starters", other));

    expect(scrollIntoView).toHaveBeenCalledTimes(2);
  });

  it("ignores an ordinary visit", () => {
    const { replace, scrollIntoView } = renderWithSearch("view=instructions");

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
  });
});
