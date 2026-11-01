# Native renderer origin preparation independent review

Verdict: **pass-with-notes**, no blocking defect found. Reviewed tip `e1ea3aeb59ee8a1d8a4b5249fd713f4332115ac9`, product `b8035617d`, against `c3db64b24`. Reviewer `/root/knowledge_web_review` is independent of the implementation. Only this review directory was written; no real check environment, database, migration, API, browser/GUI, Agent CLI, Provider, dependency installation or external CI operation ran.

The helper derives origins only from the allocated Web origin and the two existing explicitly configured Electron fixture URL variables. It never reads the source deployment's CORS/ALLOWED values. Canonical HTTP(S) URL parsing restricts hostname to localhost/127.0.0.1/[::1], rejects credentials and port0, removes path/query from allowed origins and deduplicates values. Implicit80/443 are nonzero default ports. Invalid inputs stop preparation before source copying or service start and print only a fixed content-free error; no raw credentialed URL is echoed. The source env remains byte-identical, default unconfigured behavior remains Web-only, and both HTTP and WebSocket allowlists receive the same task-only CSV.

`check_main` loads the trustedsource.env before environment preparation. Both allowlist assignments are present in the copied taskenv and loaded/exported before API launch; API configuration identity includes those actual envfile bytes. Existing local-env code does not override the renderer/CORS variables. Supplying these URLs only to a later Playwright child would remain too late, and the updated spec states that requirement explicitly. Production CORS/WebSocket/API code and exact membership policy are unchanged.

## Independent fake verification

- `bash scripts/check.test.sh`:exit0, all11 named native-origin cases pass, plus the existing fake orchestration/source-preservation/Make-launcher contracts.
-11 additional pure helper cases:pass,0fail/skip. These cover defaultWeb-only, case normalization, default port canonicalization, dedupe, remote/privatehost, localhost suffix, wildcard, IPv6port0, port overflow, userinfo and one-invalid-input failure. Raw rejected URLs containing the test-owned credential marker never appeared in output.
- `bash -n scripts/check.sh scripts/check.test.sh` and `git diff --check`:exit0.

Author fake check/dev-env/test-go log hashes and current script/spec hashes match their manifest. The final canonical test bytes reproduce the actual baseline preparation omission with exit1; the earlier scratch replay setup failure is retained and excluded from that red evidence. Exact source/diff/command/result hashes are in `review.json`, `check-fake.json`, `syntax.json` and `helper-edge-cases.json`.

## Remaining acceptance

NO-1: this proves isolated configuration preparation, not actual native browser/WebSocket acceptance. Put the existing renderer URL values in the trusted source env before the next complete source-bound candidate run.

NO-2: the intentionally interrupted c3makecheck remains incomplete; previous static/13,082 TS passes retain their limited scope. No fake result in this review relabels that old run passed. Actual full makecheck/native results remain coordinator gates, and no product self-fix was applied by this reviewer.
