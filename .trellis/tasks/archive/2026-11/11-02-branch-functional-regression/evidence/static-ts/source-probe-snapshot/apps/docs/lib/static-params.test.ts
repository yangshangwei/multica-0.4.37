import { describe, expect, it } from "vitest";
import { docsSlugStaticParams } from "./static-params";

// `source.generateParams()` hands back loosely-typed params (`lang: string`),
// so the inputs here mirror that shape — the `lang` strings are validated and
// narrowed by `docsSlugStaticParams` itself.
type RawParam = { lang: string; slug: string[] };

describe("docsSlugStaticParams", () => {
  it("returns every localized slug page and drops the home param", () => {
    // Stale generated params must not reintroduce retired routes. The empty
    // home slug is rendered by the locale root rather than the catch-all.
    const params: RawParam[] = [
      { lang: "en", slug: [] },
      { lang: "en", slug: ["agents"] },
      { lang: "en", slug: ["cli", "reference"] },
      { lang: "zh", slug: ["agents"] },
      { lang: "ko", slug: ["agents"] },
      { lang: "ko", slug: ["cli", "reference"] },
      { lang: "ja", slug: ["agents"] },
      { lang: "ja", slug: ["cli", "reference"] },
    ];

    expect(docsSlugStaticParams(params)).toEqual([
      { lang: "en", slug: ["agents"] },
      { lang: "en", slug: ["cli", "reference"] },
      { lang: "zh", slug: ["agents"] },
    ]);
  });

  it("drops unknown languages and de-duplicates repeated params", () => {
    const params: RawParam[] = [
      { lang: "zh", slug: ["agents"] },
      { lang: "zh", slug: ["agents"] },
      { lang: "fr", slug: ["agents"] },
    ];

    expect(docsSlugStaticParams(params)).toEqual([
      { lang: "zh", slug: ["agents"] },
    ]);
  });
});
