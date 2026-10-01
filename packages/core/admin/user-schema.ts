import { z } from "zod";
import { parseWithFallback } from "../api/schema";

const UserSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  username: z.string().nullable(),
  platform_role: z.enum(["super_admin", "platform_observer"]).nullable().catch(null),
  status: z.enum(["active", "disabled", "setup_required", "password_change_required", "unknown"]).catch("unknown"),
  auth_version: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  workspace_count: z.number().int().nonnegative(),
  created_at: z.string(),
  allowed_actions: z.array(z.string()),
}).transform((user) => ({
  id: user.id,
  name: user.name,
  username: user.username,
  platformRole: user.platform_role,
  status: user.status,
  authVersion: user.auth_version,
  workspaceCount: user.workspace_count,
  createdAt: user.created_at,
  allowedActions: user.allowed_actions,
}));

const UsersSchema = z.object({
  items: z.array(UserSchema),
  next_cursor: z.string().nullable(),
  scope: z.uuid(),
  as_of: z.string(),
  data_quality: z.enum(["complete", "partial"]).catch("partial"),
  registration: z.object({ enabled: z.boolean(), approval_required: z.literal(false) }),
}).transform((list) => ({
  items: list.items,
  nextCursor: list.next_cursor,
  scope: list.scope,
  asOf: list.as_of,
  dataQuality: list.data_quality,
  registration: { enabled: list.registration.enabled, approvalRequired: list.registration.approval_required },
}));

const UserDetailSchema = z.object({
  user: UserSchema,
  memberships: z.array(z.object({ workspace_id: z.uuid(), workspace_name: z.string(), role: z.string() }).transform((membership) => ({
    workspaceId: membership.workspace_id,
    workspaceName: membership.workspace_name,
    role: membership.role,
  }))),
  memberships_truncated: z.boolean().default(false),
  scope: z.uuid(),
}).transform((detail) => ({
  user: detail.user,
  memberships: detail.memberships,
  membershipsTruncated: detail.memberships_truncated,
  scope: detail.scope,
}));

const OperationSchema = z.object({
  id: z.uuid(),
  organization_id: z.uuid(),
  target_id: z.uuid(),
  kind: z.string(),
  state: z.string(),
  result_code: z.string(),
  confirmation: z.string(),
  version: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
}).transform((operation) => ({
  id: operation.id,
  organizationId: operation.organization_id,
  targetId: operation.target_id,
  kind: operation.kind,
  state: operation.state,
  resultCode: operation.result_code,
  confirmation: operation.confirmation,
  version: operation.version,
}));
const OperationsSchema = z.object({ items: z.array(OperationSchema).max(1), scope: z.uuid(), as_of: z.string() })
  .transform((list) => ({ items: list.items, scope: list.scope, asOf: list.as_of }));

export type AdminUser = z.output<typeof UserSchema>;
export type AdminUserList = z.output<typeof UsersSchema>;
export type AdminUserDetail = z.output<typeof UserDetailSchema>;
export type AdminUserOperation = z.output<typeof OperationSchema>;
export type AdminUserOperations = z.output<typeof OperationsSchema>;

export interface AdminUserFilters {
  q?: string;
  status?: string;
  role?: string;
  timeFrom?: string;
  timeTo?: string;
  cursor?: string;
  limit?: number;
  timezone?: string;
}
export type AdminAccountAction = "disable" | "restore" | "recover-password";
export interface AdminAccountChange {
  expectedAuthVersion: number;
  reason: string;
  password: string;
  temporaryPassword?: string;
  username?: string;
}
export interface AdminRoleChange {
  role: "super_admin" | "platform_observer" | null;
  expectedRole: "super_admin" | "platform_observer" | null;
  expectedAuthVersion: number;
  reason: string;
  password: string;
}

export function parseAdminUsers(raw: unknown): AdminUserList | null {
  return parseWithFallback<AdminUserList | null>(raw, UsersSchema, null, { endpoint: "GET /api/admin/users", redact: true });
}
export function parseAdminUserDetail(raw: unknown): AdminUserDetail | null {
  return parseWithFallback<AdminUserDetail | null>(raw, UserDetailSchema, null, { endpoint: "GET /api/admin/users/:id", redact: true });
}
export function parseAdminUserOperation(raw: unknown): AdminUserOperation | null {
  return parseWithFallback<AdminUserOperation | null>(raw, OperationSchema, null, { endpoint: "POST /api/admin/users/:id/action", redact: true });
}
export function parseAdminUserOperations(raw: unknown): AdminUserOperations | null {
  return parseWithFallback<AdminUserOperations | null>(raw, OperationsSchema, null, { endpoint: "GET /api/admin/operations", redact: true });
}
