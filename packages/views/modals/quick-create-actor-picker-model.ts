import type { QuickCreateActorRef } from "@multica/core/issues/stores/quick-create-store";
import { actorKey, matchesActorFilter, type ActorCatalogEntry, type ActorTypeFilter } from "../common/actor-discovery";

export type ActorPickerView = "home" | "all" | "favorites" | "project";

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
  const matchesType = (actor: ActorCatalogEntry) => matchesActorFilter(actor, type);
  return { favorites: resolve(favorites).filter(matchesType), recent: resolve(recent).filter(matchesType).slice(0, 5) };
}

export function projectActorSections(catalog: readonly ActorCatalogEntry[], project: readonly QuickCreateActorRef[], favorites: readonly QuickCreateActorRef[], recent: readonly QuickCreateActorRef[], type: ActorTypeFilter = "all") {
  const projectKeys = new Set(project.map(actorKey));
  const projectItems = actorShortcuts(catalog, project, [], type).favorites;
  const rest = actorShortcuts(catalog.filter((actor) => !projectKeys.has(actorKey(actor))), favorites, recent, type);
  const projects = projectItems.slice(0, 3);
  const pinned = rest.favorites.slice(0, 3);
  return { projects, favorites: pinned, recent: rest.recent.slice(0, Math.min(5, 8 - projects.length - pinned.length)) };
}
