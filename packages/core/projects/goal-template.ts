export interface ProjectGoalSection { id: string; title: string; body: string }
/** Templates append only explicitly selected, missing headings. */
export function projectGoalTemplateAppend(markdown: string, sections: ProjectGoalSection[], selected: string[]): string {
  const headings = new Set(markdown.split(/\r?\n/).map((line) => /^#{1,6}\s+(.+?)\s*#*\s*$/.exec(line)?.[1]?.toLowerCase()).filter(Boolean));
  return sections.filter((section) => selected.includes(section.id) && !headings.has(section.title.toLowerCase()))
    .map((section) => `## ${section.title}\n\n${section.body}`).join("\n\n");
}
