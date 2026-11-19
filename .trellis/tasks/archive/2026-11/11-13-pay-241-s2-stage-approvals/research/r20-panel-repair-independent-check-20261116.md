# R20 source-denial panel independent check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## 1. Spec compliance — PASS

The two-file repair satisfies the current S2-R08/A10 source-denial and shared-client contract. An authoritative R1 denial must remain explainable after its privacy fence correctly removes R4. The product change reuses the existing exported `collaborationStageSourceDenied(view)` and places the existing localized `role="status"` message outside the `record &&` fragment. It retains the original anonymous-R4 denial behavior and renders the message at one location.

The exact production delta is an existing-helper import and relocation of the existing message. Current query error/retry, loading and unsupported states remain distinct. The private evidence and approval rendering remain behind the original readable-source gate. The existing action derivation, reason-only return behavior, source-generation cancellation, request identities, all backend locks, SQL, tracer and locale files are unchanged. No store, protocol, dependency or platform branch was added.

The original R19 failure and the independently checked R1/R4 response ordering remain retained. This review accepts the shared panel repair and its source input; it does not accept S2, A01–A12, four runtime scenarios or the parent matrix.

## 2. Code quality and regression evidence — PASS

The new component regression first loads a real API-parsed R4 containing visible private review/report data. It then refreshes R1 through the real Query options, producing the current source-denied projection and its actual cache scrub. A replacement R4 is pending, unavailable or fails. Each variant requires one visible status message, no previous private head/review/report/decision content, no approval buttons and zero writes. The error variant also preserves explicit Refresh.

The suite separately keeps readable R1 with pending/error R4 free of a false source-denial message, retains its original unavailable-server control, and keeps the existing anonymous-R4 denial control. Core owns the strict parser/authority/cache matrices; the component cases cover the named cache-to-render regression rather than duplicating the full Core matrix.

| Original executed command | Independent retained result |
| --- | --- |
| Views named RED, `-t 'keeps R1 source denial'` | Three failures at the missing banner assertion on line80; nineteen name-filtered cases; original product hash unchanged |
| Full `stage-code-panel.test.tsx` GREEN | 22 PASS, one file, zero FAIL/SKIP |
| Five Core cache/action/presentation controls | 44 PASS, five files, zero FAIL/SKIP |
| Views `tsc --noEmit` | Exit0 |
| ESLint on the two owned files | Exit0 |

All five result records agree with their argv, ROOT cwd, exit codes and source-before/source-after hashes. The implementation report's 21 distinct file/hash references, including original raw logs, were independently rehashed with zero mismatches. Both RED and GREEN point to the same missing-banner assertion. The RED test-file hash differs from final GREEN because the implementer corrected an unused readable-loading control to the existing locale; its original RED hash and failures remain recorded. This review does not claim the complete RED test file was independently reconstructed byte-for-byte.

The retained pnpm configuration warning appears in the command stderr. It is not a lint/type/test error, and this repair did not change package configuration or dependencies.

## 3. Frozen frontend input — ACCEPTED

Independent comparison of the new input binding against the original R19 test manifest found:

- Exact file path set: 3,359 files, with 3,357 original byte/size/mode identities unchanged.
- Only `stage-code-panel.tsx` and `stage-code-panel.test.tsx` changed. Their private bytes equal the accepted ROOT bytes; private mode0600 and ROOT mode0644 are separately preserved.
- Exact link path set: 1,961 links. All26 absolute internal workspace targets move by the exact R19-to-R20 source-prefix substitution. The other1,935 targets are unchanged.
- All actual R20 declared files and link targets match the recorded input. No source or link mismatch remains.

Input binding SHA256: `0e67ada056c5adf29c4410dc97b7ff5ccfce932ec6c9c0f369f7dc5de96dbabc`.

Accepted ROOT product SHA256: `58009dc5a0ab43ffbce010b83195054488001d8454347941ef856ab99a91dc93`.

Accepted ROOT component-test SHA256: `97500d4429a41b9d78e0dc26797120002e8681fb8d036f35cb7b8bf28b081b79`.

This accepts the declared repaired frontend graph for subsequent builds. It does not establish equivalence of the entire live ROOT checkout or an already launched runtime.

## 4. Candidate builds and outputs — ACCEPTED for runtime entry preparation

The bound production Web build (`next build --webpack`), Desktop web TypeScript command and Desktop production build all completed exit0. All three recorded the accepted input binding, zero source/link drift before and after, no stream error and no signal. Their command/result stdout/stderr/environment references were rehashed and matched.

The independent output audit checked all1,676 compiled output paths, byte hashes, sizes and modes against `runtime/frontend-products-r01.json`. The actual Web `.next` and Desktop `out` file sets, with `.next/cache` explicitly excluded, match exactly: no added/missing path or identity mismatch. Actual BUILD_ID is `PpNZY2bAtTOUVfi6UgJKT`; renderer index SHA256 is `0be2384231a80a945c1de883e8f9c1971b0c5d5831f1d005c4a531870cb8a91f`, matching the manifest.

The four actual prepared Go executable paths also retain their R19 reference hashes and mode0755: R18 API harness `a2208e6b…`, R18 CLI `c089ace5…`, R18 Host daemon `090a1d60…`, and the retained Host provider `65ec5fc9…`. This verifies the referenced artifact bytes; it does not reuse an old live PID or assert a fresh API identity.

These accepted source and compiled-output inputs allow the leader to prepare a new runtime entry. Actual startup, live input/output/authority bindings and the original four-scenario execution remain separate gates. Neither successful builds nor this product-code verdict supplies those runtime results. Private launch configuration and connection-budget receipts are leader-owned later entry inputs, not accepted by this source/output report.

## Findings and handoff

No blocking spec or code-quality finding remains in the assigned two-file repair. No mechanical code fix was needed by the checker. The leader may continue constructing and independently binding the new owned Web/Desktop outputs, preserving the retained peer platform and all earlier failures.

Only this report and its JSON companion were written by the checker. Lint, type-check and tests above are independently checked **retained executions**, not repeated commands. This assignment performed no DB/API/browser/test/build command, signal, install or Git operation.
