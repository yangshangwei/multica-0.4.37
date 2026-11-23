import { useLayoutEffect, useRef, type RefObject } from "react";
import { IssueDescriptionAssist, type IssueDescriptionAssistEditor } from "../modals/issue-description-assist";
import { useT } from "../i18n";

function exceedsCharacters(value: string, limit: number) {
  let count = 0;
  for (const _character of value) if (++count > limit) return true;
  return false;
}

/** The existing draft is the sole owner; the adapter reads the live textarea. */
export function CollaborationCreationDescriptionAssist({ textareaRef, sourceId, value, title, uploading, submitting,
  isBlocked, isCurrent, onChange, onNeedsSpace }: {
  textareaRef: RefObject<HTMLTextAreaElement | null>; sourceId: string; value: string; title: string;
  uploading: boolean; submitting: boolean; isBlocked: () => boolean; isCurrent: () => boolean;
  onChange: (value: string) => void; onNeedsSpace?: () => void;
}) {
  const { t } = useT("collaboration");
  const latest = useRef({ value, isBlocked, isCurrent, onChange });
  useLayoutEffect(() => { latest.current = { value, isBlocked, isCurrent, onChange }; });
  const editorRef = useRef<IssueDescriptionAssistEditor>({
    getMarkdown: () => textareaRef.current?.value ?? latest.current.value,
    flushPendingUpdate: () => null,
    adoptContent: (text) => {
      if (!latest.current.isCurrent() || latest.current.isBlocked()) return;
      if (textareaRef.current) textareaRef.current.value = text;
    },
    focus: () => textareaRef.current?.focus({ preventScroll: true }),
  });
  const disabledReason = exceedsCharacters(value, 20_000) ? t(($) => $.creation.assistDescriptionLimit)
    : exceedsCharacters(title, 500) ? t(($) => $.creation.assistTitleLimit) : undefined;
  return <IssueDescriptionAssist className="py-0" wsId={sourceId} mode="manual" textFormat="plain" actionLabel={t(($) => $.creation.assist)}
    editorRef={editorRef} value={value} title={title} uploading={uploading} submitting={submitting}
    isBlocked={isBlocked} isCurrent={isCurrent} disabledReason={disabledReason} onNeedsSpace={onNeedsSpace}
    onChange={(text) => { if (latest.current.isCurrent() && !latest.current.isBlocked()) latest.current.onChange(text); }}
    onRevealEditor={(position) => requestAnimationFrame(() => {
      const editor = textareaRef.current;
      const body = editor?.closest<HTMLElement>("[data-collaboration-creation-body]");
      if (!editor || !body || document.activeElement !== editor || !latest.current.isCurrent()) return;
      const viewport = body.getBoundingClientRect();
      const bounds = editor.getBoundingClientRect();
      body.scrollTop += position === "end" ? bounds.bottom - viewport.bottom + 8 : bounds.top - viewport.top - 8;
    })} />;
}
