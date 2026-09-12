"use client";

import { useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle, ArrowLeft, Check, Loader2, Search } from "lucide-react";
import type { SkillTemplate } from "@multica/core/types";
import { parseFrontmatter } from "@multica/core/skills";
import { skillListOptions, skillTemplateListOptions } from "@multica/core/workspace/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import { RichContent } from "../../rich-content";
import { getBuiltinRoleSkillPresentation } from "../lib/skill-presentation";
import type { SkillCreationCandidate, TemplateSkillSession } from "../hooks/use-template-skill-session";
import { FileViewer } from "./file-viewer";

const SKILL_MD = "SKILL.md";

interface Props {
  workspaceId: string;
  session: TemplateSkillSession;
  onUseTemplate: (template: SkillTemplate, names: readonly string[], description: string) => void;
  onBack: () => void;
  onOpenCandidate: (candidate: SkillCreationCandidate) => void;
}

export function TemplateSkillCreatePanel({ workspaceId, session, onUseTemplate, onBack, onOpenCandidate }: Props) {
  const { t } = useT("skills");
  const catalog = useQuery(skillTemplateListOptions(workspaceId));
  const skills = useQuery(skillListOptions(workspaceId));
  const [search, setSearch] = useState("");
  const [mobilePreview, setMobilePreview] = useState(false);
  const [filePath, setFilePath] = useState(SKILL_MD);
  const listPanel = useRef<HTMLDivElement>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  const selectedRow = useRef<HTMLButtonElement | null>(null);
  const useTemplateButton = useRef<HTMLButtonElement>(null);
  const templates = catalog.data ?? [];
  const presentations = templates.map((template) => ({
    template,
    presentation: getBuiltinRoleSkillPresentation(template.name, t, template.description) ?? {
      name: template.name,
      description: template.description,
      searchText: `${template.name}\n${template.description}`.toLowerCase(),
    },
  }));
  const query = search.trim().toLowerCase();
  const filtered = presentations.filter(({ presentation }) => presentation.searchText.includes(query));
  const selected = filtered.find(({ template }) => template.name === session.previewName) ?? filtered[0];
  const error = session.error;
  const showError = error ? (
    <div role="alert" className="flex items-start gap-2 rounded-md bg-destructive/10 p-3 text-caption text-destructive">
      <AlertCircle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span>{t(($) => $.create.template[error])}</span>
    </div>
  ) : null;

  if (session.step === "picker" || !session.draft) {
    const unavailable = catalog.isError || (!catalog.isPending && templates.length === 0);
    return (
      <>
        {catalog.isPending || unavailable ? (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 overflow-y-auto p-6 text-center">
            {catalog.isPending ? (
              <p role="status" className="flex items-center gap-2 text-body text-muted-foreground">
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                {t(($) => $.create.template.loading)}
              </p>
            ) : (
              <>
                <p role="alert" className="max-w-md text-body text-muted-foreground">
                  {catalog.isError ? t(($) => $.create.template.load_failed) : t(($) => $.create.template.empty)}
                </p>
                <Button variant="outline" size="sm" onClick={() => void catalog.refetch()}>
                  {t(($) => $.create.template.retry)}
                </Button>
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
                  className="pl-8"
                />
              </div>
              <div aria-label={t(($) => $.create.template.list_label)} className="min-h-0 flex-1 space-y-1 overflow-y-auto px-2 pb-3">
                {filtered.length === 0 ? (
                  <p className="px-3 py-5 text-caption text-muted-foreground">{t(($) => $.create.template.no_matches)}</p>
                ) : filtered.map(({ template, presentation }) => {
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
                        active && "bg-accent text-foreground",
                      )}
                    >
                      <span className="min-w-0 flex-1">
                        <span className={cn("block break-words text-body", active ? "font-semibold" : "font-medium")}>{presentation.name}</span>
                        <span className="mt-1 line-clamp-2 text-caption text-muted-foreground">{presentation.description}</span>
                      </span>
                      {active && <Check className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />}
                    </button>
                  );
                })}
              </div>
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
                  {selected ? <RichContent content={parseFrontmatter(selected.template.content).body} density="document" phase="settled" /> : (
                    <p className="text-body text-muted-foreground">{t(($) => $.create.template.no_matches)}</p>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
        {showError && <div className="shrink-0 px-5 pb-3">{showError}</div>}
        <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t bg-muted/30 px-5 py-3">
          <p className="min-w-0 flex-1 text-caption text-muted-foreground">{t(($) => $.create.template.independent_hint)}</p>
          <div className="flex shrink-0 gap-2">
            <Button variant="ghost" size="sm" onClick={onBack}>{t(($) => $.create.back)}</Button>
            <Button ref={useTemplateButton} size="sm" disabled={!selected || catalog.isPending || unavailable || session.busy} onClick={() => {
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
  const sourceName = presentations.find(({ template }) => template.name === draft.templateName)?.presentation.name ?? draft.templateName;
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
            <Button variant="ghost" size="sm" disabled={session.busy} onClick={session.backToTemplates}>
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
