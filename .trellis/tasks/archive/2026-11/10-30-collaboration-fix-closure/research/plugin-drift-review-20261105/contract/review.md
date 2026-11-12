# 插件漂移的契约与最小后续改进

**原 E0a 行为通过与辅助器证据检查失败可以同时成立。** 现有66项独立取证支持 `finished/pass`、launcher exit0、1次真实调用；辅助器 exit1 和配置稳定性缺口也必须保留。当前不能凭文件名或更新时间将漂移认定为无害缓存，不能把新快照当成过去的基线。本次只读分析，不实施、不启动模型或服务。

## 检查分别约束什么

| 检查 | 实际契约 | 当前可得结论 |
| --- | --- | --- |
| 原 launcher 启动前 | `operator_facts()`必须与登记相等，范围含settings/CLAUDE、hooks/skills/plugins/agents/commands、managed目录和选定环境/配置文件指纹 | 当次启动前扫描所得总体指纹相等；不是所有文件在一个原子时刻的快照，也不证明实际加载了哪些插件 |
| 原 launcher 运行后 | 依据实际trial、调用次数、incomplete/blocking/进程exit检查原停止规则；没有再次扫描operator tree | 此E0a原判分通过、原stop未触发，1/25已消耗，不能重跑或退还 |
| 额外helper运行后 | 再次调用`binding()`，operator不等即抛ValueError；后续CLI/计数汇总字段因异常跳过，`review_ready=false` | 严格配置绑定证据检查未通过；它不是E0a行为判据，但仍是本次约定的整体验收门，不能直接忽略 |

源码定位：[原树哈希:76](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py:76)、[operator范围:211](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py:211)、[启动检查:312](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py:312)、[原stop核销:364](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py:364)、[helper相等检查:113](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/real-entry/e0a_entry.py:113)、[helper运行后:249](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/real-entry/e0a_entry.py:249)。

旧61,947行清单标注`b994…`，但缺少原算法需要的kind、链接目标和不可读条目；按全部普通文件重算仅得到`6064f16d…4d2a52`，无法复现其标注hash。实际启动登记是`617d…`，运行后是`91de…`。不相同的旧清单不能用来确定启动时每文件内容；之后新增的逐文件快照也不能回填该缺失。mtime无法证明写入者或“只有这三项变更”。当前安全处置是保留已验证行为/生命周期及调用核销，同时将配置稳定性作为未解决限制；不把整次运行改成绿色。[既有独立E0a复核](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261105/real-entry/e0a-independent-review.md)

## 可独立实施的最小改进

在下一版辅助取证中，为原树哈希保存**完全同口径的逐路径清单**：相对路径、种类、内容SHA或链接目标；保留不可读条目、原排序及编码语义，不按缓存/动态文件名排除任何路径。清单私有保存，其重算聚合值必须与真正启动前检查的登记指纹相等，再把清单hash、时间、helper/登记/环境hash绑定到执行记录。mtime等元数据只辅助定位，不替代内容哈希；扫描期间变动须拒绝，仍不宣称建立了原子文件系统快照。

改动应集中在树记录收集及helper `binding()`的“收集证据→验证相等”边界。若原launcher必须冻结，可在版本化helper的伴随采集函数保留相同序列化，并以原聚合值交叉核对；避免改整套实验入口。运行后先持久化新清单和added/deleted/type/link/content/unreadable差异，再执行原严格相等检查；即使检查失败，也保存原trial/CLI/计数取证，整体状态仍失败。新取证不覆写旧记录，不静默修改旧登记。

最小验证使用临时、无凭据目录：原口径聚合一致；内容/增删/类型/链接变动被精确识别且拒绝；采集到检查之间的变化被发现；运行后不一致仍完整落盘且不允许重试；文件权限和独占创建正确。保持真实operator HOME/settings不变，无需新增模型调用来验证这些取证机制。

**新增清单只能解释未来漂移，不能消除实际漂移、证明这次漂移无害，或自动放行H1。** 如果下一轮严格检查仍发现变化，它仍须失败并保留差异；是否需要调整实验可比性/接受边界是后续明确决定，不能由采集代码暗中做缓存豁免。

## 继续权限与边界

runtime已有25次预算授权，现1次已用、24次剩余；不需要仅因调查或完善取证重新索取预算授权。当前用户已授权的调查可继续形成上述可审查、可独立实现的最小改进方案；本次报告本身既未实施它，也不等于当前H1门已打开。由runtime唯一执行者对既有证据缺口作明确处置，再取得新的执行窗口和精确绑定；本报告没有代替该处置，也不建议重复E0a。原系列余8次及其E5停止规则独立保留。

补充核对：上述旧清单结论以同目录[清单复核](../inventory/retained-plugin-inventory.md)为准。runtime owner随后已书面处置E0a行为/生命周期接受与配置稳定性未知的区别，并在独立工作区推进前瞻helper；H1仍须通过其后继取证控制、独立审阅、实际绑定与共享窗口条件，本报告不打开许可。
