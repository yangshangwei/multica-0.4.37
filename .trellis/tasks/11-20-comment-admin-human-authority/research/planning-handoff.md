# Planning handoff to parent

Child PRD/design/implement and both manifests are ready for independent child review. No product or test source edits yet, no task activation. Explicit parent acceptance message is still required.

Existing-regression diagnostic: CLI-guarded go -C server test -json -count=1 ./cmd/server -run '^(TestUpdateCommentAttachments|TestEditCommentTriggers|TestMachineCredentialTaskTokenCannotMintHumanCredentials|TestMachineCredentialHumanCallerStillMints)$'. Exit0;16 named PASS;0 FAIL;0 SKIP. Parent-provided comment_red01 env/HOME only. Intent, raw JSONL and receipt including source/tool hashes live in artifacts/qa/main-acceptance-fixes-20261125/comment/. This is development-checkout diagnosis, not frozen acceptance and not a reproduction of the missing predicate itself.

Next: on acceptance activate child, author authority regression, register frozen source, reproduce RED then GREEN. Need separate parent allocations for focused red/green (comment_red01 already used by existing baseline), full handler normal/race and relevant router normal/race. All allocations remain parent-owned; this lane did not create, migrate or drop a database.

Generic upload disposition: unchanged; existing code permits same-workspace association with separate uploader provenance. Explicit product-management parity requirement remains unconfirmed; report separately rather than relabel original upload observations.
