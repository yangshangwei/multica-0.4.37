# Remaining Go conditional verification

Prepared from current candidate source; not executed by this plan. Root alone allocates fresh databases. Exact packages, names, prefixes, environment and selected patterns are in backend-conditional-plan.json / private conditional/requirements.json.

Knowledge integration adds23 unique top-level tests (14 ordinary daemon,9 host/server) behind knowledgeintegration. Run normal and race separately on four fresh databases with their source-enforced prefixes; no extra opt-in variable, no installed provider or account. Default cmd/server TestMain still requires explicit database even when selecting only tagged tests.

Wire export: TestTriageWireFixtures needs TRIAGE_WIRE_FIXTURES to private writable output; feed its7 actual JSON payloads into current parseTriageResponse/schemas. Do not validate a separately invented fixture.

Performance: existing fresh root allocation multica_core1a32_20261104_performance; no race;3 exact tests with original thresholds/samples. Run after heavy Go/browser/build work finishes.

Linux: six router cases need actual Linux, explicit E2E/namespace gates, test-built CLI and daemon. One daemon slot case needs actual namespaces. Three agent signal/stdin/root cases need actual Linux and executable temporary filesystem. Source compiles alone do not close these gates.

Default skips also include missing pg_bigm, Windows-only behavior, one intentionally unimplemented Webhook error assertion, no HTTP recipe in shipped MCP catalog, and public GitHub import opt-in. They require separate disposition; real-agent tags remain deferred by user.
