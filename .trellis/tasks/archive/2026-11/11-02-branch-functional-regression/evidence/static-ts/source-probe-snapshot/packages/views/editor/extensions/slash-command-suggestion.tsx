"use client";

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import { onlineManager, type QueryClient } from "@tanstack/react-query";
import { getI18n } from "react-i18next";
import type { SuggestionOptions } from "@tiptap/suggestion";
import { PluginKey } from "@tiptap/pm/state";
import { useAuthStore } from "@multica/core/auth";
import { useChatStore } from "@multica/core/chat";
import { getCurrentWsId } from "@multica/core/platform";
import { canAssignAgentToIssue } from "@multica/core/permissions";
import { isImeComposing } from "@multica/core/utils";
import { skillListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import type { Agent, MemberWithUser, SkillSummary } from "@multica/core/types";
import { useT } from "../../i18n";
import { useSkillPresentation } from "../../skills/hooks/use-skill-presentation";
import {
  getSkillPresentation,
  type SkillPresentationInput,
} from "../../skills/lib/skill-presentation";
import {
  createSuggestionPopupRender,
} from "./suggestion-popup";
import {
  isPickerAcceptKey,
  pickerNavigationDirection,
} from "../../common/picker-keys";
import { isTriggerArmedAt } from "./suggestion-trigger-arming";

const MAX_ITEMS = 20;

/** Known built-in command ids — the keys under editor `slash_command.commands`. */
export type BuiltinCommandKey = "note";

export interface SlashCommandItem {
  id: string;
  label: string;
  /** Workspace metadata for display only; label remains the invocation name. */
  skill?: SkillPresentationInput;
  /** Raw description (skill picker). Built-in commands use descriptionKey. */
  description?: string;
  /**
   * For built-in commands: the i18n key under editor `slash_command.commands`.
   * When set, the menu renders the translated copy instead of `description`,
   * so the visible string stays localized (the typed `/label` does not).
   */
  descriptionKey?: BuiltinCommandKey;
}

interface SlashCommandListProps {
  items: SlashCommandItem[];
  query: string;
  command: (item: SlashCommandItem) => void;
  /**
   * When true, render nothing instead of an empty-state box when there are no
   * matching items. Used by the built-in command menu in issue comments, where
   * `/` is common in prose (paths, dates) and a popup on every slash would be
   * noise. The chat skill picker leaves this false so it can still explain
   * "no skills configured".
   */
  hideOnEmpty?: boolean;
}

export interface SlashCommandListRef {
  onKeyDown: (props: { event: KeyboardEvent }) => boolean;
}

export const SlashCommandList = forwardRef<
  SlashCommandListRef,
  SlashCommandListProps
>(function SlashCommandList({ items, query, command, hideOnEmpty = false }, ref) {
  const { t } = useT("editor");
  const presentSkill = useSkillPresentation();
  const [selectedIndex, setSelectedIndex] = useState(0);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [items]);

  useEffect(() => {
    itemRefs.current[selectedIndex]?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  const selectItem = useCallback(
    (index: number) => {
      const item = items[index];
      if (!item) return;
      command(item);
    },
    [items, command],
  );

  useImperativeHandle(ref, () => ({
    onKeyDown: ({ event }) => {
      if (isImeComposing(event)) return false;
      // Arrow keys plus the Ctrl+N/J/P/K aliases the command bar accepts —
      // see pickerNavigationDirection.
      const direction = pickerNavigationDirection(event);
      if (direction !== null) {
        if (items.length === 0) return false;
        const delta = direction === "next" ? 1 : items.length - 1;
        setSelectedIndex((i) => (i + delta) % items.length);
        return true;
      }
      // Enter is the canonical accept; plain Tab is an additive alias (see
      // isPickerAcceptKey). Shift/modifier+Tab fall through to focus nav.
      if (isPickerAcceptKey(event)) {
        if (items.length === 0) return false;
        selectItem(selectedIndex);
        return true;
      }
      return false;
    },
  }));

  if (items.length === 0) {
    if (hideOnEmpty) return null;
    return (
      <div className="rounded-md border bg-popover p-2 text-caption text-muted-foreground shadow-md">
        {t(($) =>
          query.trim()
            ? $.slash_command.no_results
            : $.slash_command.no_skills_configured,
        )}
      </div>
    );
  }

  // Built-in commands carry an i18n key; workspace skills use their provenance
  // to resolve live copy while leaving the command's raw label unchanged.
  const describe = (item: SlashCommandItem): string | undefined =>
    item.descriptionKey === "note"
      ? t(($) => $.slash_command.commands.note)
      : item.description;

  return (
    // Height budget clamps to min(design max, viewport-aware
    // `--suggestion-available-height` from suggestion-popup.tsx's size
    // middleware), falling back to the design max when rendered standalone.
    // Single height authority — mirrors MentionList.
    <div className="rounded-md border bg-popover py-1 shadow-md w-72 max-h-[min(300px,var(--suggestion-available-height,300px))] overflow-y-auto">
      {items.map((item, index) => {
        const presentation = item.skill ? presentSkill(item.skill) : null;
        const description = presentation?.description ?? describe(item);
        return (
          <button
            key={item.id}
            ref={(el) => {
              itemRefs.current[index] = el;
            }}
            className={`flex w-full flex-col gap-0.5 px-3 py-1.5 text-left text-caption transition-colors ${
              selectedIndex === index ? "bg-accent" : "hover:bg-accent/50"
            }`}
            onClick={() => selectItem(index)}
          >
            <span className="font-medium">/{presentation?.name ?? item.label}</span>
            {description && (
              <span className="truncate text-muted-foreground">
                {description}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
});

const NO_MATCH = 4;

interface SearchableSkill {
  name: string;
  searchNames?: string[];
  description?: string;
  searchText?: string;
}

/** Returns the match tier: exact name, prefix, substring, then description. */
function skillMatchRank(
  skill: SearchableSkill,
  q: string,
): number {
  const names = skill.searchNames ?? [skill.name.toLowerCase()];
  if (names.some((name) => name === q)) return 0;
  if (names.some((name) => name.startsWith(q))) return 1;
  if (names.some((name) => name.includes(q))) return 2;
  if ((skill.searchText ?? skill.description ?? "").toLowerCase().includes(q)) return 3;
  return NO_MATCH;
}

/** Ranks matches by relevance while preserving configured order within each tier. */
function rankSkillMatches<T extends SearchableSkill>(
  skills: T[],
  q: string,
): T[] {
  if (!q) return skills;
  return skills
    .map((skill) => ({ skill, rank: skillMatchRank(skill, q) }))
    .filter((entry) => entry.rank !== NO_MATCH)
    .sort((a, b) => a.rank - b.rank)
    .map((entry) => entry.skill);
}

async function buildItems(qc: QueryClient, query: string): Promise<SlashCommandItem[]> {
  const wsId = getCurrentWsId();
  if (!wsId) return [];

  const agents: Agent[] = qc.getQueryData(workspaceKeys.agents(wsId)) ?? [];
  const members: MemberWithUser[] =
    qc.getQueryData(workspaceKeys.members(wsId)) ?? [];
  // Tiptap calls suggestion items outside React render, so direct store reads
  // are intentional here.
  const { selectedAgentId } = useChatStore.getState();
  const userId = useAuthStore.getState().user?.id ?? null;
  const memberRole = members.find((m) => m.user_id === userId)?.role ?? null;

  const availableAgents = agents.filter(
    (a) =>
      !a.archived_at &&
      canAssignAgentToIssue(a, { userId, role: memberRole }).allowed,
  );
  const activeAgent =
    availableAgents.find((a) => a.id === selectedAgentId) ??
    availableAgents[0] ??
    null;

  const assignedSkills = activeAgent?.skills ?? [];
  if (assignedSkills.length === 0) return [];

  const q = query.toLowerCase();
  const matchingItems = (workspaceSkills: SkillSummary[]): SlashCommandItem[] => {
    const skillsById = new Map(workspaceSkills.map((skill) => [skill.id, skill]));
    // Suggestion callbacks run outside React; read the current locale for
    // search, while SlashCommandList resolves its visible copy reactively.
    const t = getI18n()?.getFixedT(null, "skills");
    const searchableSkills = assignedSkills.map((assigned) => {
      const metadata = skillsById.get(assigned.id);
      const skill = metadata?.name === assigned.name ? metadata : undefined;
      const presentation = skill && t ? getSkillPresentation(skill, t) : null;
      const item: SlashCommandItem = {
        id: assigned.id,
        label: assigned.name,
        description: assigned.description ?? "",
        ...(skill ? { skill } : {}),
      };
      return {
        name: assigned.name,
        searchNames: presentation?.searchNames,
        description: assigned.description,
        searchText: presentation?.searchText,
        item,
      };
    });
    return rankSkillMatches(searchableSkills, q)
      .slice(0, MAX_ITEMS)
      .map(({ item }) => item);
  };

  const skillOptions = skillListOptions(wsId);
  const cachedSkills = qc.getQueryData(skillOptions.queryKey);
  const cachedItems = matchingItems(cachedSkills ?? []);
  if (
    !onlineManager.isOnline() ||
    qc.getQueryState(skillOptions.queryKey)?.fetchStatus === "paused"
  ) {
    return cachedItems;
  }

  const freshSkills = qc.fetchQuery({
    ...skillOptions,
    staleTime: 30 * 1000,
    retry: false,
  }).catch(() => cachedSkills ?? []);

  // Existing choices must not depend on a refresh. Only a cold-cache query
  // with no raw match needs provenance before it can offer a translation.
  if (cachedSkills !== undefined || cachedItems.length > 0) return cachedItems;
  return matchingItems(await freshSkills);
}

export function createSlashCommandSuggestion(qc: QueryClient): Omit<
  SuggestionOptions<SlashCommandItem>,
  "editor"
> {
  const pluginKey = new PluginKey("slashCommandSuggestion");

  return {
    char: "/",
    pluginKey,
    // Only open over a `/` the user actually typed, so a pasted path
    // (`/usr/local/bin`) never opens the skill picker (MUL-5429).
    shouldShow: ({ editor, range }) => isTriggerArmedAt(editor, range.from),
    items: ({ query }) => buildItems(qc, query),
    command: ({ editor, range, props }) => {
      const nodeAfter = editor.view.state.selection.$to.nodeAfter;
      const overrideSpace = nodeAfter?.text?.startsWith(" ");
      if (overrideSpace) {
        range.to += 1;
      }

      editor
        .chain()
        .focus()
        .insertContentAt(range, [
          {
            type: "slashCommand",
            attrs: {
              id: props.id,
              label: props.label,
              mentionSuggestionChar: "/",
            },
          },
          { type: "text", text: " " },
        ])
        .run();

      window.getSelection()?.collapseToEnd();
    },
    render: createSuggestionPopupRender<SlashCommandItem, SlashCommandItem, SlashCommandListRef, SlashCommandListProps>({
      pluginKey,
      component: SlashCommandList,
      getProps: (props) => ({
        items: props.items,
        query: props.query,
        command: props.command,
      }),
      onKeyDown: (ref, props) => ref?.onKeyDown(props) ?? false,
    }),
  };
}

// ---------------------------------------------------------------------------
// Built-in command menu (issue comments)
// ---------------------------------------------------------------------------

/**
 * Built-in slash commands offered in the issue comment composer. Unlike the
 * chat `/` picker (which lists the active agent's skills), these are a fixed,
 * hand-curated set. Currently only `/note`, which marks a comment as a
 * human-only note that won't trigger the assigned agent — mirrors the backend
 * `noteCommentPrefix` in server/internal/handler/comment.go.
 */
export const BUILTIN_COMMANDS: SlashCommandItem[] = [
  { id: "note", label: "note", descriptionKey: "note" },
];

/** Marks a menu entry as a configured quick action rather than a built-in. */
export const QUICK_ACTION_ITEM_PREFIX = "quick-action:";

export function isQuickActionItem(item: SlashCommandItem): boolean {
  return item.id.startsWith(QUICK_ACTION_ITEM_PREFIX);
}

export function quickActionIdFromItem(item: SlashCommandItem): string {
  return item.id.slice(QUICK_ACTION_ITEM_PREFIX.length);
}

// Match on the command label as a prefix only — the description is for display,
// not search. With a single command this keeps the menu predictable (typing
// `/no` surfaces `note`; an unrelated `/deploy` shows nothing).
export function buildBuiltinCommandItems(
  query: string,
  quickActions: { id: string; name: string; description?: string }[] = [],
): SlashCommandItem[] {
  const q = query.toLowerCase();
  // Quick actions lead: on an issue they are the reason a user reaches for
  // `/`, and `/note` is a rarely-used escape hatch.
  const actionItems: SlashCommandItem[] = quickActions.map((a) => ({
    id: `${QUICK_ACTION_ITEM_PREFIX}${a.id}`,
    label: a.name,
    description: a.description || undefined,
  }));
  return [...actionItems, ...BUILTIN_COMMANDS]
    .filter((c) => c.label.toLowerCase().startsWith(q))
    .slice(0, MAX_ITEMS);
}

export interface BuiltinCommandSuggestionOptions {
  /**
   * Configured quick actions offered alongside the built-ins. Read lazily on
   * every keystroke so a newly created action shows up without remounting the
   * editor.
   */
  getQuickActions?: () => { id: string; name: string; description?: string }[];
  /**
   * Resolves a quick action to the text it would post. Server-rendered, so the
   * inserted body is byte-identical to what clicking the sidebar button sends.
   * Returning "" (or throwing) must leave the composer untouched rather than
   * inserting a half-rendered prompt.
   */
  renderQuickAction?: (quickActionId: string) => Promise<string>;
  /**
   * Called when renderQuickAction rejects. The extension cannot show UI of its
   * own — a ProseMirror command runs outside React's tree — so the host turns
   * this into a toast. Without it a failed pick is completely silent.
   */
  onRenderError?: (error: unknown) => void;
}

export function createBuiltinCommandSuggestion(
  options: BuiltinCommandSuggestionOptions = {},
): Omit<SuggestionOptions<SlashCommandItem>, "editor"> {
  const pluginKey = new PluginKey("builtinCommandSuggestion");

  return {
    char: "/",
    pluginKey,
    // Only open over a `/` the user actually typed, so a pasted path
    // (`/usr/local/bin`) never opens the command menu (MUL-5429).
    shouldShow: ({ editor, range }) => isTriggerArmedAt(editor, range.from),
    items: ({ query }) => buildBuiltinCommandItems(query, options.getQuickActions?.() ?? []),
    command: ({ editor, range, props }) => {
      if (isQuickActionItem(props)) {
        const render = options.renderQuickAction;
        if (!render) return;
        const id = quickActionIdFromItem(props);

        // The "/query" text is deliberately left in place while the request
        // is in flight. Deleting first meant a failed or slow render destroyed
        // what the user typed with nothing to show for it, and the insert then
        // landed wherever the caret happened to be by the time it resolved.
        //
        // Snapshot the EXACT text under the range, not just its shape. A
        // prefix check ("does it still start with /") passes when the user
        // rewrote `/review` into `/fix` mid-request, and the stale response
        // would then overwrite the new command.
        const originalText = editor.state.doc.textBetween(range.from, range.to);

        void render(id)
          .then((content) => {
            if (!content) return;
            const withinDoc = range.to <= editor.state.doc.content.size;
            const unchanged =
              withinDoc && editor.state.doc.textBetween(range.from, range.to) === originalText;
            if (!unchanged) {
              // The command was edited, moved, or removed while the request
              // was outstanding. Inserting anywhere now would either clobber
              // the user's newer text or drop the body in an unrelated spot,
              // so this pick is simply abandoned.
              return;
            }
            editor
              .chain()
              .focus()
              // contentType: "markdown" is load-bearing. Without it Tiptap
              // inserts the string as literal TEXT, so the server-rendered
              // `[@Name](mention://agent/…)` never becomes a mention node —
              // it serialises back out with the brackets escaped
              // (`\[@Name\](…)`) and renders as raw markup in the thread.
              .insertContentAt({ from: range.from, to: range.to }, content, {
                contentType: "markdown",
              })
              .run();
            window.getSelection()?.collapseToEnd();
          })
          .catch((error: unknown) => {
            // The command text is still there, so the user can retry or edit
            // it by hand; the host surfaces why nothing was inserted.
            options.onRenderError?.(error);
          });
        return;
      }

      // Insert the plain-text prefix (e.g. "/note ") rather than a rich node,
      // so a menu selection and a hand-typed command are byte-identical and the
      // backend can detect the marker with a simple prefix match. The trailing
      // space terminates the suggestion match so the menu does not re-open.
      editor
        .chain()
        .focus()
        .insertContentAt(range, [{ type: "text", text: `/${props.label} ` }])
        .run();

      window.getSelection()?.collapseToEnd();
    },
    render: createSuggestionPopupRender<SlashCommandItem, SlashCommandItem, SlashCommandListRef, SlashCommandListProps>({
      pluginKey,
      component: SlashCommandList,
      getProps: (props) => ({
        items: props.items,
        query: props.query,
        command: props.command,
        hideOnEmpty: true,
      }),
      onKeyDown: (ref, props) => ref?.onKeyDown(props) ?? false,
    }),
  };
}
