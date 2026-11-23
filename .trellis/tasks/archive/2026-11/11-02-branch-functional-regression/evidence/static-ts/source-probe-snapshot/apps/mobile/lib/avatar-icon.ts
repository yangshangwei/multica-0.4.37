import data from "./avatar-icons.generated.json";

// Generated from the web avatar registry. See scripts/sync-avatar-icons.cjs.
export type AvatarIconName = keyof typeof data.nodes;

export function resolveAvatarIcon(value?: string | null): AvatarIconName | null {
  if (value?.startsWith("icon:")) {
    const name = value.slice(5).trim();
    return Object.hasOwn(data.nodes, name) ? name as AvatarIconName : null;
  }
  if (value?.startsWith("emoji:")) {
    const emoji = value.slice(6).trim().replace(/\uFE0F/g, "");
    if (!emoji) return null;
    return Object.hasOwn(data.legacy, emoji)
      ? (data.legacy as Record<string, AvatarIconName>)[emoji]
      : "bot";
  }
  return null;
}

export function avatarIconColor(name: AvatarIconName, dark: boolean): string {
  const tone = data.tones[name] as keyof typeof data.colors;
  return data.colors[tone][dark ? "dark" : "light"];
}

export const avatarIconNodes = data.nodes;

// Mirrors packages/views/projects/components/project-icon.tsx. Project records
// predate avatar markers and can contain a bare emoji.
export function resolveProjectIcon(value?: string | null): AvatarIconName {
  if (!value || value === "📁") return "package";
  return resolveAvatarIcon(
    value.startsWith("icon:") || value.startsWith("emoji:") ? value : `emoji:${value}`,
  ) ?? "package";
}
