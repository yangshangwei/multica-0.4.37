"use client";

import { useEffect, type ReactNode } from "react";
import { useAuthStore } from "@multica/core/auth";
import { setCurrentWorkspace } from "@multica/core/platform";
import { AdminShell } from "@multica/views/admin";
import { currentPath, useNavigation } from "@multica/views/navigation";
import { adminLoginUrl } from "./admin-path";

export function AdminRoute({ children }: { children: ReactNode }) {
  const navigation = useNavigation();
  const destination = currentPath(navigation);
  const status = useAuthStore((state) => state.status);
  // The workspace singleton is a route mirror; clear it before child queries
  // start so no admin request inherits a workspace header or storage scope.
  setCurrentWorkspace(null, null);

  useEffect(() => {
    if (status === "unauthenticated") navigation.replace(adminLoginUrl(destination));
  }, [status, navigation, destination]);

  return <AdminShell>{children}</AdminShell>;
}
