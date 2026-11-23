import { AVATAR_ICON_COMPONENTS } from "@multica/ui/lib/avatar-icon";
import { IconTile } from "./icon-tile";

const templateIcons: Record<string, keyof typeof AVATAR_ICON_COMPONENTS> = {
  "chrome-devtools": "bug",
  playwright: "workflow",
  "sequential-thinking": "brain",
  "microsoft-learn": "book-open",
  deepwiki: "search",
  serena: "code",
  "codebase-memory": "workflow",
  repomix: "package",
  markitdown: "file-text",
  dbhub: "database",
  "postgres-mcp": "database",
  gitlab: "git-pull-request",
  atlassian: "book-open",
  grafana: "chart-line",
  kubernetes: "server",
  mongodb: "database",
  redis: "database",
  clickhouse: "database",
};

/** Template identity survives instance renames; never infer it from a name. */
export function McpTemplateIcon({ templateKey, source = "builtin", category }: {
  templateKey: string;
  source?: string | null;
  category?: string;
}) {
  const builtinKey = source === "builtin" ? templateKey : "";
  const reasoning = builtinKey === "sequential-thinking" || category === "reasoning";
  const documentation = builtinKey === "microsoft-learn" || builtinKey === "deepwiki" || builtinKey === "markitdown" || category === "documentation";
  const fallback = reasoning ? "brain" : category === "database" ? "database" : category === "coding" ? "code" : "globe";

  return (
    <IconTile
      icon={AVATAR_ICON_COMPONENTS[templateIcons[builtinKey] ?? fallback]}
      tone={
        documentation
          ? "bg-skill-writing/12 text-skill-writing"
          : reasoning
            ? "bg-skill-research/12 text-skill-research"
            : "bg-skill-engineering/12 text-skill-engineering"
      }
    />
  );
}
