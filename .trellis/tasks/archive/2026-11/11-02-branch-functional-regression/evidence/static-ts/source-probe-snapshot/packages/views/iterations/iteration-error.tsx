"use client";
import { ApiError, isIterationAccessDenied } from "@multica/core/api";
import { useT } from "../i18n";

export function isDefinitiveReadError(error: unknown): boolean {
  return error instanceof ApiError && error.status < 500 && ![408, 429].includes(error.status);
}

export function IterationError({ error, context = "operation" }: { error: unknown; context?: "read" | "operation" }) {
  const { t } = useT("projects");
  const status = error instanceof ApiError ? error.status : null;
  return (
    <p role="alert">
      {t(($) =>
        isIterationAccessDenied(error)
          ? context === "read" ? $.iterations.pages.accessDenied : $.iterations.permission
          : context === "read"
            ? status === 404 ? $.iterations.pages.notFound : $.iterations.pages.readError
            : status === 409
            ? $.iterations.conflict
            : status === 413
              ? $.iterations.tooLarge
              : $.iterations.error,
      )}
    </p>
  );
}
