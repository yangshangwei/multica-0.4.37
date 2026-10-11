---
target: 项目详情概览（系统健康、目标验收、项目进展）
total_score: 26
max_score: 40
na_heuristics:
p0_count: 0
p1_count: 3
target_identity: "file:/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-overview.tsx"
target_fingerprint: "sha256:44a748d4acbdd530d118e48969405ed419c744b532187ee26bed09b807a67195"
target_path: /Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-overview.tsx
timestamp: 2026-10-10T15-22-01Z
slug: projects-components-project-overview-tsx-8a539a8c
---
## Design Health Score

| # | Heuristic | Score | Key issue |
|---|---|---:|---|
| 1 | Visibility of system status | 3/4 | Counts, health state, stale acceptance and refresh are visible; the primary risk state has little visual structure. |
| 2 | Match between system and real world | 3/4 | “阻塞 / 逾期 / 未分配 / 待验收” is understandable, but the relationship between status, reason and action is text-heavy. |
| 3 | User control and freedom | 3/4 | Risk buttons, refresh, correction and history give control; progress entries do not offer a compact way to collapse or filter the feed. |
| 4 | Consistency and standards | 3/4 | Shared buttons and typography are coherent; the sidebar progress bar still uses a hard-coded color. |
| 5 | Error prevention | 2/4 | Stale acceptance is marked, but health warnings are spread across repeated paragraphs and are easy to miss or misread. |
| 6 | Recognition rather than recall | 2/4 | Most meaning is carried by text labels and bullets; signal categories are not visually distinguishable at a glance. |
| 7 | Flexibility and efficiency | 3/4 | Risk signals drill into task sets and progress entries support correction/history; a long feed lacks filtering or progressive disclosure. |
| 8 | Aesthetic and minimalist design | 2/4 | Large blank regions coexist with a dense text column, so hierarchy feels flat rather than intentionally calm. |
| 9 | Help users recognize, diagnose and recover from errors | 3/4 | Retry, stale warnings, conflict compare and evidence links are strong; the health area does not pair every warning with a clear next action. |
| 10 | Help and documentation | 2/4 | Labels explain the data, but there is no concise explanation of “范围闭合” or why a signal matters at the moment of scan. |

**Total: 26/40 — Acceptable; significant hierarchy work is worthwhile before adding more content.**

## Design specificity verdict

The implementation is product-specific in its semantics: formal tasks, scope closure, acceptance versions, evidence and AI squads are clearly Multica concepts. The visual treatment is currently category-neutral because the core information is rendered as headings, numbers, outline buttons and bullet text. The detector found no mechanical violations in the three target components (`impeccable detect --json` returned `[]`).

## Priority issues

- **[P1] Health is a flat text stack.** `ProjectOverviewPanel` renders one status sentence, four outline buttons, a bullet list and several extra warning paragraphs. Users must read each line to understand severity and next action. Give the health state a semantic icon and status label, turn each signal into an icon + label + count action, and consolidate all reason strings into one “需要关注” list with progressive disclosure.
- **[P1] Progress has no visual chronology.** `ProjectProgress` renders each update as a border-separated article with the same text treatment, so progress, risk and acceptance entries look identical. Use a timeline rail and kind icons (`TrendingUp`, `ShieldAlert`, `ClipboardCheck`), keep author/time/revision as metadata, and collapse long body/evidence details behind “展开详情”.
- **[P1] The page spends hierarchy on headings while the key decision is buried.** “概览 / 系统健康 / 目标验收 / 项目进展” all use almost the same heading weight and spacing. Use a compact overview header, then a two-column summary row: health as the primary decision block and acceptance as a smaller state block; let the timeline follow as the activity history.
- **[P2] The active tab and section labels miss useful scan affordances.** Add restrained icons to the “概览 / 任务” tabs and section headings (`LayoutDashboard`, `ListTodo`, `ShieldCheck`, `Target`, `Activity`) while keeping text labels. Icons should reinforce meaning, never replace the label.
- **[P2] Progress color is outside the token system.** `project-detail.tsx` uses `bg-emerald-500`; replace it with the existing semantic `bg-success` (or chart token chosen for scope closure) and add an explicit `2/5 · 40%` text label so the bar is not the only cue.

## Persona red flags

- **Alex, power user:** Can jump directly into a risk signal, but must scan a long unfiltered update feed to find the latest actionable item. Add “全部 / 进展 / 风险 / 验收” filters and keep signal chips as direct drill-down actions.
- **Jordan, first-timer:** “有风险” plus five bullets does not explain which item to fix first. Pair the status icon with a short priority line such as “2 个逾期任务需要处理” and an action affordance on each reason.
- **Sam, accessibility-dependent:** The current design uses red/yellow/green as emphasis, while the semantic relationship between the status and the warning text is not grouped. Use icon + text + count, maintain heading/region semantics, and ensure the risk chip exposes its signal and count in its accessible name.

## Positive findings

- The page uses a real `dl` for statistics and semantic `section`/heading structure.
- Risk counts are actionable buttons that route to exact task sets rather than decorative badges.
- Refresh, retry, stale acceptance, revision conflicts and evidence availability already have meaningful states; the redesign should preserve these behaviors.
