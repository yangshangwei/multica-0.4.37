"use client";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { clearProtectedProjectContent, isProjectAccessLost } from "@multica/core/projects";
export function useProjectAccessGuard(error: unknown, wsId: string, projectId: string, hide: () => void) {
  const qc = useQueryClient();
  useEffect(() => { if (isProjectAccessLost(error)) { hide(); clearProtectedProjectContent(qc, wsId, projectId); } }, [error, wsId, projectId, qc, hide]);
}
