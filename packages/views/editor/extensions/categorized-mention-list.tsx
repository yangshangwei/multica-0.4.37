"use client";

import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useId,
  useRef,
  useState,
} from "react";
import { useQuery, type QueryClient } from "@tanstack/react-query";
import type { Editor } from "@tiptap/core";
import { exitSuggestion } from "@tiptap/suggestion";
import type { PluginKey } from "@tiptap/pm/state";
import { getCurrentWsId, subscribeToCurrentSlug } from "@multica/core/platform";
import { useAuthStore } from "@multica/core/auth";
import {
  agentListOptions,
  memberListOptions,
  squadListOptions,
} from "@multica/core/workspace/queries";
import {
  captureQuickCreateScope,
  isQuickCreateStoreReady,
  useQuickCreateStore,
} from "@multica/core/issues/stores/quick-create-store";
import { api } from "@multica/core/api";
import { createShortcutChord, formatShortcut } from "@multica/core/shortcuts";
import { isImeComposing } from "@multica/core/utils";
import {
  buildActorCatalog,
  searchActors,
  actorKey,
} from "../../common/actor-discovery";
import { ActorAvatar } from "../../common/actor-avatar";
import {
  isPickerAcceptKey,
  pickerNavigationDirection,
} from "../../common/picker-keys";
import { useT } from "../../i18n";
import { blockedReasonLabel } from "../../issues/blocked-trigger-copy";
import type { MentionItem, MentionListRef } from "./mention-suggestion";
import { demoteCancelledItems, issueToMention } from "./mention-ranking";
import { getRecencyMap, recordMentionUsage } from "./mention-recency";

const categories = ["all", "agent", "squad", "member", "issue"] as const;
type Category = (typeof categories)[number];
const keyOf = (item: MentionItem) => `${item.type}:${item.id}`;

export interface CategorizedMentionListProps {
  query: string;
  command: (item: MentionItem) => void;
  editor: Editor;
  qc: QueryClient;
  pluginKey: PluginKey;
  buildItems: (query: string) => MentionItem[];
}

export const CategorizedMentionList = forwardRef<
  MentionListRef,
  CategorizedMentionListProps
>(function CategorizedMentionList(
  { query, command, editor, qc, pluginKey, buildItems },
  ref,
) {
  const { t, i18n } = useT("editor");
  const { t: issuesT } = useT("issues");
  const [scope] = useState(() => ({
    ws: getCurrentWsId() ?? "",
    user: useAuthStore.getState().user?.id,
  }));
  const current = useCallback(
    () =>
      getCurrentWsId() === scope.ws &&
      useAuthStore.getState().user?.id === scope.user,
    [scope],
  );
  const members = useQuery(
    {
      ...memberListOptions(scope.ws),
      notifyOnChangeProps: ["data", "isPending", "isError"],
    },
    qc,
  );
  const agents = useQuery(agentListOptions(scope.ws), qc);
  const squads = useQuery(squadListOptions(scope.ws), qc);
  const preferences = useQuickCreateStore();
  const authStatus = useAuthStore((state) => state.status);
  const [category, setCategory] = useState<Category>("all");
  const [leaders, setLeaders] = useState(false);
  const [page, setPage] = useState({ query, category, leaders, limit: 50 });
  const limit =
    page.query === query &&
    page.category === category &&
    page.leaders === leaders
      ? page.limit
      : 50;
  if (
    page.query !== query ||
    page.category !== category ||
    page.leaders !== leaders
  ) {
    setPage({ query, category, leaders, limit: 50 });
  }
  const [selected, setSelected] = useState<string | null>(null);
  const [search, setSearch] = useState<{
    query: string;
    items: MentionItem[];
    pending: boolean;
    failed: boolean;
  }>({ query: "", items: [], pending: false, failed: false });
  const [retry, setRetry] = useState(0);
  const listId = useId();
  const toolbar = useRef<HTMLDivElement>(null);
  const rows = useRef(new Map<string, HTMLButtonElement>());
  const focusEditor = () => editor.view.focus();
  const composing = (event: KeyboardEvent) =>
    isImeComposing(event) || editor.view.composing;
  useEffect(() => {
    const closeStale = () => {
      if (!current() && !editor.isDestroyed)
        exitSuggestion(editor.view, pluginKey);
    };
    const offWorkspace = subscribeToCurrentSlug(closeStale);
    const offAuth = useAuthStore.subscribe(closeStale);
    return () => {
      offWorkspace();
      offAuth();
    };
  }, [editor, pluginKey, current]);
  useEffect(() => {
    if (
      authStatus === "authenticated" &&
      scope.user &&
      captureQuickCreateScope(scope.ws, scope.user) &&
      !isQuickCreateStoreReady(
        useQuickCreateStore.getState(),
        scope.ws,
        scope.user,
      )
    )
      void useQuickCreateStore.persist.rehydrate();
  }, [scope, authStatus, preferences.resetGeneration]);
  useEffect(() => {
    if (!query.trim()) return;
    const controller = new AbortController();
    setSearch({ query, items: [], pending: true, failed: false });
    const timer = setTimeout(() => {
      void api
        .searchIssues({
          q: query.trim(),
          limit: 20,
          include_closed: true,
          signal: controller.signal,
        })
        .then((result) => {
          if (!controller.signal.aborted && current())
            setSearch({
              query,
              items: result.issues.map(issueToMention),
              pending: false,
              failed: false,
            });
        })
        .catch(() => {
          if (!controller.signal.aborted && current())
            setSearch({ query, items: [], pending: false, failed: true });
        });
    }, 150);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [query, retry, current]);

  const base = current() ? buildItems("") : [];
  const allowed = new Set(base.map(keyOf));
  const catalog = buildActorCatalog(
    (agents.data ?? []).filter((agent) => allowed.has(`agent:${agent.id}`)),
    (squads.data ?? []).filter((squad) => allowed.has(`squad:${squad.id}`)),
  );
  const metadata = new Map(catalog.map((actor) => [actorKey(actor), actor]));
  const ranked = searchActors(
    catalog,
    query,
    "all",
    i18n.resolvedLanguage ?? "en",
  );
  const byKey = new Map(base.map((item) => [keyOf(item), item]));
  const actorItems = ranked.flatMap((actor) => {
    const item = byKey.get(actorKey(actor));
    return item ? [item] : [];
  });
  const otherItems = current()
    ? buildItems(query).filter(
        (item) => item.type !== "agent" && item.type !== "squad",
      )
    : [];
  const server = search.query === query && query.trim() ? search.items : [];
  const unique = new Map(
    [...actorItems, ...otherItems, ...server].map((item) => [
      keyOf(item),
      item,
    ]),
  );
  const matches = demoteCancelledItems([...unique.values()], query).filter(
    (item) =>
      (category === "all" ||
        item.type === category ||
        (category === "member" && item.type === "all")) &&
      (category !== "agent" ||
        !leaders ||
        !!metadata.get(keyOf(item))?.leadsSquads.length),
  );
  const ready = isQuickCreateStoreReady(preferences, scope.ws, scope.user);
  const favoriteKeys = new Set(
    ready ? preferences.favoriteActors.map(actorKey) : [],
  );
  const recency = getRecencyMap(scope.ws);
  const favorites = query.trim()
    ? []
    : matches.filter((item) => favoriteKeys.has(keyOf(item))).slice(0, 3);
  const recent = query.trim()
    ? []
    : matches
        .filter(
          (item) =>
            item.type !== "all" &&
            !favoriteKeys.has(keyOf(item)) &&
            recency[keyOf(item)],
        )
        .sort((a, b) => recency[keyOf(b)]! - recency[keyOf(a)]!)
        .slice(0, 5);
  const promoted = new Set([...favorites, ...recent].map(keyOf));
  const directory = matches.filter((item) => !promoted.has(keyOf(item)));
  const labels: Record<Category, string> = {
    all: t(($) => $.mention.category_all),
    agent: t(($) => $.mention.category_agents),
    squad: t(($) => $.mention.category_squads),
    member: t(($) => $.mention.category_members),
    issue: t(($) => $.mention.group_issues),
  };
  const localDirectory = directory.filter(
    (item) => item.type !== "issue" && item.type !== "all",
  );
  const localPage = new Set(localDirectory.slice(0, limit).map(keyOf));
  const paged = directory.filter(
    (item) =>
      item.type === "issue" ||
      item.type === "all" ||
      localPage.has(keyOf(item)),
  );
  const groups = [
    { label: "", items: paged.filter((item) => item.type === "all") },
    { label: t(($) => $.mention.frequent), items: favorites },
    { label: t(($) => $.mention.recent_mentions), items: recent },
    ...(query.trim()
      ? [{ label: "", items: paged.filter((item) => item.type !== "all") }]
      : [
          ...categories
            .filter((value) => value !== "all")
            .map((value) => ({
              label: labels[value],
              items: paged.filter((item) => item.type === value),
            })),
        ]),
  ];
  const visible = groups.flatMap((group) => group.items);
  const selectable = visible.filter((item) => !item.disabledReason);
  const active =
    selectable.find((item) => keyOf(item) === selected) ?? selectable[0];
  if (
    selected !== null &&
    !selectable.some((item) => keyOf(item) === selected)
  ) {
    setSelected(active ? keyOf(active) : null);
  }
  const accept = (item: MentionItem) => {
    if (!current() || item.disabledReason) return;
    const live = buildItems("").find(
      (candidate) => keyOf(candidate) === keyOf(item),
    );
    if (item.type !== "issue" && (!live || live.disabledReason)) return;
    recordMentionUsage(scope.ws, item);
    command(item);
  };
  const activeKey = active ? keyOf(active) : null;
  useEffect(() => {
    const dom = editor.view.dom;
    dom.setAttribute("aria-controls", listId);
    if (activeKey)
      dom.setAttribute("aria-activedescendant", `${listId}-${activeKey}`);
    else dom.removeAttribute("aria-activedescendant");
    return () => {
      dom.removeAttribute("aria-controls");
      dom.removeAttribute("aria-activedescendant");
    };
  }, [editor, listId, activeKey]);
  useEffect(() => {
    if (activeKey)
      rows.current.get(activeKey)?.scrollIntoView({ block: "nearest" });
  }, [activeKey]);
  useImperativeHandle(ref, () => ({
    onKeyDown: ({ event }) => {
      if (composing(event)) return false;
      if (event.altKey && event.key === "ArrowDown") {
        toolbar.current
          ?.querySelector<HTMLButtonElement>('[aria-pressed="true"]')
          ?.focus();
        return true;
      }
      const direction = pickerNavigationDirection(event);
      if (direction) {
        const index = selectable.findIndex((item) => item === active);
        const next =
          selectable[
            (index + (direction === "next" ? 1 : selectable.length - 1)) %
              selectable.length
          ];
        if (next) setSelected(keyOf(next));
        return true;
      }
      if (isPickerAcceptKey(event)) {
        if (active) accept(active);
        return true;
      }
      return false;
    },
  }));

  const issueCategory = category === "all" || category === "issue";
  const loading =
    members.isPending ||
    (category !== "member" &&
      category !== "issue" &&
      (agents.isPending || squads.isPending)) ||
    (issueCategory &&
      !!query.trim() &&
      (search.query !== query || search.pending));
  const failed =
    members.isError ||
    (category !== "member" &&
      category !== "issue" &&
      (agents.isError || squads.isError)) ||
    (issueCategory && query.trim() && search.query === query && search.failed);
  const control =
    "rounded px-2 py-1 text-caption hover:bg-accent aria-pressed:bg-accent aria-pressed:font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";
  return (
    <div
      data-mention-picker="categorized"
      className="flex max-h-[min(420px,var(--suggestion-available-height,420px))] w-96 max-w-[calc(100vw-16px)] flex-col overflow-hidden rounded-lg border bg-popover shadow-xl"
      onMouseDown={(event) => event.preventDefault()}
      onKeyDown={(event) => {
        if (composing(event.nativeEvent)) {
          event.preventDefault();
          event.stopPropagation();
          return;
        }
        if (event.key === "Escape") {
          event.preventDefault();
          event.stopPropagation();
          exitSuggestion(editor.view, pluginKey);
          focusEditor();
        }
        if (event.key === "Enter" || event.key === " ") event.stopPropagation();
      }}
    >
      <div className="shrink-0 border-b p-2">
        <div
          ref={toolbar}
          role="toolbar"
          aria-label={t(($) => $.mention.categories)}
          className="flex flex-wrap gap-1"
        >
          {categories.map((value, index) => (
            <button
              key={value}
              type="button"
              className={control}
              aria-pressed={category === value}
              tabIndex={category === value ? 0 : -1}
              onKeyDown={(event) => {
                if (composing(event.nativeEvent)) return;
                if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
                  event.preventDefault();
                  const next =
                    (index + (event.key === "ArrowRight" ? 1 : 4)) % 5;
                  const buttons =
                    toolbar.current?.querySelectorAll<HTMLButtonElement>(
                      "button",
                    );
                  buttons?.item(next).focus();
                }
              }}
              onClick={() => {
                setCategory(value);
                focusEditor();
              }}
            >
              {labels[value]}
            </button>
          ))}
        </div>
        {category === "agent" && (
          <div>
            {[false, true].map((value) => (
              <button
                key={String(value)}
                type="button"
                className={control}
                aria-pressed={leaders === value}
                onClick={() => {
                  setLeaders(value);
                  focusEditor();
                }}
              >
                {value
                  ? t(($) => $.mention.leaders)
                  : t(($) => $.mention.all_agents)}
              </button>
            ))}
          </div>
        )}
        <p className="mt-1 text-micro text-muted-foreground">
          {t(($) => $.mention.discovery_hint, {
            shortcut: formatShortcut(
              createShortcutChord("Down", { alt: true }),
            ),
          })}
        </p>
      </div>
      <div
        id={listId}
        role="listbox"
        aria-label={t(($) => $.mention.categories)}
        className="min-h-0 overflow-y-auto overscroll-contain"
      >
        {groups.map((group, index) => (
          <div key={index}>
            {group.items.length > 0 && group.label && (
              <div className="px-3 py-2 text-micro text-muted-foreground">
                {group.label}
              </div>
            )}
            {group.items.map((item) => {
              const actor = metadata.get(keyOf(item));
              const disabled = item.disabledReason
                ? blockedReasonLabel(item.disabledReason, issuesT)
                : "";
              return (
                <button
                  id={`${listId}-${keyOf(item)}`}
                  role="option"
                  aria-selected={active === item}
                  data-mention-key={keyOf(item)}
                  key={keyOf(item)}
                  type="button"
                  tabIndex={-1}
                  aria-disabled={!!disabled}
                  aria-label={[
                    item.type === "all"
                      ? t(($) => $.mention.all_members)
                      : item.label,
                    actor?.description,
                    actor?.leadsSquads.join(", "),
                    disabled,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                  ref={(element) => {
                    if (element) rows.current.set(keyOf(item), element);
                    else rows.current.delete(keyOf(item));
                  }}
                  onMouseMove={() => {
                    if (!disabled) setSelected(keyOf(item));
                  }}
                  onClick={() => accept(item)}
                  className={`flex w-full items-start gap-2 px-3 py-2 text-left text-caption ${active === item ? "bg-accent" : "hover:bg-accent/50"} ${disabled ? "opacity-50" : ""}`}
                >
                  {item.type !== "issue" && item.type !== "project" && (
                    <ActorAvatar
                      profileLink={false}
                      actorType={item.type === "all" ? "member" : item.type}
                      actorId={item.id}
                      size="sm"
                    />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium">
                      {item.type === "all"
                        ? t(($) => $.mention.all_members)
                        : item.label}
                    </span>
                    <span className="block truncate text-muted-foreground">
                      {actor
                        ? actor.preview || t(($) => $.mention.no_description)
                        : item.description}
                    </span>
                    {!!actor?.leadsSquads.length && (
                      <span className="block truncate text-muted-foreground">
                        {t(($) => $.mention.leads, {
                          names: actor.leadsSquads.join(", "),
                        })}
                      </span>
                    )}
                    {disabled && (
                      <span className="block text-muted-foreground">
                        {disabled}
                      </span>
                    )}
                  </span>
                  <span className="shrink-0 text-micro text-muted-foreground">
                    {actor?.leadsSquads.length
                      ? t(($) => $.mention.leaders)
                      : item.type === "all"
                        ? ""
                        : labels[item.type as Category]}
                  </span>
                </button>
              );
            })}
          </div>
        ))}
        {loading && (
          <p role="status" className="p-3 text-caption text-muted-foreground">
            {t(($) => $.mention.searching)}
          </p>
        )}
        {!loading && !failed && !visible.length && (
          <p role="status" className="p-3 text-caption text-muted-foreground">
            {t(($) => $.mention.no_results)}
          </p>
        )}
      </div>
      <div className="shrink-0 border-t p-2">
        {failed && (
          <button
            type="button"
            className={control}
            onClick={() => {
              focusEditor();
              void members.refetch();
              void agents.refetch();
              void squads.refetch();
              if (search.failed) setRetry((value) => value + 1);
            }}
          >
            {t(($) => $.mention.retry_directory)}
          </button>
        )}
        {localDirectory.length > limit && (
          <button
            type="button"
            className={control}
            onClick={() => {
              focusEditor();
              setPage({ query, category, leaders, limit: limit + 50 });
            }}
          >
            {t(($) => $.mention.show_more, {
              count: localDirectory.length - limit,
            })}
          </button>
        )}
        <button type="button" className={control} onClick={focusEditor}>
          {t(($) => $.mention.return_typing)}
        </button>
      </div>
    </div>
  );
});
