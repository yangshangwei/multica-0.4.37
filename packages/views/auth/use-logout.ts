"use client";

import { useCallback, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { clearClientSessionData } from "@multica/core/platform";
import { paths } from "@multica/core/paths";
import { useNavigation } from "../navigation";
import { toast } from "sonner";
import { useT } from "../i18n";

/**
 * Performs a complete logout: clears per-workspace client storage, legacy
 * cookies, the desktop tab state, the entire React Query cache, the
 * in-memory auth store, and finally navigates to /login. Wraps what was
 * previously duplicated in app-sidebar's logout handler so NoAccessPage's
 * "Sign in as a different user" and any future entry point can use the
 * same flow.
 *
 * Without a unified logout, callers that only do `navigate('/login')`
 * leave the auth cookie + React Query cache + local storage intact —
 * AuthInitializer then silently re-authenticates the user on the login
 * page and redirects them back where they came from.
 */
export function useLogout() {
  const queryClient = useQueryClient();
  const authLogout = useAuthStore((s) => s.logout);
  const { push } = useNavigation();
  const { t } = useT("auth");
  const pending = useRef(false);

  return useCallback(async () => {
    if (pending.current) return;
    pending.current = true;
    try {
      // Cookie teardown must succeed before erasing drafts or redirecting.
      // The store invokes this callback before publishing unauthenticated state.
      await authLogout(() => clearClientSessionData(queryClient));
      push(paths.login());
    } catch {
      toast.error(t(($) => $.password.request_failed));
    } finally {
      pending.current = false;
    }
  }, [queryClient, authLogout, push, t]);
}
