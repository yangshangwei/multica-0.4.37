# Fresh collaboration performance rehearsal

All three canonical Playwright phases passed with zero retry/skip on the isolated API (pool max25/min5, one process, no Redis), native ARM64 PostgreSQL and the current captured candidate. This is a controlled local rehearsal under concurrent workstation load, not production capacity certification or commercial model quality.

| Phase | Actual denominator | Result |
| --- | --- | --- |
| Registration/setup |100 distinct password accounts via public API|PASS; normal rate-limit windows retained|
| Status propagation |100 sockets ×100 events =10,000 samples|PASS, zero missing; pooled P95 736.1ms; per-event slowest observer P95 820.1ms; budget2000ms|
| Business result writeback |100 controlled daemon runs|PASS,100 converged/0missing; P95 278.1ms; budget5000ms|
| Finalization |100 same controlled runs|PASS,100 converged/0missing; P95 1028.7ms; budget5000ms|

Raw samples, per-event/per-run correlation, source/binary/API manifest, resource observations and full denominators are in evidence/. Private actor credentials remain only in the owned private run directory; they are not copied into the report. The daemon and100 sockets were closed; deployment gates restored to enabled=false,creation_enabled=true,host_execution_enabled=false with epoch2→4. API cleanup is recorded separately.

The Provider is deterministic; the real API, daemon and run-local CLI paths execute. Pooled statistics retain every sample; no slow/missing sample was discarded. Original registered protocol and immutable manifest are retained.
