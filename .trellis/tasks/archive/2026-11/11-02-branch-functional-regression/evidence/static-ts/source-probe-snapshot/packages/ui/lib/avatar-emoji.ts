const AVATAR_EMOJI_PREFIX = "emoji:";

// Read-only support for avatars persisted before the Lucide icon format.
export function parseAvatarEmoji(value?: string | null): string | null {
  if (!value?.startsWith(AVATAR_EMOJI_PREFIX)) return null;

  const emoji = value.slice(AVATAR_EMOJI_PREFIX.length).trim();
  return emoji || null;
}
