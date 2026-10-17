# E2E contract reconciliation

## Evidence and plan before edits

The previous full check (`/tmp/multica-password-check.log`) passed 117 browser tests and failed 9. Source and retained error snapshots identify stale test contracts:

- Navigation/settings expect `Issues`; the sidebar now names this link `Workspace issues`.
- MCP recovery expects `View Playwright`; the catalog labels it `View configuration: Playwright`.
- Squads now use `Workspace` / `Templates` tabs and a dialog chooser containing buttons, replacing a dropdown menu.
- Built-in skill cards intentionally display the localized summary and expose the full description as a title; detail/preview continue showing the full description.
- New-workspace onboarding starts at Welcome before About you (documented and implemented by `onboarding-flow.tsx`).

Update only these obsolete interactions/assertions, preserve successful-response, persisted-data, collision atomicity, localization, keyboard, viewport, and navigation coverage. Keep fixture and password test ownership with the leader. Collect tests and inspect the diff; the leader will execute the full verification pipeline.

## Changes and validation

Updated six E2E specs: localized-template-defaults, main-supplemental-qa, missing-user-session, navigation, settings-preferences, workspace-defaults. Every original scenario remains present; the skill-card assertion now checks both visible summary and full-description title. The squad chooser is scoped to its dialog to distinguish the identical empty-state action.

`pnpm exec playwright test --list` exited 0: 151 tests in 52 files. `git diff --check` passed. Browser execution remains pending the leader's full check. No product implementation changes made in this slice.

最终全仓运行还发现模板图标的可访问名称被包含在 heading 中，使 exact-heading 定位失败（125通过、1失败、27跳过）。模板已有独立且唯一的 listitem 名称，改用该语义定位验证模板可见性；生产环境 `workspace-defaults.spec.ts` 三项全部通过，完整 E2E 由主任务复验。

最终完整生产 E2E 复验退出 0：126 通过、27 条按配置跳过，6.7 分钟。日志 `/tmp/multica-password-e2e-recheck.log`。9 个原失败场景全部恢复；没有跳过失败测试或修改产品来匹配旧断言。
