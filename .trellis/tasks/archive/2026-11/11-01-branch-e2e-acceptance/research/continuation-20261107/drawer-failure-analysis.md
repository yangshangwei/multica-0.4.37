# 抽屉相关失败：只读诊断

**最终补充：15个测试文件＋32行验证规范已独立批准集成，0个补丁阻断项。原15个失败用例本轮复验通过、两安装项仍BLOCKED；canonical detail与前端新失败保持独立开放。最新哈希和集成核对见文末，前文保留各轮证据。**

**已确认账户记录测试没有关闭资料抽屉就操作背景；评论删除停在过时版本保护；Web 滚动根因仍未确定。知识恢复指针没有丢失，旧用例与新 R4 提醒入口合同不一致，不能认定产品隐藏恢复通道。**

目标 `4c817dee9397a42ac903f419af7804b11d2d863e`，首轮 `detail-journey-first` 为2 FAIL；regex-only 的 `legacy-test-repair-1` 为37 PASS / 7 FAIL / 9 SKIP。所有原始失败保留。本诊断读取原始 JSON、截图、ARIA、知识 trace 和源码，没有启动测试、访问服务/数据库或修改候选。精确路径、原始错误、源码/证据哈希见 [drawer-failure-analysis.json](drawer-failure-analysis.json)。

## 1. Copy link 被资料抽屉隔离：确定的测试交互失配

Desktop 用例在 `collaboration-account-desktop.spec.ts:426` 打开 Sub-issues，确认子任务后在428行立即调用 Copy link；函数252行等待背景按钮超时。失败截图明确保留打开的 Sub-issues modal，背景被遮罩。真实 Copy link 位于主详情 header，抽屉使用 modal Dialog，背景不能被同一点击路径访问是正常行为。

Web `collaboration-records-web.spec.ts:29` 同样在打开 Sub-issues 后点击背景 Copy link，原始结果对应180秒超时。后续 Notifications 和记录区域操作也必须尊重该边界。

可接受的最小测试修正：完成资源内容断言后，按 Escape 或实际 Close resources，等待**指定标题抽屉隐藏**，再点 Copy link / Notifications / 主记录区域。保留原真实剪贴板、绝对链接、分页、历史、授权与撤权断言。不得 force click、删除 DOM、弱化链接期望，或在通用 reveal helper 内偷偷关闭。

已静态审阅执行者的四文件 `drawer-readiness.patch`：仅增加上述关闭/隐藏检查、附件预览前重新进入，以及下节的评论内容就绪检查；没有删除原断言。仍须注意 Desktop 后继 sourcePopup 的 Sub-issues→showRecordedUsage，以及 viewer 的 Sub-issues→Notifications 两处同类转换。**静态可接受不等于完整链路已通过**，后续按独立新 run 取证。

## 2. Desktop 评论实际等待 DELETE，不是 PATCH

首轮 raw 报错在 `collaboration-detail-journey.ts:641` 等待 DELETE。此前 PATCH 的200解析、编辑表单关闭及 Edit comment 焦点检查已执行到下一步。截图与 ARIA 同时显示：

- “This comment changed. Your draft is preserved…”；
- “Use latest version”；
- **Confirm deletion disabled**。

该观察足以解释没有 DELETE：`comment-list.tsx:95` 的 stale 版本保护和 submit guard 阻止确认。不能把它写成服务器丢失 DELETE、PATCH 失败或纯键盘焦点问题。

较强但尚未完整动态证明的原因是测试提前开始下一操作：`comment-actions.ts:72` 触发异步查询失效而不等待 refetch；修改回执成功时 UI 立即关闭编辑表单；测试随即点 Delete，可捕获尚未刷新的旧 revision，等新评论到达后删除表单便成为 stale。原始用例 trace off，没有保留打开删除时的旧/新 revision 时序。

最小准备修正有源码依据：先等同一评论行**显示本次修改后的确切正文**，再打开 Delete；原 DELETE200、receipt、删除标记及焦点断言全部保留。这是确认前一操作已渲染的就绪条件，不能用自动点 Use latest version、强制提交 disabled 控件或 API 直接删除替代。该修正是否解释本次失败仍需原链路复验；目前不宣称产品缺陷或根因已关闭。

## 3. Web 620→476：保留失败，尚无足够根因证据

原始 fixture792 严格等于断言在5秒后看到476，预期是620。测试确实先验证可滚动范围>700、赋值620、等待两帧并保存实测值。失败截图只记录返回后的视图；完整 evidence 只有更早的一次宽屏截图几何，没有离开/返回时的高度、时间和滚动事件序列。

当前可证伪的候选原因：

1. **滚动捕获抑制窗口**：Web adapter 每次 `get` 将1000ms `suppressedUntil` 延后，窗口内的 scroll 不保存。若620发生在该窗口，旧476可能仍是 memento。需要时序证明，不能仅凭代码直接定性。
2. **恢复后的布局/浏览器 scroll anchoring**：详情在 query/comments 就绪时恢复一次，后续内容高度或上方区域变化可能使620变为476。若实测先到620再变化，同时区域坐标或scrollHeight改变，支持此项。
3. **scope/route memento 或 anchor intent 变化**：需对比前后完整路径/hash、账号/任务身份、焦点与恢复动作；错误重新消费 anchor 会有对应焦点/滚动行为。

下一步仅作外部复制 fixture 的观察：原620、原严格等于、原超时均保持；在赋值前/两帧后/离开前/两个scope提交后记录 performance时间、scrollTop、scrollHeight、clientHeight、关键区域坐标、URL/hash、activeElement 和被动 scroll 事件。原始失败不改写。现有证据**不支持任何降低期望、增加睡眠/重试或放宽滚动断言的修正**。

## 4. 知识原键恢复：修正“产品恢复丢失”的初步怀疑

原 `collaboration-knowledge.spec.ts:166` 在reload后直接找可访问的 Check original result，5秒找不到。独立读取 trace 的 `after@call@991` 后确认：

- Check original result **仍在 DOM**，祖先为 `role=dialog, data-closed, hidden`，知识容器为 `hidden, inert`；
- 原POST的 request ID `4d3fcdde-dddd-4dd3-864b-0ba25031d465` 仍显示在该隐藏内容中；不是指针丢失；
- 可见 error-context 明确出现 **Needs attention: → Knowledge candidates** 按钮，恢复入口被正常上报到工具栏；
- trace 的201 route.fetch、-1外层abort、403显式retry是两个UI提交的三条网络记录，不能误算三次业务创建。三条保留相同candidate/request身份。

新抽屉任务已批准的 PRD R4 要求提醒可见且能到恢复入口；design 明确“inline actionable toolbar reminder, never automatic focus stealing”。对应单元也断言 attention 不自动打开 modal。旧知识单元仍挂在 HTML details 下验证自动展开，那覆盖的是 standalone 分支；它不能说明新的 modal 也应自动展开。

因此当前结论是 **旧E2E自动展开假设与新R4交互合同失配，产品缺陷 NOT_CONFIRMED**。Root 已明确授权按新合同补验：断言可见 Needs attention 状态，再点其中准确的 Knowledge candidates CTA；这不是调用通用 disclosure helper 的手动兜底。点击后必须继续验证原 Check GET、相同request/candidate、reload后无Retry same request、恢复完成后pending消失、仅一个真实candidate。不能靠“看见CTA”就宣称恢复成功；完整CTA→恢复仍等待实际结果。

还需保留跨reload的被动 POST观察。原 `calls[]` 只在 route拦截仍注册时计数，unroute后不再观察；只报 calls.length=2不足以证明reload后没有额外POST。记录浏览器请求即可，无需改变产品、重发策略或期望。

## 复核边界

四类观察分别是：确定的modal交互失配、观察到的评论版本保护及可能的就绪竞态、未定因滚动失败、经新设计合同校正的知识恢复预期。不要把它们统一算成产品bug或统一抹成测试问题。

本切片只写本Markdown和JSON。原失败/截图/trace不动，未进行产品修改、服务操作、数据库请求或商业模型调用。后续测试修正与动态结果由原执行者在批次边界完成；最终验收状态仍依赖实际具名结果。

## 最终八文件复核与动态补证

**APPROVED_TEST_ONLY_PATCH_FOR_INTEGRATION，0 blockers。** 完整测试diff SHA-256：`2d0e8e00d4ec371bf903baedb880880adce31c1c4757e7460be34625b4e0a02d`。JSON的 `final_patch_addendum.files` 固定八个文件的候选原始和最终哈希，并嵌入完整已审阅diff。

| 文件 | 接受的修正 |
| --- | --- |
| `e2e/collaboration-account-desktop.spec.ts` | 三个 Sub-issues→背景动作显式关闭；弹出窗口读取usage后再打开抽屉点击子任务 |
| `e2e/collaboration-delivery-web.spec.ts` | 关闭Attachments写未发送评论，再重新进入真实预览和撤权链路 |
| `e2e/collaboration-records-web.spec.ts` | 关闭Sub-issues后复制链接及操作Notifications |
| `e2e/collaboration-knowledge.spec.ts` | 显式验证并使用R4的Needs attention CTA；新增被动POST观察和reload后零POST断言 |
| `e2e/collaboration-memory-web.spec.ts` | 在Submit/Accept/More actions前处理真实Shared context modal，读历史/purge前通过工具栏重新进入 |
| `e2e/fixtures/collaboration-detail-journey.ts` | 修改评论后等待确切新正文已渲染，再开始删除 |
| `e2e/fixtures/collaboration-disclosures.ts` | 已独立通过的数字匹配单反斜杠修正 |
| `e2e/fixtures/collaboration-knowledge.ts` | 原生作用域导航前关闭可见的知识来源drawer |

除了这些测试文件，实际工作树仅有已知Next生成导入变化；apps/packages/server/scripts无产品变化，也没有额外未跟踪产品/E2E源码。所有既有业务断言保留，仅helper的一个字符替换；未增加超时、重试或放宽scroll620。后继Desktop modal转换已经补齐。

实际结果已读raw并对账：

- `drawer-readiness-recheck-1`：5 PASS / 1 FAIL；账户记录、delivery、Web记录、知识恢复CTA和知识原生全部通过；memory进入新后继modal问题，原失败保留。
- `final-memory-knowledge-recheck`：2 PASS / 0 FAIL / 0 SKIP / 0 retry；memory完整到关闭后历史purge，知识恢复再验证通过。与上一轮去重后，六个原业务失败各有完整PASS，不能写成一个全绿原始运行。
- 独立解析最终知识trace：恢复GET200的candidate/request与最初POST完全一致；reload后候选/operation请求全部GET，**0 POST**。源码现也直接断言零POST；此前“pending丢失”怀疑撤销，原测试失败保留为合同适配前结果。
- helper的3个实际Chromium控制及56项精确源码表达式检查继续保留，不新增重复控制。

### Detail诊断的新证据

`detail-diagnostic-1`：Web完整1 PASS，Desktop1 FAIL，0retry。两个端均通过编辑/删除步骤，Desktop在原严格滚动断言处仍是620→476；这证明就绪修正没有把后续失败隐藏。

独立核对12个几何样本及被动scroll事件：Desktop返回后实际曾到620，然后660、620，最后在height3265→3121时落到476；记录中的最小可滚动范围2333仍远大于620。因而**不是简单maxScroll夹住，也不能只用未捕获620解释此次诊断**。前后description的“屏幕坐标＋scrollTop”均513，最终位置实际差144；中间内容/几何变化已观察到，但具体导致变化的元素及归属尚未隔离。没有降低断言或产品修复依据。Web这一次经过660暂态后回620，仅该独立诊断通过，不改写首轮Web失败。

补丁可集成与产品全验收是不同结论：diagnostics路由门仍失败，Desktop滚动门仍失败，其它平台/Provider/待运行门也不因本补丁通过而关闭。Reviewer只做读取、哈希/差异和trace解析，无新浏览器/数据库/模型调用或源码编辑。

## 最新集成范围：14个测试文件

**APPROVED_TEST_ONLY_PATCH_FOR_INTEGRATION，0阻断项。** Root另行授权了Mention折叠入口及Leave后继交互适配。最新diff SHA-256：`0e3b05048a4db2f552a504cff6ee5a5fa26356e5073ffe1edf84e6ce849a2001`，精确14文件before/after哈希和完整diff见JSON `final_patch_addendum_14`；此前八文件批准只描述较早版本。

新增control-gaps、mention-journeys、retained-attachments、resource-offers、workbench及Leave测试路径。Mention helper仅在折叠时点击真实按钮，再断言expanded与目标group可见；目标UUID、preview、身份/权限、通知、回执和附件关联不变，不应用于pending恢复。Retained附件在进入评论历史前关闭抽屉，在最后撤权前再次通过既有入口打开真实预览；字节比对、不可变AI输入、刷新/重启和撤权断言保持。

已修正三个可见性前置问题：records的全局Edit source fields count0在drawer隐藏后才执行；memory先明确panel可见再判定不可Edit/Restore；Leave为creator和handler展开已有More actions并确认menu可见后再断言无Leave。旧creator trace确实只检查了主Transfer按钮，现不再以折叠菜单制造零计数。原actual409及状态不变验证全部保留；离开后People/AI负断言也先打开Mention并证明People可见。

独立校验14个before哈希等于冻结候选、after等于执行树、完整diff等于保存patch。除Next已知生成导入，没有产品变化。删除行仅是import、regex及两个Transfer按钮定位被同名button/menuitem联合定位替代；没有删业务断言、降低scroll620或改变超时/重试。最终Leave、retained及九个deployment用例结果由正在完成的本轮映射核销，本批准不提前关闭产品验收。

### 最终范围与文档核对

最终14文件after哈希和完整patch已再次核对一致；Leave及跨客户端retained附件原用例分别在`password-leave-final-proof`、`password-retained-final-proof`完整PASS，0skip/0retry。成功时retain-on-failure不保存逐调用trace，因此此处用原先观察到的菜单差异、最终明确可见性前置源码和完整用例PASS作结论，不虚构成功trace。

Root新增`.trellis/spec/web/frontend/e2e-run-environment.md`的32行准则独立通过：modal背景动作、负断言可见前置、attention原键GET/无新POST、评论渲染后删除和保留scroll期望均与实证一致。没有删除原规范，没有API/schema/infra变更。最终批准范围是14测试文件＋该文档增量；本轮原17状态表与整体产品失败另列。

## 最终15文件批准

最新批准对象：15个测试文件，完整diff SHA-256 `a3e173f02860547f7368b7ec9c7c71e77f66f62a19d694a6644db129ee2d5f39`，以及已审阅的32行E2E环境规范。JSON `final_patch_addendum_15`记录每个before/after哈希、完整patch和root集成核对。其余旧scope批准保留证据历史。

最后两项增量已经实际复验：Gap7复用Mention显式打开后通过，原角色拒绝/不排队/状态不变断言不动；workbench最终1PASS，处理真实Activity modal再操作Participating，保留同名Transfer角色差异、旧handler的includeHidden负断言并先展开菜单。首次两项复验仍是1PASS/1FAIL，不改写；最终workbench通过另记。

最终原17表已按当前raw核验15PASS＋2BLOCKED，没有复用历史PASS。整体canonical detail两个失败仍保留，后续Web诊断PASS不替换它；前端新失败也未被此次测试维护修复。批准的是测试适配集成，不是整分支验收或发布。
