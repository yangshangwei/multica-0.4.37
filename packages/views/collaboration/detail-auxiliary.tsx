import { forwardRef, useCallback, useEffect, useId, useImperativeHandle, useRef, useState } from "react";
import { deriveCollaborationDetailState, type CollaborationIssue, type CollaborationScope } from "@multica/core/collaboration";
import type { AccountDestination } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@multica/ui/components/ui/dialog";
import { cn } from "@multica/ui/lib/utils";
import { BookOpen, FileText, GitBranch, History, Info, Paperclip, X } from "lucide-react";
import { useT } from "../i18n";
import { CollaborationActivity } from "./activity";
import { CollaborationAttachmentPanel } from "./attachment-panel";
import { CollaborationChildren } from "./children";
import { CollaborationFieldSummary } from "./field-summary";
import { CollaborationKnowledgePanel } from "./knowledge-panel";
import { useCollaborationLabels } from "./labels";
import { CollaborationMemoryPanel } from "./memory-panel";
import type { CollaborationPanelStatus } from "./panel-status";

type Panel = "details" | "files" | "memory" | "knowledge" | "children" | "activity";
const icons = { details: Info, files: Paperclip, memory: FileText, knowledge: BookOpen, children: GitBranch, activity: History };
const panels: Panel[] = ["details", "files", "memory", "knowledge", "children", "activity"];
export interface CollaborationDetailAuxiliaryHandle {
  open: (panel: Panel, opener: HTMLElement | null) => void;
  closeForNavigation: (onClosed: () => void) => void;
}

export const CollaborationDetailAuxiliary = forwardRef<CollaborationDetailAuxiliaryHandle, {
  scope: CollaborationScope; issue: CollaborationIssue; destination: AccountDestination; isScopeResume?: boolean; navigationKey?: number;
}>(function CollaborationDetailAuxiliary({ scope, issue, destination, isScopeResume = false, navigationKey = 0 }, ref) {
  const { t } = useT("collaboration");
  const labels = useCollaborationLabels();
  const id = useId();
  const [selected, setSelected] = useState<Panel>("details");
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<Partial<Record<Panel, CollaborationPanelStatus>>>({});
  const opener = useRef<HTMLElement | null>(null);
  const toolbar = useRef<HTMLDivElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const afterClose = useRef<(() => void) | null>(null);
  const navigating = useRef(false);
  const names: Record<Panel, string> = { details: t(($) => $.detail.context), files: t(($) => $.attachments.title), memory: t(($) => $.memory.title),
    knowledge: t(($) => $.knowledge.title), children: t(($) => $.children.title), activity: t(($) => $.activity.title) };
  const stage = deriveCollaborationDetailState(issue, scope.userId).stage;
  const updateStatus = useCallback((panel: Panel, value: CollaborationPanelStatus) => {
    setStatus((current) => current[panel]?.attention === value.attention && current[panel]?.count === value.count ? current : { ...current, [panel]: value });
  }, []);
  const filesStatus = useCallback((value: CollaborationPanelStatus) => updateStatus("files", value), [updateStatus]);
  const memoryStatus = useCallback((value: CollaborationPanelStatus) => updateStatus("memory", value), [updateStatus]);
  const activityStatus = useCallback((value: CollaborationPanelStatus) => updateStatus("activity", value), [updateStatus]);
  const knowledgeStatus = useCallback((value: CollaborationPanelStatus) => updateStatus("knowledge", value), [updateStatus]);
  const show = useCallback((panel: Panel, trigger: HTMLElement | null) => {
    navigating.current = false; afterClose.current = null; opener.current = trigger; setSelected(panel); setOpen(true);
  }, []);
  useImperativeHandle(ref, () => ({ open: show, closeForNavigation: (callback) => {
    if (!open && !afterClose.current) { callback(); return; }
    navigating.current = true; afterClose.current = callback; setOpen(false);
  } }), [open, show]);
  useEffect(() => {
    if (isScopeResume || !(destination.anchor === "memory" || destination.memoryVersion)) return;
    show("memory", toolbar.current?.querySelector<HTMLButtonElement>('[data-panel="memory"]') ?? null);
  }, [destination.anchor, destination.memoryVersion, navigationKey, isScopeResume, show]);
  const attention = panels.filter((panel) => status[panel]?.attention === true);
  const count = (panel: Panel) => panel === "children" ? issue.ordinary_children?.items.length : status[panel]?.count;
  const buttonContent = (panel: Panel) => {
    const Icon = icons[panel];
    const value = count(panel);
    return <><Icon aria-hidden="true" className="size-4 shrink-0" /><span>{names[panel]}</span>
      {value !== undefined && <span className="text-caption tabular-nums text-muted-foreground">{" "}{value}</span>}
      {status[panel]?.attention && <span className="size-1.5 shrink-0 rounded-full bg-destructive" aria-hidden="true" />}</>;
  };
  return <>
    <div ref={toolbar} role="group" aria-label={t(($) => $.auxiliary.title)} className="min-w-0 space-y-2 border-t border-border/60 pt-3">
      <div className="flex min-w-0 flex-wrap gap-1">{panels.map((panel) => <Button key={panel} data-panel={panel} variant="ghost" size="sm" aria-label={count(panel) !== undefined ? `${names[panel]} ${count(panel)}` : names[panel]}
        aria-haspopup="dialog" aria-expanded={open && selected === panel} aria-controls={open ? id : undefined}
        aria-describedby={status[panel]?.attention ? `${id}-attention` : undefined}
        className={cn("h-auto min-h-9 min-w-0 max-w-full gap-2 whitespace-normal text-caption text-muted-foreground pointer-coarse:min-h-11", open && selected === panel && "bg-muted font-semibold text-foreground hover:bg-muted")}
        onClick={(event) => show(panel, event.currentTarget)}>{buttonContent(panel)}</Button>)}</div>
      {attention.length > 0 && <div id={`${id}-attention`} role="status" className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
        <span>{t(($) => $.auxiliary.attention)}</span>{attention.map((panel) => <Button key={panel} variant="link" size="sm" className="h-auto min-h-8 px-1 text-caption pointer-coarse:min-h-11" onClick={(event) => show(panel, event.currentTarget)}>{names[panel]}</Button>)}
      </div>}
    </div>
    <Dialog open={open} onOpenChange={setOpen} onOpenChangeComplete={(next) => {
      if (!next && afterClose.current) { const callback = afterClose.current; afterClose.current = null; requestAnimationFrame(callback); }
    }}>
      <DialogContent id={id} keepMounted showCloseButton={false}
        initialFocus={() => !destination.memoryVersion && destination.anchor === "memory" && selected === "memory" ? content.current?.querySelector<HTMLElement>("#memory") ?? heading.current : heading.current}
        finalFocus={() => navigating.current ? false : opener.current?.isConnected ? opener.current : toolbar.current?.querySelector<HTMLButtonElement>("button")}
        className="top-0 right-0 left-auto flex h-dvh w-full max-w-full translate-x-0 translate-y-0 flex-col gap-0 rounded-none p-0 sm:max-w-[44rem] data-open:zoom-in-100 data-closed:zoom-out-100 data-open:slide-in-from-right-4 data-closed:slide-out-to-right-4 motion-reduce:animate-none">
        <div className="shrink-0 space-y-4 border-b border-border px-5 pt-5 pb-3 sm:px-6">
          <div className="flex min-w-0 items-start gap-3">
            <div className="min-w-0 flex-1 space-y-2"><DialogTitle ref={heading} tabIndex={-1} className="text-title font-semibold outline-none [overflow-wrap:anywhere]">{names[selected]}</DialogTitle>
              <DialogDescription className="line-clamp-2 text-caption [overflow-wrap:anywhere]">{issue.title}</DialogDescription></div>
            <DialogClose render={<Button variant="ghost" size="icon" className="shrink-0 pointer-coarse:size-11" />} aria-label={t(($) => $.auxiliary.close)}><X className="size-4" aria-hidden="true" /></DialogClose>
          </div>
          <div role="group" aria-label={t(($) => $.auxiliary.navigation)} className="-mx-2 flex min-w-0 flex-wrap gap-1">{panels.map((panel) => <Button key={panel} variant="ghost" size="sm" aria-label={count(panel) !== undefined ? `${names[panel]} ${count(panel)}` : names[panel]} aria-pressed={selected === panel}
            className={cn("h-auto min-h-9 min-w-0 max-w-full gap-2 whitespace-normal text-caption text-muted-foreground pointer-coarse:min-h-11", selected === panel && "bg-muted font-semibold text-foreground hover:bg-muted")}
            onClick={() => setSelected(panel)}>{buttonContent(panel)}</Button>)}</div>
        </div>
        <div ref={content} className="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain p-5 sm:p-6" data-collaboration-drawer-scroll>
          <div hidden={!open || selected !== "details"} inert={!open || selected !== "details"} className="space-y-6">
            <dl className="grid min-w-0 gap-x-8 gap-y-4 text-body sm:grid-cols-2 [&_dt]:mb-1 [&_dt]:text-caption [&_dt]:text-muted-foreground [&_dd]:[overflow-wrap:anywhere]">
              <div><dt>{t(($) => $.detail.creator)}</dt><dd>{issue.creator.name}</dd></div>
              <div><dt>{t(($) => $.detail.source)}</dt><dd>{issue.source_workspace.name}</dd></div>
              {stage !== "working" && <div><dt>{t(($) => $.detail.status)}</dt><dd>{labels.status(issue.status)}</dd></div>}
              <div><dt>{t(($) => $.policy.title)}</dt><dd>{issue.ai_policy === "manual" ? t(($) => $.policy.manual) : issue.ai_policy === "assisted" ? t(($) => $.policy.assisted) : t(($) => $.policy.unknown)}</dd></div>
            </dl><CollaborationFieldSummary issue={issue} />
          </div>
          <div hidden={!open || selected !== "files"} inert={!open || selected !== "files"} className="[&>section>h3]:sr-only"><CollaborationAttachmentPanel scope={scope} issue={issue} onStatusChange={filesStatus} active={open && selected === "files"} /></div>
          <div hidden={!open || selected !== "memory"} inert={!open || selected !== "memory"} className="[&>#memory]:border-0 [&>#memory]:p-0 [&>#memory>div:first-child>h3]:sr-only"><CollaborationMemoryPanel scope={scope} issue={issue} destination={destination} isScopeResume={isScopeResume} navigationKey={navigationKey} active={open && selected === "memory"} onStatusChange={memoryStatus} /></div>
          <div hidden={!open || selected !== "knowledge"} inert={!open || selected !== "knowledge"} className="[&>section]:border-0 [&>section]:p-0"><CollaborationKnowledgePanel scope={scope} issue={issue} onStatusChange={knowledgeStatus} /></div>
          <div hidden={!open || selected !== "children"} inert={!open || selected !== "children"} className="[&>section>h3]:sr-only"><CollaborationChildren issue={issue} /></div>
          <div hidden={!open || selected !== "activity"} inert={!open || selected !== "activity"} className="[&>section>h3]:sr-only"><CollaborationActivity scope={scope} destination={destination} onStatusChange={activityStatus} /></div>
        </div>
      </DialogContent>
    </Dialog>
  </>;
});
