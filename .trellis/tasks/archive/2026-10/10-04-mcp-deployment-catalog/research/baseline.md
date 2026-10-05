# Baseline and implementation recovery

The original checkout has pre-existing MCP parameterized-catalog changes and independent admin/onboarding work. Their bytes are the baseline, not task edits. Work in a sibling isolated git worktree with the dirty baseline copied; preserve a content manifest and backup outside tracked source. Merge back only task deltas and require current original bytes to match the copied baseline before overwriting any file.

Existing skill deployment source: server/internal/service/skill_template_dir.go and packages/views/skills/components/skill-library-catalog.tsx. Existing MCP list and resolver currently use a compiled roster independently; both must use the new Catalog. Existing template_version is TEXT/string; no version-column migration is needed.

Native architect role failed with unsupported configured model during design, so use native default agents with inherited model and specialized prompts. No external model invocation is required.
