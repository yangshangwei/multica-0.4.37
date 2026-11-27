# Planning contract review

2026-11-27 independent review found two P2 issues: maintenance upgrades had no explicit candidate creation/old-proof policy; material.yaml format field was inconsistent (format_version/schema_version).

Resolved in design.md: persisted field is schema_version; §8 defines maintenance command, supported bundle selection, exact before/after hashes, separate human review and ready-proof replacement/retention. Reviewer reread the changed section and confirmed both resolved with no new findings.

This is design review only, not product code or runtime acceptance.
