# Execution
1. Independently review plan and existing test gaps, then start.
2. trellis-implement adds bounded regressions/fixture preparation without real-model calls; only new task_capability_targeted_test.go in cmd/server or handler plus private task harness initially. Do not edit product code without reporting root cause and obtaining main ownership. Root prepares fresh DBs; child never allocates/migrates/drops.
3. Main uses actual codex exec --ephemeral/private minimal configuration with frozen synthetic prompts, at most3 trials180s, stop on gateway/auth. Read cli help rather than stale flag guesses. No real provider in go default tests.
4. trellis-check independently reviews target matrix and actual outputs; errors reported/fixed then new evidence run. Build/vet/gofmt scoped to changes. Source and report hashes retained.
5. Main updates parent delivery with actual results and limits, Lore local commit. No full milestone closure.
