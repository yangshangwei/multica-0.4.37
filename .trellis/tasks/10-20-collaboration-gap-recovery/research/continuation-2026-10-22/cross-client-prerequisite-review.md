# Independent cross-client execution prerequisite review

Decision: **approve the unchanged AC33 spec, shared runtime fixture and explicit
provider source as a verified prerequisite test slice** before lifecycle commit.
No blocking source, authority, credential, cleanup or evidence finding. The
reviewer neither implemented these files nor launches clients/runtimes here.

The exact current sources still match the retained post-extraction AC33 run5:
`collaboration-cross-client-execution.spec.ts`
SHA3485662826b2afc9ab82dadbbe3e3d71dfad6bcb9c3bee9c4419c63a6d3b5416,
`fixtures/collaboration-cross-client-execution.ts`
SHAeab460c2681c95696ea90034ed63b34fc523169bf2d10d151c079c98cdda6c40 and
`fixtures/collaboration-cross-client-provider.go`
SHA356dfab491c55e0647e1523e30334c8043c597c2426e045ca22dd25545f17c88.
No task/product behavior is changed by adding these already-executed source files
to HEAD. The existing tracked/native/TestApiClient dependencies and separately
committed controls remain single shared owners; no duplicate harness is added.

Accounts/password cookies, source issues, managed installation challenges,
binding credentials, runtimes, private offers/adoption and per-owner consent all
originate in real public APIs. Two human actors and their agent/runtime/daemon/
installation/workspace/channel identities are explicitly different. Account
reads carry no ambient workspace header. Runtime launch requires named absolute
test-built ELF binaries; no installed provider/account lookup occurs.

The daemon test executable starts the real capacity-one `runBatchPoller` and
Runner/controller using the server's original scoped managed credential. Claims,
permits, burn/start/reap/seal/proof/ACK remain production code. The stdlib provider
reads the immutable input and actual `/proc/self/ns/pid`, reports its run and
namespace, then blocks until release and emits only the business result. It
cannot forge supervisor termination proof. Real current natural-proof lifecycle
2/2 independently reuses this unchanged fixture on the refreshed backend.

Scoped credentials travel to each owned container over stdin; command arguments,
Docker environment and evidence omit them. The provider only receives the narrow
business capability/identity from the production supervisor. Safe provider
evidence removes launch nonce and does not copy raw managed credentials or
private signing keys. Log/error redaction and exact issue validation remain in
the fixture. The isolated native profile/browser and every container/socket are
owned and cleaned by the test; only two explicit binaries are copied, never user
home or private sessions. Cleanup releases test processes, waits/stops the exact
owned containers and removes only owned profiles. It is separate from product
Stop/physical proof acceptance, which is asserted before cleanup.

AC33 run5 demonstrates B natural completion while A remains busy, concurrent
distinct namespaces, one coalesced A follow-up containing exactly two valid
authors, exact single waiting-input cancellation, original source records and
history in actual Web/Electron after refresh, no extra provider visit and exact
capacity-one daemon summaries. The previously reviewed
`../continuation-2026-10-21/ac33-history-final-review.md` covers the full independent
behavior analysis; this prerequisite review verifies current source/receipt
identity and dependency readiness, without reattributing old runs to new code.

`ac33-run5-result.json` records **1/1**, zero failure/skip/flaky/retry, **20.150s**.
The reviewer reads the retained raw `.omx/reports/cwc-gap-recovery-20261021/ac33-post-extraction/cross-client-run5.json`,
verifies SHA463d26d2b225f79cb4c8df207c5363ec841dce4679f9ac0033fb7c23e182be01,
and compares actual stats plus safe identity source hashes. Explicit daemon
SHA57f9aa8c2306992964aa714eef324fb5435e8218f143bb28702378adbe67d358 and provider
SHA80c3225030ff105764e3b65309d9b3e599c223a8914def1cb37ab6f3217a9f94 also match
the new lifecycle run's current binary proof. Source hash equality, not the old
API embedded commit alone, identifies the preserved run.

Fresh lifecycle `lifecycle-clients-first*` is **2/2**, **59.501s**, no
failure/skip/flaky/retry on current owned password API18996, production Web13916
and renderer6090. `content-admin-preflight.json` verifies the refreshed application
source/configuration, listener/build/index/native identities. Lifecycle uses the
same unchanged runtime fixture and real original natural proof; its independent
exact clause review is in `lifecycle-final-review.md/json`.

Current root-scoped `cross-client-prerequisite-static*` strict TypeScript and
`cross-client-prerequisite-lint*` real ESLint pass with empty diagnostics. Their
source manifest matches current bytes; no ignored-file result is counted.
Provider gofmt and repository `git diff --check` are clean. No additional native
run is necessary for an unchanged prerequisite with preserved matching current
receipts and fresh downstream integration.

The coordinator also completed a fresh exact AC33 prerequisite rerun after this
review started. `cross-client-prerequisite-current*` is **1/1**, zero
failure/skip/flaky/retry,19.500s Playwright duration/20.464s command elapsed.
The reviewer parsed the complete raw result and original attached identities,
verified SHAfc4dc9aa6aa4062166eb740262a09d67dc444e111402c079cee0dc76187eaea3,
and compared every source/binary identity against current files. The safe
attachment copy is `cross-client-prerequisite-current-identities.json`. This fresh
current receipt is distinct from historical run5 SHA463d26d2; both are retained
with their actual source/environment evidence. No source or fixture change was
needed to achieve this pass.

Actual Electron uses production routing/IPC/preload/renderer with daemon/install
lifecycle stubs. This does not establish installed full Main or old clients.
Explicit contained test provider is separate from commercial account/quota,
general providers and other OS support. Production gates, task status and ledger
are unchanged; no new acceptance promotion or parent archive is made here.
