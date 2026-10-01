# Official source verification — 2026-10-02

Both candidates were checked against official documentation and anonymous MCP
`initialize` / `tools/list` requests (protocol 2025-03-26). Both requests returned
HTTP 200 with text/event-stream from each endpoint; three public tools were
listed by each. No tools, indexing requests, credentials or packages were used.
This is point-in-time protocol evidence, not a runtime connection guarantee.

## Microsoft Learn

- Endpoint: `https://learn.microsoft.com/api/mcp`.
- Official overview: https://learn.microsoft.com/en-us/training/support/mcp
  — “There's no authentication required to access the Microsoft Learn MCP Server.”
  — “The MCP server contains publicly available documentation, not training or user profile information.”
- Official configuration: https://learn.microsoft.com/en-us/training/support/mcp-developer-reference
  — `{"type":"http","url":"https://learn.microsoft.com/api/mcp"}`.
- Requires a Streamable HTTP client and network access to the endpoint; Microsoft
  Learn Terms of Use apply. Supports official documentation and code examples.
- Listed tools: microsoft_docs_search, microsoft_code_sample_search,
  microsoft_docs_fetch. The provider says clients should discover tools at each
  initialization; the recipe does not freeze tool names.

## DeepWiki

- Endpoint: `https://mcp.deepwiki.com/mcp`.
- Official configuration: https://docs.devin.ai/work-with-devin/deepwiki-mcp
  (Markdown: https://docs.devin.ai/work-with-devin/deepwiki-mcp.md)
  — “The DeepWiki MCP server is a free, remote, no-authentication-required service
  that provides access to public repositories.”
  — Streamable HTTP `/mcp` is “Recommended for most integrations”; `/sse` is deprecated.
- Content scope: https://docs.devin.ai/work-with-devin/deepwiki
  — “Public DeepWiki and the DeepWiki MCP provide basic documentation and Q&A capabilities.”
  — Public repositories can be submitted for indexing separately.
- Requires a Streamable HTTP client, network access, and an indexed public GitHub
  repository. Private repositories require a different authenticated Devin MCP
  service and are outside this recipe.
- Live tools: ask_wiki_question, read_wiki_contents, read_wiki_structure. The docs
  still call the first ask_question; describe capabilities, not tool names.
  Some initialization text mentions private-only tools; they were not exposed by
  anonymous tools/list and are not part of this entry.

Neither source promises an unlimited rate quota. Recipe revision 1 versions only
our endpoint/configuration recipe, not either hosted provider's implementation.
