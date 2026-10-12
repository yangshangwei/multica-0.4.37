# Browser verification

The production component fixture at `.omx/artifacts/mcp-agent-category-filters/browser-fixture/`
was rebuilt from the current shared picker and run with synthetic cached agents,
role templates and squad leader metadata.

- 5 scenarios passed: 1280px Chinese dark, 1280px English light, 390px Chinese
  dark, 390px English light, and 680px English dark with a long custom category
  and long unbroken agent content.
- Category inventory showed General-purpose, Specialists, Coordinators and
  custom Research categories. The long custom chip remained reachable through
  horizontal overflow without document overflow.
- Cross-category interaction selected one coordinator and 20 filtered
  specialists, then confirmed category switching retained all 21 selections;
  bulk actions followed the visible category/search projection.
- Save-to-selection focus handoff, full-description search, selected-only
  review, clear/reselect, keyboard Space toggling, partial assignment failure
  and retry were exercised.
- Fixed footer, bounded list scrolling, two-line preview clamp and coarse
  pointer targets passed geometry assertions. Browser page errors: 0.
