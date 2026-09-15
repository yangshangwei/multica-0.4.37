# Localize desktop daemon settings

## Goal

Show the desktop Daemon settings menu and its content in natural Simplified Chinese when Chinese is selected, without changing daemon behavior.

## Requirements

- Follow the product glossary: Daemon is 守护进程; its CLI profile is a configuration profile, not a user profile.
- Localize navigation, preference descriptions, CLI availability, authentication and external-management notices, diagnostics, state labels, and feedback.
- Preserve preference keys/defaults, IPC calls, authentication decisions, lifecycle behavior, disabled states, technical identifiers, URLs, and commands.
- Use the existing settings namespace and keep all supported locale key sets in sync.
- Preserve all pre-existing working-tree changes.

## Acceptance Criteria

- [x] Chinese users see the translated menu and every settings branch.
- [x] Existing daemon and settings regression tests pass.
- [x] Locale parity, relevant TypeScript, lint, and diff checks pass.
- [x] The rendered Chinese page keeps the existing layout and usable controls.

## Notes

- This is a presentation-only localization task, with no dependency or backend changes.
- User explicitly authorized implementation and requested unchanged functionality.
