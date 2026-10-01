import { z } from "zod";
import { parseWithFallback } from "../api/schema";

const AdminMeSchema = z.object({
  user_id: z.uuid(),
  organization_id: z.uuid(),
  role: z.enum(["super_admin", "platform_observer"]),
  allowed_actions: z.array(z.string().min(1)),
  supported: z.literal(true),
}).transform((value) => ({
  userId: value.user_id,
  organizationId: value.organization_id,
  role: value.role,
  allowedActions: value.allowed_actions,
  supported: value.supported,
}));

export type AdminIdentity = z.output<typeof AdminMeSchema>;

export function parseAdminMe(raw: unknown): AdminIdentity | null {
  return parseWithFallback<AdminIdentity | null>(raw, AdminMeSchema, null, {
    endpoint: "GET /api/admin/me",
    redact: true,
  });
}
