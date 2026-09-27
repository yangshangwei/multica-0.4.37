// @vitest-environment node

import { describe, expect, it } from "vitest";
import { templateLanguageFor } from "./use-role-templates";

describe("templateLanguageFor", () => {
  it.each([
    ["en", "en"], ["zh-Hans", "zh"], ["ja", "en"], ["ko", "en"],
    ["en-US", "en"], ["zh-Hant-TW", "zh"], ["ja-JP", "en"], ["ko-KR", "en"],
    [" ZH_cn ", "zh"], ["de-DE", "en"], ["", "en"],
  ])("maps %j to a supported template language", (locale, expected) => {
    expect(templateLanguageFor(locale)).toBe(expected);
  });
});
