// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import config from "../next.config.mjs";
import { i18n } from "./i18n";
import { homeCopy, localeLabels, uiTranslations } from "./translations";

vi.mock("fumadocs-mdx/next", () => ({
  createMDX: () => (config: object) => config,
}));

describe("retired documentation locales", () => {
  it("offers only English and Simplified Chinese", () => {
    expect(i18n.languages).toEqual(["en", "zh"]);
    expect(i18n.defaultLanguage).toBe("en");
    expect(Object.keys(localeLabels)).toEqual(["en", "zh"]);
    expect(Object.keys(homeCopy)).toEqual(["en", "zh"]);
    expect(Object.keys(uiTranslations)).toEqual(["zh"]);
  });

  it.each(["ja", "ko"])(
    "permanently redirects the %s root and nested paths under the existing basePath",
    async (language) => {
      expect(config.basePath).toBe("/docs");
      const redirects = await config.redirects?.();
      const rule = redirects?.find(
        (redirect) => redirect.source === "/" + language + "/:path*",
      );
      expect(rule).toEqual({
        source: "/" + language + "/:path*",
        destination: "/:path*",
        permanent: true,
      });
    },
  );
});
