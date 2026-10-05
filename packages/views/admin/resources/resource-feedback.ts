import { ApiError, errorCode } from "@multica/core/api";
import { AdminResourceUnsupportedError } from "@multica/core/admin";

export function resourceErrorKey(error: unknown) {
  if (error instanceof AdminResourceUnsupportedError) return "unsupported";
  switch (errorCode(error)) {
    case "admin_forbidden": return "forbidden";
    case "resource_invalid": return "invalid";
    case "resource_changed": case "resource_conflict": case "resource_idempotency_conflict": case "resource_not_found": return "conflict";
    case "resource_preview_changed": return "previewChanged";
    case "resource_publishing_disabled": return "disabled";
    case "resource_store_full": return "storeFull";
    case "resource_store_unavailable": return "unavailable";
    default: return error instanceof ApiError && [404, 405].includes(error.status) ? "unsupported" : "failed";
  }
}
