import type { CollaborationIssue } from "./schema";
import type { CollaborationParticipants } from "./participant-schema";
import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, protectCollaborationRead, CollaborationUnavailableError, isCollaborationScopeCurrent, type CollaborationScope } from "./access";
import { prepareCollaborationAction } from "./operation";

export function prepareCollaborationParticipantRemoval(scope: CollaborationScope, issue: CollaborationIssue, requestId: string, participants: CollaborationParticipants, userId: string) {
  if (!isCollaborationScopeCurrent(scope) || !issue.write_context || issue.capabilities.remove_participants !== true || issue.creator.user_id !== scope.userId ||
    participants.issue_id !== issue.id.toLowerCase() || participants.collaboration_revision !== issue.collaboration_revision ||
    userId === scope.userId || !participants.participants.some((participant) => participant.user_id === userId)) throw new CollaborationUnavailableError();
  return prepareCollaborationAction(scope, issue, requestId, { action: "remove", targetUserId: userId });
}

export function collaborationParticipantsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return queryOptions({ queryKey: [...collaborationKeys.issue(scope, issueId), "participants"], staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationParticipants(issueId, { signal })) });
}
