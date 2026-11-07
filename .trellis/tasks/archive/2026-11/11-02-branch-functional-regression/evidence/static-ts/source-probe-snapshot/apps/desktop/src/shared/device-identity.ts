/**
 * Shape of the per-installation device identity, shared by main (which
 * resolves and persists it) and preload (which hands it to the renderer).
 *
 * On a deployment with intranet device auth enabled, the server keys a user off
 * this id, so it is the closest thing this client has to an account: losing it
 * joins as a new member, copying it makes two machines one member.
 */
export interface DeviceIdentity {
  deviceId: string;
  /** Label other members see, e.g. "artisan@mac-mini". */
  deviceName: string;
}

/** Mirrors the server's accepted device_id shape (32-64 lowercase hex). */
export const DEVICE_ID_PATTERN = /^[0-9a-f]{32,64}$/;

export function isValidDeviceId(value: unknown): value is string {
  return typeof value === "string" && DEVICE_ID_PATTERN.test(value);
}
