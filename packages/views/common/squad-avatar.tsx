"use client";

import { ActorAvatar, type ActorAvatarProps } from "@multica/ui/components/common/actor-avatar";
import { formatAvatarIcon, type AvatarIconName } from "@multica/ui/lib/avatar-icon";

// Template provenance supplies a default when no avatar was persisted.
const TEMPLATE_ICONS: Record<string, AvatarIconName> = {
  "feature-delivery": "rocket",
  "bug-fix": "bug",
  "review-gate": "shield-check",
  discovery: "telescope",
  docs: "book-open",
  maintenance: "wrench",
  release: "package",
  incident: "siren",
};

export function SquadAvatar({
  templateKey,
  avatarUrl,
  ...props
}: Omit<ActorAvatarProps, "isAgent" | "isSystem" | "isSquad"> & { templateKey?: string }) {
  const icon = templateKey && Object.hasOwn(TEMPLATE_ICONS, templateKey)
    ? TEMPLATE_ICONS[templateKey]
    : undefined;
  return <ActorAvatar {...props} avatarUrl={avatarUrl || (icon ? formatAvatarIcon(icon) : null)} isSquad />;
}
