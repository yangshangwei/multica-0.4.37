# Current-source authority and upload findings

Planning review pending; product source unmodified. Original probe hashes and original failed-case names are bound in artifacts/qa/main-acceptance-fixes-20261125/comment/original-evidence.json. Legacy mode retains eight failed cases, password mode four.

## Ordinary comment route
Auth in internal/middleware/auth.go strips X-Actor-Source before verification; real mat_ row verification overwrites X-User-ID, X-Agent-ID, X-Task-ID, X-Workspace-ID and stamps task_token. Password Auth follows the same stamping. NewRouter mounts UpdateComment/DeleteComment behind Auth/workspace/TaskCapabilityGuard, but the two handler isAdmin predicates currently consider only membership role. isMachineCredentialActor centrally recognizes task_token/cloud_pat, leaving human mul_ and JWT/cookie unchanged. resolveActor is an authorship classifier, not a human-principal test.

## Generic upload separate contract
UploadFile in server/internal/handler/file.go validates membership and the selected comment's workspace; it rejects legacy collaboration writes, then accepts comment_id without a comment-author/admin predicate. Upload provenance is recorded as uploader_type/uploader_id independently from the comment author. Thus the observed ordinary-member/machine generic upload success is consistent with current implementation and differs from replacing attachment_ids through UpdateComment. No owning specification found here explicitly equates generic binding with edit/delete management. This lane leaves it unchanged and retains the original observations as a separate unresolved product-contract concern.

## Additional identity limits
mcn_ verification can be exercised with the existing local CloudPATVerifier HTTP fixture pattern, without personal Cloud accounts. This validates auth integration against a synthetic verifier, not external Fleet. mdt_ is accepted by daemon route middleware, not ordinary comment Auth. Human PATs remain human-equivalent even if held by a daemon process. Cloud provenance resolves member authorship in current code; the proposed fix only removes machine use of the admin exception, not the independently established existing author path.

## Verification design
A real HTTP client against httptest.NewServer(NewRouter(...)) proves transport plus actual auth. New tests will create rows through testutil.New/Insert, generate actual mat_ strings and stored hashes, and use dedicated workspace memberships. Direct handler calls use testutil.Call and authoritative source headers solely to prove the internal backstop. Denial snapshots must compare comments/issue/attachments/task rows and relevant notifications/events, rather than treating403 alone as proof. Original runtime probe used real claim/start; do not claim test-inserted task tokens prove that lifecycle.

Existing baseline reproduction uses the parent-allocated comment_red01 DB only. Full-package normal and race need newly allocated independent DBs from parent. No migrations, database creation or cleanup of old evidence authorized here.
