"use client";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { protectIterationRead } from "@multica/core/iterations";
import { useWorkspacePaths } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useT } from "../i18n";

export function useIterationCatalogue(wsId: string, enabled = true) {
  return useQuery({
    queryKey: ["iterations", wsId, "catalogue"],
    enabled,
    retry: false,
    queryFn: ({ client, signal }) => protectIterationRead(client, wsId, async () => {
      const items: Awaited<ReturnType<typeof api.listIterations>>["items"] = [];
      const cursors = new Set<string>();
      let cursor: string | null = null;
      do {
        const page = await api.listIterations(wsId, { limit: "100", ...(cursor ? { cursor } : {}) }, { signal });
        items.push(...page.items);
        cursor = page.next_cursor;
        if (cursor && cursors.has(cursor)) throw new Error("Repeated iteration catalogue cursor");
        if (cursor) cursors.add(cursor);
      } while (cursor);
      return items;
    }),
  });
}

export function IterationReference({ id, catalogue }: { id: string | null | undefined; catalogue: { id: string; name: string }[] }) {
  const { t } = useT("projects");
  const paths = useWorkspacePaths();
  if (id === undefined) return <span>{t(($) => $.iterations.unknownHistory)}</span>;
  if (id === null) return <span>{t(($) => $.iterations.unassignedState)}</span>;
  return <AppLink href={paths.iterationDetail(id)}>{catalogue.find((item) => item.id === id)?.name ?? t(($) => $.iterations.unavailableReference)}</AppLink>;
}
