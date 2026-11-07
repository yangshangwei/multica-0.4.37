"use client";

import { getApi } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";

/** A private navigation session never survives a server/viewer or login boundary. */
export function useWebNavigationOwner(): string | null {
  const status = useAuthStore((state) => state.status);
  const user = useAuthStore((state) => state.user);
  if (status !== "authenticated" || !user || user.requires_account_setup || user.requires_password_change) return null;
  const api = getApi();
  return JSON.stringify([api.getBaseUrl(), user.id, api.getSessionScope()]);
}
