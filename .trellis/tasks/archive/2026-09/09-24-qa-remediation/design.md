# Approved design

See docs/plans/2026-09-24-qa-remediation-plan.md sections 3-4 and task-specific design in section 5. The owner coordinates one transaction spanning child, metadata, audit and queue; notifications happen only after commit. Preserve 8 KiB metadata and bounded history. Existing wrappers share extracted transaction cores.
