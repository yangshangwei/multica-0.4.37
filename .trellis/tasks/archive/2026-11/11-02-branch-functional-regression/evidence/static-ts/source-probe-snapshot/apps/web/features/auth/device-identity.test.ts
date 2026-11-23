// jsdom on purpose: this module branches on `typeof window` and on localStorage
// throwing, and under the node environment every case would silently take the
// SSR path and still pass.
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  formatWebDeviceName,
  generateDeviceId,
  loadOrCreateWebDeviceIdentity,
} from "./device-identity";

const STORAGE_KEY = "multica_device_id";
const DEVICE_ID_PATTERN = /^[0-9a-f]{32,64}$/;

afterEach(() => {
  window.localStorage.clear();
  vi.restoreAllMocks();
});

describe("generateDeviceId", () => {
  it("produces an id the server's device_id pattern accepts", () => {
    const id = generateDeviceId();
    expect(id).toMatch(DEVICE_ID_PATTERN);
    expect(id).toHaveLength(32);
  });

  it("does not repeat itself", () => {
    const ids = new Set(Array.from({ length: 50 }, generateDeviceId));
    expect(ids.size).toBe(50);
  });
});

describe("formatWebDeviceName", () => {
  it("carries the OS and enough of the id to tell two browsers apart", () => {
    expect(formatWebDeviceName("macos", "3f9c1a2b" + "0".repeat(24))).toBe(
      "web-macos-3f9c1a2b",
    );
  });

  it("drops the OS segment when the browser gives no usable hint", () => {
    const id = "3f9c1a2b" + "0".repeat(24);
    expect(formatWebDeviceName("unknown", id)).toBe("web-3f9c1a2b");
    expect(formatWebDeviceName("", id)).toBe("web-3f9c1a2b");
  });
});

describe("loadOrCreateWebDeviceIdentity", () => {
  it("creates and persists an identity on the first visit", () => {
    const identity = loadOrCreateWebDeviceIdentity();
    expect(identity?.deviceId).toMatch(DEVICE_ID_PATTERN);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe(identity?.deviceId);
    expect(identity?.deviceName).toContain(identity!.deviceId.slice(0, 8));
  });

  it("returns the same id on every later visit", () => {
    const first = loadOrCreateWebDeviceIdentity();
    const second = loadOrCreateWebDeviceIdentity();
    expect(second?.deviceId).toBe(first?.deviceId);
    expect(second?.deviceName).toBe(first?.deviceName);
  });

  it("replaces a stored value the server would reject", () => {
    window.localStorage.setItem(STORAGE_KEY, "not-a-device-id");
    const identity = loadOrCreateWebDeviceIdentity();
    expect(identity?.deviceId).toMatch(DEVICE_ID_PATTERN);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe(identity?.deviceId);
  });

  it("offers nothing when there is no store to read, as during server rendering", () => {
    expect(loadOrCreateWebDeviceIdentity(null)).toBeNull();
  });

  it("gives up rather than hand out an identity it cannot persist", () => {
    // An unwritable store would mint a new member on every page load,
    // scattering one person's issues across a growing pile of identities. The
    // login page is the better failure.
    expect(
      loadOrCreateWebDeviceIdentity({
        getItem: () => null,
        setItem: () => {
          throw new Error("QuotaExceededError");
        },
      }),
    ).toBeNull();
  });

  it("gives up when the store cannot even be read", () => {
    expect(
      loadOrCreateWebDeviceIdentity({
        getItem: () => {
          throw new Error("SecurityError");
        },
        setItem: () => {},
      }),
    ).toBeNull();
  });
});
