"use client";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { handleProjectAccessError, isProjectAccessLost } from "@multica/core/projects";
export function useProjectAccessGuard(error: unknown, wsId: string, projectId: string, hide: () => void) {
  const qc = useQueryClient();
  useEffect(() => { if (error) { if (isProjectAccessLost(error)) hide(); handleProjectAccessError(qc, wsId, projectId, error); } }, [error, wsId, projectId, qc, hide]);
}
