/**
 * The docs pages and headings product code links to, in one place.
 *
 * Every deep link from the app used to be a hand-written `https://multica.ai/docs/...`
 * string with a locale prefix computed at the call site. Two of them were broken
 * and nobody could tell: the anchor is only correct if it matches a heading in the
 * documentation, and nothing compared the two. Naming them here gives
 * `docs-anchor-parity.test.ts` a list to check against the generated bundle, so a
 * heading that gets reworded fails a test instead of silently becoming a link that
 * lands at the top of the page.
 *
 * Anchors are raw github-slugger output (the same ids `extractToc` puts in the
 * bundle), NOT percent-encoded — `paths.docsPage()` encodes when it builds a URL.
 *
 * P0 ships Chinese content only, so these are the ids of the Chinese headings.
 * The reader has no locale switch to disagree with them yet; when other locales'
 * content lands, this map grows a locale dimension and the parity test with it.
 */

/** Page slugs, matching the file names under `apps/docs/content/docs/`. */
export const DOCS_SLUGS = {
  agents: "agents",
  autopilots: "autopilots",
  daemonRuntimes: "daemon-runtimes",
  dingtalkBot: "dingtalk-bot-integration",
  installAgentRuntime: "install-agent-runtime",
  skills: "skills",
  slackBot: "slack-bot-integration",
  telegramBot: "telegram-bot-integration",
} as const;

/**
 * Headings product code deep-links into.
 *
 * `webhookEventFilters` was `事件过滤` in every locale before this landed, while
 * the heading has always read `过滤事件` — the words in the other order. That link
 * has never resolved on the public docs site either; the parity test now holds it
 * to the real heading.
 */
export const DOCS_ANCHORS = {
  /** autopilots — "过滤事件" */
  webhookEventFilters: "过滤事件",
  /** daemon-runtimes — "自定义运行时配置" */
  customRuntimeProfiles: "自定义运行时配置",
} as const;

export type DocsSlug = (typeof DOCS_SLUGS)[keyof typeof DOCS_SLUGS];
export type DocsAnchor = (typeof DOCS_ANCHORS)[keyof typeof DOCS_ANCHORS];
