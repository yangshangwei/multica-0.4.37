"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "../auth";
import { ApiError, errorCode } from "../api";
import {
  adminApiScope, adminKeys, adminMeOptions, AdminUnsupportedError,
  clearAdminCache, isAdminKey, isAdminPermissionDenied,
} from "./queries";

function isUnsupportedResponse(error: unknown): boolean {
  return error instanceof AdminUnsupportedError ||
    (error instanceof ApiError && error.status === 403 && errorCode(error) === "admin_mode_disabled");
}

export function useAdminAccess() {
  const user = useAuthStore((state) => state.user);
  const authStatus = useAuthStore((state) => state.status);
  const apiScope = useAuthStore(() => adminApiScope());
  const userId = user?.id ?? "";
  const scopeId = JSON.stringify([apiScope, userId]);
  const [blocked, setBlocked] = useState<{ scopeId: string; reason: "denied" | "unsupported" } | null>(null);
  const reason = blocked?.scopeId === scopeId ? blocked.reason : null;
  const qc = useQueryClient();
  const query = useQuery({
    ...adminMeOptions({ apiScope, userId, organizationId: null }),
    enabled: authStatus === "authenticated" && userId !== "" && !reason,
  });

  useEffect(() => {
    const reject = (key: readonly unknown[] | undefined, error: unknown) => {
      if (adminApiScope() !== apiScope || useAuthStore.getState().user?.id !== userId) return;
      if (!isAdminKey(key) || key?.[1] !== apiScope || key?.[2] !== userId) return;
      if (!isAdminPermissionDenied(error) && !isUnsupportedResponse(error)) return;
      setBlocked({ scopeId, reason: isUnsupportedResponse(error) ? "unsupported" : "denied" });
      clearAdminCache(qc);
    };
    const unsubscribeQueries = qc.getQueryCache().subscribe((event) => {
      if (event.type === "updated" && event.action.type === "error") {
        reject(event.query.queryKey, event.action.error);
      }
    });
    const unsubscribeMutations = qc.getMutationCache().subscribe((event) => {
      if (event.type === "updated" && event.action.type === "error") {
        reject(event.mutation.options.mutationKey, event.action.error);
      }
    });
    return () => { unsubscribeQueries(); unsubscribeMutations(); };
  }, [qc, apiScope, userId, scopeId]);

  const identity = query.data;
  useEffect(() => {
    if (!identity) return;
    // A rechecked organization binding supersedes every detail from the old
    // organization. The identity probe itself has no organization yet.
    const predicate = (entry: { queryKey: readonly unknown[] }) =>
      isAdminKey(entry.queryKey) && entry.queryKey[1] === apiScope &&
      entry.queryKey[2] === userId && entry.queryKey[3] !== null &&
      entry.queryKey[3] !== identity.organizationId;
    void qc.cancelQueries({ predicate });
    qc.removeQueries({ predicate });
  }, [qc, identity, apiScope, userId]);

  const status = authStatus === "unauthenticated" ? "signed_out"
    : authStatus !== "authenticated" ? "loading"
    : reason === "unsupported" || isUnsupportedResponse(query.error) ||
      (query.error instanceof ApiError && query.error.status === 404) ? "unsupported"
    : reason === "denied" || isAdminPermissionDenied(query.error) ? "denied"
    : query.isError ? "unavailable"
    : identity ? "ready" : "loading";

  return {
    status,
    identity: status === "ready" ? identity : undefined,
    retry: () => {
      if (reason) {
        setBlocked(null);
        void qc.invalidateQueries({ queryKey: adminKeys.me({ apiScope, userId, organizationId: null }) });
      } else {
        void query.refetch();
      }
    },
  };
}
