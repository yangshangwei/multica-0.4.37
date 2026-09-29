/** Public, credential-free configuration recipe. Created copies stay unassigned. */
export interface McpServerTemplate {
  /** Stable catalog identity and default server name. */
  key: string;
  title: string;
  description: string;
  config: Record<string, unknown>;
  /** Recipe revision; it does not pin an upstream executable release. */
  version?: string;
  category?: string;
  requirements?: string[];
  documentationUrl?: string;
}
