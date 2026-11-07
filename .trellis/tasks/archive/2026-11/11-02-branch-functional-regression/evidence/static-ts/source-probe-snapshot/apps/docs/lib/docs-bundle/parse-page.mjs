// Pure parsing helpers for the docs bundle generator: frontmatter, table of
// contents, and the nav tree read off meta.<lang>.json.
//
// Kept free of filesystem access so the shapes that actually break rendering —
// a page with no title, a heading id that drifts from the anchor a product link
// points at, a nav row with no page behind it — are unit-testable.

import GithubSlugger from "github-slugger";

const FENCE_OPEN = /^[ \t]*(`{3,}|~{3,})/;

export function parseFrontmatter(source) {
  const match = /^---\r?\n([\s\S]*?)\r?\n---[ \t]*\r?\n?/.exec(source);
  if (!match) {
    throw new Error("docs page has no frontmatter block");
  }

  const fields = new Map();
  for (const line of match[1].split(/\r?\n/)) {
    const field = /^([A-Za-z_][\w-]*)\s*:\s*(.*)$/.exec(line);
    if (!field) continue;
    fields.set(field[1], unquote(field[2].trim()));
  }

  const title = fields.get("title");
  if (!title) {
    throw new Error("docs page frontmatter has no title");
  }

  const description = fields.get("description") || undefined;
  return {
    title,
    description,
    body: source.slice(match[0].length).trim(),
  };
}

function unquote(value) {
  if (value.length >= 2 && /^["']/.test(value) && value.at(-1) === value[0]) {
    return value.slice(1, -1);
  }
  return value;
}

// Heading ids must match what the docs site produces, because product code deep
// links into them (`#自定义运行时配置` from runtime-docs.ts, for one). Using the
// same github-slugger fumadocs uses is what keeps the two in step; the parity
// test is what proves it.
export function extractToc(body) {
  const slugger = new GithubSlugger();
  const toc = [];
  let fence = null;

  for (const line of body.split(/\r?\n/)) {
    if (fence) {
      const close = new RegExp(`^[ \\t]*${escapeRegex(fence[0])}{${fence.length},}[ \\t]*$`);
      if (close.test(line)) fence = null;
      continue;
    }

    const openingFence = FENCE_OPEN.exec(line);
    if (openingFence) {
      fence = openingFence[1];
      continue;
    }

    const heading = /^(#{2,3})\s+(.+?)\s*$/.exec(line);
    if (!heading) continue;

    const title = stripInlineMarkdown(heading[2]);
    toc.push({ depth: heading[1].length, title, id: slugger.slug(title) });
  }

  return toc;
}

function escapeRegex(text) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function stripInlineMarkdown(text) {
  return text
    .replace(/`([^`]+)`/g, "$1")
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/\*([^*]+)\*/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .trim();
}

// meta.<lang>.json encodes groups as pseudo-entries: "---开始使用---" opens a
// labelled section and every following slug belongs to it. Pages listed before
// the first separator keep a null label rather than being dropped.
//
// An entry can also name a subdirectory carrying its own meta file (`developers`
// is the only one today). `directories` maps such a name to that meta, and the
// directory expands into a group of its own labelled with its meta title, with
// slugs prefixed so `contributing` becomes `developers/contributing`.
export function buildNav(meta, pages, directories = new Map()) {
  if (!Array.isArray(meta?.pages)) {
    throw new Error("meta file has no pages array");
  }

  const groups = [];
  let current = null;
  const listed = new Set();

  for (const entry of meta.pages) {
    const separator = /^---(.*)---$/.exec(entry);
    if (separator) {
      current = { label: separator[1], items: [] };
      groups.push(current);
      continue;
    }

    const directory = directories.get(entry);
    if (directory) {
      // A separator immediately before a directory entry is labelling that
      // directory, not opening a section of its own — meta.zh.json ends with
      // "---参与开发---" followed by `developers`. Filling the open group keeps
      // one labelled group instead of an empty one plus a duplicate label.
      const reuseOpenGroup = current !== null && current.items.length === 0;
      const nested = reuseOpenGroup ? current : { label: directory.title ?? entry, items: [] };
      for (const child of directory.pages ?? []) {
        const slug = `${entry}/${child}`;
        const childPage = pages.get(slug);
        if (!childPage) {
          throw new Error(
            `meta lists "${slug}" but no page was generated for it — add the page or remove the nav entry`,
          );
        }
        listed.add(slug);
        nested.items.push({
          slug,
          title: childPage.title,
          ...(childPage.description ? { description: childPage.description } : {}),
        });
      }
      if (!reuseOpenGroup) groups.push(nested);
      current = nested;
      continue;
    }

    const page = pages.get(entry);
    if (!page) {
      throw new Error(
        `meta lists "${entry}" but no page was generated for it — add the page or remove the nav entry`,
      );
    }

    if (!current) {
      current = { label: null, items: [] };
      groups.push(current);
    }
    listed.add(entry);
    current.items.push({
      slug: entry,
      title: page.title,
      ...(page.description ? { description: page.description } : {}),
    });
  }

  const orphans = [...pages.keys()].filter((slug) => !listed.has(slug));
  if (orphans.length > 0) {
    throw new Error(
      `generated pages missing from meta: ${orphans.join(", ")} — they would be unreachable in the sidebar`,
    );
  }

  return { title: meta.title ?? null, groups };
}
