import { sanitizeNextUrl } from "@multica/core/auth";

export function isAdminDestination(value: string | null): boolean {
  const safe = sanitizeNextUrl(value);
  if (!safe) return false;
  const pathname = safe.split(/[?#]/, 1)[0];
  return pathname === "/admin" || pathname?.startsWith("/admin/") === true;
}

export function adminLoginUrl(path: string): string {
  const next = isAdminDestination(path) ? path : "/admin";
  return `/login?next=${encodeURIComponent(next)}`;
}

/** Start a fresh admin document after login. Next's route cache can retain the
 * fragment from the protected page visited before login and append it again
 * during a soft navigation (Next 16.2.6 segment-cache/navigation).
 */
export function navigateToAdminAfterLogin(path: string, replace = false): boolean {
  if (!isAdminDestination(path)) return false;
  if (replace) window.location.replace(path);
  else window.location.assign(path);
  return true;
}
