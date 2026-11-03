import { z } from "zod";

/** Shared account projection leaf; schemas must not import request modules. */
export const CollaborationAccountSummarySchema = z.object({
  user_id: z.string().uuid(),
  name: z.string().catch("Unavailable account"),
  avatar_url: z.string().catch(""),
  external: z.boolean(),
});
