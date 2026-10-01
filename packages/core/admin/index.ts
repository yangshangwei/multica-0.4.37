export type { AdminIdentity } from "./schema";
export { parseAdminMe } from "./schema";
export {
  adminApiScope,
  adminKeys,
  adminMeOptions,
  AdminUnsupportedError,
  clearAdminCache,
  isAdminKey,
  isAdminPermissionDenied,
  retainAdminScope,
} from "./queries";
export type { AdminScope } from "./queries";
export { useAdminAccess } from "./use-admin-access";
