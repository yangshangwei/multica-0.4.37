# CCG1 correction to actual input closure

The existing Host composite proves approved input and later invalidation, but does not create/approve a superseding candidate before a second physical process. Add one `knowledgeintegration` test in a new file, reusing the existing real router/DB/Runner/HostExecutor/captured test-binary fixture without editing production or shared fixture code.

Sequence: baseline human-approved v1 reaches actual first stdin; authenticated human HTTP routes propose v2 in the same lineage and approve the exact reviewed hash/version; new accepted input creates a separate run and physical process whose private preparation and stdin contain v2 but not v1. Read persisted candidate attribution/state and compare the first immutable preparation event before/after. Withdraw v2 through the real human route immediately before the third run's authorization, requiring real route refusal, no third physical launch, sealed nonlaunch proof, preserved first/second success and original proposal/approval receipt recovery.

This is an untested behavior-coverage gap, not a reproduced product defect. Do not fabricate a pre-fix red. The two test-owned users/actual child binary do not prove UI interaction, real model understanding, deployed environment flag wiring or account/platform acceptance. No production file, dependency, timeout, retry or database guard changes.

Root permit is READY for ONLY the new `^TestCollaborationKnowledgeHostCompositeCorrection` prefix in cmd/server under knowledgeintegration. Normal/race use their separate root-allocated fresh databases after identity verification. Raw JSON logs stay private; sanitized command/source/diff/result/hash evidence stays here. No child database creation/migration/drop, service or installed Agent CLI use.
