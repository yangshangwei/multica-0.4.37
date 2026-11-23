# Codex pilot runtime quality check — 2026-10-27

Scope: the five Codex runtime artifacts in `pilot/runtime/` and this report.
Reviewed checkout: `87939bbe0c09d7e63dc9ec79c9b7f6e4ab826286`.
This is an artifact check; it does not accept stage one or establish model readiness.

## Findings (fixed)

- File: `runtime/nginx-pilot.conf.template`
  Issue: nginx implicitly redirected bare `/api` to `/api/`; the redirect guard
  converted that response to 502 instead of the required 403. `/model` had the
  same prefix-location behavior.
  Fix: explicit exact-match denials for both paths; GET and POST regressions
  remain strict.
- File: `runtime/nginx-pilot.conf.template`
  Issue: the model proxy replaced the incoming native client's User-Agent with
  `codex-security-pilot`. The main session reported an upstream client-identity
  rejection, which cannot establish model readiness.
  Fix: forward `$http_user_agent` unchanged. Synthetic tests assert exact
  preservation; destination, TLS verification, bearer replacement and the
  explicit header allowlist remain enforced.
- File: `runtime/nginx-pilot.conf.template` and `runtime/test_codex_runtime.py`
  Issue: the main session captured genuine app-server `originator` and
  `x-codex-beta-features` headers, but the explicit model header allowlist
  dropped both. The new synthetic regression failed before the fix because
  `originator` was absent.
  Fix: transparently forward `$http_originator` and
  `$http_x_codex_beta_features`, without default values or identity substitution.
  Tests assert exact preservation of the captured non-secret values and
  omission when the caller sends neither header. Authorization replacement and
  refusal of X-Api-Key, Cookie and Proxy-Authorization remain verified.
- File: `runtime/test_codex_runtime.py`
  Issue: the initial fixture put its upstream on the provider's internal network
  and combined a managed cwd with a linked session store. It did not exercise
  the intended two-network gateway or both normal daemon layouts.
  Fix: put synthetic upstreams on a separate bridge and attach the gateways to
  both networks. Cover managed and local-directory tasks, absent API auth,
  invalid TLS hostname, preserved task environment/stdio, protected file and
  directory links, and rejected cwd/home/temp/session paths.
- File: `runtime/README.codex.md`
  Issue: the session path description omitted profile namespace hashing and
  claimed HOME was unchanged despite `runuser` changing it.
  Fix: document the actual namespace and mode contracts, `/home/node`, the
  forwarded User-Agent, bare-path denials and verification limits.

## Findings (not fixed)

No remaining mechanical defect was found in the reviewed artifacts.

- Live model execution and squad routing remain outside this review. The main
  session's latest report is that genuine standalone Codex exec succeeded,
  while the actual native SDK turn returned upstream 401. Its captured protocol
  metadata motivated the transparent forwarding fix above. An authorized
  genuine SDK retry is pending; continuing upstream refusal must not be
  bypassed by inventing another client identity. These live observations were
  not reviewer-run tests. Native SDK readiness and complete leader/member
  settlement remain unverified; owner: main session.
- The image uses floating base tags and the pilot's missing supply-chain
  attestation remains `hold/unknown`. This review records inspected bytes and
  versions; it does not certify upstream provenance or resolve the separate
  supply-chain case.
- Roles share UID 1000 and the synthetic lab. This setup does not provide
  isolation between squad members. The main session must verify its actual
  container/network configuration before relying on these fixture results.

## Permission and gateway audit

The source supports the wrapper's normal paths and permissions:

| Contract | Source and observed result |
| --- | --- |
| Task root and managed cwd | `execenv/execenv.go:1436` and `:504` create 0755 directories; synthetic managed launch writes successfully. |
| Task CODEX_HOME | `execenv/execenv.go:635`, `execenv/codex_home.go:205`; the generated home stays selected after dropping to UID 1000. |
| Task CLI config | `execenv/execenv.go:509` creates/restricts 0700; its synthetic 0600 config is readable through `MULTICA_TASK_CONFIG_ROOT`. |
| Task temp | `daemon.go:9200` creates the temp directory and `:9207` restricts it to 0700; the provider writes there without relaxing the mode. |
| Local-directory sessions | `execenv/codex_home.go:647` creates the store and parents with 0755; `:391` defines the profile namespace. Only the selected store changes owner, not its parents. |
| Root profile | Image `/root` is 0711 and `/root/.multica` is 0700. The provider cannot read the synthetic root-private 0600 sentinel. Recursive chown does not follow planted file/directory links. |
| App-server transport | Existing `codex-pilot` forwards argv/stdin/stdout and keeps diagnostic output on stderr; task token and config pointer survive the user switch. |

Paths in this table are relative to `server/internal/daemon/`.
The 0755 fixture parents match the source; no parent-mode repair or broad
permission widening was needed. `Dockerfile.codex` and `codex-pilot` are unchanged.

API traffic preserves incoming Authorization, including an absent header, and
uses the fixed API upstream. Model requests use only the fixed HTTPS upstream,
verified TLS identity and gateway-injected synthetic bearer; unrelated auth
headers are dropped. Unsupported routes/methods and CONNECT are refused;
upstream redirects become 502 without a Location header. API upgrade traffic
reaches the same fixed upstream. The provider has no IPv4 default route, cannot
connect directly to the synthetic upstream's separate bridge or the tested
external address, and cannot resolve the tested external DNS name.

## Verification

- Lint/syntax: pass — `sh -n` for the wrapper, Python AST parsing for the test
  file and embedded scripts, whitespace checks, and successful nginx parsing
  during synthetic gateway startup. ShellCheck and Ruff are not installed.
- TypeCheck: not applicable — these task-local shell/Python/nginx/Markdown
  artifacts have no configured static type checker. No TypeScript or Go source
  changed; full repository lint/typecheck/tests were outside the assigned scope.
- Tests: pass — 7 tests in 4.035 seconds, exit 0:

  ```sh
  python3 -B .trellis/tasks/10-26-security-remediation-squad/pilot/runtime/test_codex_runtime.py
  ```

The test process uses only disposable `security-codex-test-*` containers and
synthetic HTTP/TLS peers. It does not run the daemon, invoke models, inspect
real credentials, mount host profiles or touch the main session's live containers.
After verification, no `security-codex-test-*` containers or networks remained.
The checked README carries the task-local contract updates; no product or
global `.trellis/spec/` change was needed or made.

## Verified image

- Tag: `multica-security-codex-pilot:20261027`
- Image ID: `sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f`
- Platform: `linux/arm64`; installed Codex package metadata: `0.164.0`.
- Wrapper SHA-256: `55bf982b2f20fa3e30522c25ad60a088a11a54c372d6a84dadb513e9ca431e8c`.
- Binary SHA-256: `fc22801acc9b50f88c3ab059ad22ddc592596c4dc482ccd79989409330fde962`.

An additional disposable container with `--network none` compared the image's
wrapper and binary hashes against the owned wrapper and the recorded temporary
build binary. Both matched; image modes and Codex package metadata also matched.
No wrapper or Dockerfile changed, so no image rebuild was necessary. The gateway
template changes require a private re-render by the main session.
