# v9 resource publisher environment repair

The actual v6 full check stopped in ordinary cmd/server skill-template fixtures with503 resource_store_unavailable. Root traced API-only MULTICA_RESOURCE_PUBLISH_DIR to an enabled resource publisher requiring an active internal organization, while these ordinary fixtures intentionally do not create one. Missing mount directories are tolerated and are not the confirmed cause.

The v9 successor changes only mode1 package-test child environment propagation: MULTICA_RESOURCE_PUBLISH_DIR is removed immediately before invoking each actual test process. Non-test and compile forwarding and explicit mode0 test-selected configuration remain unchanged. No product source or assertion changes were made.

V6 exact source, launcher, prior whole-check JSON/log and failed database evidence remain intact. V9 creates separately named output/clearance/safe-env/package/home/registry/profile/workspace and Playwright report paths. The existing approved Git-external0700 build temporary allocation is reused; sourceenv/renderer/Redis are unchanged.

Verification:3 propagation cases againstv6 give1fail2pass;v9 passes all20 base/HOME/pool/publisher,6 global separator,38 unchanged peer package/flag matrix againstv9 with network denied, and22 successor launcher cases (86 total). Six-file Python syntax verification passes. API publishing configuration preservation is asserted in the launcher fixture.

These fake results prove environment propagation and existing guards, not the actual503 causal replay. Root owns a fresh database cmd/server replay and the next whole check after independent check. The implementer did not create clearance or start any database/server/Provider/real Go test or whole check.
