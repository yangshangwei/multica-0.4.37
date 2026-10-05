/**
 * Mobile-owned WS cache patchers for the project domain. Pure functions over
 * `QueryClient` — no React, no WS plumbing. Hooks in `use-projects-realtime.ts`
 * and `use-project-realtime.ts` translate WS events into calls into this module.
 *
 * Why mobile-owned (and not importing from packages/core/projects):
 *   - Web doesn't have project ws-updaters yet — it invalidates via the
 *     query cache mutation surface. Mobile must patch (cellular-data rule
 *     in apps/mobile/CLAUDE.md realtime § "Patch over invalidate").
 *   - Even when web adds them, mobile keys come from its own
 *     `data/queries/projects.ts` factory; binding to a foreign factory
 *     would silently drift on key-shape changes.
 *
 * Cache shapes:
 *   - Project list    (projectKeys.list)         → `Project[]`
 *   - Project detail  (projectKeys.detail)       → `Project`
 *   - Resources       (projectKeys.resources)    → `ProjectResource[]`
 */
import type { QueryClient } from "@tanstack/react-query";
import type { Issue, Project } from "@multica/core/types";
import { projectKeys } from "@/data/queries/projects";
import { ProjectSchema } from "@multica/core/api/schemas";
import { isProjectAccessDenied } from "./project-access";

/** Mirrors core/triage isFormalAdmission without importing its hook barrel.
 * Only absent fields mean a pre-T1 server; unknown admission states fail closed. */
export function isFormalProjectIssue(issue: Pick<Issue, "project_id" | "admission_status">, projectId: string) {
  return issue.project_id === projectId && (issue.admission_status === undefined ||
    issue.admission_status === "not_required" || issue.admission_status === "accepted");
}

/** Missing additive fields mean an older producer, not a request to clear.
 * Explicit null still clears nullable fields. Never regress a known revision. */
export function mergeProjectSnapshot(current: Project | undefined, partial: Partial<Project> & { id: string }): Project | undefined {
  if (current?.revision !== undefined && (partial.revision === undefined || partial.revision < current.revision)) return current;
  const defined = Object.fromEntries(Object.entries(partial).filter(([, value]) => value !== undefined));
  const candidate = { ...current, ...defined };
  // A legacy aggregate cannot refresh the newer breakdown authoritatively.
  if (current && (partial.issue_count !== undefined && partial.issue_count !== current.issue_count ||
      partial.done_count !== undefined && partial.done_count !== current.done_count) &&
      partial.statistics_complete === undefined) candidate.statistics_complete = false;
  if (!ProjectSchema.safeParse(candidate).success) return current;
  return candidate as Project;
}

function canPatch(qc: QueryClient, wsId: string, partial: Partial<Project> & { id: string }) {
  if (isProjectAccessDenied(wsId)) return false;
  if (partial.workspace_id !== undefined && partial.workspace_id !== wsId) return false;
  if (Object.keys(partial).every((key) => key === "id" || key === "workspace_id")) {
    void qc.invalidateQueries({ queryKey: projectKeys.all(wsId) });
    return false;
  }
  return true;
}

export function patchProjectsList(
  qc: QueryClient,
  wsId: string,
  partial: Partial<Project> & { id: string },
) {
  if (!canPatch(qc, wsId, partial)) return;
  qc.setQueryData<Project[]>(projectKeys.list(wsId), (old) =>
    old
      ? old.map((p) => (p.id === partial.id ? mergeProjectSnapshot(p, partial) ?? p : p))
      : old,
  );
  if (partial.revision === undefined) void qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
}

/** Prepend if not present, replace in place if it is. List ordering is
 *  server-driven; on `project:created` the list will resync to the
 *  authoritative order via the next refetch / reconnect. */
export function upsertIntoProjectsList(
  qc: QueryClient,
  wsId: string,
  project: Project,
) {
  if (!canPatch(qc, wsId, project)) return;
  qc.setQueryData<Project[]>(projectKeys.list(wsId), (old) => {
    const valid = mergeProjectSnapshot(undefined, project);
    if (!valid) return old;
    if (!old) return [valid];
    const idx = old.findIndex((p) => p.id === project.id);
    if (idx === -1) return [valid, ...old];
    const copy = old.slice();
    copy[idx] = mergeProjectSnapshot(old[idx], project) ?? old[idx]!;
    return copy;
  });
}

export function removeFromProjectsList(
  qc: QueryClient,
  wsId: string,
  projectId: string,
) {
  qc.setQueryData<Project[]>(projectKeys.list(wsId), (old) =>
    old ? old.filter((p) => p.id !== projectId) : old,
  );
}

export function patchProjectDetail(
  qc: QueryClient,
  wsId: string,
  project: Partial<Project> & { id: string },
) {
  if (!canPatch(qc, wsId, project)) return;
  const key = projectKeys.detail(wsId, project.id);
  const current = qc.getQueryData<Project>(key);
  const merged = mergeProjectSnapshot(current, project);
  if (merged) qc.setQueryData<Project>(key, merged);
  if (!merged || current?.revision !== undefined && project.revision === undefined) {
    void qc.invalidateQueries({ queryKey: key });
  }
}

export function clearProjectDetail(
  qc: QueryClient,
  wsId: string,
  projectId: string,
) {
  qc.removeQueries({ queryKey: projectKeys.detail(wsId, projectId) });
  qc.removeQueries({ queryKey: projectKeys.resources(wsId, projectId) });
}
