"use client";

import { use } from "react";
import { DocsPage } from "@multica/views/docs";

// A catch-all rather than `[page]` because docs slugs can be nested: the bundle
// publishes `developers/contributing` as one page, and a single dynamic segment
// would 404 on it. Next decodes each segment already, so they are joined back
// into the slug as-is — decoding again would corrupt a slug containing a `%`.
export default function DocsPageRoute({
  params,
}: {
  params: Promise<{ page: string[] }>;
}) {
  const { page } = use(params);
  return <DocsPage slug={page.join("/")} />;
}
