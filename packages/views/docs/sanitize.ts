/**
 * Sanitize schema for the in-app documentation renderer.
 *
 * Built from the canonical product schema in `@multica/ui/markdown` rather than
 * from `defaultSchema`, so a future XSS fix there lands here too. Two deltas,
 * both required by what this surface renders:
 *
 *  1. The three custom element names the directive plugins emit. Sanitize drops
 *     unknown tag names, which would strip the wrapper off all 73 callouts and
 *     delete both leaf directives outright.
 *  2. `clobberPrefix: ""`. The default prefixes every `id` with `user-content-`,
 *     which would leave the DOM id (`user-content-概览`) disagreeing with the
 *     table of contents and the public docs site (`概览`) — every product deep
 *     link would land at the top of the page instead of its heading.
 *
 * DOM-clobbering is the risk `clobberPrefix` exists to manage, and it is
 * acceptable here for a reason specific to this surface: these ids come from
 * headings in markdown compiled into the server binary at build time, not from
 * anything a user can write. Do not copy this relaxation to a surface that
 * renders user content.
 *
 * `rehype-raw` is deliberately NOT part of this pipeline. The generator emits no
 * raw HTML (`assertNoUnknownComponents` rejects JSX, and the bundle has none), so
 * there is no reason to enable HTML passthrough on a document renderer.
 */

import type { Options } from "rehype-sanitize";
import { markdownSanitizeSchema } from "@multica/ui/markdown";
import {
  CALLOUT_ELEMENT,
  COMMUNITY_LINKS_ELEMENT,
  VIDEO_EMBED_ELEMENT,
} from "./directives";

export const docsSanitizeSchema: Options = {
  ...markdownSanitizeSchema,
  // Unprefixed ids so an anchor matches the bundle's toc. See the note above.
  clobberPrefix: "",
  tagNames: [
    ...(markdownSanitizeSchema.tagNames ?? []),
    CALLOUT_ELEMENT,
    VIDEO_EMBED_ELEMENT,
    COMMUNITY_LINKS_ELEMENT,
  ],
  attributes: {
    ...markdownSanitizeSchema.attributes,
    [CALLOUT_ELEMENT]: ["dataTone"],
    [VIDEO_EMBED_ELEMENT]: ["dataProvider", "dataId", "dataTitle"],
    [COMMUNITY_LINKS_ELEMENT]: [
      "dataDiscordDescription",
      "dataGithubDescription",
      "dataXDescription",
    ],
  },
};
