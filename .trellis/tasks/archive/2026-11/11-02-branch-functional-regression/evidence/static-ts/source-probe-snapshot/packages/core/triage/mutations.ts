import { useMutation, useQueryClient, type MutateOptions } from "@tanstack/react-query";
import { api } from "../api";
import type {
  CreateTriageItemInput, TriageActionInput, TriageBatchInput, TriageBatchPreviewInput,
  TriageImportCommitInput, TriageImportPreviewInput, UpdateTriageSettingsInput,
} from "../types/triage";
import { triageKeys } from "./queries";
import { cacheTriageItem, invalidateTriage } from "./cache";

interface TriageCommand<TInput> { workspaceId: string; input: TInput }

/** Capture scope in each invocation: TanStack can replace a paused mutation's
 * options after navigation, before the transport function has ever run. */
function useScopedTriageMutation<TData, TInput>(wsId: string, options: {
  mutationFn: (workspaceId: string, input: TInput) => Promise<TData>;
  onSuccess?: (data: TData, workspaceId: string) => void;
  onSettled?: (workspaceId: string) => void;
}) {
  const mutation = useMutation<TData, Error, TriageCommand<TInput>>({
    mutationFn: command => options.mutationFn(command.workspaceId, command.input),
    onSuccess: (data, command) => options.onSuccess?.(data, command.workspaceId),
    onSettled: (_data, _error, command) => options.onSettled?.(command.workspaceId),
  });
  // Preserve the public mutation API: UI callbacks and variables see the user's
  // original input, while the internal command keeps transport/cache scope fixed.
  function callbacks(value?: MutateOptions<TData, Error, TInput>): MutateOptions<TData, Error, TriageCommand<TInput>> | undefined {
    if (!value) return undefined;
    return {
      onSuccess: (data, command, result, context) => value.onSuccess?.(data, command.input, result, context),
      onError: (error, command, result, context) => value.onError?.(error, command.input, result, context),
      onSettled: (data, error, command, result, context) => value.onSettled?.(data, error, command.input, result, context),
    };
  }
  return {
    ...mutation,
    variables: mutation.variables?.input,
    mutate: (input: TInput, value?: MutateOptions<TData, Error, TInput>) => mutation.mutate({ workspaceId: wsId, input }, callbacks(value)),
    mutateAsync: (input: TInput, value?: MutateOptions<TData, Error, TInput>) => mutation.mutateAsync({ workspaceId: wsId, input }, callbacks(value)),
  };
}

// Decisions/imports are never optimistic. The caller retains request IDs on
// transport retries; authoritative receipts are the only success signal.
export function useUpdateTriageSettings(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, input: UpdateTriageSettingsInput) => api.updateTriageSettings(scope, input),
    onSuccess: (settings, scope) => { qc.setQueryData(triageKeys.settings(scope), settings); },
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function useCreateTriageItem(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, input: CreateTriageItemInput) => api.createTriageItem(scope, input),
    onSuccess: (item, scope) => cacheTriageItem(qc, scope, item),
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function useTriageAction(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, { id, input }: { id: string; input: TriageActionInput }) => api.performTriageAction(scope, id, input),
    onSuccess: (result, scope) => cacheTriageItem(qc, scope, result.item),
    // Refetch conflicts and uncertain transport outcomes, preserving the draft.
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function useRetryTriageExecution(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, actionId: string) => api.retryTriageExecution(scope, actionId),
    onSuccess: (result, scope) => cacheTriageItem(qc, scope, result.item),
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function usePreviewTriageBatch(wsId: string) {
  return useScopedTriageMutation(wsId, { mutationFn: (scope, input: TriageBatchPreviewInput) => api.previewTriageBatch(scope, input) });
}
export function useCommitTriageBatch(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, input: TriageBatchInput) => api.commitTriageBatch(scope, input),
    onSuccess: (response, scope) => {
      for (const row of response.results) {
        if (row.status === "success" && row.result) cacheTriageItem(qc, scope, row.result.item);
      }
    },
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function usePreviewTriageImport(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, input: TriageImportPreviewInput) => api.previewTriageImport(scope, input),
    onSuccess: (preview, scope) => { qc.setQueryData(triageKeys.import(scope, preview.batch_id), preview); },
  });
}
export function useCommitTriageImport(wsId: string) {
  const qc = useQueryClient();
  return useScopedTriageMutation(wsId, {
    mutationFn: (scope, { id, input }: { id: string; input: TriageImportCommitInput }) => api.commitTriageImport(scope, id, input),
    onSettled: scope => invalidateTriage(qc, scope),
  });
}
export function useDownloadTriageFailures(wsId: string) {
  return useScopedTriageMutation(wsId, { mutationFn: (scope, id: string) => api.downloadTriageFailures(scope, id) });
}
