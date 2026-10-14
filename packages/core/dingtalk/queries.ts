import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import { withQueryAccessBoundary } from "../query-access";

/** Query key namespace for everything DingTalk-installation-related. Realtime
 * sync invalidates `installations(wsId)` on `dingtalk_installation:*` events so
 * the Settings panel updates without a manual refetch (e.g. after a binding
 * lands the install in another tab). */
export const dingtalkKeys = {
  all: (wsId: string) => ["dingtalk", wsId] as const,
  installations: (wsId: string) => [...dingtalkKeys.all(wsId), "installations"] as const,
  groups: (wsId: string) => [...dingtalkKeys.all(wsId), "groups"] as const,
  agentGroups: (wsId: string, agentId: string) =>
    [...dingtalkKeys.groups(wsId), "agent", agentId] as const,
  inactiveGroups: (wsId: string, installationId: string) =>
    [...dingtalkKeys.groups(wsId), "inactive", installationId] as const,
  agentInactiveGroups: (wsId: string, agentId: string, installationId: string) =>
    [...dingtalkKeys.agentGroups(wsId, agentId), "inactive", installationId] as const,
};

export const dingtalkInstallationsOptions = (wsId: string) =>
  queryOptions({
    queryKey: dingtalkKeys.installations(wsId),
    queryFn: withQueryAccessBoundary(() => api.listDingTalkInstallations(wsId)),
    enabled: !!wsId,
  });

export const dingtalkGroupsOptions = (wsId: string) => {
  const queryFn = withQueryAccessBoundary(() => api.listDingTalkGroups(wsId));
  return queryOptions({
    queryKey: dingtalkKeys.groups(wsId),
    queryFn,
    enabled: !!wsId,
    // Group discovery arrives through DingTalk Stream callbacks rather than an
    // HTTP mutation, so refresh lightly while the permission-filtered Settings
    // inventory is open. Stop after an error instead of hammering a backend.
    refetchInterval: (query) =>
      query.state.status === "success" &&
      query.state.data?.group_discovery_supported === true
        ? 5_000
        : false,
  });
};

export const dingtalkAgentGroupsOptions = (wsId: string, agentId: string) => {
  const queryFn = withQueryAccessBoundary(() => api.listAgentDingTalkGroups(agentId));
  return queryOptions({
    queryKey: dingtalkKeys.agentGroups(wsId, agentId),
    queryFn,
    enabled: !!wsId && !!agentId,
    // Agent detail uses a separate cache and a permission-scoped endpoint; it
    // must never borrow the admin workspace inventory and filter it client-side.
    refetchInterval: (query) =>
      query.state.status === "success" &&
      query.state.data?.group_discovery_supported === true
        ? 5_000
        : false,
  });
};

export const dingtalkInactiveGroupsOptions = (wsId: string, installationId: string) => {
  // Both scopes feed the same panel; keep options composable while the key
  // factories retain their exact tuples and workspace/agent separation.
  const queryKey: readonly string[] = dingtalkKeys.inactiveGroups(wsId, installationId);
  return infiniteQueryOptions({
    queryKey,
    initialPageParam: 0,
    queryFn: (context) =>
      withQueryAccessBoundary(() => api.listDingTalkGroups(wsId, {
        activity: "inactive",
        installationId,
        offset: context.pageParam,
      }))(context),
    getNextPageParam: (page) => page.next_offset,
    enabled: !!wsId && !!installationId,
  });
};

export const dingtalkAgentInactiveGroupsOptions = (
  wsId: string,
  agentId: string,
  installationId: string,
) => {
  const queryKey: readonly string[] = dingtalkKeys.agentInactiveGroups(wsId, agentId, installationId);
  return infiniteQueryOptions({
    queryKey,
    initialPageParam: 0,
    queryFn: (context) =>
      withQueryAccessBoundary(() => api.listAgentDingTalkGroups(agentId, {
        activity: "inactive",
        installationId,
        offset: context.pageParam,
      }))(context),
    getNextPageParam: (page) => page.next_offset,
    enabled: !!wsId && !!agentId && !!installationId,
  });
};
