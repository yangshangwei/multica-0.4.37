"use client";

import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { Sparkles, X } from "lucide-react";
import { ApiError } from "@multica/core/api";
import { useRecommendIssueCreators } from "@multica/core/issues/mutations";
import { captureQuickCreateScope, isQuickCreateScopeCurrent, type QuickCreateActorRef, type QuickCreateScope } from "@multica/core/issues/stores/quick-create-store";
import type { IssueCreatorRecommendation } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import type { ContentEditorRef } from "../editor/content-editor";
import { actorKey, type ActorCatalogEntry } from "./quick-create-actor-picker-model";

type Context = { text: string; projectId: string | null; actorKey: string; identityKey: string };
type Request = { controller: AbortController; context: Context; scope: QuickCreateScope; descriptions: Map<string, string> };
type Result = { context: Context; scope: QuickCreateScope; descriptions: Map<string, string>; items: IssueCreatorRecommendation[] };
interface Props {
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
  const { mutateAsync, reset } = useRecommendIssueCreators(props.wsId);
  const latest = useRef(props);
  useLayoutEffect(() => { latest.current = props; });
  const request = useRef<Request | null>(null);
  const failureContext = useRef<Pick<Request, "context" | "scope"> | null>(null);
  const button = useRef<HTMLButtonElement>(null);
  const [pending, setPending] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const [error, setError] = useState<"unavailable" | "busy" | "timeout" | "failed" | null>(null);
  const textNow = () => (latest.current.editorRef.current?.getMarkdown() ?? latest.current.value).trim();
  const matches = (context: Context, scope: QuickCreateScope) => isQuickCreateScopeCurrent(scope)
    && context.text === textNow() && context.projectId === latest.current.projectId
    && context.actorKey === (latest.current.actor ? actorKey(latest.current.actor) : "")
    && context.identityKey === latest.current.identityKey;
  const clear = () => {
    request.current?.controller.abort(); request.current = null; failureContext.current = null;
    setPending(false); setResult(null); setError(null); reset();
  };
  useEffect(() => () => { request.current?.controller.abort(); request.current = null; reset(); }, [props.wsId, reset]);
  useEffect(() => {
    if ((request.current && !matches(request.current.context, request.current.scope)) || (result && !matches(result.context, result.scope)) || (failureContext.current && !matches(failureContext.current.context, failureContext.current.scope))) clear();
  });

  const start = async () => {
    if (request.current) return;
    latest.current.editorRef.current?.flushPendingUpdate();
    const text = textNow();
    if (!text || Array.from(text).length > 20000) return;
    const scope = captureQuickCreateScope(props.wsId);
    if (!scope) return;
    const active: Request = { controller: new AbortController(), scope, descriptions: new Map(props.actors.map((entry) => [actorKey(entry), entry.description])), context: {
      text, projectId: props.projectId, actorKey: props.actor ? actorKey(props.actor) : "", identityKey: props.identityKey,
    } };
    request.current = active; failureContext.current = null; props.onNeedsSpace?.(); setPending(true); setError(null); setResult(null);
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
  return <div className="mt-1" data-testid="creator-recommendations">
    <div className="flex items-center gap-2">
      <Button ref={button} type="button" variant="ghost" size="sm" disabled={!props.value.trim() || tooLong} aria-disabled={pending}
        onClick={() => { void start(); }}><Sparkles aria-hidden className="size-3.5" />{t(($) => $.create_issue.creator_recommendations.choose)}</Button>
      {pending && <><span role="status" className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.loading)}</span>
        <button type="button" className="text-caption underline" onClick={() => { button.current?.focus(); clear(); }}>{t(($) => $.create_issue.creator_recommendations.cancel)}</button></>}
      {result && <button type="button" aria-label={t(($) => $.create_issue.creator_recommendations.dismiss)} className="rounded p-1 focus-visible:ring-2 focus-visible:ring-ring" onClick={() => { button.current?.focus(); clear(); }}><X aria-hidden className="size-3.5" /></button>}
    </div>
    {tooLong && <p className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.too_long)}</p>}
    {error && <p role="alert" className="text-caption text-muted-foreground">{error === "unavailable" ? t(($) => $.create_issue.creator_recommendations.unavailable)
      : error === "busy" ? t(($) => $.create_issue.creator_recommendations.busy) : error === "timeout" ? t(($) => $.create_issue.creator_recommendations.timeout) : t(($) => $.create_issue.creator_recommendations.failed)}</p>}
    {result && items.length === 0 && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.empty)}</p>}
    {result && items.length > 0 && <ul className="max-h-[min(13rem,25dvh)] overflow-y-auto py-1">
      {items.map((item) => <li key={`${item.actor_type}:${item.actor_id}`} className="flex items-start justify-between gap-3 py-2">
        <div className="min-w-0"><p className="truncate text-body font-medium">{item.name} <span className="text-caption font-normal text-muted-foreground">{item.actor_type === "agent" ? t(($) => $.create_issue.actor_picker.agent) : t(($) => $.create_issue.actor_picker.squad)}</span></p>
          <p className="break-words text-caption text-muted-foreground">{t(($) => $.create_issue.creator_recommendations.evidence)} <span>{item.reason}</span></p></div>
        <Button type="button" size="sm" variant="outline" aria-label={t(($) => $.create_issue.creator_recommendations.use_named, { name: item.name })}
          onClick={() => {
            if (!matches(result.context, result.scope) || !latest.current.actors.some((actor) => actor.type === item.actor_type && actor.id === item.actor_id && actor.description === result.descriptions.get(actorKey(actor)) && actor.description.includes(item.reason))) { clear(); return; }
            props.onPick({ type: item.actor_type, id: item.actor_id }); clear();
          }}>{t(($) => $.create_issue.creator_recommendations.use)}</Button>
      </li>)}
    </ul>}
  </div>;
}
