# Research: R23 Submit result 被已打开菜单的正常遮罩挡住

- Query: 第四个实际场景的 Submit result 点击失败属于测试 discovery/locator 还是产品 pointer-blocking 缺陷；确定最小修复与回归范围。
- Scope: internal；只读原 R23 结果、第四场景证据/PNG、实际 consumed frontend 与 ROOT、已安装 Base UI 源码。不做源码编辑、测试、浏览器/API/DB/network/process/Git 操作。
- Date: 2026-11-17，Asia/Shanghai。
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`。

## Findings

### 最小已证实原因

**本次失败由测试无条件打开一个不包含 Submit result 的 modal 菜单，再尝试点击菜单外的现有工具栏按钮造成。** 原日志、实际失败截图和已消费产品源码一致支持此结论。没有证据要求修改产品 portal/modal/pointer-events。

不能直接把 locator 改为 `menu.getByRole("menuitem", {name:"Submit result"})`：这个实际页面的菜单没有该项。也不能通过 `force:true`、DOM 顺序 `.first()`、移除遮罩或延长期限绕过真实 actionability。

R23 原四场景结果保留为 **3 PASS / 1 FAIL / 0 NOT_RUN，全部 retry 0**。前三项依次实际通过角色/评审/QA、发布/回滚和 late-authenticated deployment，耗时 74.289 / 34.303 / 38.289 秒。第四项在 60.260 秒结束，具体是原 **20 秒 locator.click** 失败，而非耗尽原 480 秒 case 期限。证据中的通用 `failure:"scenario_timeout"` 标签不能升级为 480 秒整场 timeout。

原结果 SHA256 `440d404d437fa34b1e808e56ffefa98bca30ea2768cb8970db3466f24db7a21f`；reporter SHA256 `f3ef5d1d6061ab1557c1cadaea296b23e0567f86867da35e17f983f89850d406`。本研究没有改动该结果或重新执行任何场景。

### 原错误与可见界面

第四项实际 consumed `e2e/collaboration-stage-execution.spec.ts:468–470`：

```ts
const d = await j.open(browser, D, id, f.title);
await revealCollaborationDisclosure(d, "More actions");
await d.getByRole("button", { name: "Submit result", exact: true })
  .or(d.getByRole("menuitem", { name: "Submit result", exact: true }))
  .first().click();
```

点击日志明确显示 locator 解析到 `<button data-slot="button">Submit result</button>`，且每次均 visible/enabled/stable；随后一个 `<div role="presentation" data-base-ui-inert="">`，属于 `<div id="_r_1t_" data-base-ui-portal="">` subtree，拦截 pointer events。38 次最后阶段检查仍是相同拦截者。这是按钮存在/可见与实际能接收 pointer 的区别。

实际 `test-failed-5.png` 显示：

- 当前处理人是合成 ReleaseDeveloper，状态 Todo；S2 当前业务阶段 Verification。
- 工具栏依次是 **Start work、Submit result、Request information、More actions**。
- 展开的菜单仅有 **Add participant、Transfer、Edit issue、Edit source fields、Change iteration**；**没有 Submit result**。
- 没有打开 Submit result form。原 `:471` 后的填写与正例 POST 没有执行。

第四项证据中已记录的同 root `acceptance-submissions` POST 恰是三个 earlier off/observe/enforce 负例，均为409；本次点击没有产生 positive submission。它们不能替代后面未到达的 shared M1 completion 正例与其写入追踪。

### 已消费产品与菜单实现

| 文件/事实 | 确定行为 |
| --- | --- |
| `packages/core/collaboration/presentation.ts:36/:58/:61` | 当前非 creator 的 handler、有 submit capability、无 pending request/result、Todo 时 primary 为 `change_status`（Start work）；其余 available 按既定顺序返回 secondary，Submit result 与 Request information 居前。 |
| `packages/views/collaboration/workflow.tsx:78–80` | primary 独立；secondary 只取前两项；`more` 明确排除 primary/secondary。 |
| 同文件 `:119–126` | primary/secondary 渲染 Button；only-more 渲染 DropdownMenuItem。两个 surface 不是同一动作的副本。 |
| `e2e/fixtures/collaboration-disclosures.ts:7–14` | `More actions` 的 helper 在 aria-expanded 不是true时真实点击 trigger，再等待 role menu visible；它既不选择 Submit result，也不在返回后自动关菜单。 |
| `packages/ui/components/ui/dropdown-menu.tsx:30–70` | 复用真实 Base UI Menu.Portal/Positioner/Popup；未把 modal 关闭。 |
| 实际 `@base-ui/react` **1.3.0** `menu/store/MenuStore.js:17/:127` | root modal 默认true。 |
| Base UI `menu/positioner/MenuPositioner.js:256–270` | 打开的 root modal menu 创建 InternalBackdrop；cutout 只给该 dropdown trigger，而非邻近 toolbar action。 |
| Base UI `utils/InternalBackdrop.js:34–47` | 真实 `role=presentation/data-base-ui-inert` 的固定全屏 div，与错误日志特征一致。 |

Base UI popup 的 focus manager 对 context-menu 和普通 dropdown 的 focus-modal 配置有单独规则（`MenuPopup.js:118`），不应把它误读为没有 pointer backdrop；本次 pointer shield 来自 MenuPositioner 的 root modal 配置。

八条相关 ROOT/consumed 路径在本次读取时字节相同：execution spec、disclosure helper、control-gaps helper、workflow、collaboration-detail、dropdown-menu、英文 locale、Core presentation。两处 `packages/ui/node_modules/@base-ui/react` 链接也解析到同一个已安装1.3.0包。before/after consuming-source 记录均 `source_product_drift:0`，test-only deltas 仅原 R23 codec 两路径。这里是选定源/既有绑定核对，不是全仓或所有生产产品重新构建证明；其他 peer diff 没有被覆盖。

### 已有成功模式与为什么不直接照搬所有 helper

`e2e/fixtures/collaboration-control-gaps.ts:138–153` 的 `submitResultThroughUI` 先找可见 action，只有不存在可见 toolbar action 时才打开 More actions，然后等真实 status-options GET、填写并等 acceptance-submissions POST。`collaboration-multi-account-closure.ts:77–81` 的 invite/transfer，以及 `collaboration-business-scenarios.ts:174–177` 采用相同的 conditional discovery 思路。它们避免本场景无条件打开无关菜单。

这些 helper 的 role union/`.first()` 不是新的可靠性保证：当已经有别的菜单或 dialog 打开时，也必须按实际 active surface 操作。本轮无需重写所有 helper；只修本处已证明错误。现有 `workflow.test.tsx:88–108` 测试直接可见 Submit result button 打开 named form；`:438–448` 的菜单 Edit issue 测试真实选择 menuitem 后打开一个 dialog并恢复 trigger focus。它们属于 retained component tests，本研究没有运行，且 jsdom 不提供本次浏览器 pointer hit-testing 证明。

### 推荐修复归属与保持项

**最窄初始写域仅 `e2e/collaboration-stage-execution.spec.ts`。** 对本处已确定的工具栏动作，删除无条件 More actions discovery，直接点击 exact、唯一的 `button` Submit result；若该 import 在文件中不再使用，同时删除该 import。可在点击前显式 assert唯一/可见，但不能用 `.first()` 或 force 消解歧义。原 actionTimeout20秒与case480秒不变。

这一个调整恢复真实用户路径：在现有工具栏点击自己的 Submit result。它不更改角色、人工批准、stage/M1、表单字段、status-options、POST身份、结果选择、shared Web/Desktop completion或后面的实际写入断言。之后必须用新 actual run 完成原第四场景以及完整四场景最终 gate；不能把原FAIL改为PASS。

**如果确实需要可重用 helper**，才把写域扩大到既有 `e2e/fixtures/collaboration-disclosures.ts` 和 spec接线：先查当前 toolbar exact button；button存在时不打开无关菜单；只有实际在菜单的action才通过返回的 visible menu locator选择唯一menuitem。该函数必须区分已经打开的菜单，不复制 UI 的 permissions/action ordering，不重写全仓 callers。扩域需要主会话先核对后续用例与 owning same-layer regression；本次不要求新增通用 abstraction。

没有必要修改 `packages/views/collaboration/workflow.tsx`、UI dropdown/dialog、Base UI dependency、locale、服务端或数据库。若 main 的真实只读 repro 发现用正确的 active surface仍被异常 portal挡住，再按新的观察扩大产品分析，不能仅根据原超时先修改它们。

### 紧凑回归与主会话只读补证

原R23已是这个错误 discovery 的真实 browser RED。主会话可以在已绑定 production Web、同类合成 handler页面只做UI读操作：

1. 捕获当前工具栏 Submit result button 的唯一身份与 More actions 关闭状态；不开结果写请求。
2. 按原 helper开More actions，捕获 aria-expanded、visible menuitem labels、按钮位置和该点的 `elementFromPoint` 命中目标；应重现 backdrop遮挡与menuitem缺失。不要再次让20秒click执行写入，纯观察足够解释actionability。
3. 用真实 Escape/trigger关闭菜单，等待菜单正常卸载；点击原 exact工具栏button仅打开 Submit result form及status-options GET。断言named form/dialog出现、唯一、无接受提交或completion POST；随后正常 Close/Escape。

这补上实际DOM/命中信息与最小GREEN读路径；不是第四场景完成。若另外创建可重用 helper，owning E2E Playwright regression应复用同一个production Base UI菜单与shared workflow读夹具：**有Submit toolbar button和不含Submit的More菜单时，helper不打开菜单并正常打开named form；真正菜单action则scope到menuitem并正常打开form**。不要用手写div遮罩、mock Menu或仅Node/string断言替代真实语义；不要在component suite重新测试stage/M1矩阵。新增测试文件由main明确分配并保留原selector RED、新selector GREEN来源。

对当前一处spec修复，已有原实际RED、main只读UI补证和新完整原四场景是最小有意义回归；不需要增加与代码实现一一对应的纯selector测试。第四项后续 submit/authorize/completion/recovery/R38/project exits 仍保留全部原断言，完整四分母保持不变。

## Files Found / Evidence

- 原 R23 `runtime/four-scenarios-r01/{result.json,playwright-report.json,runner.stdout.log,runner.stderr.log}`：实际3/1及20秒click调用日志。
- 第四项 `test-results/*365ef*/stage-execution-evidence.json`：已到达步骤、49个HTTP记录、此前三个submission负例、cleanup和未到达的正例。
- 同目录 `test-failed-5.png`：直接核对工具栏与不包含Submit的打开菜单；`test-failed-1.png`查看当前Desktop stage部分，不能作为D点击命中capture。
- 实际 `frontend-base-r01` 的对应八文件，与ROOT比较字节；`current-consuming-source-before-four-r01.json` 和after记录保留原产品/input引用。
- `@base-ui/react`1.3.0上述MenuStore/Positioner/InternalBackdrop/Portal/Popup源码：已消费依赖的正常modal pointer规则。
- Companion JSON绑定准确路径、文件SHA、选择字段和未执行操作。

## Related Specs / External References

沿已加载 `.trellis/spec/server/collaboration-stage-approvals.md` 的当前authority/原结果身份、`.trellis/spec/web/frontend/e2e-run-environment.md` 的production产品/source绑定，以及Trellis工作流的研究/实施隔离。Base UI源码标注官方Menu文档 `https://base-ui.com/react/components/menu`；本研究未访问该网页，不声称查询了未安装的新版本。

## Caveats / Not Found

- 原spec和运行配置都明确 `trace:"off"`；未找到完整DOM snapshot、Playwright trace ZIP或pointer坐标capture。本次DOM依据是原失败日志内真实元素片段；菜单项依据是已看原PNG并与产品source交叉核对，不虚称执行了DOM inventory。
- 本研究没有做API/DB/process/stop当前探针；原after与资源恢复由main/独立owner处理。也没有执行旧selector或新selector测试。
- 正例submission和后面M1传输/完成/恢复尚未到达，不能仅凭工具栏form能打开关闭A09/A11/A12或S2。父24仍由main逐项更新，完整S3/S4/S5与实际业务签字没有变化。
- 选定consumed source与ROOT相同不证明整仓一致；peer差异不得被本次修复覆盖。原R21/R22锁与已接受R23codec不重新归因。
