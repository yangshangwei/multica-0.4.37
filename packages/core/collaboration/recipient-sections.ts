import type { CollaborationRecipientAccount } from "./schema";

export type CollaborationRecipientSectionKey = "self" | "selected" | "recent" | "all";
export interface CollaborationRecipientSection { key: CollaborationRecipientSectionKey; accounts: CollaborationRecipientAccount[] }

/** Always returns self, selected, recent and all in that order; an account appears only in its first section. */
export function collaborationRecipientSections({ selected, recent, pages, self }: {
  selected: readonly CollaborationRecipientAccount[];
  recent: readonly CollaborationRecipientAccount[];
  pages: readonly { recipients: readonly CollaborationRecipientAccount[] }[];
  self?: CollaborationRecipientAccount | null;
}): CollaborationRecipientSection[] {
  const seen = new Set<string>();
  const take = (accounts: readonly CollaborationRecipientAccount[]) => accounts.filter((account) => {
    const id = account.user_id.toLowerCase();
    if (seen.has(id)) return false;
    seen.add(id);
    return true;
  });
  return [
    { key: "self", accounts: take(self ? [self] : []) },
    { key: "selected", accounts: take(selected) },
    { key: "recent", accounts: take(recent) },
    { key: "all", accounts: take(pages.flatMap((page) => page.recipients)) },
  ];
}
