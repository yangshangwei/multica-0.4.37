# 实施与检查计划

## 执行授权与评审门

用户已明确授权本会话完成任务创建、计划、评审、实现与检查。先完成 PRD 收敛、设计/计划独立评审和 manifest 校验，通过后 task.py start；按项目 auto 配置派 trellis-implement，结束后由 trellis-check 独立检查。原工作区未提交文件基线在 research/worktree-baseline.json.gz。

## 1. 计划就绪（主会话）

- [x] 读取 AGENTS/CLAUDE、Trellis workflow/index/相关规范与正式文案约定。
- [x] 复制原审计与结构化证据，记录已有文件 hash 和分支。
- [x] 研究创建/恢复契约、拒绝码、身份 projection、picker 调用方、焦点和滚动边界。
- [x] 基线：creation-form/account-picker/field-editing 三文件 15 tests 通过。
- [x] 收敛 PRD 的 R1–R8 与设计/执行计划。
- [x] 评审发现并行 source-picker 改动；记录 pre-implementation 快照，保留新 popup/重试/失权清理/searchMaxLength/tests，防抖接入新实现。
- [x] 独立研究 reviewer 对 R1–R8、scope、ARIA、portal、恢复、验收/环境逐项挑战，保存 research/planning-review.md；修正所有阻塞项并复核。
- [x] 配置 implement/check JSONL（仅 spec 与 research，不注册待改代码），运行 task.py validate。
- [x] 汇报最终范围/验收/决策，task.py start 后才派实现。

## 2. P1 实现（trellis-implement，主会话保持单一代码 owner）

1. 在 creation-form.test.tsx 补延迟创建/恢复来源锁定、完成一次、拒绝后解锁、闲置 redacted recovery 不锁死的必要 wiring 回归；先运行新测试确认命中原缺陷。Core 的 receipt/epoch 矩阵不重复造一套。
2. creation-form.tsx 派生 scoped pending + record 锁，锁定 select/change；保留 active/scope guard 和原 core executor。
3. account-picker/property-picker 的必要 DOM 回归：listbox 关联、stable option/selected/disabled、活动后代、空结果/关闭/重排、IME/Escape、多选/分页；先见失败，再加 opt-in 关系。共享旧 keyboard suite 是其他调用方的兼容门。
4. 局部 opaque faint-foreground 焦点覆盖，包括 portal；不以 class 字符串测试代替浏览器合成测量。
5. 运行对应 P1 测试，确认无 scope/恢复/键盘回退，再开始 P2。

## 3. P2 实现（同一 owner）

1. quota 的真实 mocked POST + correlated rejection GET 回归（双语），保留 title/files 且无自动重提；扩展 labels 和两个 locale 的原因/后续动作映射。未知响应继续走 uncertain recovery。
2. 同名/同 UUID 前缀账号 regression；完整 ID 在候选、exact result、single trigger 显示；保留独立 selected-name 句与 external/membersOnly 行为。
3. account option、DialogContent 本地 close 命中区、field-inputs 的 parent search/stage 增加 coarse 44px；无全局行高改造。
4. 按现有 flex header/body/footer 重排受限弹窗，取消 callback 可选且原生 submit 不变；补独立 panel、取消 focus-return/no-source 与 native submit wiring 回归。
5. 父搜索 fake timer 回归：raw text 立即、249ms 不请求、250ms 最终查询；新输入后立即 Enter 不能选中旧候选；换来源清定时器并保留 query/authority/abort。加最小 250ms effect，在等待窗口也隐藏旧候选；既有晚到搜索测试等待请求实际开始后再切换，保留原断言。

## 4. 隔离浏览器（主会话；研究/证据文件独占）

- 先检查 Node/pnpm、构建脚本、现有 listener 清单和归属；本轮禁止 make 环境生命周期命令。
- 在本任务 research/preview 保存 fixture/配置/driver；使用已有 Vite/PostCSS/React/Playwright，不安装依赖。生产构建输出置于 task-owned 临时目录，loopback ephemeral port 静态服务，finally 只关闭本次 listener/context。
- 构建并采样编辑前原始组件以复核 R1–R8；记录自身 process/build/source provenance 和模拟 API，持久复制原截图。
- 每轮截图检查同时覆盖 desktop light/dark、390×740 和 320×568 coarse、默认/展开长表单；可访问 submit/cancel/close、无横溢、焦点/滚动、接收人 identity/AX 与实测坐标的合成触摸、Escape 和请求日志。
- source/title/description/parent/stage/recipient trigger+search+option/footer/close 的实际焦点 rgba 与相邻背景合成 ≥3:1。主题过渡稳定后测量。
- 使用 visual-verdict 比较原风格与新截图，保存 task research 和 .omx/state/collaboration-creation-audit-fixes/ralph-progress.json；首轮集中发现后只做必要修复，再一次批量确认。不得开无边界 polish 循环。
- 不发真实 API、不碰真实账号或用户 Chrome；Electron、读屏朗读、物理设备/iOS 不计为完成证据。

## 5. 检查门（trellis-check；主会话最终验收）

首轮浏览器反馈收敛为一个修正批次：显式 native outline-solid、局部标签 popup/入口焦点覆盖，以及受限主体的 mutation 反馈揭示。全量确认同时测双语 quota 文字与 unknown 恢复控件的实际可见性，不只检查 DOM 内容。44px 目标在 popup 入场过渡结束后测量。

实现者先运行并保存结果，再由 checker 检视代码/计划及缺漏，机械问题可在本次文件内自修；语义/设计问题报主会话，不能越界。

必要命令（按实际改动补齐文件名）：

```bash
pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx collaboration/field-editing.test.tsx collaboration/attachment-picker.test.tsx issues/components/pickers/property-picker.test.tsx locales/parity.test.ts
pnpm --filter @multica/views exec vitest run issues/components/pickers/assignee-picker.keyboard.test.tsx issues/components/pickers/stage-picker.test.tsx projects/components/project-picker.test.tsx projects/components/project-picker.open-state.test.tsx modals/quick-create-actor-picker.test.tsx
pnpm --filter @multica/core exec vitest run collaboration/delivery.test.ts collaboration/delivery-api.test.ts collaboration/delivery-form.test.tsx collaboration/access.test.ts collaboration/draft.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/ui typecheck
pnpm --filter @multica/web typecheck
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/views lint
pnpm check:ui-exports
git diff --check -- packages/views
```

先用 rg --files 核对所有文件/脚本存在，再运行；若某 Core suite 的正确名字不同，沿用实际文件，不猜路径。未改 UI primitive 时无需完整 UI lint；改到则加。无 Go/DB 改动不运行 make test/check，避免环境副作用。若全包 lint 因基线其他文件失败，保存输出，改动文件单独 lint 并明确范围，不清理他人代码。

实现完成后运行一次 impeccable detect --json 对改动 UI 目标；人工/自动发现须核实当前来源，合理保留现有风格。只在新改动、失败或明确未解风险时重复验证。

## 6. 收尾（主会话）

- [x] R1–R8 每项附当前行为、测试/浏览器证据和实际范围。
- [x] 核对 baseline 文件 hash、owned path diff 与所有新文件；他人变化记录为并行外部变化，不能覆盖或纳入本次提交。
- [x] 读取 trellis-update-spec，捕获必要 source/picker/focus 非显然契约；仅更新本次相关 spec。
- [x] 记录 progress/verification，按用户授权与工作区状态进行 scoped commit/任务生命周期，避免脚本自动 stage 他人 journal/task 变化。最终报告提供任务、修改文件、检查结果、实际范围及剩余风险。

## 回退点

P1 来源/ARIA、P2 文案/身份、布局、防抖按局部文件/段落可分别回退。任何回退都针对本次改动，不用 git reset/clean，不动现有服务。若功能要求迫使新增 API/store 或全局 UI 改造，停止扩大，回主会话记录设计变更并评审。
