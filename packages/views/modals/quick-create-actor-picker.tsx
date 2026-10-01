"use client";

import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, Lock, Pin, PinOff, UserMinus } from "lucide-react";
import type { Agent, MemberWithUser, Squad } from "@multica/core/types";
import type { QuickCreateActorRef } from "@multica/core/issues/stores/quick-create-store";
import { ActorAvatar } from "../common/actor-avatar";
import { useLocale, useT } from "../i18n";
import { PickerItem, PropertyPicker } from "../issues/components/pickers/property-picker";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import {
  ACTOR_PAGE_SIZE, actorKey, actorPage, actorShortcuts, projectActorSections, buildActorCatalog, searchActors, matchesActorFilter,
  type ActorCatalogEntry, type ActorPickerView, type ActorTypeFilter,
} from "./quick-create-actor-picker-model";

type QueryState = { pending: boolean; error: boolean; hasData: boolean; onRetry: () => void };
type AgentSummary = Pick<Agent, "id" | "name" | "description" | "system_key">;
type SquadSummary = Pick<Squad, "id" | "name" | "description"> & Partial<Pick<Squad, "leader_id">>;
export type QuickCreateActorPickerProps = {
  actor: QuickCreateActorRef | null;
  visibleAgents: AgentSummary[];
  visibleSquads: SquadSummary[];
  selectedAgent?: AgentSummary;
  selectedSquad?: SquadSummary;
  favoriteActors: QuickCreateActorRef[];
  recentActors: QuickCreateActorRef[];
  preferencesReady: boolean;
  onToggleFavorite: (actor: QuickCreateActorRef) => void;
  onPick: (actor: QuickCreateActorRef) => void;
  defaultActor?: QuickCreateActorRef | null;
  onSetDefault?: (actor: QuickCreateActorRef | null) => void;
  projectActors?: QuickCreateActorRef[];
  projectState?: QueryState;
  agentState?: QueryState;
  squadState?: QueryState;
  trigger?: React.ReactNode;
  triggerRender?: React.ReactElement;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  side?: "top" | "bottom";
  align?: "start" | "center" | "end";
  /** Chat selects individual agents; squads only describe their leadership. */
  chat?: {
    hint: string;
    disabledReasons: ReadonlyMap<string, string>;
    canPin: boolean;
    favoritesPending: boolean;
  };
  /** Direct assignment shares discovery, but owns member/empty values and permissions. */
  assignment?: {
    members: (Pick<MemberWithUser, "user_id" | "name"> & Partial<Pick<MemberWithUser, "username" | "email">>)[];
    selectedMemberId: string | null;
    onPickMember: (id: string) => void;
    onClear: () => void;
    disabledReasons: ReadonlyMap<string, string>;
    privateAgentIds: ReadonlySet<string>;
    memberState?: QueryState;
  };
};

const EMPTY_REFS: QuickCreateActorRef[] = [];
const READY: QueryState = { pending: false, error: false, hasData: true, onRetry: () => {} };
const ACTION_CLASS = "rounded-md px-2 py-1.5 text-caption hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50";

export function QuickCreateActorPicker({
  actor, visibleAgents, visibleSquads, selectedAgent, selectedSquad,
  favoriteActors, recentActors, preferencesReady, onToggleFavorite, onPick,
  agentState = READY, squadState = READY, defaultActor = null, onSetDefault, projectActors = EMPTY_REFS, projectState,
  trigger, triggerRender, open: controlledOpen, onOpenChange, assignment, chat, side = "bottom", align = "start",
}: QuickCreateActorPickerProps) {
  const { t } = useT("modals");
  const { t: tIssues } = useT("issues");
  const locale = useLocale();
  const [internalOpen, setOpen] = useState(false);
  const open = controlledOpen ?? internalOpen;
  const [query, setQuery] = useState("");
  const [view, setView] = useState<ActorPickerView>("home");
  const [typeFilter, setTypeFilter] = useState<ActorTypeFilter | "member">("all");
  const [limit, setLimit] = useState(ACTOR_PAGE_SIZE);
  const searchRef = useRef<HTMLInputElement>(null);
  const rows = useRef(new Map<string, HTMLDivElement>());
  const pendingFocus = useRef<{ key: string; index: number } | null>(null);
  const actorFilter = typeFilter === "member" ? "all" : typeFilter;

  const chatMode = !!chat;
  const catalog = useMemo(() => buildActorCatalog(
    agentState.hasData ? visibleAgents : [],
    agentState.hasData && squadState.hasData ? visibleSquads : [],
  ).filter((item) => !chatMode || item.type === "agent"), [visibleAgents, visibleSquads, agentState.hasData, squadState.hasData, chatMode]);
  const shortcuts = useMemo(() => actorShortcuts(catalog,
    preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : []),
  [catalog, favoriteActors, recentActors, preferencesReady]);
  const filteredShortcuts = useMemo(() => actorShortcuts(catalog,
    preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : [], actorFilter),
  [catalog, favoriteActors, recentActors, preferencesReady, actorFilter]);
  const related = projectState?.hasData === false ? EMPTY_REFS : projectActors;
  const projectItems = useMemo(() => actorShortcuts(catalog, related, [], actorFilter).favorites, [catalog, related, actorFilter]);
  const home = useMemo(() => projectActorSections(catalog, related, preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : [], actorFilter), [catalog, related, favoriteActors, recentActors, preferencesReady, actorFilter]);
  const defaultItem = defaultActor && catalog.find((item) => actorKey(item) === actorKey(defaultActor));
  const isCurrentDefault = !!defaultActor && !!actor && actorKey(defaultActor) === actorKey(actor);
  const defaultKnown = agentState.hasData && (defaultActor?.type !== "squad" || squadState.hasData);
  const favoriteKeys = new Set(shortcuts.favorites.map(actorKey));
  const hasShortcuts = shortcuts.favorites.length + shortcuts.recent.length + related.length > 0;
  const browsingView = typeFilter === "member" || (view === "home" && !hasShortcuts) ? "all" : view;
  const searching = query.trim().length > 0;
  const effectiveView = searching ? "search" : browsingView;
  const results = useMemo(() => searchActors(catalog, query, actorFilter, locale), [catalog, query, actorFilter, locale]);
  const showMembers = !!assignment && (typeFilter === "all" || typeFilter === "member")
    && effectiveView !== "favorites" && effectiveView !== "project";
  const memberQuery = query.trim().toLowerCase();
  const matchingMembers = showMembers ? assignment.members.filter((member) =>
    member.name.toLowerCase().includes(memberQuery) || (member.username || member.email || "").toLowerCase().includes(memberQuery) || matchesPinyin(member.name, memberQuery)) : [];
  const memberItems = matchingMembers.slice(0, limit);
  const allCount = (typeFilter === "member" ? 0 : catalog.filter((item) => matchesActorFilter(item, actorFilter)).length)
    + (assignment && (typeFilter === "all" || typeFilter === "member") ? assignment.members.length : 0);
  const homeFavorites = home.favorites;
  const candidates = typeFilter === "member" ? [] : effectiveView === "home" ? [...home.projects, ...homeFavorites, ...home.recent]
    : effectiveView === "favorites" ? filteredShortcuts.favorites : effectiveView === "project" ? projectItems : results;
  const page = actorPage(candidates, effectiveView === "home" ? 8 : limit);
  const remaining = page.remaining + matchingMembers.length - memberItems.length;
  // The full result order changes on filtering/reordering, but not on a page
  // append. PropertyPicker gives a simultaneous typed query priority.
  const navigationResetKey = JSON.stringify([effectiveView, typeFilter, matchingMembers.map((member) => member.user_id), candidates.map(actorKey)]);

  useLayoutEffect(() => {
    const target = pendingFocus.current;
    if (!target) return;
    pendingFocus.current = null;
    const survivingPin = rows.current.get(target.key)?.querySelector<HTMLButtonElement>("button[data-actor-pin]");
    if (survivingPin && !survivingPin.disabled) {
      survivingPin.focus();
      return;
    }
    const fallback = page.items[Math.min(target.index, page.items.length - 1)];
    const primary = fallback && rows.current.get(actorKey(fallback))?.querySelector<HTMLButtonElement>("button[data-picker-item]");
    (primary && !primary.disabled ? primary : searchRef.current)?.focus();
  });

  const changeView = (next: ActorPickerView) => {
    // View navigation can remove its own button. Keep keyboard focus inside
    // the popup so Escape dismisses this layer, not the containing dialog.
    searchRef.current?.focus();
    setView(next);
    setLimit(ACTOR_PAGE_SIZE);
  };
  const changeOpen = (next: boolean) => {
    setOpen(next);
    onOpenChange?.(next);
    if (!next) {
      setQuery(""); setView("home"); setTypeFilter("all"); setLimit(ACTOR_PAGE_SIZE);
    }
  };
  const selected = actor ? catalog.find((item) => actorKey(item) === actorKey(actor)) : undefined;
  const display = selected ?? (actor?.type === "squad" ? selectedSquad : selectedAgent);
  const agentOnly = typeFilter === "agent" || typeFilter === "mika";
  const memberState = assignment?.memberState ?? READY;
  const relevantStates = typeFilter === "member" ? [memberState]
    : agentOnly ? [agentState] : [agentState, squadState, ...(showMembers ? [memberState] : [])];
  const pending = relevantStates.some((state) => state.pending && !state.hasData);
  const complete = relevantStates.every((state) => state.hasData);

  const renderRow = (item: ActorCatalogEntry) => {
    const key = actorKey(item);
    const pinned = favoriteKeys.has(key);
    const isSelected = actor !== null && actorKey(actor) === key;
    const ref = { type: item.type, id: item.id };
    const disabledReason = (chat ?? assignment)?.disabledReasons.get(key);
    return <div key={key} data-actor-key={key} ref={(node) => {
      if (node) rows.current.set(key, node); else rows.current.delete(key);
    }} className="flex min-w-0 items-center gap-0.5 [&>button:first-child]:min-w-0">
      <PickerItem selected={isSelected} disabled={!!disabledReason}
        onClick={() => { if (!disabledReason) { onPick(ref); changeOpen(false); } }}
        tooltip={disabledReason ?? (item.description ? (
          <span className="block max-h-[min(24rem,50vh)] min-w-0 overflow-y-auto whitespace-pre-wrap wrap-anywhere">
            {item.description}
          </span>
        ) : undefined)}>
        <ActorAvatar actorType={item.type} actorId={item.id} size="sm" showStatusDot={(!!assignment || chatMode) && item.type === "agent"} />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex min-w-0 items-baseline gap-2">
            <span className={`truncate ${isSelected ? "font-semibold text-foreground" : ""}`}>{item.name}</span>{" "}
            <span className="shrink-0 text-caption text-muted-foreground">{item.isMika ? t(($) => $.create_issue.actor_picker.mika_badge) : item.leadsSquads.length > 0 ? t(($) => $.create_issue.actor_picker.coordinator_badge) : item.type === "agent" ? t(($) => $.create_issue.actor_picker.agent) : t(($) => $.create_issue.actor_picker.squad)}</span>
            {defaultActor && actorKey(defaultActor) === key && <span className="shrink-0 text-caption text-muted-foreground">{t(($) => $.create_issue.actor_picker.default_badge)}</span>}
          </span>{" "}
          <span className="truncate text-caption text-muted-foreground">{item.preview || t(($) => $.create_issue.actor_picker.no_description)}</span>{" "}
          {item.leadsSquads.length > 0 && <span className="truncate text-caption text-muted-foreground" title={item.leadsSquads.join(", ")}>
            {t(($) => $.create_issue.actor_picker.leads_squads, { names: item.leadsSquads.join(", ") })}
          </span>}
          {isSelected && <span className="sr-only">{t(($) => $.create_issue.actor_picker.selected)}</span>}
          {chatMode && disabledReason && <span className="text-caption text-muted-foreground">{disabledReason}</span>}
        </span>
        {item.type === "agent" && assignment?.privateAgentIds.has(item.id) && <Lock aria-hidden className="size-3 shrink-0 text-muted-foreground" />}
      </PickerItem>
      <button type="button" data-actor-pin disabled={!preferencesReady || chat?.favoritesPending || (!pinned && (!!disabledReason || chat?.canPin === false))} aria-pressed={pinned}
        aria-label={pinned ? t(($) => $.create_issue.actor_picker.unpin, { name: item.name }) : t(($) => $.create_issue.actor_picker.pin, { name: item.name })}
        className={`${ACTION_CLASS} shrink-0 ${pinned ? "text-foreground" : "text-muted-foreground"}`}
        onClick={() => {
          pendingFocus.current = { key, index: page.items.findIndex((candidate) => actorKey(candidate) === key) };
          onToggleFavorite(ref);
        }}>
        {pinned ? <PinOff aria-hidden className="size-4" /> : <Pin aria-hidden className="size-4" />}
      </button>
    </div>;
  };

  const retryNotice = (state: QueryState, type: "agent" | "squad" | "member") => state.error && <div role="alert" className="px-2 py-2 text-caption text-muted-foreground">
    <span>{type === "member"
      ? state.hasData ? t(($) => $.create_issue.actor_picker.members_refresh_failed) : t(($) => $.create_issue.actor_picker.members_failed)
      : type === "agent"
      ? state.hasData ? t(($) => $.create_issue.actor_picker.agents_refresh_failed) : t(($) => $.create_issue.actor_picker.agents_failed)
      : state.hasData ? t(($) => $.create_issue.actor_picker.squads_refresh_failed) : t(($) => $.create_issue.actor_picker.squads_failed)}</span>
    <button type="button" className={`${ACTION_CLASS} ml-1 text-foreground`} onClick={() => {
      searchRef.current?.focus();
      state.onRetry();
    }}>
      {type === "member" ? t(($) => $.create_issue.actor_picker.retry_members) : type === "agent" ? t(($) => $.create_issue.actor_picker.retry_agents) : t(($) => $.create_issue.actor_picker.retry_squads)}
    </button>
  </div>;

  return <PropertyPicker open={open} onOpenChange={changeOpen} width="w-96 max-w-[calc(100vw-24px)] max-h-(--available-height)" align={align} side={side} searchable
    searchInputRef={searchRef} searchPlaceholder={t(($) => $.create_issue.actor_picker.search_placeholder)}
    onSearchChange={(value) => { setQuery(value); setLimit(ACTOR_PAGE_SIZE); }} navigationResetKey={navigationResetKey}
    triggerRender={triggerRender}
    trigger={trigger ?? <span title={t(($) => $.create_issue.actor_picker.creator_hint)} className="flex min-w-0 items-center gap-2 text-caption text-muted-foreground hover:text-foreground transition-colors">
      <span className="shrink-0">{t(($) => $.create_issue.agent.created_by)}</span>{" "}
      {actor && display ? <span className="flex min-w-0 items-center gap-1.5 text-foreground">
        <ActorAvatar actorType={actor.type} actorId={actor.id} size="sm" /><span className="truncate">{display.name}</span>
      </span> : <span>{t(($) => $.create_issue.agent.pick_an_agent)}</span>}
    </span>}
    header={<div>
      <p className="px-4 pt-2 text-caption text-muted-foreground">{chat ? chat.hint : typeFilter === "coordination" || selected?.leadsSquads.length
        ? t(($) => $.create_issue.actor_picker.coordinator_hint)
        : assignment ? t(($) => $.create_issue.actor_picker.assignee_hint) : t(($) => $.create_issue.actor_picker.creator_hint)}</p>
      <div className="flex flex-wrap items-center gap-1 px-2 py-1.5">
      {(["all", "mika", "agent", "squad", "coordination"] as const).filter((type) => !chatMode || type !== "squad").map((type) => <button key={type} type="button" aria-pressed={typeFilter === type}
        className={`${ACTION_CLASS} ${typeFilter === type ? "bg-accent font-semibold text-foreground" : "text-muted-foreground"}`}
        onClick={() => {
          setTypeFilter(type);
          setLimit(ACTOR_PAGE_SIZE);
          if (type === "mika" || type === "coordination" || typeFilter === "mika" || typeFilter === "coordination") changeView("all");
        }}>
        {type === "mika" ? t(($) => $.create_issue.actor_picker.mika) : type === "coordination" ? t(($) => $.create_issue.actor_picker.coordination) : type === "all" ? t(($) => $.create_issue.actor_picker.all) : type === "agent" ? t(($) => $.create_issue.actor_picker.agents) : t(($) => $.create_issue.actor_picker.squads)}
      </button>)}
      {assignment && <button type="button" aria-pressed={typeFilter === "member"}
        className={`${ACTION_CLASS} ${typeFilter === "member" ? "bg-accent font-semibold text-foreground" : "text-muted-foreground"}`}
        onClick={() => { setTypeFilter("member"); changeView("all"); }}>
        {tIssues(($) => $.pickers.assignee.members_group)}
      </button>}
    </div></div>}
    footer={<div className="flex flex-wrap items-center justify-between gap-1">
      {view !== "home" && (hasShortcuts || view === "favorites" || view === "project") && <button type="button" className={`${ACTION_CLASS} inline-flex items-center gap-1`} onClick={() => {
        if (typeFilter === "member") setTypeFilter("all");
        changeView("home");
      }}>
        <ArrowLeft aria-hidden className="size-3.5" />{t(($) => $.create_issue.actor_picker.back)}
      </button>}
      {/* Disabling this focused control after navigation blurs it to the body;
          Escape would then reach the containing dialog as well as the picker. */}
      <button type="button" className={`${ACTION_CLASS} ml-auto`} onClick={() => changeView("all")}>
        {t(($) => $.create_issue.actor_picker.browse_all, { count: allCount })}
      </button>
      {onSetDefault && <div className="flex w-full flex-wrap items-center gap-1 border-t pt-1">
        {defaultActor && defaultKnown && <span className="w-full truncate px-2 text-caption text-muted-foreground">{defaultItem
          ? t(($) => $.create_issue.actor_picker.default_label, { name: defaultItem.name })
          : t(($) => $.create_issue.actor_picker.default_unavailable)}</span>}
        <button type="button" className={ACTION_CLASS} disabled={!preferencesReady || (!isCurrentDefault && !selected)}
          onClick={() => { searchRef.current?.focus(); onSetDefault(isCurrentDefault ? null : actor); }}>
          {isCurrentDefault ? t(($) => $.create_issue.actor_picker.clear_default) : t(($) => $.create_issue.actor_picker.set_default)}
        </button>
        {defaultActor && !isCurrentDefault && <button type="button" className={ACTION_CLASS} disabled={!preferencesReady}
          onClick={() => { searchRef.current?.focus(); onSetDefault(null); }}>{t(($) => $.create_issue.actor_picker.clear_default)}</button>}
      </div>}
    </div>}>
    {assignment && <PickerItem emptyValue selected={!actor && !assignment.selectedMemberId}
      onClick={() => { assignment.onClear(); changeOpen(false); }}>
      <UserMinus aria-hidden className="size-3.5 text-muted-foreground" />
      <span className="text-muted-foreground">{tIssues(($) => $.pickers.assignee.trigger_unassigned)}</span>
    </PickerItem>}
    {pending && <div role="status" className="px-2 py-3 text-caption text-muted-foreground">{assignment ? t(($) => $.create_issue.actor_picker.assignees_loading) : t(($) => $.create_issue.actor_picker.loading)}</div>}
    {typeFilter !== "member" && retryNotice(agentState, "agent")}
    {typeFilter !== "member" && !agentOnly && retryNotice(squadState, "squad")}
    {showMembers && retryNotice(memberState, "member")}
    {assignment && memberItems.length > 0 && <section>
      <h3 className="px-2 pt-2 pb-1 text-caption font-medium text-muted-foreground">{tIssues(($) => $.pickers.assignee.members_group)}</h3>
      {memberItems.map((member) => <PickerItem key={member.user_id} selected={assignment.selectedMemberId === member.user_id}
        onClick={() => { assignment.onPickMember(member.user_id); changeOpen(false); }}>
        <ActorAvatar actorType="member" actorId={member.user_id} size="sm" />
        <span className="min-w-0 flex-1"><span className={`truncate ${assignment.selectedMemberId === member.user_id ? "font-semibold text-foreground" : ""}`}>{member.name}</span><span className="block truncate text-caption text-muted-foreground">{member.username || member.email}</span></span>
      </PickerItem>)}
    </section>}
    {projectState?.pending && !projectState.hasData && <p role="status" className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.create_issue.actor_picker.project_loading)}</p>}
    {projectState?.error && <div role="alert" className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.create_issue.actor_picker.project_failed)}
      <button type="button" className={ACTION_CLASS} onClick={() => { searchRef.current?.focus(); projectState.onRetry(); }}>{t(($) => $.create_issue.actor_picker.retry_project)}</button></div>}
    {effectiveView === "home" ? <>
      {home.projects.length > 0 && <section>
        <div className="flex items-center justify-between gap-1 px-2 pt-2 pb-1"><h3 className="text-caption font-medium text-muted-foreground">{t(($) => $.create_issue.actor_picker.project_group)}</h3>
          <button type="button" className={ACTION_CLASS} onClick={() => changeView("project")}>{t(($) => $.create_issue.actor_picker.all_project, {count:projectItems.length})}</button></div>
        {home.projects.map(renderRow)}
      </section>}
      {filteredShortcuts.favorites.length > 0 && <section>
        <div className="flex items-center justify-between gap-1 px-2 pt-2 pb-1">
          <h3 className="text-caption font-medium text-muted-foreground">{t(($) => $.create_issue.actor_picker.favorites)}</h3>
          <button type="button" className={ACTION_CLASS} onClick={() => changeView("favorites")}>{t(($) => $.create_issue.actor_picker.all_favorites, { count: filteredShortcuts.favorites.length })}</button>
        </div>
        {homeFavorites.map(renderRow)}
      </section>}
      {home.recent.length > 0 && <section>
        <h3 className="px-2 pt-3 pb-1 text-caption font-medium text-muted-foreground">{t(($) => $.create_issue.actor_picker.recent)}</h3>
        {home.recent.map(renderRow)}
      </section>}
    </> : <>
      {effectiveView === "favorites" && <h3 className="px-2 pt-2 pb-1 text-caption font-medium text-muted-foreground">{t(($) => $.create_issue.actor_picker.favorites)}</h3>}
      {effectiveView === "project" && <h3 className="px-2 pt-2 pb-1 text-caption font-medium text-muted-foreground">{t(($) => $.create_issue.actor_picker.project_group)}</h3>}
      {effectiveView === "all" && typeFilter !== "mika" && typeFilter !== "coordination" && page.items.length > 0 && <h3 className="px-2 pt-2 pb-1 text-caption font-medium text-muted-foreground">
        {typeFilter === "squad" ? t(($) => $.create_issue.actor_picker.squads) : typeFilter === "agent" ? t(($) => $.create_issue.actor_picker.agents) : t(($) => $.create_issue.actor_picker.all)}
      </h3>}
      {page.items.map(renderRow)}
    </>}
    {complete && (!(effectiveView === "project" || effectiveView === "home") || projectState?.hasData !== false) && page.items.length === 0 && memberItems.length === 0 && <div className="px-2 py-3 text-caption text-muted-foreground">
      {effectiveView === "favorites" ? t(($) => $.create_issue.actor_picker.no_favorites)
        : effectiveView === "project" ? t(($) => $.create_issue.actor_picker.no_project_matches)
        : effectiveView === "home" ? t(($) => $.create_issue.actor_picker.no_shortcuts)
          : assignment || searching || typeFilter === "mika" || typeFilter === "coordination" ? t(($) => $.create_issue.actor_picker.no_matches) : t(($) => $.create_issue.agent.no_agents)}
    </div>}
    {remaining > 0 && <button type="button" className={`${ACTION_CLASS} mt-1 w-full`} onClick={() => {
      searchRef.current?.focus();
      setLimit((current) => current + ACTOR_PAGE_SIZE);
    }}>
      {t(($) => $.create_issue.actor_picker.show_more, { count: remaining })}
    </button>}
  </PropertyPicker>;
}
