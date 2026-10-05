import { queryOptions } from "@tanstack/react-query";
import { protectProjectRequest } from "./access";
import { api, ApiError } from "../api";
import { projectKeys } from "./queries";
import type { ProjectRiskSignal } from "../types/project-p1";

export const projectP1Keys = {
  all: (wsId: string, id: string) => [...projectKeys.detail(wsId, id), "management"] as const,
  capabilities: (wsId: string, connection: string) => [...projectKeys.all(wsId), "capabilities", connection] as const,
  timezone: (wsId: string) => [...projectKeys.all(wsId), "planning-timezone"] as const,
  overview: (wsId: string, id: string) => [...projectP1Keys.all(wsId, id), "overview"] as const,
  risks: (wsId: string, id: string) => [...projectP1Keys.all(wsId, id), "risk"] as const,
  updates: (wsId: string, id: string) => [...projectP1Keys.all(wsId, id), "updates"] as const,
};
const retry = (count: number, error: Error) => !(error instanceof ApiError && [400, 401, 403, 404, 422].includes(error.status)) && count < 1;
export function projectCapabilitiesOptions(wsId: string) {
  return queryOptions({ queryKey: projectP1Keys.capabilities(wsId, api.getBaseUrl?.() ?? ""),
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, undefined, () => api.getProjectCapabilities(wsId, { signal })), retry });
}
export function projectOverviewOptions(wsId: string, id: string) {
  return queryOptions({ queryKey: projectP1Keys.overview(wsId, id), queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.getProjectOverview(wsId, id, { signal })), retry });
}
export function projectRiskOptions(wsId: string, id: string, risk: ProjectRiskSignal, cursor?: string, version?: string) {
  return queryOptions({ queryKey: [...projectP1Keys.risks(wsId, id), risk, cursor ?? null, version ?? null],
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.getProjectRiskIssues(wsId, id, { signal: risk, cursor, version }, { signal })), retry });
}
export function projectUpdatesOptions(wsId: string, id: string, cursor?: string) {
  return queryOptions({ queryKey: [...projectP1Keys.updates(wsId, id), cursor ?? null],
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.listProjectUpdates(wsId, id, cursor, { signal })), retry });
}
export function projectUpdateRevisionsOptions(wsId: string, id: string, updateId: string, cursor?: string) {
  return queryOptions({ queryKey: [...projectP1Keys.all(wsId, id), "history", updateId, cursor ?? null],
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.listProjectUpdateRevisions(wsId, id, updateId, cursor, { signal })), retry });
}
export function projectDeleteImpactOptions(wsId: string, id: string) {
  return queryOptions({ queryKey: [...projectP1Keys.all(wsId, id), "delete-impact"], queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.getProjectDeleteImpact(wsId, id, { signal })), retry });
}

export function projectExecutionEvidenceOptions(wsId: string, id: string, updateId: string, revision: number, taskId: string) {
  return queryOptions({ queryKey: [...projectP1Keys.all(wsId, id), "execution-evidence", updateId, revision, taskId],
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.getProjectExecutionEvidence(wsId, id, updateId, revision, taskId, { signal })), retry });
}
