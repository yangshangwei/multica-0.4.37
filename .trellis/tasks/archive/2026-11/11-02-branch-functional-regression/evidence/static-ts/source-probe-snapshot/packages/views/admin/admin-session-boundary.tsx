"use client";

import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { adminApiScope, retainAdminScope } from "@multica/core/admin";

/** Lives above routes so cached admin data is erased after leaving the console too. */
export function AdminSessionBoundary() {
  const qc = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const status = useAuthStore((state) => state.status);
  const apiScope = useAuthStore(() => adminApiScope());

  useEffect(() => {
    const clean = () => {
      const state = useAuthStore.getState();
      retainAdminScope(qc, state.status === "authenticated" && state.user
        ? { apiScope: adminApiScope(), userId: state.user.id }
        : null);
    };
    clean();
    // Store notifications run synchronously: a new user never inherits the
    // previous user's cache while React is scheduling its next render.
    return useAuthStore.subscribe(clean);
  }, [qc, user, status, apiScope]);

  return null;
}
