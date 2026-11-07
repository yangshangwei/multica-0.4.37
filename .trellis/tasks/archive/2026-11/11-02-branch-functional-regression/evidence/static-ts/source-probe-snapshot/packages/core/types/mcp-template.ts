export interface McpTemplateInput {
  key: string;
  label: string;
  description: string;
  required: boolean;
  secret: boolean;
}

/** Public recipe and input definitions, never user values. Created copies stay unassigned. */
export interface McpServerTemplate {
  /** Together with source, identifies a recipe and its default server name. */
  key: string;
  /** Missing on older catalogs means builtin; unknown sources are not actionable. */
  source?: string;
  /** Deployment recipes expose transport instead of their private config. */
  transport?: string;
  title: string;
  description: string;
  /** Public builtin configuration; normalized to an empty object for deployment. */
  config: Record<string, unknown>;
  /** Recipe revision; it does not pin an upstream executable release. */
  version?: string;
  category?: string;
  requirements?: string[];
  documentationUrl?: string;
  inputs?: McpTemplateInput[];
}
