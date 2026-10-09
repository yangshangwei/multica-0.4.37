import { queryOptions } from "@tanstack/react-query";
import { api, errorCode } from "../api";
import { protectIterationRead } from "./access";

export const iterationCatalogueOptions = (wsId: string) => queryOptions({
  queryKey: ["iterations", wsId, "catalogue"] as const,
  retry: false,
  queryFn: ({ client, signal }) => protectIterationRead(client, wsId, async () => {
    for (let attempt = 0; ; attempt++) {
      try {
        const items: Awaited<ReturnType<typeof api.listIterations>>["items"] = [];
        const cursors = new Set<string>();
        const ids = new Set<string>();
        let cursor: string | null = null;
        do {
          signal.throwIfAborted();
          const page = await api.listIterations(wsId, {
            limit: "100",
            ...(cursor ? { cursor } : {}),
          }, { signal });
          if (page.workspace_id !== wsId) throw new Error("Iteration catalogue workspace mismatch");
          for (const item of page.items) {
            if (item.workspace_id !== wsId) throw new Error("Iteration catalogue workspace mismatch");
            if (ids.has(item.id)) throw new Error("Repeated iteration catalogue identity");
            ids.add(item.id);
            items.push(item);
          }
          cursor = page.next_cursor;
          if (cursor && cursors.has(cursor)) throw new Error("Repeated iteration catalogue cursor");
          if (cursor) cursors.add(cursor);
        } while (cursor);
        return items;
      } catch (error) {
        // A stale cursor belongs to an obsolete collection. Discard all of
        // that traversal, and bound automatic restart to one complete retry.
        if (attempt > 0 || errorCode(error) !== "cursor_stale") throw error;
      }
    }
  }),
});
