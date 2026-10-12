import { useCallback, useEffect, useRef, useState } from "react";
import {
  classifyConnectionFailure,
  type AuthRecoveryState,
  type ConnectionFailure,
} from "@multica/core/auth";
import type { useWorkspaceList } from "@multica/core/workspace";

type WorkspaceListState = Pick<ReturnType<typeof useWorkspaceList>,
  "ready" | "unavailable" | "isFetching" | "error" | "errorUpdatedAt" | "refetch"
>;

/** Retries only an initial desktop list failure; Query keeps owning the data. */
export function useWorkspaceConnectionRecovery({
  ready, unavailable, isFetching, error, errorUpdatedAt, refetch,
}: WorkspaceListState, enabled: boolean) {
  const [failure, setFailure] = useState<ConnectionFailure | null>(null);
  const [nextRetryAt, setNextRetryAt] = useState<number | null>(null);
  const [retrying, setRetrying] = useState(false);
  const retryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const retryCount = useRef(0);
  const attempt = useRef<object | null>(null);

  const clearTimer = useCallback(() => {
    if (retryTimer.current !== null) clearTimeout(retryTimer.current);
    retryTimer.current = null;
  }, []);

  const retry = useCallback(() => {
    if (!enabled || ready || isFetching || attempt.current) return;
    clearTimer();
    setNextRetryAt(null);
    const currentAttempt = {};
    attempt.current = currentAttempt;
    retryCount.current += 1;
    setRetrying(true);
    const finish = () => {
      if (attempt.current !== currentAttempt) return;
      attempt.current = null;
      setRetrying(false);
    };
    // Keep a request that Query already owns instead of cancelling and duplicating it.
    void refetch({ cancelRefetch: false }).then(finish, finish);
  }, [enabled, ready, isFetching, refetch, clearTimer]);

  useEffect(() => {
    if (enabled && !ready) return;
    clearTimer();
    attempt.current = null;
    retryCount.current = 0;
    setRetrying(false);
    setFailure(null);
    setNextRetryAt(null);
  }, [enabled, ready, clearTimer]);

  useEffect(() => {
    if (!enabled || ready || !unavailable || isFetching) return;
    setFailure(classifyConnectionFailure(error, errorUpdatedAt || Date.now()));
  }, [enabled, ready, unavailable, isFetching, error, errorUpdatedAt]);

  useEffect(() => {
    if (!enabled || ready || !unavailable || isFetching || retrying) {
      setNextRetryAt(null);
      return;
    }
    const delay = Math.min(1_000 * 2 ** retryCount.current, 30_000);
    setNextRetryAt(Date.now() + delay);
    retryTimer.current = setTimeout(retry, delay);
    return clearTimer;
  }, [enabled, ready, unavailable, isFetching, retrying, errorUpdatedAt, retry, clearTimer]);

  useEffect(() => () => {
    clearTimer();
    attempt.current = null;
  }, [clearTimer]);

  const recovery: AuthRecoveryState | null = enabled && !ready && (failure || unavailable)
    ? {
        isRetrying: isFetching || retrying,
        nextRetryAt,
        failure: failure ?? classifyConnectionFailure(error, errorUpdatedAt || Date.now()),
      }
    : null;
  return { recovery, retry };
}
