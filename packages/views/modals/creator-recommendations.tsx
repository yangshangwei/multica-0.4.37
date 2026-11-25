"use client";

import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { LoaderCircle, Sparkles, X } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useRecommendIssueCreators } from "@multica/core/issues/mutations";
import { captureQuickCreateScope, isQuickCreateScopeCurrent, type QuickCreateActorRef, type QuickCreateScope } from "@multica/core/issues/stores/quick-create-store";
import type { IssueCreatorRecommendation } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import type { ContentEditorRef } from "../editor/content-editor";
import { AI_ASSIST_BUTTON_TONE } from "./issue-description-assist";
import { actorKey, type ActorCatalogEntry } from "../common/actor-discovery";

type Context = { text: string; projectId: string | null; actorKey: string; identityKey: string };
type Request = { controller: AbortController; context: Context; scope: QuickCreateScope; descriptions: Map<string, string> };
type Result = { context: Context; scope: QuickCreateScope; descriptions: Map<string, string>; items: IssueCreatorRecommendation[] };
interface Props {
  children?: ReactNode;
  wsId: string;
  value: string;
  projectId: string | null;
  actor: QuickCreateActorRef | null;
  identityKey: string;
  editorRef: RefObject<Pick<ContentEditorRef, "getMarkdown" | "flushPendingUpdate"> | null>;
  actors: readonly Pick<ActorCatalogEntry, "type" | "id" | "name" | "description">[];
  onPick: (actor: QuickCreateActorRef) => void;
  onNeedsSpace?: () => void;
}

export function CreatorRecommendations(props: Props) {
  const { t } = useT("modals");
  const hintId = useId();
  const noCandidatesId = useId();
  const { mutateAsync, reset } = useRecommendIssueCreators(props.wsId);
  const latest = useRef(props);
  useLayoutEffect(() => { latest.current = props; });
  const request = useRef<Request | null>(null);
  const failureContext = useRef<Pick<Request, "context" | "scope"> | null>(null);
  const button = useRef<HTMLButtonElement>(null);
  const [pending, setPending] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const [error, setError] = useState<"unavailable" | "busy" | "timeout" | "failed" | null>(null);
  // Says why a result or an in-flight request vanished after the context moved on.
  const [stale, setStale] = useState(false);
  const textNow = () => (latest.current.editorRef.current?.getMarkdown() ?? latest.current.value).trim();
  const matches = (context: Context, scope: QuickCreateScope) => isQuickCreateScopeCurrent(scope)
    && context.text === textNow() && context.projectId === latest.current.projectId
    && context.actorKey === (latest.current.actor ? actorKey(latest.current.actor) : "")
    && context.identityKey === latest.current.identityKey;
  // `lostWork` marks a context change that threw away a result or a request.
  const clear = (lostWork = false) => {
    request.current?.controller.abort(); request.current = null; failureContext.current = null;
    setPending(false); setResult(null); setError(null); setStale(lostWork); reset();
  };
  useEffect(() => () => { request.current?.controller.abort(); request.current = null; reset(); }, [props.wsId, reset]);
  useEffect(() => {
    const lostWork = (request.current !== null && !matches(request.current.context, request.current.scope)) || (result !== null && !matches(result.context, result.scope));
    if (!lostWork && !(failureContext.current && !matches(failureContext.current.context, failureContext.current.scope))) return;
    // A discarded error needs no explanation; discarded work does.
    clear(lostWork);
  });

  const start = async () => {
    // Nobody to suggest: the request could only come back empty.
    if (request.current || props.actors.length === 0) return;
    latest.current.editorRef.current?.flushPendingUpdate();
    const text = textNow();
    if (!text || Array.from(text).length > 20000) return;
    const scope = captureQuickCreateScope(props.wsId);
    if (!scope) return;
    const active: Request = { controller: new AbortController(), scope, descriptions: new Map(props.actors.map((entry) => [actorKey(entry), entry.description])), context: {
      text, projectId: props.projectId, actorKey: props.actor ? actorKey(props.actor) : "", identityKey: props.identityKey,
    } };
    request.current = active; failureContext.current = null; props.onNeedsSpace?.(); setPending(true); setError(null); setResult(null); setStale(false);
    try {
      const response = await mutateAsync({ text, signal: active.controller.signal });
      if (request.current !== active || !matches(active.context, active.scope)) return;
      setResult({ context: active.context, scope, descriptions: active.descriptions, items: response.recommendations });
    } catch (failure) {
      if (request.current !== active || active.controller.signal.aborted || !matches(active.context, active.scope)) return;
      failureContext.current = { context: active.context, scope: active.scope };
      const code = failure instanceof ApiError ? (failure.body as {code?: string} | undefined)?.code : undefined;
      setError(code === "ai_unavailable" || (failure instanceof ApiError && failure.status === 404) ? "unavailable"
        : code === "ai_busy" ? "busy" : code === "ai_timeout" ? "timeout" : "failed");
    } finally {
      if (request.current === active) { request.current = null; setPending(false); reset(); }
    }
  };
  const items = result?.items.flatMap((item) => {
    const actor = props.actors.find((entry) => entry.type === item.actor_type && entry.id === item.actor_id);
    return actor && actor.description === result.descriptions.get(actorKey(actor)) && actor.description.includes(item.reason) ? [{ ...item, name: actor.name }] : [];
  }) ?? [];
  const tooLong = Array.from(props.value.trim()).length > 20000;
  const empty = !props.value.trim();
  const noCandidates = props.actors.length === 0;
  return <div data-testid="creator-recommendations">
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      {props.children && <div className="min-w-0 max-w-full">{props.children}</div>}
      {/* Same contract as the description helper: while pending this button is the cancel control. */}
      <Button ref={button} type="button" variant="ghost" size="sm" className={`max-sm:min-h-11 pointer-coarse:min-h-11 ${AI_ASSIST_BUTTON_TONE}`} disabled={!pending && (empty || tooLong || noCandidates)}
        aria-describedby={pending ? undefined : noCandidates ? noCandidatesId : empty || tooLong ? hintId : undefined}
        onClick={() => { if (pending) clear(); else void start(); }}>
        {pending ? <LoaderCircle aria-hidden className="size-3.5 animate-spin motion-reduce:animate-none" /> : <Sparkles aria-hidden className="size-3.5" />}
        {pending ? t(($) => $.create_issue.creator_recommendations.cancel) : t(($) => $.create_issue.creator_recommendations.choose)}</Button>
      {noCandidates && <span id={noCandidatesId} className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.no_candidates)}</span>}
      {empty && !noCandidates && <span id={hintId} className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.empty_hint)}</span>}
      {pending && <span role="status" className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.loading)}</span>}
      {stale && !pending && <span role="status" className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.stale)}</span>}
      {result && <button type="button" aria-label={t(($) => $.create_issue.creator_recommendations.dismiss)} className="flex size-7 items-center justify-center rounded focus-visible:ring-2 focus-visible:ring-ring max-sm:size-11 pointer-coarse:size-11" onClick={() => { button.current?.focus(); clear(); }}><X aria-hidden className="size-3.5" /></button>}
    </div>
    {tooLong && <p id={hintId} className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.too_long)}</p>}
    {error && <div className="flex flex-wrap items-center gap-x-3 gap-y-1 py-1">
      <p role="alert" className="text-caption text-destructive">{error === "unavailable" ? t(($) => $.create_issue.creator_recommendations.unavailable)
        : error === "busy" ? t(($) => $.create_issue.creator_recommendations.busy) : error === "timeout" ? t(($) => $.create_issue.creator_recommendations.timeout) : t(($) => $.create_issue.creator_recommendations.failed)}</p>
      {/* Retrying unmounts this button, so park focus on the persistent one first. */}
      <Button type="button" variant="outline" size="sm" className="max-sm:min-h-11 pointer-coarse:min-h-11" disabled={empty || tooLong || noCandidates}
        onClick={() => { button.current?.focus(); void start(); }}>{t(($) => $.create_issue.creator_recommendations.retry)}</Button>
    </div>}
    {result && items.length === 0 && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.empty)}</p>}
    {result && items.length > 0 && <ul className="max-h-[min(9rem,18dvh)] overflow-y-auto py-1 sm:max-h-[min(13rem,25dvh)]">
      {items.map((item) => <li key={`${item.actor_type}:${item.actor_id}`} className="flex items-start justify-between gap-3 py-2">
        <div className="min-w-0"><p className="truncate text-body font-medium">{item.name} <span className="text-caption font-normal text-muted-foreground">{item.actor_type === "agent" ? t(($) => $.create_issue.actor_picker.agent) : t(($) => $.create_issue.actor_picker.squad)}</span></p>
          <p className="break-words text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.evidence)} <span>{item.reason}</span></p></div>
        <Button type="button" size="sm" variant="outline" className="max-sm:min-h-11 pointer-coarse:min-h-11" aria-label={t(($) => $.create_issue.creator_recommendations.use_named, { name: item.name })}
          onClick={() => {
            if (!matches(result.context, result.scope) || !latest.current.actors.some((actor) => actor.type === item.actor_type && actor.id === item.actor_id && actor.description === result.descriptions.get(actorKey(actor)) && actor.description.includes(item.reason))) { clear(true); return; }
            props.onPick({ type: item.actor_type, id: item.actor_id }); clear();
          }}>{t(($) => $.create_issue.creator_recommendations.use)}</Button>
      </li>)}
    </ul>}
  </div>;
}
