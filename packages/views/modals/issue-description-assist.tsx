"use client";

import { useEffect, useLayoutEffect, useId, useRef, useState, type RefObject } from "react";
import { ChevronDown, LoaderCircle, Sparkles } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useOptimizeIssueDescription } from "@multica/core/issues/mutations";
import type { Attachment } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
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
  onNeedsSpace?: () => void;
  onRevealEditor?: (position: "start" | "end") => void;
}

interface Suggestion {
  text: string;
}

interface UndoDraft {
  original: string;
  adopted: string;
  mergedCount?: number;
  clarifications?: { questions: string[]; answers: string[] };
}

/** Streamed text stays provisional; validated completion updates the active draft. */
export function IssueDescriptionAssist({
  wsId, mode, editorRef, value, title, attachments, onChange, uploading, isBlocked, submitting, onNeedsSpace, onRevealEditor,
}: IssueDescriptionAssistProps) {
  const { t } = useT("modals");
  const headingId = useId();
  const hintId = useId();
  const replacementHintId = useId();
  const assistRef = useRef<HTMLDivElement>(null);
  const { mutateAsync } = useOptimizeIssueDescription(wsId);
  const requestRef = useRef<AbortController | null>(null);
  const latestRef = useRef({ submitting, uploading, isBlocked, onChange, onRevealEditor });
  useLayoutEffect(() => { latestRef.current = { submitting, uploading, isBlocked, onChange, onRevealEditor }; });
  const returnFocusRef = useRef(false);
  const [pending, setPending] = useState(false);
  const [partialText, setPartialText] = useState("");
  const [suggestion, setSuggestion] = useState<Suggestion | null>(null);
  const [questions, setQuestions] = useState<string[]>([]);
  const [questionsOpen, setQuestionsOpen] = useState(true);
  const [answers, setAnswers] = useState<string[]>([]);
  const [undo, setUndo] = useState<UndoDraft | null>(null);
  const [error, setError] = useState<"unavailable" | "failed" | null>(null);
  const [notice, setNotice] = useState<"stale" | "applied" | "undone" | "blocked" | "answered" | null>(null);

  useEffect(() => () => {
    requestRef.current?.abort();
    requestRef.current = null;
  }, [wsId, mode]);

  const writeDraft = (editor: AssistEditor, text: string, original: string, clarifications?: UndoDraft["clarifications"]) => {
    const shouldReveal = assistRef.current?.contains(document.activeElement);
    editor.flushPendingUpdate();
    editor.adoptContent(text);
    const adopted = readMarkdown(editor);
    latestRef.current.onChange(adopted);
    setUndo({ original, adopted, clarifications, mergedCount: clarifications?.answers.filter((answer) => answer.trim()).length });
    if (shouldReveal) {
      editor.focus();
      latestRef.current.onRevealEditor?.(clarifications ? "end" : "start");
    }
  };

  const cancel = () => {
    returnFocusRef.current = true;
    requestRef.current?.abort();
    requestRef.current = null;
    setPending(false);
    setPartialText("");
  };

  const generate = async () => {
    const editor = editorRef.current;
    if (!editor || submitting || uploading || isBlocked()) return;
    const flushed = editor.flushPendingUpdate();
    if (flushed !== null) onChange(flushed);
    const source = readMarkdown(editor);
    if (!source.trim()) return;
    requestRef.current?.abort();
    const controller = new AbortController();
    requestRef.current = controller;
    onNeedsSpace?.();
    setPending(true);
    setQuestionsOpen(true);
    setPartialText("");
    setSuggestion(null);
    setQuestions([]);
    setAnswers([]);
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
      const currentEditor = editorRef.current;
      const current = latestRef.current;
      const nextQuestions = (result.questions ?? []).slice(0, 2);
      if (!currentEditor || readMarkdown(currentEditor) !== source || current.submitting || current.uploading || current.isBlocked()) {
        setSuggestion({ text: result.text });
        setNotice(currentEditor && readMarkdown(currentEditor) !== source ? "stale" : "blocked");
        return;
      }
      writeDraft(currentEditor, result.text, source);
      setQuestions(nextQuestions);
      setAnswers(nextQuestions.map(() => ""));
      setNotice("applied");
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

  const mergeAnswers = () => {
    const editor = editorRef.current;
    if (!editor || submitting || uploading || isBlocked()) return;
    const answered = questions.flatMap((question, index) => {
      const answer = answers[index]?.trim();
      return answer ? [`${question}\n${answer}`] : [];
    });
    if (!answered.length) return;
    const original = readMarkdown(editor);
    const details = `${t(($) => $.create_issue.ai_optimize.details)}\n\n${answered.join("\n\n")}`;
    writeDraft(editor, `${original}\n\n${details}`, original, { questions, answers });
    const remaining = questions.filter((_, index) => !answers[index]?.trim());
    setQuestions(remaining);
    setAnswers(remaining.map(() => ""));
    setNotice(remaining.length ? "applied" : "answered");
  };

  const undoAdoption = () => {
    const editor = editorRef.current;
    if (!editor || !undo || submitting || uploading || isBlocked()) return;
    if (readMarkdown(editor) !== undo.adopted) {
      setUndo(null);
      setNotice("stale");
      return;
    }
    editor.flushPendingUpdate();
    editor.adoptContent(undo.original);
    onChange(readMarkdown(editor));
    setUndo(null);
    setQuestions(undo.clarifications?.questions ?? []);
    setAnswers(undo.clarifications?.answers ?? []);
    setQuestionsOpen(true);
    setNotice("undone");
    editor.focus();
    onRevealEditor?.("start");
  };

  const blocked = uploading || submitting;
  const empty = !value.trim();
  const label = mode === "manual"
    ? t(($) => $.create_issue.ai_optimize.description)
    : t(($) => $.create_issue.ai_optimize.instructions);

  const draftMatches = !undo || !editorRef.current || readMarkdown(editorRef.current) === undo.adopted;
  const status = pending ? t(($) => $.create_issue.ai_optimize.generating)
    : notice === "stale" ? t(($) => $.create_issue.ai_optimize.stale)
    : notice === "blocked" ? t(($) => $.create_issue.ai_optimize.blocked)
    : !draftMatches ? null
    : notice === "answered" ? t(($) => $.create_issue.ai_optimize.answered, { count: undo?.mergedCount ?? 0 })
    : notice === "applied" ? (questions.length ? t(($) => $.create_issue.ai_optimize.applied) : t(($) => $.create_issue.ai_optimize.ready))
    : notice === "undone" ? t(($) => $.create_issue.ai_optimize.undone)
    : null;

  return (
    <div ref={assistRef} className="min-w-0 shrink-0 space-y-2 py-2">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        {!suggestion && !error && (
          <Button
            type="button" variant="ghost" size="sm"
            className="min-h-11 text-muted-foreground sm:min-h-7"
            ref={(button) => {
              if (!button || !returnFocusRef.current) return;
              returnFocusRef.current = false;
              if (button.disabled) editorRef.current?.focus();
              else button.focus();
            }}
            disabled={!pending && (blocked || empty)}
            aria-describedby={pending ? undefined : empty ? hintId : replacementHintId}
            onClick={() => { if (pending) cancel(); else void generate(); }}
          >
            {pending
              ? <LoaderCircle className="size-3.5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
              : <Sparkles className="size-3.5" aria-hidden="true" />}
            {pending ? t(($) => $.create_issue.ai_optimize.cancel) : label}
          </Button>
        )}
        {empty && <span id={hintId} className="sr-only">{t(($) => $.create_issue.ai_optimize.empty_hint)}</span>}
        <span id={replacementHintId} className={!status && !error && !suggestion && !pending ? "text-caption text-muted-foreground" : "sr-only"}>
          {t(($) => $.create_issue.ai_optimize.replacement_hint)}
        </span>
        <div className="ml-auto flex min-w-0 items-center gap-1">
          <p role="status" aria-live="polite" className="min-w-0 break-words text-caption text-muted-foreground empty:hidden">{status}</p>
          {undo && (
            <Button type="button" variant="ghost" size="sm" className="min-h-11 sm:min-h-7" disabled={blocked} onClick={undoAdoption}>
              {t(($) => $.create_issue.ai_optimize.undo)}
            </Button>
          )}
        </div>
      </div>
      {questions.length > 0 && (
        <section aria-labelledby={headingId} className="min-w-0 rounded-md border border-border px-3 py-1">
          <h3>
            <Button id={headingId} type="button" variant="ghost" size="sm"
              className="min-h-11 w-full justify-start whitespace-normal px-0 text-left sm:min-h-8"
              aria-expanded={questionsOpen} aria-controls={`${headingId}-questions`}
              onClick={() => setQuestionsOpen((open) => !open)}>
              <ChevronDown className={`size-3.5 ${questionsOpen ? "" : "-rotate-90"}`} aria-hidden="true" />
              {t(($) => $.create_issue.ai_optimize.questions, { count: questions.length })}
              {!questionsOpen && <span className="ml-auto text-muted-foreground">{t(($) => $.create_issue.ai_optimize.questions_pending, { count: questions.length })}</span>}
            </Button>
          </h3>
          <div id={`${headingId}-questions`} hidden={!questionsOpen} aria-describedby={`${headingId}-hint`} className="space-y-2 pb-2">
            <p id={`${headingId}-hint`} className="sr-only">{t(($) => $.create_issue.ai_optimize.questions_hint)}</p>
            {questions.map((question, index) => (
              <div key={index} className="space-y-1">
                <div className="flex flex-col items-start gap-x-2 sm:flex-row sm:items-center">
                  <label htmlFor={`${headingId}-${index}`} className="min-w-0 flex-1 break-words text-caption">{question}</label>
                  <Button type="button" variant="ghost" size="sm" className="min-h-11 px-0 text-muted-foreground sm:min-h-7 sm:px-2"
                    disabled={blocked || pending}
                    onClick={() => setAnswers((current) => current.map((answer, i) => i === index ? t(($) => $.create_issue.ai_optimize.delegate) : answer))}>
                    {t(($) => $.create_issue.ai_optimize.delegate)}
                  </Button>
                </div>
                <Textarea id={`${headingId}-${index}`} rows={1} value={answers[index] ?? ""} disabled={blocked || pending}
                  className="min-h-11 max-h-[calc(4lh+1rem)] resize-none overflow-y-auto sm:min-h-9"
                  placeholder={t(($) => $.create_issue.ai_optimize.answer_placeholder)}
                  onChange={(event) => setAnswers((current) => current.map((answer, i) => i === index ? event.target.value : answer))} />
              </div>
            ))}
            <Button type="button" variant="secondary" size="sm" className="min-h-11 sm:min-h-7" disabled={blocked || pending || !answers.some((answer) => answer.trim())} onClick={mergeAnswers}>
              {t(($) => $.create_issue.ai_optimize.merge_answers)}
            </Button>
          </div>
        </section>
      )}
      {error && (
        <div className="space-y-2">
          <p role="alert" className="text-caption text-destructive">
            {error === "unavailable"
              ? t(($) => $.create_issue.ai_optimize.unavailable)
              : t(($) => $.create_issue.ai_optimize.failed)}
          </p>
          <Button type="button" variant="outline" size="sm" className="min-h-11 sm:min-h-7" disabled={blocked} onClick={() => { void generate(); }}>
            {t(($) => $.create_issue.ai_optimize.retry)}
          </Button>
        </div>
      )}
      {(suggestion !== null || (pending && partialText.length > 0)) && (
        <section aria-labelledby={headingId} aria-busy={pending || undefined} className="min-w-0 space-y-3 rounded-md border border-border bg-muted/30 p-3">
          <h3 id={headingId} className="text-caption font-medium">{t(($) => $.create_issue.ai_optimize.suggestion)}</h3>
          <div className="max-h-40 space-y-3 overflow-y-auto overscroll-contain break-words" tabIndex={0}>
            {suggestion ? (
              <ReadonlyContent content={suggestion.text} attachments={attachments} />
            ) : (
              <div className="whitespace-pre-wrap text-body">{partialText}</div>
            )}
          </div>
          {suggestion && (
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="ghost" size="sm" className="min-h-11 sm:min-h-7" disabled={blocked} onClick={() => { void generate(); }}>
                {t(($) => $.create_issue.ai_optimize.regenerate)}
              </Button>
              <Button type="button" variant="ghost" size="sm" className="min-h-11 sm:min-h-7" onClick={() => { returnFocusRef.current = true; setSuggestion(null); setNotice(null); }}>
                {t(($) => $.create_issue.ai_optimize.discard)}
              </Button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
