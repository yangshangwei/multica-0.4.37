"use client";

import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { LoaderCircle, Sparkles } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useOptimizeIssueDescription } from "@multica/core/issues/mutations";
import type { Attachment } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import type { ContentEditorRef } from "../editor/content-editor";
import { ReadonlyContent } from "../editor/readonly-content";
import { useT } from "../i18n";

type AssistEditor = Pick<ContentEditorRef, "getMarkdown" | "flushPendingUpdate" | "adoptContent" | "focus">;

// Match ContentEditor's persisted Markdown, including its trailing-paragraph normalization.
function readMarkdown(editor: AssistEditor): string {
  return editor.getMarkdown().trim();
}

interface IssueDescriptionAssistProps {
  wsId: string;
  mode: "manual" | "agent";
  editorRef: RefObject<AssistEditor | null>;
  value: string;
  title?: string;
  attachments?: Attachment[];
  onChange: (text: string) => void;
  uploading: boolean;
  isBlocked: () => boolean;
  submitting: boolean;
}

interface Suggestion {
  source: string;
  text: string;
  questions: string[];
}

/** Preview stays local; only explicit adoption writes back to the active draft. */
export function IssueDescriptionAssist({
  wsId, mode, editorRef, value, title, attachments, onChange, uploading, isBlocked, submitting,
}: IssueDescriptionAssistProps) {
  const { t } = useT("modals");
  const headingId = useId();
  const hintId = useId();
  const { mutateAsync } = useOptimizeIssueDescription(wsId);
  const requestRef = useRef<AbortController | null>(null);
  const returnFocusRef = useRef(false);
  const [pending, setPending] = useState(false);
  const [partialText, setPartialText] = useState("");
  const [suggestion, setSuggestion] = useState<Suggestion | null>(null);
  const [undo, setUndo] = useState<{ original: string; adopted: string } | null>(null);
  const [error, setError] = useState<"unavailable" | "failed" | null>(null);
  const [notice, setNotice] = useState<"stale" | "applied" | "undone" | null>(null);

  useEffect(() => () => {
    requestRef.current?.abort();
    requestRef.current = null;
  }, []);

  useEffect(() => {
    if (suggestion && editorRef.current && readMarkdown(editorRef.current) !== suggestion.source) {
      setNotice("stale");
    }
  }, [value, suggestion, editorRef]);

  const cancel = () => {
    returnFocusRef.current = true;
    requestRef.current?.abort();
    requestRef.current = null;
    setPending(false);
    setPartialText("");
  };

  const generate = async () => {
    const editor = editorRef.current;
    if (!editor || submitting || isBlocked()) return;
    const flushed = editor.flushPendingUpdate();
    if (flushed !== null) onChange(flushed);
    const source = readMarkdown(editor);
    if (!source.trim()) return;
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    setPending(true);
    setPartialText("");
    setSuggestion(null);
    setUndo(null);
    setError(null);
    setNotice(null);
    try {
      const result = await mutateAsync({
        text: source, title, mode, signal: controller.signal,
        onText: (text) => {
          if (requestRef.current === controller && !controller.signal.aborted) setPartialText(text);
        },
      });
      if (requestRef.current !== controller) return;
      setSuggestion({ source, text: result.text, questions: result.questions ?? [] });
      if (editorRef.current && readMarkdown(editorRef.current) !== source) setNotice("stale");
    } catch (cause) {
      if (requestRef.current !== controller) return;
      setError(cause instanceof ApiError && cause.status === 503 ? "unavailable" : "failed");
    } finally {
      if (requestRef.current === controller) {
        requestRef.current = null;
        setPending(false);
        setPartialText("");
      }
    }
  };

  const apply = () => {
    const editor = editorRef.current;
    if (!editor || !suggestion || submitting || isBlocked()) return;
    // Debounced host state can lag behind typing and attachment insertion.
    if (readMarkdown(editor) !== suggestion.source) {
      setNotice("stale");
      return;
    }
    editor.flushPendingUpdate();
    editor.adoptContent(suggestion.text);
    const adopted = readMarkdown(editor);
    onChange(adopted);
    // Tiptap may normalize Markdown on adoption; guard undo against those bytes.
    setUndo({ original: suggestion.source, adopted });
    setSuggestion(null);
    setNotice("applied");
    editor.focus();
  };

  const undoAdoption = () => {
    const editor = editorRef.current;
    if (!editor || !undo || submitting || isBlocked()) return;
    if (readMarkdown(editor) !== undo.adopted) {
      setUndo(null);
      setNotice("stale");
      return;
    }
    editor.flushPendingUpdate();
    editor.adoptContent(undo.original);
    onChange(readMarkdown(editor));
    setUndo(null);
    setNotice("undone");
    editor.focus();
  };

  const blocked = uploading || submitting;
  const empty = !value.trim();
  const label = mode === "manual"
    ? t(($) => $.create_issue.ai_optimize.description)
    : t(($) => $.create_issue.ai_optimize.instructions);

  return (
    <div className="shrink-0 space-y-2 py-2">
      <div className="flex flex-wrap items-center gap-2">
        {!suggestion && !error && !pending && (
          <Button
            type="button" variant="ghost" size="sm"
            className="text-muted-foreground"
            ref={(button) => {
              if (!button || !returnFocusRef.current) return;
              returnFocusRef.current = false;
              if (button.disabled) editorRef.current?.focus();
              else button.focus();
            }}
            disabled={blocked || empty}
            aria-describedby={empty ? hintId : undefined}
            onClick={() => { void generate(); }}
          >
            <Sparkles className="size-3.5" aria-hidden="true" />
            {label}
          </Button>
        )}
        {empty && <span id={hintId} className="sr-only">{t(($) => $.create_issue.ai_optimize.empty_hint)}</span>}
        {pending && (
          <>
            <span className="flex items-center gap-2 text-caption text-muted-foreground">
              <LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              {t(($) => $.create_issue.ai_optimize.generating)}
            </span>
            <Button type="button" variant="ghost" size="sm" onClick={cancel}>
              {t(($) => $.create_issue.ai_optimize.cancel)}
            </Button>
          </>
        )}
        {undo && (
          <Button type="button" variant="ghost" size="sm" disabled={blocked} onClick={undoAdoption}>
            {t(($) => $.create_issue.ai_optimize.undo)}
          </Button>
        )}
      </div>
      <p role="status" aria-live="polite" className={pending ? "sr-only" : "text-caption text-muted-foreground empty:hidden"}>
        {pending ? t(($) => $.create_issue.ai_optimize.generating)
          : notice === "stale" ? t(($) => $.create_issue.ai_optimize.stale)
          : notice === "applied" ? t(($) => $.create_issue.ai_optimize.applied)
          : notice === "undone" ? t(($) => $.create_issue.ai_optimize.undone)
          : suggestion ? t(($) => $.create_issue.ai_optimize.ready) : null}
      </p>
      {error && (
        <div className="space-y-2">
          <p role="alert" className="text-caption text-destructive">
            {error === "unavailable"
              ? t(($) => $.create_issue.ai_optimize.unavailable)
              : t(($) => $.create_issue.ai_optimize.failed)}
          </p>
          <Button type="button" variant="outline" size="sm" disabled={blocked} onClick={() => { void generate(); }}>
            {t(($) => $.create_issue.ai_optimize.retry)}
          </Button>
        </div>
      )}
      {(suggestion !== null || (pending && partialText.length > 0)) && (
        <section aria-labelledby={headingId} aria-busy={pending || undefined} className="min-w-0 space-y-3 rounded-md border border-border bg-muted/30 p-3">
          <h3 id={headingId} className="text-caption font-medium">{t(($) => $.create_issue.ai_optimize.suggestion)}</h3>
          <div className="max-h-64 space-y-3 overflow-y-auto overscroll-contain break-words" tabIndex={0}>
            {suggestion ? (
              <ReadonlyContent content={suggestion.text} attachments={attachments} />
            ) : (
              <div className="whitespace-pre-wrap text-body">{partialText}</div>
            )}
            {suggestion && suggestion.questions.length > 0 && (
              <div className="space-y-1 text-caption text-muted-foreground">
                <h4 className="font-medium">{t(($) => $.create_issue.ai_optimize.questions)}</h4>
                <ul className="list-disc space-y-1 pl-4">
                  {suggestion.questions.map((question, index) => <li key={index}>{question}</li>)}
                </ul>
              </div>
            )}
          </div>
          {suggestion && (
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="secondary" size="sm" disabled={blocked || notice === "stale"} onClick={apply}>
                {t(($) => $.create_issue.ai_optimize.apply)}
              </Button>
              <Button type="button" variant="ghost" size="sm" disabled={blocked} onClick={() => { void generate(); }}>
                {t(($) => $.create_issue.ai_optimize.regenerate)}
              </Button>
              <Button type="button" variant="ghost" size="sm" onClick={() => { returnFocusRef.current = true; setSuggestion(null); setNotice(null); }}>
                {t(($) => $.create_issue.ai_optimize.discard)}
              </Button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
