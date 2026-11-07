import { z } from "zod";

const PersonalAccessTokenSchema = z.object({
  id: z.string().trim().min(1),
  name: z.string(),
  token_prefix: z.string(),
  created_at: z.string(),
  expires_at: z.string().nullable().optional().default(null),
  last_used_at: z.string().nullable().optional().default(null),
});

// A list never exposes credentials, even if a newer server includes them.
export const PersonalAccessTokenListSchema = z.array(PersonalAccessTokenSchema);
export const CreatePersonalAccessTokenResponseSchema = PersonalAccessTokenSchema.extend({
  token: z.string().refine((value) => value.trim().length > 0),
});
