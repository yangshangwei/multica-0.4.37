# P4 VCS router proof: public policy fixture upgrade

This supplement changes only
`server/cmd/server/delivery_completion_webhook_integration_test.go`.
The earlier P3 reports, frozen source hashes and 17-case passing run remain
historical evidence and are not rewritten.

The final fixture no longer inserts an enforce policy row directly. After the
actual controlled artifact execution and PR/integration preparation, it reads the
current member role, makes the fixture human a current administrator, performs
real JWT-authenticated GET/PUT `/api/projects/{id}/delivery-policy`, and checks
the returned enforce mode, project, next CAS revision and actual human actor.
It restores the exact previous member role before authorization issuance; cleanup
also restores that role if a policy assertion fails. The fixture then rereads the
current issue/project revisions and uses the returned policy revision in every
new authorization request.

The existing five top-level tests still contain 18 named results. No product
code, unrelated scenario or database environment was changed. The intended final
proof now spans public policy enablement, actual tar/grant creation, three
provider signature paths, real issue state and terminal receipt. Negative
cases still prove no unauthorized completion and no repeated consumption.

One initial apply_patch attempt did not match the struct's whitespace alignment.
The entire patch was rejected before writing; the exact current lines were read
and the corrected patch applied. This was an edit-context failure, not product
behavior red. No historical failure record was changed.

Current test SHA-256: `ab50aab92a90da334a8a6a9a485a45dd963cf0abd67da267e1c3c5deb76ff35f`.

Gofmt, whitespace and UTF-8/single-final-LF checks pass. No test/database command
has been run for this new fixture yet. The parent confirmed P4 production is
stable and will include this source in its final cmd/server compilation rather
than run a redundant separate compile. The parent owns final fresh normal/race
databases and rebuilt binaries.
Neither this fixture change nor the earlier intermediate proof establishes final
all-writer/M1 acceptance by itself.
