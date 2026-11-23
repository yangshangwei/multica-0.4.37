import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import { withQueryAccessBoundary } from "../query-access";

export const propertyKeys = {
  all: (wsId: string) => ["properties", wsId] as const,
  list: (wsId: string, includeArchived = false) =>
    [...propertyKeys.all(wsId), "list", includeArchived] as const,
};

export function propertyListOptions(wsId: string, includeArchived = false) {
  const queryFn = withQueryAccessBoundary(() => api.listProperties(includeArchived));
  return queryOptions({
    queryKey: propertyKeys.list(wsId, includeArchived),
    queryFn,
    select: (data) => data.properties,
  });
}
