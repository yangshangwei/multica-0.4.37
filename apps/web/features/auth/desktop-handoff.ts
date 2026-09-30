export function desktopCallbackUrl(token: string, state: string): string {
  const url = `multica://auth/callback?token=${encodeURIComponent(token)}`;
  return state ? `${url}&desktop_state=${encodeURIComponent(state)}` : url;
}

export function desktopStateFromOAuth(state: string): string {
  const value = state.split(",").find((part) => part.startsWith("desktop_state:"));
  if (!value) return "";
  try { return decodeURIComponent(value.slice("desktop_state:".length)); }
  catch { return ""; }
}
