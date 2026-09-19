"use client";

import { useEffect, useRef, useState, type ChangeEvent } from "react";
import {
  AlertCircle,
  ArrowLeft,
  ChevronRight,
  Download,
  FileArchive,
  FolderOpen,
  HardDrive,
  LayoutTemplate,
  Loader2,
  Pencil,
  Plus,
  X as XIcon,
} from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type { Skill } from "@multica/core/types";
import {
  EMPTY_SKILL_PRESENTATION,
  prepareSkillArchiveFromPickerFiles,
  wrapExistingSkillArchive,
  writeSkillPresentationMeta,
  type PreparedSkillArchive,
  type SkillPresentationMeta,
} from "@multica/core/skills";
import { useWorkspaceId } from "@multica/core/hooks";
import { isImeComposing } from "@multica/core/utils";
import { cacheSkillResponse as seedAfterCreate } from "@multica/core/workspace/queries";
import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@multica/ui/components/ui/alert-dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@multica/ui/components/ui/tooltip";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Label } from "@multica/ui/components/ui/label";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import { cn } from "@multica/ui/lib/utils";
import { openExternal } from "../../platform";
import { RuntimeLocalSkillImportPanel } from "./runtime-local-skill-import-panel";
import { TemplateSkillCreatePanel } from "./template-skill-create-panel";
import { SkillPresentationFields } from "./skill-presentation-fields";
import { ResourceLabelPicker } from "../../labels/resource-label-picker";
import { useTemplateSkillSession } from "../hooks/use-template-skill-session";
import { useT } from "../../i18n";
import { isNameConflictError } from "../lib/utils";

type Method = "chooser" | "manual" | "template" | "local" | "url" | "runtime";

// ---------------------------------------------------------------------------
// Chooser — initial method picker
// ---------------------------------------------------------------------------

function MethodChooser({ onChoose }: { onChoose: (m: Method) => void }) {
  const { t } = useT("skills");
  const methods: {
    key: Exclude<Method, "chooser">;
    icon: typeof Plus;
  }[] = [
    { key: "manual", icon: Plus },
    { key: "template", icon: LayoutTemplate },
    { key: "local", icon: FolderOpen },
    { key: "url", icon: Download },
    { key: "runtime", icon: HardDrive },
  ];
  return (
    <div className="grid min-h-0 flex-1 gap-2 overflow-y-auto p-5">
      {methods.map(({ key, icon: Icon }) => (
        <button
          key={key}
          type="button"
          onClick={() => onChoose(key)}
          className="group flex items-start gap-3 rounded-lg border bg-card p-4 text-left transition-colors hover:border-primary/40 hover:bg-accent/40"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground group-hover:text-foreground">
            <Icon className="h-4 w-4" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-body font-medium">
              {t(($) => $.create.method[key].title)}
            </div>
            <div className="mt-0.5 text-caption text-muted-foreground">
              {t(($) => $.create.method[key].desc)}
            </div>
          </div>
          <ChevronRight className="h-4 w-4 shrink-0 text-faint-foreground transition-colors group-hover:text-muted-foreground" />
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Manual form
// ---------------------------------------------------------------------------

function ManualForm({
  initialPresentation,
  onCreated,
  onCancel,
}: {
  initialPresentation?: Partial<SkillPresentationMeta>;
  onCreated: (skill: Skill) => void;
  onCancel: () => void;
}) {
  const { t } = useT("skills");
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  // Seeded once from the caller (e.g. a category empty state pre-filling
  // its category); later prop changes never overwrite what the user picked.
  const [presentation, setPresentation] = useState<SkillPresentationMeta>(() => ({
    ...EMPTY_SKILL_PRESENTATION,
    ...initialPresentation,
  }));
  // Draft-mode label selection: workspace `skill` labels picked before the
  // skill exists, attached by the server in the create transaction.
  const [labelIds, setLabelIds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const fadeStyle = useScrollFade(scrollRef);

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    setLoading(true);
    setError("");
    try {
      const config = writeSkillPresentationMeta(undefined, presentation);
      const skill = await api.createSkill({
        name: trimmed,
        description: description.trim(),
        ...(Object.keys(config).length > 0 ? { config } : {}),
        ...(labelIds.length > 0 ? { label_ids: labelIds } : {}),
      });
      seedAfterCreate(qc, wsId, skill);
      toast.success(t(($) => $.create.manual.toast_created));
      onCreated(skill);
    } catch (err) {
      setError(err instanceof Error ? err.message : t(($) => $.create.manual.fallback_error));
      setLoading(false);
    }
  };

  return (
    <>
      <div
        ref={scrollRef}
        style={fadeStyle}
        className="flex-1 min-h-0 space-y-4 overflow-y-auto px-5 py-4"
      >
        <div className="space-y-1.5">
          <Label
            htmlFor="create-skill-name"
            className="text-caption text-muted-foreground"
          >
            {t(($) => $.create.manual.name_label)}
          </Label>
          <Input
            id="create-skill-name"
            autoFocus
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setError("");
            }}
            placeholder={t(($) => $.create.manual.name_placeholder)}
            onKeyDown={(e) => {
              if (isImeComposing(e)) return;
              if (e.key === "Enter") submit();
            }}
          />
          <p className="text-caption text-muted-foreground">
            {t(($) => $.create.manual.name_hint)}
          </p>
        </div>

        <div className="space-y-1.5">
          <Label
            htmlFor="create-skill-desc"
            className="text-caption text-muted-foreground"
          >
            <Pencil className="h-3 w-3" />
            {t(($) => $.create.manual.description_label)}
          </Label>
          <Textarea
            id="create-skill-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t(($) => $.create.manual.description_placeholder)}
            rows={3}
            className="resize-none"
          />
        </div>

        <SkillPresentationFields
          value={presentation}
          onChange={setPresentation}
        />

        <div className="space-y-1.5">
          <span className="block text-caption text-muted-foreground">
            {t(($) => $.detail.overview.labels)}
          </span>
          <ResourceLabelPicker
            resourceType="skill"
            selectedIds={labelIds}
            onSelectedIdsChange={setLabelIds}
            canEdit
          />
        </div>

        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-caption text-destructive"
          >
            <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>
              {error}
              {isNameConflictError(error) && (
                <>{t(($) => $.create.manual.name_conflict_hint)}</>
              )}
            </span>
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onCancel}
          disabled={loading}
        >
          {t(($) => $.create.manual.cancel)}
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={submit}
          disabled={!name.trim() || loading}
        >
          {loading ? (
            <>
              <Loader2 className="h-3 w-3 animate-spin" />
              {t(($) => $.create.manual.submitting)}
            </>
          ) : (
            t(($) => $.create.manual.submit)
          )}
        </Button>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// URL import form
// ---------------------------------------------------------------------------

type DetectedSource = "clawhub" | "skills.sh" | "github" | null;

function detectUrlSource(url: string): DetectedSource {
  const u = url.trim().toLowerCase();
  if (u.includes("clawhub.ai")) return "clawhub";
  if (u.includes("skills.sh")) return "skills.sh";
  if (u.includes("github.com")) return "github";
  return null;
}

function SourceCard({
  label,
  exampleHost,
  browseUrl,
  active,
}: {
  label: string;
  exampleHost: string;
  browseUrl: string;
  active: boolean;
}) {
  return (
    <div
      className={`rounded-md border px-3 py-2.5 transition-colors ${
        active ? "border-primary bg-primary/5" : ""
      }`}
    >
      <div className="text-caption font-medium">{label}</div>
      <button
        type="button"
        onClick={() => openExternal(browseUrl)}
        className="mt-0.5 block max-w-full truncate text-left font-mono text-caption text-brand underline decoration-brand/40 underline-offset-2 hover:decoration-brand"
      >
        {exampleHost}
      </button>
    </div>
  );
}

function UrlForm({
  onCreated,
  onCancel,
}: {
  onCreated: (skill: Skill) => void;
  onCancel: () => void;
}) {
  const { t } = useT("skills");
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const source = detectUrlSource(url);
  const scrollRef = useRef<HTMLDivElement>(null);
  const fadeStyle = useScrollFade(scrollRef);

  const submit = async () => {
    const trimmed = url.trim();
    if (!trimmed) return;
    setLoading(true);
    setError("");
    try {
      const skill = await api.importSkill({ url: trimmed });
      seedAfterCreate(qc, wsId, skill);
      toast.success(t(($) => $.create.url.toast_imported));
      onCreated(skill);
    } catch (err) {
      setError(err instanceof Error ? err.message : t(($) => $.create.url.fallback_error));
      setLoading(false);
    }
  };

  const submittingLabel = (() => {
    if (!loading) return t(($) => $.create.url.import);
    if (source === "clawhub") return t(($) => $.create.url.importing_clawhub);
    if (source === "skills.sh") return t(($) => $.create.url.importing_skills_sh);
    if (source === "github") return t(($) => $.create.url.importing_github);
    return t(($) => $.create.url.importing);
  })();

  return (
    <>
      <div
        ref={scrollRef}
        style={fadeStyle}
        className="flex-1 min-h-0 space-y-4 overflow-y-auto px-5 py-4"
      >
        <div className="space-y-1.5">
          <Label htmlFor="import-url" className="text-caption text-muted-foreground">
            {t(($) => $.create.url.url_label)}
          </Label>
          <Input
            id="import-url"
            autoFocus
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              setError("");
            }}
            placeholder="https://clawhub.ai/owner/skill"
            className="font-mono text-body"
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
          />
        </div>

        <div>
          <p className="mb-2 text-caption text-muted-foreground">
            {t(($) => $.create.url.supported_sources)}
          </p>
          <div className="grid grid-cols-3 gap-2">
            <SourceCard
              label="ClawHub"
              exampleHost="clawhub.ai/owner/skill"
              browseUrl="https://clawhub.ai"
              active={source === "clawhub"}
            />
            <SourceCard
              label="Skills.sh"
              exampleHost="skills.sh/owner/repo/skill"
              browseUrl="https://skills.sh"
              active={source === "skills.sh"}
            />
            <SourceCard
              label="GitHub"
              exampleHost="github.com/owner/repo"
              browseUrl="https://github.com"
              active={source === "github"}
            />
          </div>
        </div>

        {error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-caption text-destructive"
          >
            <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>
              {error}
              {isNameConflictError(error) && (
                <>{t(($) => $.create.url.name_conflict_hint)}</>
              )}
            </span>
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onCancel}
          disabled={loading}
        >
          {t(($) => $.create.url.cancel)}
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={submit}
          disabled={!url.trim() || loading}
        >
          {loading ? (
            <>
              <Loader2 className="h-3 w-3 animate-spin" />
              {submittingLabel}
            </>
          ) : (
            <>
              <Download className="h-3 w-3" />
              {submittingLabel}
            </>
          )}
        </Button>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Local folder / archive import
// ---------------------------------------------------------------------------

function LocalForm({
  prepared,
  preparing,
  onCreated,
  onCancel,
  onChooseFolder,
  onChooseArchive,
}: {
  prepared: PreparedSkillArchive | null;
  preparing: boolean;
  onCreated: (skill: Skill) => void;
  onCancel: () => void;
  onChooseFolder: () => void;
  onChooseArchive: () => void;
}) {
  const { t } = useT("skills");
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);
  const fadeStyle = useScrollFade(scrollRef);

  // Import stays disabled until a valid selection exists, so an invalid one
  // never reaches here — `prepareError` below already explains why.
  const submit = async () => {
    if (!prepared?.ok) return;
    setLoading(true);
    setError("");
    try {
      const skill = await api.importSkillArchive(prepared.file, "fail");
      seedAfterCreate(qc, wsId, skill);
      toast.success(t(($) => $.create.local.toast_imported));
      onCreated(skill);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t(($) => $.create.local.fallback_error),
      );
      setLoading(false);
    }
  };

  const prepareError =
    prepared && !prepared.ok
      ? {
          missing_skill_md: t(($) => $.create.local.missing_skill_md),
          too_large: t(($) => $.create.local.too_large),
          empty: t(($) => $.create.local.empty),
          too_many_files: t(($) => $.create.local.too_many_files),
        }[prepared.error]
      : "";
  const displayError = error || prepareError;

  return (
    <>
      <div
        ref={scrollRef}
        style={fadeStyle}
        className="flex-1 min-h-0 space-y-4 overflow-y-auto px-5 py-4"
      >
        {preparing && (
          <div className="flex items-center gap-2 text-caption text-muted-foreground">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            {t(($) => $.create.local.importing)}
          </div>
        )}

        {prepared?.ok && (
          <div className="space-y-3">
            <div className="rounded-md border px-3 py-2.5">
              <div className="text-caption text-muted-foreground">
                {prepared.preview.source === "archive"
                  ? t(($) => $.create.local.archive_label)
                  : t(($) => $.create.local.folder_label)}
              </div>
              <div className="mt-0.5 truncate text-body font-medium">
                {prepared.preview.displayName}
              </div>
            </div>
            <div className="space-y-1">
              <div className="text-body font-medium">{prepared.preview.skillName}</div>
              {prepared.preview.description ? (
                <p className="text-caption text-muted-foreground">
                  {prepared.preview.description}
                </p>
              ) : null}
              {prepared.preview.fileCount != null ? (
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.create.local.files, {
                    count: prepared.preview.fileCount,
                  })}
                </p>
              ) : null}
            </div>
          </div>
        )}

        {/* Source pickers belong in the body: four buttons in the footer
            overflow the dialog width and clip Cancel (MUL-6794). */}
        <div className="grid grid-cols-2 gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full"
            onClick={onChooseFolder}
            disabled={loading || preparing}
          >
            <FolderOpen className="h-3 w-3" />
            {t(($) => $.create.local.choose_folder)}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full"
            onClick={onChooseArchive}
            disabled={loading || preparing}
          >
            <FileArchive className="h-3 w-3" />
            {t(($) => $.create.local.choose_archive)}
          </Button>
        </div>

        {displayError && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-destructive/10 px-3 py-2 text-caption text-destructive"
          >
            <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>
              {displayError}
              {isNameConflictError(displayError) && (
                <>{t(($) => $.create.local.name_conflict_hint)}</>
              )}
            </span>
          </div>
        )}
      </div>

      <div className="flex shrink-0 items-center justify-end gap-2 border-t bg-muted/30 px-5 py-3">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onCancel}
          disabled={loading}
        >
          {t(($) => $.create.local.cancel)}
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={submit}
          disabled={!prepared?.ok || loading || preparing}
        >
          {loading ? (
            <>
              <Loader2 className="h-3 w-3 animate-spin" />
              {t(($) => $.create.local.importing)}
            </>
          ) : (
            <>
              <FolderOpen className="h-3 w-3" />
              {t(($) => $.create.local.import)}
            </>
          )}
        </Button>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Root dialog
// ---------------------------------------------------------------------------

export function CreateSkillDialog({
  onClose,
  onCreated,
  initialTemplateName,
  initialPresentation,
}: {
  onClose: () => void;
  onCreated?: (skill: Skill) => void;
  initialTemplateName?: string;
  /** Pre-fills the manual form's category / icon (e.g. from a category empty state). */
  initialPresentation?: Partial<SkillPresentationMeta>;
}) {
  const { t } = useT("skills");
  const [method, setMethod] = useState<Method>(() => initialTemplateName ? "template" : "chooser");
  const initialTemplate = useRef(initialTemplateName);
  const wsId = useWorkspaceId();
  const [discardOpen, setDiscardOpen] = useState(false);
  const pendingAction = useRef<(() => void) | null>(null);
  const [localPrepared, setLocalPrepared] = useState<PreparedSkillArchive | null>(
    null,
  );
  const [localPreparing, setLocalPreparing] = useState(false);
  const [localEpoch, setLocalEpoch] = useState(0);
  const localGeneration = useRef(0);
  const folderInputRef = useRef<HTMLInputElement>(null);
  const archiveInputRef = useRef<HTMLInputElement>(null);

  const handleCreated = (skill: Skill) => {
    onCreated?.(skill);
    onClose();
  };

  const templateSession = useTemplateSkillSession(wsId, (skill, newlyCreated) => {
    if (newlyCreated) toast.success(t(($) => $.create.template.created));
    handleCreated(skill);
  });
  const previewTemplate = useRef(templateSession.preview);
  previewTemplate.current = templateSession.preview;

  // A catalog entry seeds only the preview. Explicit adoption still owns the
  // editable draft, so locale changes and refreshed catalogs cannot replace it.
  useEffect(() => {
    if (initialTemplate.current) previewTemplate.current(initialTemplate.current);
  }, [wsId]);

  useEffect(() => {
    pendingAction.current = null;
    setDiscardOpen(false);
  }, [wsId]);

  const confirmDiscard = (action: () => void) => {
    if (templateSession.busy) return;
    if (templateSession.dirty || templateSession.hasUnconfirmedSubmission) {
      pendingAction.current = action;
      setDiscardOpen(true);
    } else {
      action();
    }
  };

  const requestClose = () => confirmDiscard(() => {
    templateSession.reset();
    onClose();
  });

  const beginLocalSelection = (): number => {
    const next = localGeneration.current + 1;
    localGeneration.current = next;
    setLocalEpoch(next);
    return next;
  };

  const resetLocal = () => {
    beginLocalSelection();
    setLocalPrepared(null);
    setLocalPreparing(false);
  };

  const bindDirectoryInput = (el: HTMLInputElement | null) => {
    folderInputRef.current = el;
    if (!el) return;
    el.setAttribute("webkitdirectory", "");
    el.setAttribute("directory", "");
  };

  const openFolderPicker = () => {
    folderInputRef.current?.click();
  };

  const openArchivePicker = () => {
    archiveInputRef.current?.click();
  };

  const switchMethod = (next: Method) => {
    if (next !== "template") templateSession.reset();
    if (next === "local") {
      // Switch first so cancelling the picker still lands on the local
      // panel (choose-folder / choose-archive), rather than silently
      // staying on the chooser with no way to pick a .skill file.
      setMethod("local");
      openFolderPicker();
      return;
    }
    resetLocal();
    setMethod(next);
  };

  const handleChoose = (next: Method) => {
    if (next === "template") switchMethod(next);
    else confirmDiscard(() => switchMethod(next));
  };

  const handleBack = () => {
    if (templateSession.busy) return;
    if (method === "template" && templateSession.step === "editor") {
      templateSession.backToTemplates();
      return;
    }
    resetLocal();
    setMethod("chooser");
  };

  const onFolderPicked = async (e: ChangeEvent<HTMLInputElement>) => {
    // FileList is live: clearing the input empties the same object. Snapshot
    // File handles first so resetting `value` (to allow picking the same
    // folder again) cannot drop the selection.
    const files = Array.from(e.target.files ?? []);
    e.target.value = "";
    if (files.length === 0) return;
    const gen = beginLocalSelection();
    setMethod("local");
    setLocalPreparing(true);
    setLocalPrepared(null);
    try {
      const prepared = await prepareSkillArchiveFromPickerFiles(files);
      if (gen !== localGeneration.current) return;
      setLocalPrepared(prepared);
    } catch {
      if (gen !== localGeneration.current) return;
      setLocalPrepared({ ok: false, error: "empty" });
    } finally {
      if (gen === localGeneration.current) setLocalPreparing(false);
    }
  };

  const onArchivePicked = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    beginLocalSelection();
    setMethod("local");
    setLocalPreparing(false);
    setLocalPrepared(wrapExistingSkillArchive(file));
  };

  const wide = method === "runtime";
  const fromTemplate = method === "template";
  const backLabel = fromTemplate
    ? templateSession.step === "editor"
      ? t(($) => $.create.template.back_to_templates)
      : t(($) => $.create.template.back_to_methods)
    : t(($) => $.create.back_aria);

  return (
    <>
    <Dialog open onOpenChange={(v) => !v && requestClose()}>
      <DialogContent
        showCloseButton={false}
        className={cn(
          "flex flex-col gap-0 overflow-hidden p-0",
          "!transition-all !duration-300 !ease-out",
          fromTemplate
            ? "!h-[min(720px,85dvh)] !max-w-4xl !w-[calc(100%-2rem)]"
            : wide
              ? "!h-[min(600px,85vh)] !max-w-2xl !w-[calc(100%-2rem)]"
              : "!h-auto !max-h-[85vh] !max-w-md !w-[calc(100%-2rem)]",
        )}
      >
        {/* Header */}
        <div className="flex shrink-0 items-start justify-between gap-3 border-b px-5 pt-4 pb-3">
          <div className="flex items-start gap-2 min-w-0">
            {method !== "chooser" && (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <button
                      type="button"
                      onClick={handleBack}
                      disabled={templateSession.busy}
                      className="-ml-1 mt-px rounded-sm p-1 text-faint-foreground transition-colors hover:bg-accent/60 hover:text-muted-foreground"
                      aria-label={backLabel}
                    >
                      <ArrowLeft className="h-3.5 w-3.5" />
                    </button>
                  }
                />
                <TooltipContent side="bottom">{backLabel}</TooltipContent>
              </Tooltip>
            )}
            <div className="min-w-0">
              <DialogTitle className="truncate text-title-sm font-medium">
                {t(($) => $.create.method[method].title)}
              </DialogTitle>
              <p className="mt-0.5 text-caption text-muted-foreground">
                {t(($) => $.create.method[method].desc)}
              </p>
            </div>
          </div>
          <Tooltip>
            <TooltipTrigger
              render={
                <button
                  type="button"
                  onClick={requestClose}
                  disabled={templateSession.busy}
                  className="rounded-sm p-1 text-faint-foreground transition-colors hover:bg-accent/60 hover:text-muted-foreground"
                  aria-label={t(($) => $.create.close_aria)}
                >
                  <XIcon className="h-3.5 w-3.5" />
                </button>
              }
            />
            <TooltipContent side="bottom">{t(($) => $.create.close)}</TooltipContent>
          </Tooltip>
        </div>

        {/* Hidden pickers stay mounted so the chooser click can open them
            in the same user-gesture (browsers otherwise block input.click()). */}
        <input
          ref={bindDirectoryInput}
          type="file"
          multiple
          className="sr-only"
          tabIndex={-1}
          aria-hidden
          onChange={onFolderPicked}
        />
        <input
          ref={archiveInputRef}
          type="file"
          accept=".skill,.zip,application/zip"
          className="sr-only"
          tabIndex={-1}
          aria-hidden
          onChange={onArchivePicked}
        />

        {/* Method body — each form owns its scroll middle + footer */}
        {method === "chooser" && <MethodChooser onChoose={handleChoose} />}
        {method === "manual" && (
          <ManualForm
            initialPresentation={initialPresentation}
            onCreated={handleCreated}
            onCancel={() => setMethod("chooser")}
          />
        )}
        {method === "template" && (
          <TemplateSkillCreatePanel
            workspaceId={wsId}
            session={templateSession}
            onBack={handleBack}
            onUseTemplate={(template, names, description) => {
              const applyTemplate = () => templateSession.useTemplate(template, names, description);
              if (templateSession.draft && templateSession.draft.templateName !== template.name) confirmDiscard(applyTemplate);
              else applyTemplate();
            }}
            onOpenCandidate={(candidate) => {
              const open = () => templateSession.openCandidate(candidate.skill);
              if (candidate.matches && candidate.matchesDraft) open();
              else confirmDiscard(open);
            }}
          />
        )}
        {method === "local" && (
          <LocalForm
            key={localEpoch}
            prepared={localPrepared}
            preparing={localPreparing}
            onCreated={handleCreated}
            onCancel={() => {
              resetLocal();
              setMethod("chooser");
            }}
            onChooseFolder={openFolderPicker}
            onChooseArchive={openArchivePicker}
          />
        )}
        {method === "url" && (
          <UrlForm
            onCreated={handleCreated}
            onCancel={() => setMethod("chooser")}
          />
        )}
        {method === "runtime" && (
          <RuntimeLocalSkillImportPanel
            onImported={handleCreated}
            onBulkDone={onClose}
          />
        )}
      </DialogContent>
    </Dialog>
    <AlertDialog open={discardOpen} onOpenChange={(open) => {
      setDiscardOpen(open);
      if (!open) pendingAction.current = null;
    }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t(($) => $.create.template.discard_title)}</AlertDialogTitle>
          <AlertDialogDescription>
            {templateSession.hasUnconfirmedSubmission
              ? t(($) => $.create.template.discard_unknown_description)
              : t(($) => $.create.template.discard_description)}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t(($) => $.create.template.keep_editing)}</AlertDialogCancel>
          <AlertDialogAction onClick={() => {
            const action = pendingAction.current;
            pendingAction.current = null;
            setDiscardOpen(false);
            action?.();
          }}>{t(($) => $.create.template.discard)}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    </>
  );
}
