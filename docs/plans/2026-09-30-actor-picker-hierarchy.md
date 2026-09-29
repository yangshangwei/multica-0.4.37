# Actor picker hierarchy

Approved direction: show specialists and squads first; fold coordination roles into a secondary group while retaining explicit selection and search.

Implementation:
1. Use eligible squads' `leader_id` relationships to identify current squad leaders. Never infer inability to execute from editable names or template provenance. Keep saved descriptions intact.
2. Fold leaders beneath the primary choices, including shortcut views. Search exposes matching leaders. Keep typed identity, favorites, defaults, paging, and keyboard navigation.
3. Explain that choosing a leader alone differs from selecting its squad. Keep actual creation semantics honest; user confirmed keeping the existing creation flow and truthful copy. Retain Creation assistant and explain default assignment plus explicit-request precedence.
4. Validate model relationships, real picker disclosure/search/selection, parent regressions, locale parity, lint/typecheck and visual layout.

No dependencies, persisted-state changes, or dispatch changes are required for hierarchy.

## Verification

- 127 tests passed: actor catalog, real picker primitives, quick-create parent and locale parity.
- Views TypeScript check and changed-file ESLint passed; targeted diff whitespace check passed.
- Impeccable static detector returned no findings.
- Independent review caught leader-only favorite navigation/counts; fixed with a regression test.
- Real component browser preview at 1100px and 390px passed overflow and search/Enter identity checks. The preview stubs data-backed avatars and does not exercise backend dispatch.
- Updated existing E2E expectations for folded leader rows. Full application E2E was not run: the configured frontend at localhost:13492 did not respond.
- Classification deliberately uses current eligible squad relationships only. A custom standalone coordinator with no squad relationship cannot be reliably identified by current metadata.
