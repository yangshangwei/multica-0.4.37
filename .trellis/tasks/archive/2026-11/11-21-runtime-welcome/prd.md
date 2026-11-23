# 运行时欢迎介绍页

## Goal

Help members understand runtimes and connect their first computer. Follow the supplied reference: introduction and primary action on the left, two illustrated connection paths on the right. With inventory present, display the existing operational machine list.

## Authorization

On 2026-11-21 the user explicitly requested task creation, planning, and continued development. This authorizes the complete scoped implementation and verification without another process confirmation.

## Background

- Shared Web/Desktop page and empty gate: `packages/views/runtimes/components/runtimes-page.tsx:92`, `:184`, `:570`.
- Incumbent responsive welcome/carousel: `packages/views/agents/components/agent-welcome.tsx:114`.
- Intranet deployments remove hardcoded public desktop download links (`packages/views/layout/help-launcher.test.tsx:127`). Existing onboarding uses validated operator configuration (`packages/views/onboarding/steps/step-welcome.tsx:55`): reuse `desktopDownloadUrl` when configured, otherwise use workspace-scoped bundled runtime help.
- Connection steps depend on OS/deployment and separate configuration, login and start (`.trellis/spec/views/frontend/connect-remote-computer.md`); do not promise two universally sufficient commands.

## Requirements

- R1: Render welcome only after successful runtime/profile inventory reads confirm no machines or pending profiles, no local placeholder, and no bootstrap. Loading/error must not masquerade as empty. Offline machines count as existing inventory.
- R2: Use the reference headline “给智能体一台干活的电脑”, a short runtime explanation, and two clearly labeled illustrative examples: desktop auto-discovery and remote CLI setup. Demo devices/providers/status are examples, never workspace facts. Preserve the shell/header.
- R3: Primary “添加运行时” opens the existing `ConnectRemoteDialog`; secondary “下载桌面端” uses the existing operator-configured download URL when present, otherwise “查看连接指南” opens the current workspace's bundled runtime guide. Retain existing header/cloud actions.
- R4: When inventory appears, replace welcome with the existing machine list automatically. Preserve grouping, row navigation, local bootstrap, pending profiles, and existing management/onboarding behavior.
- R5: English/Simplified Chinese, both themes, narrow Web and shared Desktop containers. Existing semantic tokens and primitives; manual two-card carousel with named keyboard controls, disabled endpoints, announced position, inactive-slide inertness, reduced motion and no autoplay.

## Acceptance Criteria

- AC1 (R1): Successful empty inventory renders one welcome; loading/error/bootstrap/local-placeholder/pending-profile states do not. Failed inventory offers retry without hiding cached nonempty inventory.
- AC2 (R2): Headline, explanation and two examples follow the reference's hierarchy and composition; illustration labels remain explicit.
- AC3 (R3): Hero action opens the real dialog; configured desktop download uses the validated operator URL, and its absence falls back to the workspace-local reader.
- AC4 (R4): Runtime appearance after invalidation removes welcome and renders a navigable machine, including an offline machine.
- AC5 (R5): Both locales/themes fit desktop and 390/320px widths; keyboard/manual carousel and reduced-motion checks pass.
- AC6: Focused regressions, affected lint/typecheck and production-browser visual/interaction verification have recorded evidence.

## Out of Scope

Backend/API changes, provisioning, connection-wizard redesign, public desktop distribution, new routes, native mobile, broad extraction, dependencies, and unrelated worktree changes.

## Risks

The worktree contains extensive unrelated changes: preserve them and commit only verified task files. Environment ownership reports mismatched listeners: verify an isolated production target before browser acceptance.
