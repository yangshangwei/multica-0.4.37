"use client";

import { useRef, useState } from "react";
import {
  adminApiScope, useAdminAccess, useAdminUser,
  type AdminAccountAction, type AdminUser,
} from "@multica/core/admin";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import { AdminUserActionForm } from "./user-action-form";

export function AdminUserDetailPage({ id }: { id: string }) {
  const { t } = useT("admin");
  const { identity } = useAdminAccess();
  const scope = { apiScope: adminApiScope(), userId: identity?.userId ?? "", organizationId: identity?.organizationId ?? null };
  const query = useAdminUser(scope, id);
  const [selected, setSelected] = useState<{ action: AdminAccountAction | "role"; user: AdminUser } | null>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  if (query.isPending) return <p role="status">{t(($) => $.state.loading)}</p>;
  if (query.isError || !query.data) return <div role="alert" className="space-y-3"><p>{t(($) => $.users.loadError)}</p><Button variant="outline" onClick={() => void query.refetch()}>{t(($) => $.users.retry)}</Button></div>;
  const { user, memberships } = query.data;
  const statuses = { active: t(($) => $.users.statusActive), disabled: t(($) => $.users.statusDisabled), setup_required: t(($) => $.users.statusSetup), password_change_required: t(($) => $.users.statusChange), unknown: t(($) => $.users.statusUnknown) };
  const actions: { key: AdminAccountAction | "role"; label: string }[] = [
    { key: "disable", label: t(($) => $.users.disable) }, { key: "restore", label: t(($) => $.users.restore) },
    { key: "recover-password", label: t(($) => $.users.recover) }, { key: "role", label: t(($) => $.users.roleAction) },
  ];
  const available = identity?.role === "super_admin" && user.status !== "unknown" ? actions.filter((action) => user.allowedActions.includes(action.key)) : [];
  return (
    <section className="space-y-6">
      <AppLink href="/admin/users" className="text-body text-muted-foreground underline-offset-4 hover:underline">{t(($) => $.users.back)}</AppLink>
      <div className="space-y-2"><h1 className="break-words text-title font-semibold">{user.name}</h1><p className="text-body text-muted-foreground">{user.username ?? t(($) => $.users.statusSetup)} · {statuses[user.status]}</p></div>
      <dl className="grid gap-4 sm:grid-cols-3">
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.role)}</dt><dd className="text-body">{user.platformRole === "super_admin" ? t(($) => $.users.superAdmin) : user.platformRole === "platform_observer" ? t(($) => $.users.observer) : t(($) => $.users.noRole)}</dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.created)}</dt><dd className="text-body"><time dateTime={user.createdAt}>{new Date(user.createdAt).toLocaleString()}</time></dd></div>
        <div><dt className="text-caption text-muted-foreground">{t(($) => $.users.workspaces)}</dt><dd className="text-body tabular-nums">{user.workspaceCount}</dd></div>
      </dl>
      <div className="space-y-3"><h2 className="text-body-lg font-semibold">{t(($) => $.users.actions)}</h2>{available.length ? <div className="flex flex-wrap gap-3">{available.map((action) => <Button key={action.key} variant="outline" disabled={selected !== null} onClick={(event) => { trigger.current = event.currentTarget; setSelected({ action: action.key, user }); }}>{action.label}</Button>)}</div> : <p className="text-body text-muted-foreground">{t(($) => $.users.noActions)}</p>}</div>
      {selected && <AdminUserActionForm key={`${selected.user.id}:${selected.action}`} user={selected.user} action={selected.action} scope={scope} onRefresh={() => void query.refetch()} onClose={() => { setSelected(null); requestAnimationFrame(() => trigger.current?.focus()); }} />}
      <section className="space-y-3"><h2 className="text-body-lg font-semibold">{t(($) => $.users.workspaces)}</h2>{memberships.length ? <ul className="divide-y divide-surface-border">{memberships.map((membership) => <li key={membership.workspaceId} className="flex flex-wrap justify-between gap-2 py-3 text-body"><span className="break-words">{membership.workspaceName}</span><span className="text-muted-foreground">{membership.role}</span></li>)}</ul> : <p className="text-body text-muted-foreground">{t(($) => $.users.noMemberships)}</p>}{query.data.membershipsTruncated && <p className="text-caption text-muted-foreground">{t(($) => $.users.truncated)}</p>}</section>
    </section>
  );
}
