# S4 preparation summary browser/native fixture

Status: authored, discoverable, typechecked and linted; actual Web/native
execution is pending the coordinator's matching rebuild. This slice changes
only E2E fixtures and specs; product source was already handed off for review.

The fixture creates two synthetic users and uploads/creates through the real
loopback API. It inserts two independent retained run identities in one
transaction: a prepared run with its initial prompt/manifest and an explicit
business failure while termination remains unknown, plus an unclaimed legacy
run with neither summary nor outcome/proof. It does not update a frozen prompt
or preparation snapshot, disable a trigger, create a real runtime/executable, or
invoke a daemon/provider.

A temporary pure Go program uses the actual protocol Seal/Validate helpers.
The protected HTTP journey asserts the public allowlist, exact file version,
fixed commit, unchanged result/proof and a stable old cursor after a later
preparation event. The Web and native journeys reuse those records across
refresh and two viewers. Native capture uses the existing production IPC harness
and shared captureNativeSection helper, with no daemon lifecycle.

## Delivered files

- `e2e/fixtures/collaboration-preparation-records.ts`: isolated public-human setup,
  initial immutable run snapshots, production protocol sealer/validator, exact
  protected HTTP assertions, and the shared two-viewer UI journey.
- `e2e/collaboration-preparation-records-web.spec.ts`: actual production-Web
  wiring with wide/narrow viewport screenshots and geometry checks.
- `e2e/collaboration-preparation-records-desktop.spec.ts`: the existing real
  Electron IPC harness with fresh temporary profiles, shared `captureNativeSection`
  wide/200% scroll coverage, current account headers, and zero daemon starts.

The coordinator's existing `collaboration-native.ts` exports are reused. This
slice does not import another spec, modify product source, change a schema,
add a dependency or start a service/build. Records remain in the task-owned
database for inspection/TTL cleanup; native profiles and sealer files are removed.

## Synthetic and actual evidence boundaries

The DB rows are deliberate fixtures, not proof of a real launch, preparation,
provider result or termination. The prepared run has a synthetic recorded
business failure and **unknown** proof, with no settled time. The legacy run is
unclaimed, has no prompt/preparation/outcome, and also has unknown proof. No
agent/runtime, task, capability or reservation is created.

Preparation and prompt are supplied only in each run's first INSERT. No trigger
is disabled and no already frozen snapshot is patched. Prepared observations
and provider-result shape are validated by the production protocol package.
The synthetic attempt nonce is correlated to its retained attempt row. The
initial run-event watermark is 4; after capturing its cursor the fixture adds
only observation 5. The old page must keep the initial summary, while new reads
from both authorized users must see the same new summary with unchanged run
outcome/proof and original result text. All public record bodies must omit
instructions, manifest/provider/Git-bundle digests, attempt/launch nonces,
installation/binding IDs, local paths and manifest destinations.

The UI journey opens exact file details and the fixed commit, checks prepared,
failed and not-recorded stage labels, loads the next records page to the original
result, reloads, and repeats with the other viewer. The legacy disclosure must
show not recorded and termination unconfirmed. Web captures verify exact PNG
dimensions and no horizontal overflow. Native captures use Electron capturePage,
real zoom and the existing complete-section scroll-coverage assertions.

`visual-s2-s3-integration.json` was read as the established visual reference.
It is not a screenshot or functional pass for this summary. Actual new captures
must be inspected by the coordinator before any product styling follow-up.

## Authoring verification performed

- `pnpm exec playwright test e2e/collaboration-preparation-records-web.spec.ts e2e/collaboration-preparation-records-desktop.spec.ts --project=chromium --list`
  discovers **2 tests in 2 files** without executing them.
- Strict direct TypeScript checking of the fixture and both specs passed with
  ES2022, ES modules, bundler resolution, esModuleInterop and skipLibCheck.
- Scoped ESLint passed using the repository core config and the explicit
  `**/e2e/**` devDependency classification. The first lint command used the
  production module classification and correctly reported `pg` / `@playwright/test`
  as devDependencies; both are already declared test dependencies. No rule was
  disabled and no dependency was added.
- Extracted the exact Go program from the TS fixture and executed it with
  Go 1.26.6. It accepted a synthetic manifest/observation pair and produced the
  same manifest digest in the observation; changing the frozen input ID was
  rejected. This check accessed no API, database, browser or provider.
- Scoped diff whitespace checks passed.

## Required environment and run commands

Use the isolated **legacy-auth** test API, because `TestApiClient.login` uses
the existing synthetic email-code flow. S5's separate password/admin environment
is not a replacement. Creation must be enabled only in this isolated DB;
execution does not need to be enabled. Required settings:

- `DATABASE_URL`: explicit loopback non-default task database, migration 737+
- `COLLABORATION_E2E_API_URL` and `NEXT_PUBLIC_API_URL`: identical loopback origin
- `MULTICA_DEPLOYMENT_ID`: the task deployment UUID
- `PLAYWRIGHT_BASE_URL`: matching rebuilt production Web origin
- Native: matching `COLLABORATION_ELECTRON_RENDERER_URL`; set
  `COLLABORATION_ELECTRON_SOURCE_ROOT` to the coordinator's built checkout when
  it differs from the main checkout. It must contain the built preload, real
  Electron executable/dependencies and production IPC fixture.
- `go` on PATH with Go 1.26.6 available; the temporary sealer sets that toolchain
  and uses `-mod=readonly`. It never discovers an agent CLI.

Run the newly authored specs from the main checkout, with the isolated service
environment injected by the coordinator and Electron source-root override:

```bash
pnpm exec playwright test e2e/collaboration-preparation-records-web.spec.ts \
  --project=chromium --workers=1 --retries=0 --reporter=json \
  --output=.omx/reports/cwc-gap-recovery-20261021/preparation-web
pnpm exec playwright test e2e/collaboration-preparation-records-desktop.spec.ts \
  --project=chromium --workers=1 --retries=0 --reporter=json \
  --output=.omx/reports/cwc-gap-recovery-20261021/preparation-native
```

Test identities:

- `Web reads synthetic frozen preparation without inferring termination across refresh and viewers`
- `Desktop reads synthetic preparation with native 200 percent captures and unchanged proof`

Attach matching API/Web/Electron source/configuration provenance to the resulting
HTTP evidence and screenshot geometry. The previous API18985/Web13905/Electron6079
build predates this summary and was not used for a claimed summary pass here.
Actual browser/native runs remain with the coordinator after a matching rebuild.
