# Codex continuation: phase-one execution boundary

## Current evidence

The user requested continuation after switching the underlying runtime to Codex.
The existing Trellis task is already approved and `in_progress`; this is a
continuation, not a new task. Read-only inspection on 2026-10-27 found that the
eight historical pilot roles still reference the offline OpenCode runtime
`39443bc2-1bf7-4515-8bdd-4684a41d64c9`. A separate host Codex runtime is online,
but it is outside the pilot container and belongs to another daemon. The pilot
project's `/work/lab` resource still belongs to the original daemon.

The remaining behavior gap is genuine role dispatch and return, independent
diagnosis/implementation/QA, accurate reports, and enforced network destinations.
The prior valid synthetic patch and the prior failed routing attempts remain
historical evidence; neither is rewritten as a successful Codex run.

## Execution plan

1. Prepare task-local Codex runtime artifacts using the installed CLI version
   (`0.164.0`) and a Multica binary built from the recorded checkout commit.
   Keep the daemon's development profile private to root, run the provider as
   UID 1000, and mount neither the host home nor the Docker socket.
2. Place the provider container on an internal Docker network. A separate
   gateway exposes only the local development Multica API and the already
   configured model provider. Preserve API Authorization headers, keep model
   authentication private to the gateway, disable external DNS for the provider,
   and verify denied direct connections and unsupported gateway destinations.
3. Use a new dedicated daemon identity and inspect its registered namespaces
   before allowing task consumption. Preserve historical shared role bindings.
   Create seven listed members through supported template creation. The
   unlisted maintenance leader cannot be duplicated through that endpoint, so
   create a private custom coordinator from its actual instructions, explicitly
   set its approved Coordinator level, and record `derived_from` evidence while
   leaving its real `template_key` empty. Verify content, skills, autonomy and
   Access before use. Limit roster/instruction updates and the local-directory
   owner change to the pilot squad/project, capturing prior values and using
   normal API concurrency checks.
4. Copy the unchanged synthetic fixture into a fresh isolated lab. Record its
   source hashes and commit. Recheck the six-test baseline: four successes and
   the two expected assertion failures. Do not edit repository fixtures.
5. Trigger a true leader with an exact squad mention. Prove one read-only
   leader-to-member-to-leader roundtrip first, including `action` on dispatch,
   actual member author/task attribution, and a final decision after return.
6. Run the dependency, authorization, and missing-supply-chain-evidence cases
   with real role identities. Include migration review and diagnosis, and keep
   supply-chain risk `hold/unknown` with an explicit evidence owner. Correct
   test-count/source/enum claims against raw commands and final files.
7. Recheck the synthetic production-escalation stop condition without issuing
   any production command, credential operation, or external notification.
8. Have an independent Trellis reviewer audit current evidence and task-local
   artifacts. Only update acceptance checkboxes when the corresponding current
   run proves them. Phase two remains gated on phase-one acceptance.

## Change scope

- `pilot/runtime/`: Codex wrapper, image/gateway configuration, and setup/check
  instructions needed to execute the existing isolation requirement.
- `pilot/evidence/` and task-local research: redacted runtime, source, routing,
  command, result, and cleanup evidence.
- Task planning/status/results files and relevant specs: current progress,
  verified contracts, and remaining gaps.

This continuation does not edit product code, built-in templates, database
schemas, shared role policies, personal model configuration, or production.
The gateway is a pilot boundary, not a new product execution subsystem. All
credentials stay out of tracked files, command arguments, and evidence output.
