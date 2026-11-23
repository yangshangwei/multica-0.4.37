# Claim lane takeover (live)

Owner: native Codex child `/root/knowledge_claim` under session `01a1979f-c71b-7ea2-8734-185ab2bdc8f8`.
Scope: only `mc-kfix-lane-claim`, H3/M3 and claim review follow-ups; no integration-tree writes.
Base: `3110295d0`, clean at takeover. H1/M1 commits preserved.

H3 deep-history regressions reproduced against the base: 260 claimed-only rows block legal resume and prevent valid deep-exposure controls. M3 original proposed seq_scan assertion was ineffective because PostgreSQL uses a full admin_task_page_idx scan; replaced it with an EXPLAIN probe requiring a negotiated-row partial history index. Separate baseline-overlay red evidence now fails as intended.

Current implementation: exact-session keyset pages of 128; complete UNION namespace key pass then primary-key delivery batches; missing/NULL-version batch rows refuse. New concurrent indexes 761–763 and cleanup registration. No real Provider calls.

Important: see `claim-takeover-incident-2026-11-01.md` for an isolation violation caused by an unscoped migration command. It must be included in coordinator reporting. Default local multica was forward-migrated; no rollback attempted. Explicitly scoped green database and all subsequent commands are guarded.

Coordination limitation: all inbound `collaboration` payloads shown to this child have been empty. This file is authoritative live status until the messaging problem is resolved. The original dispatch text is visible in inherited context and has been followed. The trailing --settings review follow-up has NOT been started because pkg/agent/knowledge_launch_test.go is dirty in the launch lane; file ownership must be settled first.
