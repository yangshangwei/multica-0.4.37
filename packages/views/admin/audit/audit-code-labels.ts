// Codes emitted by managed_installation, platform_admin, platform_account,
// admin_alert and admin_operation in server/internal/service.
export const auditActionCodes = {
  "resource.publish": "resourcePublish",
  "resource.withdraw": "resourceWithdraw",
  "installation.enroll": "installationEnroll",
  "installation.bind": "installationBind",
  "installation.renew": "installationRenew",
  "installation.admission": "installationAdmission",
  "task.cancel": "taskCancel",
  "user.role": "userRole",
  "user.role.bootstrap": "userRoleBootstrap",
  "user.disable": "userDisable",
  "user.restore": "userRestore",
  "user.password.recover": "userPasswordRecover",
  "alert.acknowledge": "alertAcknowledge",
  "alert.assign": "alertAssign",
  "alert.close": "alertClose",
} as const;

export const auditResultCodes = {
  resource_requested: { label: "resourceRequested", tone: "statusInfo" },
  resource_publish_applied: { label: "resourcePublishApplied", tone: "statusSuccess" },
  resource_withdraw_applied: { label: "resourceWithdrawApplied", tone: "statusNeutral" },
  applied: { label: "applied", tone: "statusInfo" },
  role_changed: { label: "roleChanged", tone: "statusSuccess" },
  account_disabled: { label: "accountDisabled", tone: "statusNeutral" },
  account_restored: { label: "accountRestored", tone: "statusSuccess" },
  password_recovered: { label: "passwordRecovered", tone: "statusSuccess" },
  admission_stopped: { label: "admissionStopped", tone: "statusNeutral" },
  admission_accepting: { label: "admissionAccepting", tone: "statusSuccess" },
  alert_acknowledged: { label: "alertAcknowledged", tone: "statusInfo" },
  alert_assigned: { label: "alertAssigned", tone: "statusInfo" },
  alert_closed: { label: "alertClosed", tone: "statusNeutral" },
  cancellation_accepted: { label: "cancellationAccepted", tone: "statusInfo" },
  cancelled_before_dispatch: { label: "cancelledBeforeDispatch", tone: "statusNeutral" },
  awaiting_daemon_confirmation: { label: "awaitingDaemonConfirmation", tone: "statusWarning" },
  confirmation_unavailable: { label: "confirmationUnavailable", tone: "statusWarning" },
  already_cancelled_unverified: { label: "alreadyCancelledUnverified", tone: "statusWarning" },
  already_terminal: { label: "alreadyTerminal", tone: "statusNeutral" },
  execution_fence_conflict: { label: "executionFenceConflict", tone: "statusDanger" },
} as const;

export const auditPhaseCodes = {
  requested: "request",
  request: "request",
  applied: "applied",
  failed: "failed",
  succeeded: "succeeded",
  unconfirmed: "unconfirmed",
} as const;

export const auditTargetKinds = {
  resource: "resource",
  installation: "installation",
  task: "task",
  user: "user",
  alert: "alert",
} as const;

// Unknown server codes must remain visible, including object-property names.
export function auditCodeEntry<T extends Record<string, unknown>>(codes: T, code: string): T[keyof T] | undefined {
  return Object.hasOwn(codes, code) ? codes[code as keyof T] : undefined;
}
