import { accountPath, parseAccountPath } from "@multica/core/paths/account";

/** Main/preload boundaries accept only canonical account routes. */
export function parseDesktopAccountPath(value: unknown): string | null {
  if (typeof value !== "string" || value.length > 2048) return null;
  const destination = parseAccountPath(value);
  return destination ? accountPath(destination) : null;
}

export function parseAccountDeepLink(value: string): string | null {
  try {
    const url = new URL(value);
    if (url.protocol !== "multica:" || url.hostname !== "account" || url.username || url.password || url.port || (url.pathname !== "" && url.pathname !== "/")) return null;
    return parseDesktopAccountPath(`/account${url.search}${url.hash}`);
  } catch {
    return null;
  }
}
