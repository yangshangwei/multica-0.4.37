"use client";

import { useState, useEffect } from "react";
import { cn } from "@multica/ui/lib/utils";
import {
  AVATAR_SIZE_PX,
  DEFAULT_AVATAR_SIZE,
  type AvatarSize,
} from "@multica/ui/lib/avatar-size";
import {
  AVATAR_ICON_COMPONENTS,
  AVATAR_ICON_TONE,
  resolveAvatarIcon,
} from "@multica/ui/lib/avatar-icon";
import { MulticaIcon } from "./multica-icon";

interface ActorAvatarProps {
  name: string;
  initials: string;
  avatarUrl?: string | null;
  isAgent?: boolean;
  isSystem?: boolean;
  isSquad?: boolean;
  size?: AvatarSize;
  className?: string;
}

function ActorAvatar({
  name,
  initials,
  avatarUrl,
  isAgent,
  isSystem,
  isSquad,
  size = DEFAULT_AVATAR_SIZE,
  className,
}: ActorAvatarProps) {
  const [imgError, setImgError] = useState(false);
  const px = AVATAR_SIZE_PX[size];
  const isFuAvatar = isAgent && (
    avatarUrl === "emoji:🦄" ||
    /^(?:https?:\/\/[^/]+)?\/api\/avatars\/builtin\/afu-seal-v1\.png$/.test(avatarUrl ?? "")
  );
  const iconName = resolveAvatarIcon(avatarUrl)
    ?? ((!avatarUrl || imgError) && !isSystem ? (isAgent ? "bot" : isSquad ? "users" : null) : null);
  const Icon = iconName ? AVATAR_ICON_COMPONENTS[iconName] : null;
  const tone = isFuAvatar ? AVATAR_ICON_TONE.bot : iconName ? AVATAR_ICON_TONE[iconName] : null;

  useEffect(() => {
    setImgError(false);
  }, [avatarUrl]);

  // Every actor — member, agent, squad, or system — renders as a circle. This
  // is the single source of truth for avatar shape; the upload editors mirror
  // it (packages/views/common/avatar-upload-control.tsx).
  return (
    <div
      data-slot="avatar"
      className={cn(
        "inline-flex shrink-0 items-center justify-center font-medium overflow-hidden",
        (!avatarUrl || imgError) && "bg-muted text-muted-foreground",
        tone?.bg,
        tone?.text,
        className,
        // rounded-full stays last so a call-site `className` can never override
        // the circle — avatar shape is a hard invariant, not a per-site choice.
        "rounded-full"
      )}
      style={{ width: px, height: px, fontSize: px * 0.45 }}
    >
      {isFuAvatar ? (
        <span role="img" aria-label={name}>孚</span>
      ) : Icon ? (
        <Icon
          role="img"
          aria-label={name}
          style={{ width: px * 0.5, height: px * 0.5 }}
        />
      ) : avatarUrl && !imgError ? (
        <img
          src={avatarUrl}
          alt={name}
          className="h-full w-full object-cover"
          onError={() => setImgError(true)}
        />
      ) : isSystem ? (
        <MulticaIcon noSpin style={{ width: px * 0.55, height: px * 0.55 }} />
      ) : (
        initials
      )}
    </div>
  );
}

export { ActorAvatar, type ActorAvatarProps };
