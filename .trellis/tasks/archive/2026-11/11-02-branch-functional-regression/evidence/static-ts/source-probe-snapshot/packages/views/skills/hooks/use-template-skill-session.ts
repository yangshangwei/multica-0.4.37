"use client";

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api, ApiError, SkillCreationUnconfirmedError } from "@multica/core/api";
import { useAuthStore } from "@multica/core/auth";
import {
  buildSkillTemplateCreateRequest,
  createSkillTemplateDraft,
  type SkillTemplateDraft,
} from "@multica/core/skills";
import type { CreateSkillRequest, Skill, SkillTemplate } from "@multica/core/types";
import { cacheSkillResponse } from "@multica/core/workspace/queries";

const REQUEST_TIMEOUT_MS = 30_000;

type SessionError =
  | "invalid_template"
  | "create_failed"
  | "conflict"
  | "result_unknown"
  | "timeout_unknown"
  | "no_confirmed_result";

interface Submission {
  workspaceId: string;
  userId: string | null;
  request: CreateSkillRequest;
}

export interface SkillCreationCandidate {
  skill: Skill;
  matches: boolean;
  matchesDraft: boolean;
}

interface TemplateState {
  step: "picker" | "editor";
  previewName: string | null;
  draft: SkillTemplateDraft | null;
  baseline: string;
  contentMode: "edit" | "preview";
  status: "idle" | "submitting" | "uncertain" | "checking";
  error: SessionError | null;
  submission: Submission | null;
  hasUnconfirmedSubmission: boolean;
  candidates: SkillCreationCandidate[];
}

function initialState(): TemplateState {
  return {
    step: "picker", previewName: null, draft: null, baseline: "",
    contentMode: "edit", status: "idle", error: null,
    submission: null, hasUnconfirmedSubmission: false, candidates: [],
  };
}

function draftSignature(draft: SkillTemplateDraft): string {
  return JSON.stringify([draft.name.trim(), draft.description, draft.body]);
}

function fileSignature(files: { path: string; content: string }[]): string {
  return JSON.stringify(files.map(({ path, content }) => ({ path, content }))
    .sort((a, b) => a.path.localeCompare(b.path)));
}

function hasIdentity(skill: Skill, workspaceId: string): boolean {
  return typeof skill?.id === "string" && !!skill.id.trim()
    && skill.workspace_id === workspaceId;
}

function matchesRequest(skill: Skill, request: CreateSkillRequest): boolean {
  return skill.name === request.name
    && skill.description === request.description
    && skill.content === request.content
    && fileSignature(skill.files ?? []) === fileSignature(request.files ?? []);
}

function matchesSubmission(skill: Skill, submission: Submission): boolean {
  return !!submission.userId && skill.created_by === submission.userId
    && matchesRequest(skill, submission.request);
}

/** The root dialog calls this hook so returning to the chooser retains the draft. */
export function useTemplateSkillSession(
  workspaceId: string,
  onCreated: (skill: Skill, newlyCreated: boolean) => void,
) {
  const qc = useQueryClient();
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const [state, setState] = useState<TemplateState>(initialState);
  const currentWorkspace = useRef(workspaceId);
  currentWorkspace.current = workspaceId;
  const onCreatedRef = useRef(onCreated);
  onCreatedRef.current = onCreated;
  const mounted = useRef(true);
  const generation = useRef(0);
  const busy = useRef(false);
  const controller = useRef<AbortController | null>(null);
  const timeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    mounted.current = true;
    setState(initialState());
    busy.current = false;
    return () => {
      mounted.current = false;
      generation.current += 1;
      controller.current?.abort();
      if (timeout.current) clearTimeout(timeout.current);
      busy.current = false;
    };
  }, [workspaceId]);

  const isCurrent = (id: number, origin: string) => mounted.current
    && generation.current === id && currentWorkspace.current === origin;

  const beginRequest = (status: "submitting" | "checking") => {
    const id = ++generation.current;
    const abort = new AbortController();
    controller.current = abort;
    busy.current = true;
    setState((previous) => ({ ...previous, status, error: null, candidates: [] }));
    const timer = setTimeout(() => {
      if (!isCurrent(id, workspaceId)) return;
      generation.current += 1;
      abort.abort();
      busy.current = false;
      setState((previous) => ({ ...previous, status: "uncertain", hasUnconfirmedSubmission: true, error: "timeout_unknown" }));
    }, REQUEST_TIMEOUT_MS);
    timeout.current = timer;
    return { id, abort, timer };
  };

  const finishRequest = (id: number, timer: ReturnType<typeof setTimeout>) => {
    clearTimeout(timer);
    if (generation.current === id) {
      busy.current = false;
      controller.current = null;
      timeout.current = null;
    }
  };

  const useTemplate = (template: SkillTemplate, names: readonly string[], description: string) => {
    if (busy.current) return;
    if (state.draft?.templateName === template.name) {
      setState((previous) => ({ ...previous, step: "editor" }));
      return;
    }
    try {
      const draft = createSkillTemplateDraft(template, names, description);
      setState({ ...initialState(), step: "editor", previewName: template.name, draft, baseline: draftSignature(draft) });
    } catch {
      setState((previous) => ({ ...previous, error: "invalid_template" }));
    }
  };

  const updateDraft = (field: "name" | "description" | "body", value: string) => {
    if (busy.current || state.status === "uncertain") return;
    setState((previous) => previous.draft
      ? { ...previous, draft: { ...previous.draft, [field]: value }, error: null }
      : previous);
  };

  const submit = async (reservedNames: readonly string[]) => {
    if (busy.current || state.status !== "idle" || !state.draft) return;
    if (reservedNames.includes(state.draft.name.trim())) return;
    let request: CreateSkillRequest;
    try {
      request = buildSkillTemplateCreateRequest(state.draft);
    } catch {
      setState((previous) => ({ ...previous, error: "invalid_template" }));
      return;
    }
    const submission: Submission = { workspaceId, userId, request };
    setState((previous) => ({ ...previous, submission }));
    const { id, abort, timer } = beginRequest("submitting");
    try {
      const skill = await api.createSkill(request, { workspaceId, signal: abort.signal });
      if (!hasIdentity(skill, workspaceId)) throw new SkillCreationUnconfirmedError();
      // A late valid response can refresh its original workspace, but cannot
      // close a different dialog or overwrite a draft resumed after a timeout.
      cacheSkillResponse(qc, workspaceId, skill);
      if (!isCurrent(id, workspaceId)) return;
      setState(initialState());
      onCreatedRef.current(skill, true);
    } catch (error) {
      if (!isCurrent(id, workspaceId)) return;
      const rejected = error instanceof ApiError && error.status >= 400
        && error.status < 500 && error.status !== 408;
      setState((previous) => ({
        ...previous,
        status: rejected ? "idle" : "uncertain",
        hasUnconfirmedSubmission: previous.hasUnconfirmedSubmission || !rejected,
        error: rejected
          ? error.status === 409 ? "conflict" : "create_failed"
          : "result_unknown",
      }));
    } finally {
      finishRequest(id, timer);
    }
  };

  const checkResult = async () => {
    if (busy.current || !state.submission) return;
    const submission = state.submission;
    if (submission.workspaceId !== workspaceId) return;
    const { id, abort, timer } = beginRequest("checking");
    try {
      const options = { workspaceId: submission.workspaceId, signal: abort.signal };
      const skills = await api.listSkills(options);
      const sameName = skills.filter((skill) => skill.name === submission.request.name && skill.id);
      const details = await Promise.all(sameName.map((skill) => api.getSkill(skill.id, options)));
      if (!isCurrent(id, workspaceId)) return;
      let currentDraft: CreateSkillRequest | null = null;
      try {
        if (state.draft) currentDraft = buildSkillTemplateCreateRequest(state.draft);
      } catch {
        // An incomplete resumed draft must still survive inspecting an older result.
      }
      const candidates = details.filter((skill) => hasIdentity(skill, submission.workspaceId))
        .map((skill) => ({
          skill,
          matches: matchesSubmission(skill, submission),
          matchesDraft: !!currentDraft && matchesRequest(skill, currentDraft),
        }));
      // Even an exact match is offered for explicit inspection; a name alone
      // does not identify which request created a record.
      setState((previous) => ({ ...previous, status: "uncertain", candidates, error: candidates.length ? null : "no_confirmed_result" }));
    } catch {
      if (isCurrent(id, workspaceId)) {
        setState((previous) => ({ ...previous, status: "uncertain", error: "no_confirmed_result" }));
      }
    } finally {
      finishRequest(id, timer);
    }
  };

  const reset = () => {
    generation.current += 1;
    controller.current?.abort();
    if (timeout.current) clearTimeout(timeout.current);
    busy.current = false;
    setState(initialState());
  };

  return {
    ...state,
    busy: state.status === "submitting" || state.status === "checking",
    dirty: !!state.draft && draftSignature(state.draft) !== state.baseline,
    useTemplate,
    updateDraft,
    submit,
    checkResult,
    reset,
    preview: (name: string) => setState((previous) => ({ ...previous, previewName: name })),
    backToTemplates: () => setState((previous) => ({ ...previous, step: "picker" })),
    setContentMode: (contentMode: "edit" | "preview") => setState((previous) => ({ ...previous, contentMode })),
    continueEditing: () => setState((previous) => ({ ...previous, status: "idle", error: null, candidates: [] })),
    openCandidate: (skill: Skill) => {
      if (!hasIdentity(skill, workspaceId)) return;
      cacheSkillResponse(qc, workspaceId, skill);
      reset();
      onCreatedRef.current(skill, false);
    },
  };
}

export type TemplateSkillSession = ReturnType<typeof useTemplateSkillSession>;
