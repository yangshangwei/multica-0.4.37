# Independent v14 live source and resource witness

These are actual read-only observations of the existing sole-owner run, not a new test run or whole-check acceptance. Source candidate: `d0b132fff0c477eb2d744933e13db730240231aa` at `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`. Existing coordinator/executor/claim were retained. Knowledge allocation/calls remain0/0 and actual API injection is false.

## Phase3 baseline and migrated API

At `2026-11-03T14:45:35.030319+00:00`, the explicitly registered API database `multica_check_20261103141424_39386_api` was reachable as user `multica`, with `schema_migrations_present=false`. No max migration query ran before API health existed.

Stage4 was observed at23:02:11 local. At `2026-11-03T15:02:43.077743+00:00`, stage5 API health returned `{"status":"ok","pid":74970,"commit":"d0b132fff","started_at":"2026-11-03T15:02:20Z"}`. PID74970 was the actual listener on18974; PGID74907 and physical cwd are the registered candidate's `server/`. Registry command is current-source `go run -ldflags '-X main.commit=d0b132fff' ./cmd/server`; actual executable was in the registered Git-free build temp, SHA-256 `551d4ba84a932db55a820da4312f6ca55ce4a315dac81454afbf720cde94df98`.

The API process's actual DATABASE_URL was read in memory and exactly matched the registered environment; no raw URL or credential was recorded. Direct read-only SQL returned the registered database/user and latest migration `764_collaboration_ai_policy`. API TCP sockets and same-DB PostgreSQL connection observations are saved separately. Actual process flags were legacy auth, managed installations false, device auth false and knowledge false, matching this full-check environment.

## Production Web and E2E live snapshot

At `2026-11-03T15:03:44.386665+00:00`, stage6 was live. Web listener75426, PGID75407, physical cwd `.../mc-collaboration-closure-v12-20261103/apps/web`; parent75407 ran the candidate-owned pinned pnpm command `next start --port13894`. Actual registry provenance says production/commit d0b132fff and build ID `0PjcBGIACQBVc6g8g3Wp5`, matching the current `.next/BUILD_ID`.

The actual anonymous HTTP HTML serialized `apiBaseUrl=http://localhost:18974` and `wsUrl=ws://localhost:18974/ws`, matching the supplied environment. Its full public HTML and SHA-256 were captured while E2E was running, before cleanup. API74970 was still healthy and the database was still current at this snapshot. The Next server process's displayed argv did not expose environment values; the parent process environment and actual rendered values are separately observed evidence.

## Renderer, Redis and source

Renderer55132 actual PID35776/listener/cwd and HTTP index bytes match the registered current Desktop artifact (SHA-256 `0793d8aea46f27db7c7cfd48dbb0b04c0efa475291200ac8c3d4b0fd84f69847`). Redis55131 actual container/image/labels match its original resource ownership: container `6da37607cf2597b4abca9f7ae81306af6bb2fbae6b0c96828455113bf12b5dc8`; owner `01a1a186-9a5c-7b00-96f8-0430b47edb4c`, candidate d0b132fff. No resources were replaced or restarted.

Actual source HEAD equals d0b132fff. At stage6 the sole dirty path was the previously registered Next-generated import rewrite. Selected current source hashes and physical process roots are observed; whole tracked-file/tool/workspace/CLI/Desktop binding remains the registered final auditor's responsibility. This witness never invokes a second auditor.

## Source evidence

- `snapshot-144535030361.json`: phase3 own-DB empty baseline. SHA-256 `4c20ce86361e03b8244ba5c77e654c2a8398704e3a1a64d62a9e7f5e0b0beaba`.
- `snapshot-150211891007.json`: actual phase4 entry before health. SHA-256 `6171c6d795ae9c1ae7210613f8c6e3bfd8ad6b5bdc8cea8eda949e23fad58771`.
- `snapshot-150243077797.json`: actual API/process/migration/config/source ownership. SHA-256 `504bd469091c56a0d3e1efee04f1fbe1df8fcd686749846dac6242c87c6b4176`.
- `snapshot-150344386750.json`: stage6 live API/Web/database/renderer/Redis and actual rendered client config. SHA-256 `7a64adc214ddb5bf0bc8508154d0e6e059621335eb707717796e846e6c9da1aa`.
- `api-database-binary-binding-150458535156.json`: direct process URL equality and actual temporary API binary hash. SHA-256 `a546202902da8a5838d93afc5e2f6ccafafb2fc202d436d1084cd4f583343f99`.
- `observations.jsonl`: timestamped phase/state snapshots; later periodic observations remain separate.

The witness starts only read-only ps/lsof/git/HTTP and explicit own-DB SQL, plus read-only inspect of the registered Redis. It starts no tests/builds/Go/pnpm/browser/Provider processes, no DB writes or service changes, and writes only this evidence directory. Actual sole make exit, automatic final auditor result/hashes, complete named/explicit-skip inventory and root terminal acceptance remain pending.
