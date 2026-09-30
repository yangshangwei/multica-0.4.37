"use client";

import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, Pin, PinOff } from "lucide-react";
import type { Agent, Squad } from "@multica/core/types";
import type { QuickCreateActorRef } from "@multica/core/issues/stores/quick-create-store";
import { ActorAvatar } from "../common/actor-avatar";
import { useLocale, useT } from "../i18n";
import { PickerItem, PropertyPicker } from "../issues/components/pickers/property-picker";
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
};

const EMPTY_REFS: QuickCreateActorRef[] = [];
const READY: QueryState = { pending: false, error: false, hasData: true, onRetry: () => {} };
const ACTION_CLASS = "rounded-md px-2 py-1.5 text-caption hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50";

export function QuickCreateActorPicker({
  actor, visibleAgents, visibleSquads, selectedAgent, selectedSquad,
  favoriteActors, recentActors, preferencesReady, onToggleFavorite, onPick,
  agentState = READY, squadState = READY, defaultActor = null, onSetDefault, projectActors = EMPTY_REFS, projectState,
}: QuickCreateActorPickerProps) {
  const { t } = useT("modals");
  const locale = useLocale();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [view, setView] = useState<ActorPickerView>("home");
  const [typeFilter, setTypeFilter] = useState<ActorTypeFilter>("all");
  const [limit, setLimit] = useState(ACTOR_PAGE_SIZE);
  const searchRef = useRef<HTMLInputElement>(null);
  const rows = useRef(new Map<string, HTMLDivElement>());
  const pendingFocus = useRef<{ key: string; index: number } | null>(null);

  const catalog = useMemo(() => buildActorCatalog(
    agentState.hasData ? visibleAgents : [],
    agentState.hasData && squadState.hasData ? visibleSquads : [],
  ), [visibleAgents, visibleSquads, agentState.hasData, squadState.hasData]);
  const shortcuts = useMemo(() => actorShortcuts(catalog,
    preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : []),
  [catalog, favoriteActors, recentActors, preferencesReady]);
  const filteredShortcuts = useMemo(() => actorShortcuts(catalog,
    preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : [], typeFilter),
  [catalog, favoriteActors, recentActors, preferencesReady, typeFilter]);
  const related = projectState?.hasData === false ? EMPTY_REFS : projectActors;
  const projectItems = useMemo(() => actorShortcuts(catalog, related, [], typeFilter).favorites, [catalog, related, typeFilter]);
  const home = useMemo(() => projectActorSections(catalog, related, preferencesReady ? favoriteActors : [], preferencesReady ? recentActors : [], typeFilter), [catalog, related, favoriteActors, recentActors, preferencesReady, typeFilter]);
  const defaultItem = defaultActor && catalog.find((item) => actorKey(item) === actorKey(defaultActor));
  const isCurrentDefault = !!defaultActor && !!actor && actorKey(defaultActor) === actorKey(actor);
  const defaultKnown = agentState.hasData && (defaultActor?.type !== "squad" || squadState.hasData);
  const favoriteKeys = new Set(shortcuts.favorites.map(actorKey));
  const hasShortcuts = shortcuts.favorites.length + shortcuts.recent.length + related.length > 0;
  const browsingView = view === "home" && !hasShortcuts ? "all" : view;
  const searching = query.trim().length > 0;
  const effectiveView = searching ? "search" : browsingView;
  const results = useMemo(() => searchActors(catalog, query, typeFilter, locale), [catalog, query, typeFilter, locale]);
  const allCount = catalog.filter((item) => matchesActorFilter(item, typeFilter)).length;
  const homeFavorites = home.favorites;
  const candidates = effectiveView === "home" ? [...home.projects, ...homeFavorites, ...home.recent]
    : effectiveView === "favorites" ? filteredShortcuts.favorites : effectiveView === "project" ? projectItems : results;
  const page = actorPage(candidates, effectiveView === "home" ? 8 : limit);
  // The full result order changes on filtering/reordering, but not on a page
  // append. PropertyPicker gives a simultaneous typed query priority.
  const navigationResetKey = JSON.stringify([effectiveView, typeFilter, candidates.map(actorKey)]);

  useLayoutEffect(() => {
    const target = pendingFocus.current;
    if (!target) return;
    pendingFocus.current = null;
    const survivingPin = rows.current.get(target.key)?.querySelector<HTMLButtonElement>("button[data-actor-pin]");
    if (survivingPin) {
      survivingPin.focus();
      return;
    }
    const fallback = page.items[Math.min(target.index, page.items.length - 1)];
    const primary = fallback && rows.current.get(actorKey(fallback))?.querySelector<HTMLButtonElement>("button[data-picker-item]");
    (primary || searchRef.current)?.focus();
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
    if (!next) {
      setQuery(""); setView("home"); setTypeFilter("all"); setLimit(ACTOR_PAGE_SIZE);
    }
  };
  const selected = actor ? catalog.find((item) => actorKey(item) === actorKey(actor)) : undefined;
  const display = selected ?? (actor?.type === "squad" ? selectedSquad : selectedAgent);
  const agentOnly = typeFilter === "agent" || typeFilter === "mika";
  const relevantStates = agentOnly ? [agentState] : [agentState, squadState];
  const pending = relevantStates.some((state) => state.pending && !state.hasData);
  const complete = relevantStates.every((state) => state.hasData);

  const renderRow = (item: ActorCatalogEntry) => {
    const key = actorKey(item);
    const pinned = favoriteKeys.has(key);
    const isSelected = actor !== null && actorKey(actor) === key;
    const ref = { type: item.type, id: item.id };
    return <div key={key} data-actor-key={key} ref={(node) => {
      if (node) rows.current.set(key, node); else rows.current.delete(key);
    }} className="flex min-w-0 items-center gap-0.5 [&>button:first-child]:min-w-0">
      <PickerItem selected={isSelected} onClick={() => { onPick(ref); changeOpen(false); }}>
        <ActorAvatar actorType={item.type} actorId={item.id} size="sm" />
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
        </span>
      </PickerItem>
      <button type="button" data-actor-pin disabled={!preferencesReady} aria-pressed={pinned}
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

  const retryNotice = (state: QueryState, type: "agent" | "squad") => state.error && <div role="alert" className="px-2 py-2 text-caption text-muted-foreground">
    <span>{type === "agent"
      ? state.hasData ? t(($) => $.create_issue.actor_picker.agents_refresh_failed) : t(($) => $.create_issue.actor_picker.agents_failed)
      : state.hasData ? t(($) => $.create_issue.actor_picker.squads_refresh_failed) : t(($) => $.create_issue.actor_picker.squads_failed)}</span>
    <button type="button" className={`${ACTION_CLASS} ml-1 text-foreground`} onClick={() => {
      searchRef.current?.focus();
      state.onRetry();
    }}>
      {type === "agent" ? t(($) => $.create_issue.actor_picker.retry_agents) : t(($) => $.create_issue.actor_picker.retry_squads)}
    </button>
  </div>;

  return <PropertyPicker open={open} onOpenChange={changeOpen} width="w-96 max-w-[calc(100vw-24px)] max-h-(--available-height)" align="start" searchable
    searchInputRef={searchRef} searchPlaceholder={t(($) => $.create_issue.actor_picker.search_placeholder)}
    onSearchChange={(value) => { setQuery(value); setLimit(ACTOR_PAGE_SIZE); }} navigationResetKey={navigationResetKey}
    trigger={<span title={t(($) => $.create_issue.actor_picker.creator_hint)} className="flex min-w-0 items-center gap-2 text-caption text-muted-foreground hover:text-foreground transition-colors">
      <span className="shrink-0">{t(($) => $.create_issue.agent.created_by)}</span>{" "}
      {actor && display ? <span className="flex min-w-0 items-center gap-1.5 text-foreground">
        <ActorAvatar actorType={actor.type} actorId={actor.id} size="sm" /><span className="truncate">{display.name}</span>
      </span> : <span>{t(($) => $.create_issue.agent.pick_an_agent)}</span>}
    </span>}
    header={<div>
      <p className="px-4 pt-2 text-caption text-muted-foreground">{typeFilter === "coordination" || selected?.leadsSquads.length
        ? t(($) => $.create_issue.actor_picker.coordinator_hint)
        : t(($) => $.create_issue.actor_picker.creator_hint)}</p>
      <div className="flex flex-wrap items-center gap-1 px-2 py-1.5">
      {(["all", "mika", "agent", "squad", "coordination"] as const).map((type) => <button key={type} type="button" aria-pressed={typeFilter === type}
        className={`${ACTION_CLASS} ${typeFilter === type ? "bg-accent font-semibold text-foreground" : "text-muted-foreground"}`}
        onClick={() => {
          setTypeFilter(type);
          setLimit(ACTOR_PAGE_SIZE);
          if (type === "mika" || type === "coordination" || typeFilter === "mika" || typeFilter === "coordination") changeView("all");
        }}>
        {type === "mika" ? t(($) => $.create_issue.actor_picker.mika) : type === "coordination" ? t(($) => $.create_issue.actor_picker.coordination) : type === "all" ? t(($) => $.create_issue.actor_picker.all) : type === "agent" ? t(($) => $.create_issue.actor_picker.agents) : t(($) => $.create_issue.actor_picker.squads)}
      </button>)}
    </div></div>}
    footer={<div className="flex flex-wrap items-center justify-between gap-1">
      {view !== "home" && (hasShortcuts || view === "favorites" || view === "project") && <button type="button" className={`${ACTION_CLASS} inline-flex items-center gap-1`} onClick={() => changeView("home")}>
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
    {pending && <div role="status" className="px-2 py-3 text-caption text-muted-foreground">{t(($) => $.create_issue.actor_picker.loading)}</div>}
    {retryNotice(agentState, "agent")}
    {!agentOnly && retryNotice(squadState, "squad")}
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
    {complete && (!(effectiveView === "project" || effectiveView === "home") || projectState?.hasData !== false) && page.items.length === 0 && <div className="px-2 py-3 text-caption text-muted-foreground">
      {effectiveView === "favorites" ? t(($) => $.create_issue.actor_picker.no_favorites)
        : effectiveView === "project" ? t(($) => $.create_issue.actor_picker.no_project_matches)
        : effectiveView === "home" ? t(($) => $.create_issue.actor_picker.no_shortcuts)
          : searching || typeFilter === "mika" || typeFilter === "coordination" ? t(($) => $.create_issue.actor_picker.no_matches) : t(($) => $.create_issue.agent.no_agents)}
    </div>}
    {page.remaining > 0 && <button type="button" className={`${ACTION_CLASS} mt-1 w-full`} onClick={() => {
      searchRef.current?.focus();
      setLimit((current) => current + ACTOR_PAGE_SIZE);
    }}>
      {t(($) => $.create_issue.actor_picker.show_more, { count: page.remaining })}
    </button>}
  </PropertyPicker>;
}
