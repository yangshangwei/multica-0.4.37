"use client";

import { useRef, useState, type MouseEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ArrowLeft, Check, Loader2, Search } from "lucide-react";
import type { SkillTemplate } from "@multica/core/types";
import { paths } from "@multica/core/paths";
import { parseFrontmatter } from "@multica/core/skills";
import { skillListOptions, skillTemplateListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@multica/ui/components/ui/tabs";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { RichContent } from "../../rich-content";
import { AppLink, resolveClickIntent, useNavigation, type LinkClickIntent } from "../../navigation";
import { useSkillPresentation } from "../hooks/use-skill-presentation";
import { getRelatedWorkspaceSkills, getSkillTemplateDiscoveryItems } from "../lib/skill-template-discovery";
import type { SkillCreationCandidate, TemplateSkillSession } from "../hooks/use-template-skill-session";
import { FileViewer } from "./file-viewer";

const SKILL_MD = "SKILL.md";

export interface RelatedSkillNavigationRequest {
  readonly sourceWorkspaceId: string;
  readonly sourceWorkspaceSlug: string;
  readonly skillId: string;
  readonly path: string;
  readonly title: string;
  readonly intent: LinkClickIntent;
}

interface Props {
  workspaceId: string;
  workspaceSlug: string;
  session: TemplateSkillSession;
  onUseTemplate: (template: SkillTemplate, names: readonly string[], description: string) => void;
  onBack: () => void;
  onOpenCandidate: (candidate: SkillCreationCandidate) => void;
  onRequestOpenSkill: (request: RelatedSkillNavigationRequest) => void;
}

export function TemplateSkillCreatePanel({ workspaceId, workspaceSlug, session, onUseTemplate, onBack, onOpenCandidate, onRequestOpenSkill }: Props) {
  const { t } = useT("skills");
  const catalog = useQuery(skillTemplateListOptions(workspaceId));
  const skills = useQuery(skillListOptions(workspaceId));
  const [search, setSearch] = useState("");
  const [chosenSource, setChosenSource] = useState<"builtin" | "deployment" | null>(null);
  const navigation = useNavigation();
  const presentSkill = useSkillPresentation();
  const [mobilePreview, setMobilePreview] = useState(false);
  const [filePath, setFilePath] = useState(SKILL_MD);
  const listPanel = useRef<HTMLDivElement>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  const selectedRow = useRef<HTMLButtonElement | null>(null);
  const useTemplateButton = useRef<HTMLButtonElement>(null);
  const items = getSkillTemplateDiscoveryItems(catalog.data ?? [], t);
  const templates = items.map(({ template }) => template);
  const query = search.trim().toLowerCase();
  const filtered = items.filter(({ presentation }) => presentation.searchText.includes(query));
  const builtinGroup = filtered.filter(({ source }) => source === "builtin");
  const deploymentGroup = filtered.filter(({ source }) => source === "deployment");
  const hasBuiltinTemplates = items.some(({ source }) => source === "builtin");
  const seed = items.find(({ template }) => template.name === session.previewName);
  const sourceTab = chosenSource ?? seed?.source ?? (hasBuiltinTemplates ? "builtin" : "deployment");
  const visibleItems = sourceTab === "builtin" ? builtinGroup : deploymentGroup;
  const selected = visibleItems.find(({ template }) => template.name === session.previewName) ?? visibleItems[0];
  const hasDeploymentTemplates = items.some(({ source }) => source === "deployment");
  const hasCatalogData = catalog.data !== undefined;
  const catalogLoading = !hasCatalogData && catalog.isPending;
  const catalogFailed = !hasCatalogData && catalog.isError;
  const related = selected ? getRelatedWorkspaceSkills(selected.template.name, skills.data ?? [], presentSkill) : [];

  const requestOpenSkill = (event: MouseEvent<HTMLAnchorElement>, skillId: string, title: string) => {
    if (event.defaultPrevented || (event.type === "auxclick" && event.button !== 1)) return;
    const intent = resolveClickIntent(event);
    // Web owns modified native anchors. Every Desktop adapter intent can
    // activate an existing tab and unmount this dialog, including middle-click.
    if (!navigation.openInNewTab && (intent !== "push" || event.shiftKey)) return;
    event.preventDefault();
    event.currentTarget.focus();
    onRequestOpenSkill(Object.freeze({
      sourceWorkspaceId: workspaceId,
      sourceWorkspaceSlug: workspaceSlug,
      skillId,
      path: paths.workspace(workspaceSlug).skillDetail(skillId),
      title,
      intent,
    }));
  };
  const renderTemplateRow = ({ template, presentation, summary }: (typeof filtered)[number]) => {
    const active = template.name === selected?.template.name;
    return (
      <button
        key={template.name}
        type="button"
        aria-pressed={active}
        onClick={(event) => {
          selectedRow.current = event.currentTarget;
          session.preview(template.name);
          setMobilePreview(true);
          requestAnimationFrame(() => {
            if (listPanel.current && getComputedStyle(listPanel.current).display === "none") {
              useTemplateButton.current?.focus();
            }
          });
        }}
        className={cn(
          "flex w-full items-start gap-2 rounded-md px-3 py-3 text-left transition-colors hover:bg-accent/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          active && "bg-accent text-foreground hover:bg-accent",
        )}
      >
        <span className="min-w-0 flex-1">
          <span className={cn("block break-words text-body", active ? "font-semibold" : "font-medium")}>{presentation.name}</span>
          <span className="mt-1 line-clamp-2 text-caption text-muted-foreground">{summary}</span>
        </span>
        {active && <Check className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />}
      </button>
    );
  };
  const error = session.error;
  const showError = error ? (
    <div role="alert" className="flex items-start gap-2 rounded-md bg-destructive/10 p-3 text-caption text-destructive">
      <AlertCircle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span>{t(($) => $.create.template[error])}</span>
    </div>
  ) : null;

  if (session.step === "picker" || !session.draft) {
    const unavailable = catalogFailed || (!catalogLoading && templates.length === 0);
    return (
      <>
        {hasCatalogData && (catalog.isError || catalog.isFetching) && (
          <div role={catalog.isError ? "alert" : "status"} className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b px-5 py-2 text-caption text-muted-foreground">
            <span>{catalog.isError ? t(($) => $.create.template.refresh_failed) : t(($) => $.create.template.refreshing)}</span>
            {catalog.isError && <Button variant="ghost" size="sm" onClick={() => void catalog.refetch()}>{t(($) => $.create.template.retry)}</Button>}
          </div>
        )}
        {catalogLoading || unavailable ? (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 overflow-y-auto p-6 text-center">
            {catalogLoading ? (
              <p role="status" className="flex items-center gap-2 text-body text-muted-foreground">
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                {t(($) => $.create.template.loading)}
              </p>
            ) : (
              <>
                <p role={catalogFailed ? "alert" : "status"} className="max-w-md text-body text-muted-foreground">
                  {catalogFailed ? t(($) => $.create.template.load_failed) : t(($) => $.create.template.empty)}
                </p>
                {catalogFailed && <Button variant="outline" size="sm" onClick={() => void catalog.refetch()}>
                  {t(($) => $.create.template.retry)}
                </Button>}
              </>
            )}
          </div>
        ) : (
          <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[250px_minmax(0,1fr)]">
            <div ref={listPanel} className={cn("min-h-0 flex-col md:flex md:border-r", mobilePreview ? "hidden" : "flex")}>
              <div className="relative shrink-0 p-3">
                <Search className="pointer-events-none absolute left-5.5 top-5.5 size-4 text-muted-foreground" aria-hidden="true" />
                <Input
                  ref={searchInput}
                  autoFocus
                  aria-label={t(($) => $.create.template.search_placeholder)}
                  placeholder={t(($) => $.create.template.search_placeholder)}
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  className={cn("pl-8", query && "pr-24")}
                />
                {query && <Button variant="ghost" size="sm" className="absolute right-4 top-4 h-6" onClick={() => { setSearch(""); searchInput.current?.focus(); }}>{t(($) => $.create.template.clear_search)}</Button>}
              </div>
              <Tabs
                value={sourceTab}
                onValueChange={(value) => { setChosenSource(value === "deployment" ? "deployment" : "builtin"); setMobilePreview(false); }}
                className="min-h-0 flex-1 gap-2"
              >
                <TabsList className="mx-3 min-h-8 w-auto shrink-0 items-stretch self-stretch group-data-horizontal/tabs:h-auto">
                  <TabsTrigger value="builtin" className="h-auto min-w-0 flex-1 gap-1.5 whitespace-normal text-center">
                    <span>{t(($) => $.create.template.group_builtin)}</span>
                    <span className="text-caption tabular-nums text-muted-foreground">{builtinGroup.length}</span>
                  </TabsTrigger>
                  <TabsTrigger value="deployment" className="h-auto min-w-0 flex-1 gap-1.5 whitespace-normal text-center">
                    <span>{t(($) => $.create.template.group_deployment_label)}</span>
                    <span className="text-caption tabular-nums text-muted-foreground">{deploymentGroup.length}</span>
                  </TabsTrigger>
                </TabsList>
                <TabsContent
                  value="builtin"
                  aria-label={t(($) => $.create.template.group_builtin)}
                  className="min-h-0 flex-1 space-y-1 overflow-y-auto px-2 pb-3"
                >
                  {builtinGroup.length > 0 ? (
                    builtinGroup.map(renderTemplateRow)
                  ) : (
                    <p className="px-3 py-5 text-caption text-muted-foreground">{hasBuiltinTemplates && query ? t(($) => $.create.template.no_matches) : t(($) => $.create.template.source_empty)}</p>
                  )}
                </TabsContent>
                <TabsContent
                  value="deployment"
                  aria-label={t(($) => $.create.template.group_deployment_label)}
                  className="min-h-0 flex-1 space-y-1 overflow-y-auto px-2 pb-3"
                >
                  <p className="px-3 pb-1 pt-1 text-caption leading-relaxed text-muted-foreground">
                    {hasDeploymentTemplates
                      ? t(($) => $.create.template.group_deployment_info)
                      : t(($) => $.create.template.deployment_empty_hint)}
                  </p>
                  {deploymentGroup.length > 0
                    ? deploymentGroup.map(renderTemplateRow)
                    : hasDeploymentTemplates && (
                        <p className="px-3 py-5 text-caption text-muted-foreground">{t(($) => $.create.template.no_matches)}</p>
                      )}
                </TabsContent>
              </Tabs>
            </div>
            <div className={cn("min-h-0 flex-col md:flex", mobilePreview ? "flex" : "hidden")}>
              <div className="shrink-0 px-4 pt-3 md:hidden">
                <Button variant="ghost" size="sm" onClick={() => {
                  setMobilePreview(false);
                  requestAnimationFrame(() => {
                    if (selectedRow.current?.isConnected) selectedRow.current.focus();
                    else searchInput.current?.focus();
                  });
                }}>
                  <ArrowLeft className="size-3.5" aria-hidden="true" />{t(($) => $.create.template.back_to_templates)}
                </Button>
              </div>
              <div aria-label={t(($) => $.create.template.preview_label)} className="min-h-0 flex-1 overflow-y-auto">
                <div className="mx-auto max-w-[68ch] px-5 py-5">
                  {selected ? (
                    <>
                      <div className="mb-6 space-y-3">
                        <h2 className="break-words text-title-sm font-semibold">{selected.presentation.name}</h2>
                        <p className="whitespace-pre-wrap break-words text-body text-muted-foreground">{selected.presentation.description}</p>
                        <div className="space-y-2 text-caption">
                          {skills.data === undefined ? (
                            <div role={skills.isError ? "alert" : "status"} className="flex flex-wrap items-center gap-2 text-muted-foreground">
                              <span>{skills.isError ? t(($) => $.create.template.related_load_failed) : t(($) => $.create.template.related_loading)}</span>
                              {skills.isError && <Button variant="ghost" size="sm" onClick={() => void skills.refetch()}>{t(($) => $.create.template.retry)}</Button>}
                            </div>
                          ) : (
                            <>
                              <p className="text-muted-foreground">{related.length === 0 ? t(($) => $.create.template.related_none) : t(($) => $.create.template.related_count, { count: related.length })}</p>
                              {related.length > 0 && <ul className="space-y-2">
                                {related.map((skill) => {
                                  const presentation = presentSkill(skill);
                                  return <li key={skill.id} className="min-w-0">
                                    <AppLink
                                      href={paths.workspace(workspaceSlug).skillDetail(skill.id)}
                                      newTabTitle={presentation.name}
                                      className="inline-block max-w-full break-words rounded-sm text-foreground underline underline-offset-4 hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                                      onClick={(event) => requestOpenSkill(event, skill.id, presentation.name)}
                                      onAuxClick={(event) => requestOpenSkill(event, skill.id, presentation.name)}
                                    >{presentation.name}</AppLink>
                                    {presentation.name !== skill.name && <span className="ml-2 break-words text-muted-foreground">{skill.name}</span>}
                                  </li>;
                                })}
                              </ul>}
                              {skills.isError && <div role="alert" className="flex flex-wrap items-center gap-2 text-muted-foreground">
                                <span>{t(($) => $.create.template.related_refresh_failed)}</span>
                                <Button variant="ghost" size="sm" onClick={() => void skills.refetch()}>{t(($) => $.create.template.retry)}</Button>
                              </div>}
                            </>
                          )}
                        </div>
                      </div>
                      <RichContent content={parseFrontmatter(selected.template.content).body} density="document" phase="settled" />
                    </>
                  ) : (
                    <p className="text-body text-muted-foreground">{query ? t(($) => $.create.template.no_matches) : t(($) => $.create.template.source_empty)}</p>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
        {showError && <div className="shrink-0 px-5 pb-3">{showError}</div>}
        <div className="flex shrink-0 flex-col gap-3 border-t bg-muted/30 px-5 py-3 md:flex-row md:items-center md:justify-between">
          <p className="min-w-0 text-caption text-muted-foreground md:flex-1">{t(($) => $.create.template.independent_hint)}</p>
          <div className="flex shrink-0 justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={onBack}>{t(($) => $.create.back)}</Button>
            <Button ref={useTemplateButton} size="sm" disabled={!selected || catalogLoading || unavailable || session.busy} onClick={() => {
              if (!selected) return;
              setFilePath(SKILL_MD);
              onUseTemplate(selected.template, (skills.data ?? []).map((skill) => skill.name), selected.presentation.description);
            }}>{t(($) => $.create.template.use_template)}</Button>
          </div>
        </div>
      </>
    );
  }

  const draft = session.draft;
  const sourceName = items.find(({ template }) => template.name === draft.templateName)?.presentation.name ?? draft.templateName;
  const reservedNames = templates.map((template) => template.name);
  const nameError = !draft.name.trim() ? "name_required" : reservedNames.includes(draft.name.trim()) ? "reserved_name" : null;
  const uncertain = session.status === "uncertain" || session.status === "checking";
  const recoveryError = error === "timeout_unknown" || error === "no_confirmed_result" || error === "result_unknown" ? error : "result_unknown";
  const disabled = session.busy || uncertain;
  const selectedFile = draft.files.find((file) => file.path === filePath);
  const mainFile = !selectedFile;

  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="space-y-4 px-5 py-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-caption text-muted-foreground">{t(($) => $.create.template.source_template, { name: sourceName })}</p>
            <Button variant="ghost" size="sm" disabled={session.busy} onClick={onBack}>
              {t(($) => $.create.template.choose_another)}
            </Button>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="template-skill-name" className="text-caption text-muted-foreground">{t(($) => $.create.manual.name_label)}</Label>
              <Input id="template-skill-name" autoFocus value={draft.name} disabled={disabled} aria-invalid={!!nameError || error === "conflict"}
                onChange={(event) => session.updateDraft("name", event.target.value)} />
              <p className={cn("text-caption", nameError ? "text-destructive" : "text-muted-foreground")}>
                {nameError ? t(($) => $.create.template[nameError]) : t(($) => $.create.template.name_hint)}
              </p>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="template-skill-description" className="text-caption text-muted-foreground">{t(($) => $.create.manual.description_label)}</Label>
              <Textarea id="template-skill-description" rows={3} value={draft.description} disabled={disabled} className="resize-none"
                onChange={(event) => session.updateDraft("description", event.target.value)} />
            </div>
          </div>
          {session.hasUnconfirmedSubmission ? (
            <>
            {error && error !== recoveryError && showError}
            <div className="space-y-3 rounded-md border bg-muted/30 p-3">
              <p role="alert" className="text-caption">{t(($) => $.create.template[recoveryError])}</p>
              <div className="flex flex-wrap gap-2">
                <Button variant="outline" size="sm" disabled={session.busy} onClick={() => void session.checkResult()}>
                  {session.status === "checking" && <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />}
                  {session.status === "checking" ? t(($) => $.create.template.checking) : t(($) => $.create.template.check_result)}
                </Button>
                <Button variant="ghost" size="sm" disabled={session.busy} onClick={session.continueEditing}>{t(($) => $.create.template.continue_editing)}</Button>
              </div>
              <p className="text-caption text-muted-foreground">{t(($) => $.create.template.retry_hint)}</p>
              {session.candidates.map((candidate) => (
                <div key={candidate.skill.id} className="flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                  <div className="min-w-0 flex-1">
                    <p className="break-words text-body font-medium">{candidate.skill.name}</p>
                    <p className="mt-1 text-caption text-muted-foreground">{candidate.matches ? t(($) => $.create.template.matching_candidate) : t(($) => $.create.template.different_candidate)}</p>
                  </div>
                  <Button variant="outline" size="sm" onClick={() => onOpenCandidate(candidate)}>{t(($) => $.create.template.open_skill)}</Button>
                </div>
              ))}
            </div>
            </>
          ) : showError}
          <div className="space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Label htmlFor="template-skill-body" className="text-caption text-muted-foreground">{t(($) => $.create.template.body_label)}</Label>
              <div role="group" aria-label={t(($) => $.create.template.view_label)} className="flex gap-1">
                <Button variant="ghost" size="sm" aria-pressed={session.contentMode === "edit"} onClick={() => session.setContentMode("edit")}
                  className={cn(session.contentMode === "edit" && "bg-accent font-semibold")}>{t(($) => $.detail.files.mode_edit)}</Button>
                <Button variant="ghost" size="sm" aria-pressed={session.contentMode === "preview"} onClick={() => session.setContentMode("preview")}
                  className={cn(session.contentMode === "preview" && "bg-accent font-semibold")}>{t(($) => $.detail.files.mode_preview)}</Button>
              </div>
            </div>
            {draft.files.length > 0 && (
              <div role="group" aria-label={t(($) => $.create.template.included_files)} className="flex gap-1 overflow-x-auto pb-1">
                <Button variant="ghost" size="sm" aria-pressed={mainFile} onClick={() => setFilePath(SKILL_MD)}>{SKILL_MD}</Button>
                {draft.files.map((file) => <Button key={file.path} variant="ghost" size="sm" aria-pressed={selectedFile?.path === file.path} onClick={() => setFilePath(file.path)}>{file.path}</Button>)}
              </div>
            )}
            {selectedFile ? (
              <div className="h-64 overflow-hidden rounded-md border"><FileViewer path={selectedFile.path} content={selectedFile.content} mode="raw" readOnly onChange={() => {}} /></div>
            ) : session.contentMode === "preview" ? (
              <div className="min-h-56 rounded-md border p-4"><RichContent content={draft.body} density="document" phase="settled" /></div>
            ) : (
              <Textarea id="template-skill-body" aria-label={t(($) => $.create.template.body_label)} rows={12} value={draft.body} disabled={disabled}
                onChange={(event) => session.updateDraft("body", event.target.value)} className="min-h-56 resize-y font-mono text-body leading-relaxed" />
            )}
            {!draft.body.trim() && <p className="text-caption text-destructive">{t(($) => $.create.template.body_required)}</p>}
          </div>
        </div>
      </div>
      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button variant="ghost" size="sm" disabled={session.busy} onClick={onBack}>{t(($) => $.create.back)}</Button>
        <Button size="sm" disabled={disabled || !!nameError || !draft.body.trim()} onClick={() => void session.submit(reservedNames)}>
          {session.status === "submitting" && <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />}
          {session.status === "submitting" ? t(($) => $.create.template.creating) : t(($) => $.create.template.create)}
        </Button>
      </div>
    </>
  );
}
