"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "@multica/core/auth";
import { setCurrentWorkspace } from "@multica/core/platform";
import { currentPath, useNavigation } from "@multica/views/navigation";

/** Account access depends on authentication, independently of workspace discovery. */
export function AccountRoute({ children }: { children: ReactNode }) {
  const navigation = useNavigation();
  const destination = currentPath(navigation);
  const status = useAuthStore((state) => state.status);
  const user = useAuthStore((state) => state.user);
  setCurrentWorkspace(null, null);

  useEffect(() => {
    if (status === "unauthenticated") navigation.replace(`/login?next=${encodeURIComponent(destination)}`);
  }, [status, navigation, destination]);

  if (status !== "authenticated" || !user || user.requires_account_setup === true || user.requires_password_change === true) return null;
  return children;
}
