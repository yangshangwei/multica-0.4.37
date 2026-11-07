"use client";

import { DocsPage } from "@multica/views/docs";

// Next validates a page's default export against the page-props shape at
// build time, and the shared DocsPage's `{ slug?: string }` props do not
// qualify — wrap it instead of re-exporting it, like the catch-all route does.
export default function DocsIndexRoute() {
  return <DocsPage />;
}
