import type { QueryFunctionContext } from "@tanstack/react-query";
import { ApiError } from "./api/client";

/**
 * A denied snapshot must stay absent through later retries and remounts.
 * Bind the wrapped function before queryOptions when using select so its
 * response type is inferred before the selector is contextually typed.
 */
export function withQueryAccessBoundary<TData>(
  queryFn: (context: QueryFunctionContext) => Promise<TData>,
): (context: QueryFunctionContext) => Promise<TData> {
  return async (context) => {
    const { client, queryKey, signal } = context;
    const query = client.getQueryCache().find({ queryKey, exact: true });
    try {
      return await queryFn(context);
    } catch (error) {
      if (!signal.aborted && error instanceof ApiError && !error.isStaleAuthResponse &&
        (error.status === 401 || error.status === 403)) {
        // A manual update also replaces Query's cancellation rollback snapshot.
        // setState alone lets an unmount during retry restore denied data.
        query?.setData(undefined, { manual: true, updatedAt: 0 });
      }
      throw error;
    }
  };
}
