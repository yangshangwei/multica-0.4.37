import { isMap, isScalar, parseDocument, visit } from "yaml";
import type { CreateSkillRequest, SkillTemplate } from "../types";
import {
  EMPTY_SKILL_PRESENTATION,
  isSkillCategory,
  isSkillIconName,
  writeSkillPresentationMeta,
} from "./presentation";

export interface SkillTemplateDraft {
  templateName: string;
  templateVersion: number;
  /** Template-declared presentation defaults; unknown values are dropped. */
  templateCategory: string | null;
  templateIcon: string | null;
  sourceContent: string;
  name: string;
  description: string;
  body: string;
  files: { path: string; content: string }[];
}

function parseTemplateContent(content: string) {
  const opening = /^---(\r?\n)/.exec(content);
  if (!opening) {
    throw new Error("Template YAML frontmatter is missing or incomplete.");
  }
  const remainder = content.slice(opening[0].length);
  const closing = /^---(?:\r?\n|$)/m.exec(remainder);
  if (!closing) {
    throw new Error("Template YAML frontmatter is missing or incomplete.");
  }
  const document = parseDocument(remainder.slice(0, closing.index));
  if (document.errors.length > 0 || !isMap(document.contents)) {
    throw new Error("Template YAML frontmatter must be a valid mapping.");
  }
  try {
    // Alias resolution errors are reported when reading values, after parsing.
    document.toJS();
  } catch {
    throw new Error("Template YAML frontmatter contains an invalid reference.");
  }
  return {
    document,
    newline: opening[1]!,
    body: remainder.slice(closing.index + closing[0].length),
  };
}

export function createSkillTemplateDraft(
  template: SkillTemplate,
  existingNames: readonly string[],
  description?: string,
): SkillTemplateDraft {
  if (!template.name.trim()) throw new Error("Template name is required.");
  const { body } = parseTemplateContent(template.content);
  if (!body.trim()) throw new Error("Skill body is required.");

  const names = new Set(existingNames);
  const baseName = `${template.name.trim()}-copy`;
  let name = baseName;
  for (let suffix = 2; names.has(name); suffix += 1) {
    name = `${baseName}-${suffix}`;
  }

  return {
    templateName: template.name,
    templateVersion: template.version,
    templateCategory: isSkillCategory(template.category) ? template.category : null,
    templateIcon: isSkillIconName(template.icon) ? template.icon : null,
    sourceContent: template.content,
    name,
    description: description ?? template.description,
    body,
    files: template.files.map(({ path, content }) => ({ path, content })),
  };
}

export function buildSkillTemplateCreateRequest(draft: SkillTemplateDraft): CreateSkillRequest {
  const name = draft.name.trim();
  if (!name) throw new Error("Skill name is required.");
  if (!draft.body.trim()) throw new Error("Skill body is required.");

  const { document, newline } = parseTemplateContent(draft.sourceContent);
  const editedNodes = new Set([document.get("name", true), document.get("description", true)]);
  visit(document, {
    Alias(_key, node) {
      const source = node.resolve(document);
      // Unknown fields that reference the old metadata keep their old values.
      if (isScalar(source) && editedNodes.has(source)) {
        return document.createNode(source.value);
      }
      return undefined;
    },
  });
  document.set("name", name);
  document.set("description", draft.description);
  const frontmatter = document.toString({ lineWidth: 0 }).replace(/\n/g, newline);

  return {
    name,
    description: draft.description,
    content: `---${newline}${frontmatter}---${newline}${draft.body}`,
    files: draft.files.map(({ path, content }) => ({ path, content })),
    config: writeSkillPresentationMeta(
      { template_source: { name: draft.templateName, version: draft.templateVersion } },
      {
        ...EMPTY_SKILL_PRESENTATION,
        category: isSkillCategory(draft.templateCategory)
          ? draft.templateCategory
          : EMPTY_SKILL_PRESENTATION.category,
        icon: isSkillIconName(draft.templateIcon) ? draft.templateIcon : null,
      },
    ),
  };
}
