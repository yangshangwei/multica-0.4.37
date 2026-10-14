# Native settings acceptance

F01–F17 have passing implementation and verification evidence. The final native
manifest is [acceptance-final.json](native-verification/acceptance-final.json).
Source fingerprints and the test checkout are recorded in
[provenance-final.json](native-verification/provenance-final.json) and
[source.json](verification/source.json).

## Environment and method

- Electron 39.8.7 on macOS, using the checkout's existing renderer at port 5666
  and API at port 18572. Node 22.23.2 runs the Playwright/CDP probes.
- Native windows: 1440×1000, 900×900, 900×700, and 900×1000 at 200% native zoom
  (450×500 CSS pixels).
- A disposable profile and task-local harness isolate platform behavior.
  Windows preference controls use explicit IPC/platform fixtures; this is not
  native Windows acceptance.
- Reads needed for the existing session remain local. All tested writes are
  memory fixtures. Main-process and renderer guards block other writes,
  external requests, agent starts and updater installation.
- Final runtime: zero page errors, external opens, unmocked mutation attempts,
  daemon starts or installer calls; no active fixture remained. See
  [runtime-final.json](native-verification/runtime-final.json).

## Accepted results

| Findings | Evidence and observed result |
| --- | --- |
| F01 | `probe-status-loop-response500.json`: two configured attempts over 5.1 seconds, followed by a stable Retry state. |
| F02 | `probe-query-recovery-results.json`: five cold read failures expose error/Retry; unknown membership does not enable Leave. Real QueryClient regressions additionally cover independently denied invitations, share links, repository pages and DingTalk permission joins. |
| F03 | `layout-menu-acceptance.json`: all 18 catalog/window/sidebar combinations pass visible geometry, center hit testing, pointer opening and keyboard opening. No horizontal scroll was used to reveal an action. |
| F04 | `probe-configuration-results.json`: a 12-option property dialog stays inside y=16..684 at 900×700 and y=16..484 at 450×500. Name, final option and Save can each be focused and reached. |
| F05/F06 | Configuration AX plus `desktop-semantics-results.json` name the eight audited controls; shortcut AX descriptions expose current bindings. |
| F07 | Warning text measures 6.04:1 light / 8.76:1 dark. Connected text measures 5.40:1 light / 7.31:1 dark. Both exceed 4.5:1. |
| F08 | `state-mcp.json` and `state-triage.json`: the same focused input/switch and draft survive transient refresh failure. MCP's 403→500 sequence does not revive its denied snapshot. Provider and DingTalk component tests cover their additional retained nodes and independent subqueries. |
| F09 | `probe-personal-results.json`: failed avatar persistence retains the crop dialog and produces no success toast. Workspace avatar behavior is also covered by its component suite. |
| F10 | `state-profile.json`: an invalid name blocks persistence; correcting it saves the retained `profile_description` together with the name, without replacing either input. Workspace behavior is covered in its owning suite. |
| F11/F12 | Invitation Enter dispatches once. `state-token.json` verifies all four acknowledgement-controlled dismissal paths, one pending revoke, retained failure and successful retry. Member/prefix confirmation variants are covered by component regressions. |
| F13 | `probe-integrations-results.json`: competing GitHub controls are disabled during one accepted write; no second operation is silently accepted. |
| F14 | `state-repository.json`: the final row and confirmation survive pending/failure, then disappear only after the successful retry. |
| F15 | `state-timezone.json`: the pending Europe/London choice blocks another action; after failure a later Asia/Tokyo choice succeeds and remains current. |
| F16 | `probe-token-schema-results.json`: real ApiClient parsing of HTTP 200 JSON null produces two bounded reads and a recoverable alert. The token panel remains mounted; no page/console errors occur. Core tests cover malformed create responses and credential redaction. |
| F17 | `desktop-semantics-results.json`: success, failure and recovery preserve the same status node and button focus. Native testing first reproduced focus falling to BODY with `disabled`; the corrected `aria-disabled`/handler-guard implementation passes. |

At compact widths the main sidebar is a modal Sheet. Its open state correctly
makes the underlying page inert; catalog actions were verified after dismissal.
At 1440px both inline sidebar states were verified. These tests preserve the
existing navigation contract.

Visual review compared original captures with the corrected dialog and compact
menus. Both scoped verdicts are 96/100, pass; see the `visual-verdict-*.json`
files. Geometry and keyboard evidence establish reachability separately.

## Probe corrections and cleanup

The final manifest selects the successful evidence for each finding. Earlier
failure logs remain as diagnosis history. Old audit scripts depended on removed
fixture names, assumed a token list always refetched on mount, and did not wait
for dialog exit animations. The task-local probes now seed their own rows,
clear the token query before parser testing, and query retained background
controls with their actual ARIA visibility and pending labels. Profile writes
assert the current `profile_description` boundary field.

Child tool lifecycle errors twice removed a harness control process while
leaving an orphaned Electron. Only proven task-owned processes and profiles
were recovered; the main session owned the final harness. The final orderly
shutdown removed that profile and closed ports 9367/9368. Existing API and
renderer services both return HTTP 200. See
[cleanup-final.json](native-verification/cleanup-final.json).

These fixtures do not establish real backend persistence, native Windows/Linux
behavior, spoken VoiceOver/NVDA output, physical touch behavior, production
performance, billing or plugin lifecycle acceptance. The gated E01 appendix
remains outside this task.
