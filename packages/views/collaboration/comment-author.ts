import type { CollaborationComment } from "@multica/core/collaboration";

export function collaborationCommentAuthor(comment: CollaborationComment | null | undefined, labels: { agent: string; unknownAuthor: string }) {
  if (comment?.author_type === "agent") {
    const name = comment.author_agent?.name?.trim();
    return { name: name || labels.agent, agentLabel: name ? labels.agent : null };
  }
  return { name: comment?.author?.name ?? labels.unknownAuthor, agentLabel: null };
}
