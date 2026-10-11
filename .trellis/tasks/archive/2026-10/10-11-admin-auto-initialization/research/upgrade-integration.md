# Upgrade integration evidence

Research date: 2026-10-11. Planning only; no product implementation performed.

## Existing delivery boundary

- `scripts/offline-upgrade.sh:153` resolves the package Compose and ordered operator overlays; line 174 validates the selected images with Node in the already-loaded frontend image. The target machine needs Docker/Compose, not host Node or Go.
- `scripts/offline-upgrade.sh:195` backs up the database and deployment configuration; lines 218–235 preserve the final Compose command and start the selected images. Lines 242–259 wait for `/healthz` before reporting completion.
- `docker/entrypoint.sh:27` runs migrations, and line 41 only starts the API after migration succeeds. A separate CLI executed after the upgrade health gate can rely on the migrated schema and must still validate its required indexes.
- `Dockerfile:31` uses Alpine, sets `/app` as its working directory and carries `/app/server`. There is currently no runtime `USER` declaration. A one-off initialization container can use the invoking operator's numeric UID/GID without adding a host binary or runtime dependency.
- `docker-compose.selfhost.yml:118` forwards authentication, administration and deployment-ID settings. Existing backend volumes do not expose the host deployment directory or a private initial-password file.
- `scripts/install-changelog.sh:57` delegates dotenv parsing to Compose. Lines 69–97 demonstrate validating structured Compose output using the loaded frontend image, and lines 109–114 demonstrate an operator-owned deployment-directory bind used by a short-lived maintenance container.
- `scripts/build-offline-upgrade.sh:72` copies the canonical upgrade guide into both `README.md` and `操作文档.md`.
- `scripts/offline-changelog.test.mjs:130` exercises the real upgrade wrapper with mocked external image/container boundaries and verifies exact guide copies plus packaged links.

## Recommended integration

1. Add a bundled `initialize-platform-admin.sh` maintenance helper and call it after migration/health success but before `Upgrade completed`.
2. Preserve all ordered operator overlays. Resolve effective backend settings through Compose rather than sourcing `.env`, reading the build host environment or inferring mode from a public endpoint that may belong to another service.
3. Skip without creating a password file when password mode or administration is disabled. For an eligible deployment, require its already configured valid deployment UUID; do not generate or change deployment identity implicitly.
4. Run the selected backend image as a one-off Compose service with its actual database/network configuration, the `/app/server` entrypoint, no dependency startup, no service ports, `--pull never`, and the operator's UID/GID.
5. Add a temporary structured Compose override containing the deployment-directory bind and UID/GID. Generate/validate it with the already-loaded frontend's Node, using structured JSON and proper Compose dollar escaping. This avoids `-v host:container` ambiguity for legal host path punctuation and does not introduce a permanent credential mount into the API service. Reject a conflicting operator mount at the reserved target before launch.
6. The Go command owns password generation, durable file handling and the database transaction; the shell sees only a bounded nonsecret JSON result. No secret passes through shell command substitution, environment variables, Docker labels, stdout, or `compose-command.txt`.
7. Keep the standalone helper available for operators who enable admin after an upgrade. Ordinary API starts/restarts do not automatically create accounts; the normal offline upgrade invokes the helper automatically.

## Persistence and failure limits

The host file must exist durably before the account transaction commits. A lost CLI response therefore leaves a recoverable file plus an authoritative database marker. A failed transaction may leave a prepared credential file, which a retry can validate and reuse without inventing another password.

A separate committed `prepared` database state is not necessary: it adds abandoned-reservation recovery without protecting against external deletion of the credential file. Use a terminal per-organization marker, stable file intent ID and the existing platform advisory lock.

External deletion/replacement of the file after a committed, still-restricted account is created is a controlled recovery case. Never reset the account automatically, overwrite an unknown file, or fabricate a successful password-delivery result. After the user changes their password the initial file is stale and may be removed; the database marker remains permanent.

Concurrent initializers sharing a database must result in one account. The winning operator host owns the credential file; another host must not create an unrelated local password and present it as valid. A committed pending-change account with no matching local file returns non-success `credentials_unavailable`, whether the cause is another host or external deletion. Report the committed identity without disclosing or pretending to deliver its password.

Retrying a legitimate existing intent file must sync that file and its parent directory again before account commit. A previous attempt may have published the filename but failed directory sync; existence is not durability evidence.

## Verification to carry into implementation

- Effective disabled/legacy modes skip before file creation; enabled mode and `ALLOW_SIGNUP=false` still initialize through deployment authority.
- Tests assert actual Compose argv/config, ordered overlays, selected image, UID/GID, no published service ports, no pulls, and literal spaces/dollars/quotes/commas/colons in paths.
- Test the helper without host Node/Go; only the loaded images provide tooling.
- Induce file creation/fsync failure, database rollback, lost commit response, helper interruption and repeat invocation. Never announce a successful whole upgrade after initialization fails.
- Check final runtime archives and Docker build contexts exclude `admin-initial-password.txt` and its private staging files. Existing `.gitignore` and `.dockerignore` do not yet cover that new artifact name.
- Keep both canonical guide copies in the server upgrade archive. Update the source docs pages and regenerate their committed in-app bundle with `pnpm generate:docs-bundle` if their administrator initialization instructions change.

## Explicit implementation boundary

The selected flow is the offline upgrade and its reusable maintenance helper, not a public API, new login page, registration hook, or an unconditional server-start hook. It does not change authentication mode, registration policy or feature flags. Fresh installation UX beyond running the same helper is deferred.
