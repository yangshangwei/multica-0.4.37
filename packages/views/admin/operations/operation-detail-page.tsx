"use client";
import { adminApiScope, useAdminAccess } from "@multica/core/admin";
import { AdminOperationReceipt } from "./operation-receipt";
export function AdminOperationDetailPage({ id }: { id: string }) {
  const access = useAdminAccess();
  if (access.status !== "ready" || !access.identity) return null;
  const scope = { apiScope: adminApiScope(), userId: access.identity.userId, organizationId: access.identity.organizationId };
  return <AdminOperationReceipt key={id} scope={scope} id={id} detail />;
}
