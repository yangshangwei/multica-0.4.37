import { protectIterationRead } from "./access";
import {
  queryOptions,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { api, ApiError } from "../api";
import type { IterationWriteInput } from "../api/iteration-schemas";
export type {
  Iteration,
  IterationDraft,
  IterationPreview,
  IterationWriteInput,
  IterationCreateInput,
} from "../api/iteration-schemas";
export const iterationKeys = {
  all: (wsId: string) => ["iterations", wsId] as const,
};
export const iterationCapabilitiesOptions = (wsId: string) =>
  queryOptions({
    queryKey: [
      ...iterationKeys.all(wsId),
      "capabilities",
      api.getBaseUrl?.() ?? "",
    ],
    queryFn: ({ signal, client }) =>
      protectIterationRead(
        client,
        wsId,
        () => api.getIterationCapabilities(wsId, { signal }),
        true,
      ),
    retry: false,
  });
export const iterationSettingsOptions = (wsId: string) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "settings"],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, () =>
        api.getIterationSettings(wsId, { signal }),
      ),
    retry: false,
  });
export const iterationListOptions = (
  wsId: string,
  params: Record<string, string> = {},
) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "list", params],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, () =>
        api.listIterations(wsId, params, { signal }),
      ),
    retry: false,
  });
export const iterationDetailOptions = (wsId: string, id: string) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "detail", id],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, () =>
        api.getIteration(wsId, id, { signal }),
      ),
    retry: false,
  });
export const iterationIssuesOptions = (
  wsId: string,
  id: string,
  params: Record<string, string> = {},
) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "issues", id, params],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, () =>
        api.getIterationIssues(wsId, id, params, { signal }),
      ),
    retry: false,
  });
export function useIterationOperation(wsId: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: IterationWriteInput) =>
      api.applyIterationOperation(wsId, input),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: iterationKeys.all(wsId) });
      await client.invalidateQueries({ queryKey: ["issues", wsId] });
    },
  });
}

export const iterationGroupedIssuesOptions = (
  wsId: string,
  id: string,
  params: Record<string, string> = {},
) => queryOptions({
  queryKey: [...iterationKeys.all(wsId), "grouped-issues", id, params],
  queryFn: ({ signal, client }) => protectIterationRead(client, wsId, async () => {
    const items: Awaited<ReturnType<typeof api.getIterationIssues>>["items"] = [];
    const cursors = new Set<string>();
    const members = new Set<string>();
    let cursor: string | null = null;
    let revision: number | undefined;
    let total: number | undefined;
    let filterOptions: Awaited<ReturnType<typeof api.getIterationIssues>>["filter_options"];
    do {
      const page = await api.getIterationIssues(wsId, id, { ...params, limit: "100", ...(cursor ? { cursor } : {}) }, { signal });
      if (revision === undefined) filterOptions = page.filter_options;
      revision ??= page.scope_revision;
      total ??= page.total;
      if (page.scope_revision !== revision || page.total !== total) throw new ApiError("Iteration scope changed", 409, "Conflict", { code: "cursor_stale" });
      for (const item of page.items) {
        if (members.has(item.issue_id)) throw new Error("Repeated iteration member in grouped results");
        members.add(item.issue_id);
        items.push(item);
      }
      cursor = page.next_cursor;
      if (cursor && cursors.has(cursor)) throw new Error("Repeated iteration cursor");
      if (cursor) cursors.add(cursor);
    } while (cursor);
    if (items.length !== total) throw new Error("Incomplete iteration grouping results");
    return { workspace_id: wsId, iteration_id: id, scope_revision: revision, total, items, filter_options: filterOptions };
  }),
  retry: false,
});
export { prepareIterationDraft } from "./prepare";
export { useIterationCommand, definitelyRejected } from "./command";
export const iterationChoicesOptions = (wsId: string) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "choices"],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, async () => {
        const items = [];
        let cursor: string | null = null;
        do {
          const page = await api.listIterations(
            wsId,
            { limit: "100", ...(cursor ? { cursor } : {}) },
            { signal },
          );
          items.push(...page.items);
          cursor = page.next_cursor;
        } while (cursor);
        return items.filter((item) =>
          item.mode === "manual" && ["planned", "active"].includes(item.status),
        );
      }),
    retry: false,
  });
export const iterationEventsOptions = (
  wsId: string,
  id: string,
  params: Record<string, string> = {},
) =>
  queryOptions({
    queryKey: [...iterationKeys.all(wsId), "events", id, params],
    queryFn: ({ signal, client }) =>
      protectIterationRead(client, wsId, () =>
        api.getIterationEvents(wsId, id, params, { signal }),
      ),
    retry: false,
  });
export { iterationDefaultDates, iterationIsOverdue } from "./calendar";
export { iterationTimeline, type IterationTimelineFilters } from "./timeline";
export { iterationCatalogueOptions } from "./catalogue";
export {
  iterationActivityOptions,
  projectIterationEvent,
  groupIterationActivity,
  iterationActivityDays,
  type IterationEvent,
  type IterationActivityFilter,
  type IterationActivityKind,
  type IterationActivityIdentity,
  type IterationActivityActor,
  type IterationActivityStatus,
  type IterationActivityFacts,
  type IterationActivityChange,
  type IterationActivityEntry,
  type IterationActivityGroup,
} from "./activity";

export { protectIterationRead, hasIterationReadAccess } from "./access";

export { validateIterationName } from "./validation";

export {
  usePendingIterationCommands,
  type PendingIterationCommand,
} from "./command";
