# Implementation contracts

## Catalog

`GET /api/skills/templates` returns `{ templates: SkillTemplate[] }` under the existing authentication and workspace middleware. Each template has `name`, `version`, `description`, `content`, and `files: { path, content }[]`. It has no database ID. Copy creation uses the existing `POST /api/skills`.

## Core exports consumed by views

```ts
interface SkillRequestOptions {
  workspaceId?: string;
  signal?: AbortSignal;
}

interface SkillTemplateDraft {
  templateName: string;
  templateVersion: number;
  sourceContent: string;
  name: string;
  description: string;
  body: string;
  files: { path: string; content: string }[];
}

api.listSkillTemplates(workspaceId: string, signal?: AbortSignal): Promise<SkillTemplate[]>;
api.listSkills(options?: SkillRequestOptions): Promise<SkillSummary[]>;
api.getSkill(id: string, options?: SkillRequestOptions): Promise<Skill>;
api.createSkill(data: CreateSkillRequest, options?: SkillRequestOptions): Promise<Skill>;

skillTemplateListOptions(wsId: string);
createSkillTemplateDraft(template: SkillTemplate, existingNames: readonly string[], description?: string): SkillTemplateDraft;
buildSkillTemplateCreateRequest(draft: SkillTemplateDraft): CreateSkillRequest;
```

Draft helpers are exported from `@multica/core/skills`; the draft type is exported there too. `SkillTemplate` is exported from `@multica/core/types`. Query options live in `@multica/core/workspace/queries`. Existing API calls without options remain supported.

The create builder preserves unknown YAML field types, replaces only `name` and `description`, preserves the edited body, copies supporting files, and sets informational `config.template_source = { name, version }` without official `origin` metadata. It rejects invalid YAML or empty name/body. Canonical-name rejection in the new UI uses the loaded catalog; default candidate generation always uses a `-copy` suffix.

The root dialog owns template session/draft state. Preview selection is independent from the current draft. A response with invalid identity must not be cached or navigated to; use a typed `SkillCreationUnconfirmedError` from `@multica/core/api` for malformed successful creation responses. Network, abort and 5xx failures are also treated as uncertain by the new flow. Ordinary 4xx validation/conflict errors keep the draft for correction.
