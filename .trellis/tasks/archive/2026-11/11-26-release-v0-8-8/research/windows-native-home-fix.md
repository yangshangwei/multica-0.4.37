# Native acceptance home alignment

Independent review identified a statically provable split: packaged acceptance config calls Electron app.setPath(home), while daemon-profile.ts uses Node os.homedir(). The native controller spawn only supplied MULTICA_PACKAGED_ACCEPTANCE_HOME, so inherited runner HOME/USERPROFILE remained the daemon profile root. Fixture ownership checks expect that profile under the acceptance home. This is a distinct prospective fixture defect, not a claim that the currently running native job has failed at this point.

Scope: set HOME and USERPROFILE only in the native controller's child env, using the existing nativeAcceptanceHome-validated value. Electron launch and its daemon already inherit this environment, so no duplicate helper or product change is needed. Do not mutate parent process.env. Leave container branch and every identity/path assertion intact.

## Validation

Existing controller Node test uses the repository's established VM extraction pattern to execute the actual native constructor spawn block (only import.meta.dirname is supplied as its original fixture directory). It redirects the command to a real Node probe, preserving the constructor-produced child env. That process imports the actual production daemon-profile.ts and spawns another Node descendant with inherited env, reporting os.homedir/profileDir.

Before the two env assignments, the real child returned the host user's home instead of the explicit acceptance home; this failed the new regression. Afterward child and descendant both return the owned path (including space and literal percent), and actual profileDir is under the same .multica/profiles root. Invalid/relative/missing/newline homes fail through the existing validator before any spawn; parent HOME/USERPROFILE remain unchanged. Container branch was not edited. No new environment abstraction or product behavior was introduced.

Related controller/layout/Windows plan/workflow tests:32PASS/0FAIL/0SKIP; diff-check pass. This host is macOS; Windows branch assignments and path validation are exercised structurally, but a Windows child/installed Electron confirmation still requires the new native job. Current run45681903824 was not cancelled or rebuilt by this agent. Its later navigate failure is separate pending diagnosis; this test does not invent that run's root cause.

## Existing daemon failure diagnostic projection

The later run45681903824 reaches boot/runtime/login and fails in the metadata wrapper. Its existing private error embeds one `Installed daemon diagnostic: ` single-line JSON. Added a small projection in the current diagnostics module, without controller protocol or product changes. It considers only parsed failure message/stack, at most16failures,128KiB/input,128lines and64KiB/marker line. Only an exact line prefix is eligible; malformed/oversized/nonobject values return invalid; absent returns unavailable.

The projected daemon shape contains expected/actual state from a fixed enum, strict boolean-or-null ownership/install/capture fields, and five fixed file-existence/outside-home/read-failure groups. Paths, IDs, PID, config values, text, arbitrary errors and extra fields never survive. Existing stdout/stderr are not searched for marker JSON. Regression cases cover marker surrounding secrets, nested real-shaped secret fields, malformed/oversized data, wrong-type booleans, fake prefixes, and correct integration through parsed Playwright results.

Node configuration/workflow tests15PASS, syntax/diff checks pass. Home change remains separately reproduced with real child/grandchildos.homedir and actualprofileDir. Neither this projection nor localtests determine which metadata branch caused the latest Windows failure. No commit/push/dispatch or job cancellation by this agent.
