import { queryOptions, useMutation } from "@tanstack/react-query";
import { api } from "../api";
import { withQueryAccessBoundary } from "../query-access";
import type { CreatePersonalAccessTokenRequest } from "../types";

export const personalAccessTokensOptions = (userId: string | null) => queryOptions({
  queryKey: ["personal-access-tokens", userId],
  queryFn: withQueryAccessBoundary(() => api.listPersonalAccessTokens()),
  enabled: !!userId,
});

export function useCreatePersonalAccessToken(onCreated: (token: string) => void) {
  return useMutation({
    mutationFn: async (request: CreatePersonalAccessTokenRequest): Promise<void> => {
      const result = await api.createPersonalAccessToken(request);
      // Deliver the one-time secret locally; mutation data must never retain it.
      onCreated(result.token);
    },
    // A failed response can still mean the server created the token.
    retry: false,
    // Do not queue credential writes for a later, potentially different session.
    networkMode: "always",
  });
}

export function useRevokePersonalAccessToken() {
  return useMutation({
    mutationFn: (id: string) => api.revokePersonalAccessToken(id),
    retry: false,
    networkMode: "always",
  });
}
