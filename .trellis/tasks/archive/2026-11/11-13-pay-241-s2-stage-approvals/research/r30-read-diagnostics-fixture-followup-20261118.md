# R30 diagnostics fixture follow-up

**FROZEN_FIXTURE_ONLY_READY_FOR_MAIN_VERIFICATION.** Only the new test's capture-directory setup changed in the fresh GREEN copy. The original0700 assertion,0600log and every forwarding/cap/principal/test-name assertion remain unchanged. No observer or product repair was made; no fresh test pass is claimed.

Changed file: `/Volumes/artisan/.multica-acceptance/pay241-r30-qa-read-20261118-pohc3l9t/source-green-r01/server/cmd/server/pay241_read_diagnostics_scope_test.go` —249lines,0644, SHA256 `1fa8a5ec3f709b7b98b4d53ec5a31bff3cc56b7583d1fe396b84bf6c45431c1e`. RED remains246lines/0644/`093f606876e53035624868dd2e00411f4e009a503cadb6e5352837c6e8ac1fbc`. Existing observer remains236lines/0600/`326e53f91153e08762929c85f1b83246f35c5f6b649a0279e527402ef48a1dd6`.

The complete patch is:

```diff
--- RED/server/cmd/server/pay241_read_diagnostics_scope_test.go
+++ GREEN/server/cmd/server/pay241_read_diagnostics_scope_test.go
@@ -94,7 +94,10 @@
 // an account, run SQL, or fabricate a native query/PID/SQLSTATE witness.
 func r30DiagnosticPreservesResponse(t *testing.T, path string, status int, payload []byte, authenticated, atLimit bool) {
 	t.Helper()
-	parent := t.TempDir()
+	parent := filepath.Join(t.TempDir(), "capture")
+	if err := os.Mkdir(parent, 0700); err != nil {
+		t.Fatal(err)
+	}
 	logPath := filepath.Join(parent, "read-diagnostics.jsonl")
 	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
 	if err != nil {
```

The fixture creates its own `capture` leaf with checked `os.Mkdir(parent,0700)` **before** `OpenFile`. Test cleanup still owns the outer `t.TempDir`. Installed Go's `os/file.go:324–330` documents and implements requested permission bits before umask; `os/tempfile.go:82` likewise documents MkdirTemp0700. These files were read, not executed. The original assertion still rejects a directory whose actual permissions differ. No shared root was chmodded, and no environment, ancestor-trust rule, pool or timeout was altered.

Main's genuine first normal run is preserved: **2top/9leaf/11named;7PASS/4FAIL/0SKIP**, where all six path leaves and their parent pass and the three response leaves plus parent fail. Raw result SHA256 `59180ad8a2ef1f800420db50dac350a18843bf8e1201394d71354c262636aba6`; stdout `076111092e338b1bf8810e785071bd321c2a52a87706894c20f1d2293cff581b`. Each response leaf stops at “the isolated capture parent is not mode0700: <nil>” before observer forwarding. This is an **infrastructure setup failure**, not a business/observer RED or proof of the original QA503 cause. Original race was NOT_EXECUTED. The copy/failure boundary receipt `0bf74c24afcb0c54bc12c35601228c0aac693edd6b0ec68607472aed49722070` and all RED/raw/build/prior reports remain intact.

Static verification completed: **5053same paths/modes; exactly1test-file delta;5052other files identical; REDdrift0**. Exact whole-file substitution proves the only edit is the four-line directory block above. Test inventory retains **2top/9leaf/11named** and selector `^(TestPAY241ReadDiagnosticExactGETScope|TestPAY241ReadDiagnosticResponsePreservation)$`; response/capture assertions and every other byte remain unchanged. Final canonical5053manifest fingerprint is `43249cefbf9209eef3002c0d57a4d5b575e57e34278aa0f899032926010dfab1`; JSON includes the manifest, patch and exact names. The single-file gofmt-w and gofmt-l both exited0 with empty output. This agent ran no Go AST/compile/vet/test/list/TestMain, DB/SQL/API/browser, signal or Git command.

Main next step: use its new fresh router normal/race profiles, allocation SHA256 `0adca7a0313bc8d71bfa070bd4d8b2e37e291bfc1af15353360752a32c1a0069`, to verify the corrected GREEN source and actual11name terminals. A follow-on forwarding failure would be new evidence; do not assume GREEN from this setup correction. Compile/vet and only the matching harness rebuild belong to main; the old harness binds the failed RED test bytes. Retain all other accepted R29 products and the original failed/completed history. After independent acceptance, main may bind this source/current harness to the planned original4once under unchanged deadlines and no retries. The QA503 native cause/holder and full S2/A01–A12/parent/S3–S5 remain unresolved by this fixture change. Captured UTC `2026-11-17T20:24:18.808226+00:00`.
