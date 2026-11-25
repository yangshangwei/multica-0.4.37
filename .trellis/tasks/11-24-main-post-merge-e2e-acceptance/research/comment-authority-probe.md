# Current comment authority HTTP probe preparation

Date: 2026-11-24. Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

Status: **ready for independent review**, not live acceptance. No real API, SQL, Docker, Agent CLI or model request was executed by this preparation.

## Current-source evidence

- `server/internal/handler/comment.go:3296` loads the current-workspace comment and resolves the actor. At `:3333`, editing authorization still accepts `member.Role` owner/admin without separately requiring a human principal.
- `server/internal/handler/comment.go:3556` uses the same pattern for deletion, with the owner/admin branch at `:3594`.
- `server/cmd/server/router.go:2534` mounts ordinary `PUT /api/comments/{commentId}` and `DELETE /api/comments/{commentId}` without `RequireHumanActor`. This is distinct from the collaboration comment routes and must not be inferred from their stricter contract.
- `server/internal/middleware/auth.go:115` verifies real `mat_` token rows, forces user/agent/task/workspace headers from them, and stamps `X-Actor-Source: task_token`. Password-mode authentication follows the same task source in `middleware/password_auth.go:51`.
- `handler/actor_guards.go` and `.trellis/spec/server/security-boundaries.md` classify verified `mat_` / `mcn_` principals as machines; a legitimate `mul_` PAT remains human-equivalent, including when a daemon process happens to hold it. Process naming is not a credential-type witness.
- `handler/handler.go:952` trusts the authoritative task-token source for the agent author. An unchanged original comment author therefore does not prove its content was changed by that author.
- `handler/daemon.go:3646` mints an agent task credential through the real claim transaction; `:3930` starts it through the actual daemon endpoint. The probe uses these paths, not a fabricated JWT/token or direct fixture insertion.
- `handler/task_capability.go:27` retains the ordinary untemplated-agent path when no capability profile exists. The probe creates a manual Agent rather than a restricted Reporter/Reviewer template so a template refusal cannot be misreported as the comment administrative boundary.

These facts establish a current risk to test. They are **not** a live reproduction or a claim that the merge introduced the issue. The older product task `11-20-comment-admin-human-authority` remains planning and is not modified by this QA task.

## Additional upload observation

`handler/file.go:494` checks that a supplied `comment_id` exists in the same workspace, then attaches the managed file. No comment-author/administrator check is visible on this path. The attachment insert returns an incremented comment revision and publishes an updated comment.

The parent explicitly authorized a separate live observation of this entrance. The probe therefore checks ordinary-member and owner/admin-bound task callers uploading a tiny file to another author's comment. A successful unauthorized-looking mutation is **OBSERVED_RISK**, with real response and before/after state, rather than an invented upload-specific normative PASS/FAIL. A legitimate human administrator upload and a machine upload to its own comment first establish controls.

## Prepared files

- Private program: `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/probes/comment_authority.py`.
- Offline counterexamples: same directory `comment_authority_selftest.py`.
- Non-authorizing review template: same directory `comment_authority.review-template.json`.
- Current preparation evidence: `artifacts/qa/main-post-merge-e2e-20261124-080504/probes/prepare-selftests-02.json` and `.log`.
- The first preparation log/receipt is retained as `prepare-selftests-01.*`; subsequent control/observation refinements have new fingerprints.

Current program SHA-256: `15a0e3c03bb3a7d52301fd8fbee07ac53216405aece6307bcab7915ad38ba5ff`.

Selftest SHA-256: `cd90b39b93042dd713d1b502effccc3aeb6312a3a415167c44e0a8a2e87431af`.

## Credential and fixture flow

The legacy lane registers three new synthetic email-code accounts using the lane's configured verification code. It completes onboarding through the actual APIs, creates one workspace, and invites/accepts admin/member accounts through the public invitation endpoints. A real member-list read verifies the granted roles.

Each owner/admin actor registers a single HTTP-simulated Codex runtime, creates a manually authored Agent and an assigned ordinary issue, claims the new queued task, and starts it. Claim identity is verified against that issue's task list. Its API-issued `mat_` credential must be refused by the human-only `/api/tokens` reader, must create a comment attributed to its bound Agent, and must edit/delete its own comment through legitimate controls. No provider executable is launched.

Human JWT and API-minted human `mul_` PAT controls edit and delete another actor's comments without changing the author and with the expected revision increment. Ordinary members must not edit/delete an owner's comment. The owner/admin-bound task credentials are then checked against another human author's comment for:

1. Content editing with exact current `content_base` and `expected_revision`.
2. Deletion.
3. Attachment-only replacement, with a real tiny uploaded file present before `attachment_ids: []`.
4. A valid task token with forged/stripped actor headers.
5. An existing comment in a second workspace despite a forged workspace header.
6. Reuse of the previously valid task credential after an actual human cancellation.

Before and after reads retain the complete relevant comment fields, original author, content, revision, attachment IDs, issue state, task identities/statuses and comment IDs. Cookie/password/token values and signed attachment URLs are redacted from logs/reports. Failed mutated fixture rows are retained in the owned lane database rather than deleted or restored over the original witness.

## Password and other-source boundaries

The password lane uses real `/auth/register`, questionnaire/completion, and the account-version-aware task claim. Password registration creates a user without an email (`handler/auth_password.go:155`), while public invitations resolve email (`handler/invitation.go:63`). This probe does not invent a verified email or seed a fake credential/member.

Consequently password mode exercises a valid owner/task boundary. Human administrative controls operate on the Agent author's comment; machine negative targets are the human owner's comments, so they remain distinct actors despite the shared owning-user UUID. The password admin/member public-invitation submatrix is explicitly NOT_RUN. The complete owner/admin/member role matrix is exercised in legacy mode.

Valid `mcn_` Cloud principals require the configured Cloud verifier; arbitrary invalid prefix strings do not prove their authority boundary. Those principals are explicitly NOT_RUN here. `mdt_` is accepted by `DaemonAuth` for daemon routes and is not a general comment bearer; it is not mislabeled as an authorized ordinary-comment machine control.

## Runtime interface and review gate

The runtime coordinator's `--probe` hook invokes this Python program only after its own source/tool/resource review. The exact probe sidecar must be:

`<absolute-path>/comment_authority.py.review.json`

It must be a canonical, regular owner file, mode0600, with:

```json
{
  "verdict": "accepted",
  "blocking_findings_open": 0,
  "candidate_sha": "103e2bc329a9fb845d9f45c20b2a9bb3ebb54625",
  "probe_sha256": "15a0e3c03bb3a7d52301fd8fbee07ac53216405aece6307bcab7915ad38ba5ff"
}
```

The prepared template is pending and **does not grant this gate**. The parent/independent reviewer must inspect current bytes and create the actual sidecar. This implementer did not self-sign acceptance.

After the runtime itself receives clearance and a fresh owned API allocation, the exact command shapes are:

```sh
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime/run_runtime.py run --mode legacy --label comment-authority-legacy-01 --probe /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/probes/comment_authority.py --probe-only
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime/run_runtime.py run --mode password --label comment-authority-password-01 --probe /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/probes/comment_authority.py --probe-only
```

The hook provides a private `MULTICA_QA_PROBE_CONTEXT` with current candidate, loopback API, database/deployment/mode and output identity. The program rechecks API input agreement, nondefault exact loopback database routing without query overrides, current Git HEAD, review SHA and the live `/health` + `/api/config` before any fixture creation. It never reads or uses `JWT_SECRET` to forge a principal.

Output is the lane's `probe-result.json`, owner mode0600, plus an append-only `comment-authority-events.jsonl`. The top-level status is explicit lowercase `pass`, `fail`, `blocked` or `partial`. Actual failures and observed risks return nonzero; absent/bad private input or lack of a valid auth/control is not a passing permission denial.

## Verification performed

- Both private Python files compile successfully.
- **21 offline safety/evidence counterexamples PASS** in the second preparation record. They cover auth/setup 400/401 not proving authority, writes hidden behind unchanged author fields, mutation despite a refusal, deletion, malformed/default/redirectable database inputs, stale candidate, API credentials/bad ports, symlink/nonprivate context, retained output refusal, valid control invariants and secret/URL redaction.
- No live product test, HTTP fixture, database query, Docker service, model or Agent CLI was executed.

## Remaining follow-up

1. Independent current-source/script review and matching sidecar.
2. Runtime clearance and a fresh per-mode API/database allocation.
3. Actual legacy matrix and password owner/task run, preserving any failure and source/resource audit.
4. Classify upload observations against the owning product requirement; they must not silently acquire a newly invented normative contract.
5. Database-wide audit/notification/WS effects and valid Cloud credentials remain separate evidence. This HTTP probe does not replace the owning Go/race matrices or claim complete credential/platform acceptance.
