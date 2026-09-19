/**
 * A built-in MCP server template: one of the mainstream MCP servers a workspace
 * can add with one click.
 *
 * Selecting a template pre-fills the ordinary "add MCP server" form; saving
 * produces an ORDINARY workspace MCP server — write-only, renameable, and still
 * unassigned to any agent until an owner attaches it. Unlike the write-only
 * library, `config` is served in full because these templates are public,
 * credential-free content the workspace has not yet adopted.
 *
 * There is no `version` and no provenance stamp: the add form lets a user edit
 * the config before saving, so recording "came from template X" would be
 * dishonest. The transport is derived from `config` on the client, so it is not
 * a field here.
 */
export interface McpServerTemplate {
  /** Stable identity and default server name. Matches `^[A-Za-z0-9_-]+$`. */
  key: string;
  /** Localized catalog label. Follows the requested language. */
  title: string;
  /** Localized catalog description. */
  description: string;
  /** The raw MCP server entry copied verbatim into the add form. */
  config: Record<string, unknown>;
}
