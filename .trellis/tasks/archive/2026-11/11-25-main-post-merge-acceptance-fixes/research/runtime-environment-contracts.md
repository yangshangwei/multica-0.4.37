# Research: Namespace environment and generic upload authority

- Query: Is a compliant namespace acceptance environment presently established, and does generic comment upload inherit edit authority?
- Scope: mixed; current source, existing task contracts, read-only environment inventory and primary documentation
- Date: 2026-11-25

## Findings

### Namespace

The current host reports Darwin. Its Docker engine reports Linux/aarch64 with security options `name=seccomp,profile=builtin` and `name=cgroupns`. No task resource was started, modified or stopped for this assessment.

`server/internal/daemon/collaborationrun/controller_linux.go:27` creates USER/PID/MOUNT/IPC/UTS namespaces in one clone, maps the caller to namespace root, and disables setgroups. `preparation_linux.go:197` adds a network namespace for contained preparation. The supervisor then mounts its private root and applies additional restrictions. macOS does not implement that Linux controller.

The original cross-client fixture (`e2e/fixtures/collaboration-cross-client-execution.ts:260`) explicitly launches privileged containers with both seccomp and AppArmor unconfined. That exact configuration is prohibited for this task. Docker documents default seccomp restrictions on namespace-creating clone and unshare operations: [Docker seccomp profile](https://docs.docker.com/engine/security/seccomp/). Linux supports unprivileged user namespaces when the kernel and existing host policy permit them: [Linux user_namespaces(7)](https://man7.org/linux/man-pages/man7/user_namespaces.7.html). These explain the environmental requirements; neither identifies the historical Probe failure.

A compliant alternative would be a **pre-provisioned Linux VM or host whose unchanged policy already permits the actual controller's unprivileged user/mount/PID/network namespace probe**, running only task-owned fake providers and isolated resources. That changes the execution fixture/transport and requires fresh full nine-identity acceptance. It is not established by merely installing a Linux container or by Host-mode success. No such independent Linux environment is supplied or proven available in the observed task inventory. Do not add SYS_ADMIN, privileged, unconfined, change sysctls/LSM policy, or enter the Docker VM by an elevated container as a workaround.

Disposition: original nine remain policy-BLOCKED; prior default-policy helper_start errno1 remains evidence for that separate configuration only. Specific missing prerequisite is an already-permitting Linux namespace host plus an adapted safety-preserving fixture, not generic approval to weaken current policy.

### Generic UploadFile(comment_id)

Current ordinary upload behavior in `server/internal/handler/file.go:450–505` requires current workspace membership and a same-workspace comment, rejects collaboration comments in favor of the private draft flow, and creates independent uploader identity at `:456–465`. `:586–606` can publish comment-updated after attachment revision changes. It does not call comment-author/admin edit authorization. `packages/core/api/client.ts:5978` exposes this association as a separate upload option.

`handler/comment.go` UpdateComment protects both body and replacement attachment_ids through the comment-management gate. The upload association path can therefore change displayed comment attachments through a distinct current authority rule. That observation is material, but does not prove which intended product contract should win. Task PRD `11-20-comment-admin-human-authority/prd.md:39` explicitly leaves it as a separate contract question; its report and parent independent review preserve that boundary. The built-in working-on-issues source map explicitly calls it a separate existing contract. Search of documentation/built-in skills and owning upload tests found no explicit requirement granting every member append access to another author's comment or requiring edit-equivalent authority.

The product choice needed is whether ordinary comment attachment association is (A) comment editing, requiring the same current author/human admin or verified-task-author rules, or (B) an independent member contribution, requiring explicit separate uploader presentation/audit. Do not silently change either behavior, cite existing implementation as proof of intended policy, or mark the observation fixed. Parent owns user clarification and any implementation.

## Files found / related specs

- `e2e/fixtures/collaboration-cross-client-execution.ts`: original namespace container transport and prohibited flags.
- `server/internal/daemon/collaborationrun/controller_linux.go`, `preparation_linux.go`: actual namespace/mount requirements.
- `.trellis/spec/server/collaboration-host-execution.md`: Host and Linux guarantees remain distinct.
- `.trellis/tasks/11-25-acceptance-runtime/report.md`: exact nine blocked identities and separate default-policy probe.
- `server/internal/handler/file.go`, `comment.go`, `packages/core/api/client.ts`: current separate authority paths.
- `.trellis/tasks/11-20-comment-admin-human-authority/{prd,design,report}.md`: explicit deferred contract scope.
- `.trellis/spec/server/security-boundaries.md`, `collaboration-runtime-output.md`: verified task provenance and private collaboration output boundaries.

## Caveats / Not Found

No remote host, user account or VM was probed. Current Docker settings do not establish every filter rule or kernel setting; use the actual controller Probe on any newly supplied compliant host. No historical failure has been reattributed. No upload API behavior was changed. The product decision remains necessary.
