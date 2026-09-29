# 第一期实施计划

**目标：** 落实 PRD R1–R7，并以 AC01–AC14 为最终验收标准。

**架构：** 扩展 core 的既有快速创建偏好，views 内封装专用选择器与纯数据模型，复用查询和基础组件；不新增依赖或服务端合同。

**技术栈：** React、TypeScript、Zustand、TanStack Query、Base UI／PropertyPicker、Vitest、Playwright。

**执行依据：** 使用 writing-plans 的分步验证方式；阶段切换与实施／检查分工遵循本仓库 Trellis。所有命令从仓库根目录执行。

## 当前进度

- [x] 用户确认第一期方向并要求创建 Trellis 任务。
- [x] 建立独立 planning 任务，保存现状截图。
- [x] 完成现状调研、需求和技术设计。
- [x] 独立方案审查、修订与上下文校验（APPROVE，见 research/plan-review.md）。
- [x] 2026-09-29 用户明确要求开始实施，已激活任务并建立隔离工作区。
- [x] 实施步骤 1–5：代码、真实交互、Web／Electron 及性能验收通过。
- [x] 整体验收与规范更新；提交和归档按本次交付记录收尾。

## 0. 开始实施前

- 重读 prd.md、design.md、研究记录和 implement.jsonl 中的真实上下文。
- 检查当前工作树：本任务规划时已有其他工作修改 quick-create-issue.tsx、相关测试、modals.json；不能回退、覆盖或顺手提交。
- 本任务通过 `create --no-start` 建立，未覆盖其他会话的 task fallback。获得本任务最终方案的实施确认后，使用当前会话身份运行：

```bash
python3 .trellis/scripts/task.py start .trellis/tasks/09-29-quick-create-actor-picker-phase1
python3 .trellis/scripts/task.py current --source
```

- 确认 active path 是本任务，再分派实施。不要对技能库拆分任务执行 finish、start 或 archive。

## 1. 固定与最近偏好，先锁定跨工作区回归

**文件：** `packages/core/issues/stores/quick-create-store.ts` 及现有测试。

1. 添加会失败的行为测试：固定去重／移除、最近上限与顺序、旧 lastActor 兼容、缺失字段／坏 JSON、A→空 B、切换后微任务前的旧就绪标志、过期 hydration 完成、logout 清理及非持久会话代次。
2. 运行该套件确认测试失败是缺少目标行为，而不是安装或测试环境故障。
3. 最小扩展现有 store；只持久化白名单偏好，处理 rehydrate 默认值和错误路径。
4. 跑同一测试直到通过；确认没有把服务端对象或临时 UI 状态保存到 store。

```bash
pnpm --filter @multica/core exec vitest run issues/stores/quick-create-store.test.ts platform/workspace-storage.test.ts
```

检查点：偏好数据规则通过，业务 UI 尚未接入也可独立评估。

## 2. 搜索与分组模型

**新增：** `packages/views/modals/quick-create-actor-picker-model.ts` 及 `.test.ts`。

1. 先以纯函数测试覆盖非法／缺失描述安全降级、身份 type:id、稳定顺序、收藏／最近去重、快捷上限、完整目录搜索、类型交集、长描述末尾匹配、分页不缩小搜索域。
2. 复用 matchesPinyin 和 descriptionPreview，保留原始描述作为搜索源；明确空描述和自定义对象行为。
3. 测试中采用名称相同但类型／ID 不同、不可用引用、0／1／550 对象等数据。
4. 单一 node 测试层拥有完整矩阵，组件测试不再次复制所有组合。

```bash
pnpm --filter @multica/views exec vitest run modals/quick-create-actor-picker-model.test.ts issues/components/description-preview.test.ts
```

检查点：未引入查询、浏览器 storage 或 DOM 依赖的纯规则通过。

## 3. 专用选择器、真实键盘交互与文案

**新增：** `quick-create-actor-picker.tsx`、`.test.tsx`。
**修改：** PropertyPicker；新增其专用测试；en／zh-Hans modals.json。

1. 先写真实基础组件测试，禁止将 PropertyPicker 整体 mock 成 div 后声称验证了键盘。
2. 构建 home／all／favorites 与派生 search；常用、最近、完整目录和分批显示。
3. 两行候选、明确类型、兄弟固定按钮、加载／错误／无结果状态与重试。
4. 实现 navigationResetKey 的可选行为和焦点恢复，保证无此参数的调用方不变。
5. 用键盘验证输入法、上下选择、固定、取消固定末行、筛选后 Enter、Esc 返回触发器；补齐中英文文案及 aria 标签。

```bash
pnpm --filter @multica/views exec vitest run modals/quick-create-actor-picker.test.tsx issues/components/pickers/property-picker.test.tsx issues/components/pickers/assignee-picker.keyboard.test.tsx locales/parity.test.ts
```

检查点：独立组件可操作，手工选择与辅助操作互不触发，不能只靠截图验收。

## 4. 接入表单，保护既有创建行为

**修改：** `packages/views/modals/quick-create-issue.tsx` 与 `.test.tsx`。

1. 抽出原 ActorPicker，接入新组件及查询状态；不改其他创建模式和 IssueDescriptionAssist。
2. 在普通快速创建与评论上下文创建共用的成功返回位置调用 recordSuccessfulActor，发请求前捕获被提交对象、工作区、用户及会话代次；请求期间更换选择不能改写这次历史对象。
3. 两种创建分支均先补“成功接受才记录”“失败／取消不记录”“A 请求迟到不写 B”“登出同账号重登录不写旧会话”的集成回归。
4. 复查显式对象→草稿→lastActor→首个可用智能体的预选顺序、权限／版本、小队路由与正文保留。

```bash
pnpm --filter @multica/views exec vitest run modals/quick-create-issue.test.tsx modals/quick-create-scenario.test.ts modals/create-issue-dialog.test.tsx modals/create-issue.test.tsx
```

检查点：完整流程正确，新增推荐／分类或后端改动视为范围扩张，应回到设计评审。

## 5. 浏览器、规模与视觉验证

**新增：** `e2e/quick-create-actor-picker.spec.ts`。

- 使用现有 TestApiClient／测试 fixture 准备数据，或在 UI 专项场景中拦截目录和提交响应。提交验证使用假运行时／受控响应，禁止调用用户已安装的真实智能体 CLI。
- 冷启动、3 个常用＋5 个最近、超过 3 个常用、全部常用为空、AI小队筛选、查询尾部对象、跨工作区、重开恢复及失败重试。
- 550 对象场景验证分页可以到达末尾、搜索覆盖全目录。性能采样至少 20 个有代表性的查询，记录构建模式／设备／浏览器及 P95，不将机器相关阈值加入易抖动的单元测试。
- Web 1280px、Web 375px 窄视口、桌面实际渲染器：截图记录两行排版、搜索栏可达、悬停／选中区分及中英文长名。
- 每次视觉检查按 visual-verdict 执行；记录 JSON 到 `.omx/state/quick-create-actor-picker-phase1/ralph-progress.json`，截图及结论存本任务 verification.md / research。此状态文件仅为视觉证据，不启动 Ralph 模式。

```bash
pnpm exec playwright test e2e/quick-create-actor-picker.spec.ts --workers=1
```

## 6. 最终检查与收尾

```bash
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm lint
pnpm typecheck
pnpm knip
pnpm check:ui-exports
git diff --check
```

结果须记录在本任务 verification.md，包含命令、通过／失败、AC 映射、浏览器与截图路径、未验证项。已有全仓错误与本任务引入的错误分开记录，不能把未运行检查写为通过。无 Go 修改，无需为本任务运行 Go 数据库测试；若实施扩展到后端，先更新方案及验证范围。

最终检查必须覆盖全部改动，尤其 core／views／PropertyPicker 的跨层边界。更新英文规范 `.trellis/spec/views/frontend/quick-create-actor-picker.md` 和 index.md，记录当前真实实现，不将尚未完成的方案写成规范。

按项目 Lore 协议提交本任务变更，独立列出并排除其他人的未提交文件。不推送／部署。验证通过且所有 AC 有证据后，按 Trellis 完成归档与记录。

## 人员与依赖

这是一个功能交付，采用单任务、串行集成；不人为拆成无法独立验收的子任务。

- 主负责人：维护需求、处理设计取舍、整合验证证据及任务状态。
- 实施：一个 executor（本仓库可用时使用 trellis-implement 的角色指引），负责上述明确文件范围，先读取最新并行改动。
- 审查：独立 code-reviewer／trellis-check，覆盖全部文件及 AC；不能只检查最后一批修改。
- core 偏好与纯模型可并行，但组件依赖两者，表单接入依赖组件；最终浏览器验证须等待集成完成。

## 回退点

1. store／模型未接入前，撤销本任务的独立改动，不影响现有 UI。
2. 已接入后，需要回退时先恢复旧选择器及提交连接，再撤销专用文件与可选 shared API；保留其他任务修改。
3. 新增本地偏好可被旧版本忽略，可能在旧版重写时丢失；任务服务端数据不受影响。
