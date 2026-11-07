import type { McpServerTemplate, WorkspaceMcpServer } from "@multica/core/types";

export type McpCatalogSource = "all" | "deployment" | "builtin";

export function mcpTemplateIdentity(template: McpServerTemplate): string {
  return `${template.source ?? "builtin"}:${template.key}`;
}

export function matchesMcpTemplate(
  server: Pick<WorkspaceMcpServer, "template_key" | "template_source">,
  template: McpServerTemplate,
): boolean {
  return server.template_key === template.key &&
    (server.template_source === undefined ? "builtin" : server.template_source) === (template.source ?? "builtin");
}

export function mcpTemplateTransport(template: McpServerTemplate): string {
  if (template.transport) return template.transport;
  if ((template.source ?? "builtin") !== "builtin") return "";
  return typeof template.config?.command === "string" ? "stdio" :
    typeof template.config?.url === "string" ? "http" : "";
}

/** Unknown source capabilities must not reach a custom-configuration fallback. */
export function isUsableMcpTemplate(template: McpServerTemplate): boolean {
  if (!template.key) return false;
  switch (template.source ?? "builtin") {
    case "builtin":
      return Object.keys(template.config ?? {}).length > 0;
    case "deployment":
      return typeof template.version === "string" && template.version.length > 0 &&
        (template.transport === "stdio" || template.transport === "http");
    default:
      return false;
  }
}

export function filterMcpTemplates(
  templates: readonly McpServerTemplate[],
  source: McpCatalogSource,
  category: string,
  search: string,
): McpServerTemplate[] {
  const query = search.trim().toLocaleLowerCase();
  return templates.filter((template) =>
    (source === "all" || (template.source ?? "builtin") === source) &&
    (category === "all" || template.category === category) &&
    `${template.key} ${template.title} ${template.description}`.toLocaleLowerCase().includes(query),
  );
}
