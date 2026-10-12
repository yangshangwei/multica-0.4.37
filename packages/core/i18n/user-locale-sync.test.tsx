// @vitest-environment jsdom

import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleAdapterProvider } from "./adapter-context";
import { UserLocaleSync } from "./user-locale-sync";
import type { LocaleAdapter } from "./types";

const state = vi.hoisted(() => ({
  user: { language: null as string | null },
  i18n: { language: "en" },
}));

vi.mock("../auth", () => ({
  useAuthStore: Object.assign(
    (selector: (store: { user: typeof state.user }) => unknown) => selector(state),
    { getState: () => state },
  ),
}));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ i18n: state.i18n }) }));

const reload = vi.fn();
const persist = vi.fn();
const adapter: LocaleAdapter = {
  getUserChoice: () => null,
  getSystemPreferences: () => ["zh-CN"],
  persist,
};

function mountSync(localeAdapter: LocaleAdapter = adapter) {
  return render(
    <LocaleAdapterProvider adapter={localeAdapter}>
      <UserLocaleSync />
    </LocaleAdapterProvider>,
  );
}

beforeEach(() => {
  state.user.language = null;
  state.i18n.language = "en";
  vi.clearAllMocks();
  vi.stubGlobal("window", new Proxy(window, {
    get(target, property, receiver) {
      if (property === "location") return { reload };
      return Reflect.get(target, property, receiver);
    },
  }));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("UserLocaleSync", () => {
  it("lets the platform coordinate refresh after persisting the normalized preference", () => {
    state.user.language = "ja";
    state.i18n.language = "zh-Hans";
    const platformReload = vi.fn(() => {
      expect(persist).toHaveBeenCalledExactlyOnceWith("en");
    });
    mountSync({ ...adapter, reload: platformReload });
    expect(platformReload).toHaveBeenCalledOnce();
    expect(reload).not.toHaveBeenCalled();
  });

  it("does not request a platform refresh when the preference already matches", () => {
    state.user.language = "en";
    const platformReload = vi.fn();
    mountSync({ ...adapter, reload: platformReload });
    expect(persist).not.toHaveBeenCalled();
    expect(platformReload).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it.each(["ja", "ko"])("normalizes old server preference %s and converges after reload", (language) => {
    state.user.language = language;
    state.i18n.language = "zh-Hans";
    const first = mountSync();
    expect(persist).toHaveBeenCalledExactlyOnceWith("en");
    expect(reload).toHaveBeenCalledOnce();

    first.unmount();
    state.i18n.language = "en";
    mountSync();
    expect(persist).toHaveBeenCalledOnce();
    expect(reload).toHaveBeenCalledOnce();
  });

  it.each([null, "", "fr", "<script>"])("ignores unsupported or absent server choice %s", (language) => {
    state.user.language = language;
    mountSync();
    expect(persist).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it("keeps retained Chinese cross-device synchronization", () => {
    state.user.language = "zh-Hans";
    mountSync();
    expect(persist).toHaveBeenCalledExactlyOnceWith("zh-Hans");
    expect(reload).toHaveBeenCalledOnce();
  });
});
