// @vitest-environment node
import { expect, it } from "vitest";
import { desktopCallbackUrl, desktopStateFromOAuth } from "./desktop-handoff";
it('preserves the desktop nonce across OAuth and callback encoding', () => {
  const state = desktopStateFromOAuth('platform:desktop,desktop_state:nonce%26x');
  expect(new URL(desktopCallbackUrl('token',state)).searchParams.get('desktop_state')).toBe('nonce&x');
});
it('keeps old handoffs compatible and rejects malformed state encoding', () => {
  expect(desktopCallbackUrl('token','')).toBe('multica://auth/callback?token=token');
  expect(desktopStateFromOAuth('desktop_state:%')).toBe('');
});
