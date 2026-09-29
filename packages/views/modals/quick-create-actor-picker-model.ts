import type { QuickCreateActorRef } from "@multica/core/issues/stores/quick-create-store";
import { matchesPinyin } from "../editor/extensions/pinyin-match";
import { descriptionPreview } from "../issues/components/description-preview";

export type ActorTypeFilter = "all" | QuickCreateActorRef["type"];
export type ActorPickerView = "home" | "all" | "favorites" | "project";
export type ActorCatalogEntry = QuickCreateActorRef & {
  name: string;
  description: string;
  preview: string;
  leadsSquads: string[];
};
type SavedActor = { id: string; name: unknown; description?: unknown };

export const ACTOR_PAGE_SIZE = 50;
export const actorKey = (actor: QuickCreateActorRef) => `${actor.type}:${actor.id}`;

/** Inputs already obey the parent's permission/runtime eligibility rules. */
export function buildActorCatalog(agents: readonly SavedActor[], squads: readonly (SavedActor & { leader_id?: string })[]): ActorCatalogEntry[] {
  const leaders = new Map<string, string[]>();
  for (const squad of squads) {
    if (squad.leader_id && typeof squad.name === "string") {
      leaders.set(squad.leader_id, [...(leaders.get(squad.leader_id) ?? []), squad.name]);
    }
  }
  const project = (items: readonly SavedActor[], type: QuickCreateActorRef["type"]) => items.map((item) => {
    const description = typeof item.description === "string" ? item.description : "";
    return { type, id: item.id, name: typeof item.name === "string" ? item.name : "", description, preview: descriptionPreview(description), leadsSquads: type === "agent" ? leaders.get(item.id) ?? [] : [] };
  });
  return [...project(agents, "agent"), ...project(squads, "squad")];
}

export function actorShortcuts(catalog: readonly ActorCatalogEntry[], favorites: readonly QuickCreateActorRef[], recent: readonly QuickCreateActorRef[], type: ActorTypeFilter = "all") {
  const available = new Map(catalog.map((actor) => [actorKey(actor), actor]));
  const seen = new Set<string>();
  const resolve = (refs: readonly QuickCreateActorRef[]) => refs.flatMap((ref) => {
    const key = actorKey(ref);
    const actor = available.get(key);
    if (!actor || seen.has(key)) return [];
    seen.add(key);
    return [actor];
  });
  const matchesType = (actor: ActorCatalogEntry) => type === "all" || actor.type === type;
  return { favorites: resolve(favorites).filter(matchesType), recent: resolve(recent).filter(matchesType).slice(0, 5) };
}

/** Search the full saved description, never the truncated display preview. */
export function searchActors(catalog: readonly ActorCatalogEntry[], query: string, type: ActorTypeFilter, locale: string): ActorCatalogEntry[] {
  const q = query.trim().toLowerCase();
  const collator = new Intl.Collator(locale);
  const rank = (actor: ActorCatalogEntry) => {
    const name = actor.name.toLowerCase();
    if (!q || name === q) return 0;
    if (name.includes(q)) return 1;
    if (matchesPinyin(actor.name, q)) return 2;
    return actor.description.toLowerCase().includes(q) ? 3 : -1;
  };
  return catalog.filter((actor) => type === "all" || actor.type === type)
    .map((actor) => ({ actor, rank: rank(actor) }))
    .filter((item) => item.rank >= 0)
    .sort((a, b) => a.rank - b.rank || collator.compare(a.actor.name, b.actor.name)
      || a.actor.type.localeCompare(b.actor.type) || a.actor.id.localeCompare(b.actor.id))
    .map(({ actor }) => actor);
}

export function actorPage(items: readonly ActorCatalogEntry[], limit: number) {
  return { items: items.slice(0, limit), remaining: Math.max(0, items.length - limit) };
}


export function projectActorSections(catalog: readonly ActorCatalogEntry[], project: readonly QuickCreateActorRef[], favorites: readonly QuickCreateActorRef[], recent: readonly QuickCreateActorRef[], type: ActorTypeFilter = "all") {
  const projectKeys = new Set(project.map(actorKey));
  const projectItems = actorShortcuts(catalog, project, [], type).favorites;
  const rest = actorShortcuts(catalog.filter((actor) => !projectKeys.has(actorKey(actor))), favorites, recent, type);
  const projects = projectItems.slice(0, 3);
  const pinned = rest.favorites.slice(0, 3);
  return { projects, favorites: pinned, recent: rest.recent.slice(0, Math.min(5, 8 - projects.length - pinned.length)) };
}
