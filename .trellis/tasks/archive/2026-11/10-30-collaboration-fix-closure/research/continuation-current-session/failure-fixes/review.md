# Focused browser-fixture repair verified

The two v11 editing failures share a stale native-select interaction. The
existing picker patch also used a nonexistent `parent.name`; the fixture now
retains the ordinary issue identifier and chooses the exact identifier/title
button. Both creation and subsequent editing address the current optional-stage
label. Existing UUID, stage, recipient and field persistence assertions remain.

The delivery refusal failure is an exact-copy mismatch. The current locale and
component regression use `The issue was not created. Your draft and files are
preserved.` and render the typed cause separately. The existing E2E copy sync
was retained. Its durable rejection, file preservation, resubmission, new-key and
old-key assertions remain unchanged. No product or locale change was needed.

Package-scoped creation/field-input/field-editing tests passed 35/35 in three
files. Focused package ESLint, explicit Playwright discovery and targeted E2E
TypeScript validation with repository ESNext/React settings exited 0. The first
ad hoc TypeScript attempt used mismatched ES2022/no-JSX options; its configuration
failure is retained separately, followed by the correct passing command.

These checks do not execute a real browser, native Electron, database or
Provider. Root owns the new frozen candidate's directed browser verification and
complete acceptance gates. Exact commands, logs, source/log hashes and limits
are recorded in `verification.json`.
