# 真实验收交接索引

## 独立台账

| 系列 | 已授权上限 | 已用 | 剩余 | 状态 |
| --- | --- | --- | --- | --- |
| AC43 | 原用户授权不限次数 | 不与其他系列合并计账 | 沿用原授权 | 小队受阻；用量/账单对账排除 |
| Runtime RK1 | 25 | 12 | 13 | 真实准入关闭；下一H3a-t3 |
| 原 E0–E6 | 原独立账本 | 19 | 8 | E5有效失败后停止，不能自动续跑 |
| 知识 | 17 | 2 | 15 | E0b两次403；K1–K3未执行 |
| 研发真实小队 | 沿用原场景授权 | 见原证据 | 未在本任务新增 | leader超时结论保留 |

这些数字是2026-11-12阶段交接快照，不代替下一次运行前读取原账本。

## 来源

- [跨账号协作阶段](../archive/2026-11/10-10-cross-workspace-collaboration/stage-closure-2026-11-12.md)
- [父任务阶段](../archive/2026-11/10-30-collaboration-context-handoff/stage-closure-2026-11-12.md)
- [Runtime最新进度](../archive/2026-11/10-30-collaboration-context-handoff-runtime/progress.md)
- [原E5结果及限制](../archive/2026-11/10-30-collaboration-fix-closure/verification-2026-11-05.md)
- [知识最新台账](../archive/2026-11/10-30-collaboration-knowledge-promotion/task.json)
- [研发体系阶段](../archive/2026-11/11-05-agent-squad-rnd-improvements/stage-closure-2026-11-12.md)
- [机器可读路径与预算映射](source-map.json)

历史文件的内容哈希不能因目录移动被重写。目录移动影响路径解析，不提供任何重新执行许可。后续绑定审查应记录旧路径→归档路径、当前源码与框架、CLI、配置、数据库、窗口和准入的证明。旧launcher封存，新的归档感知入口须独立核验。

## 本轮复核补充：续作框架仍有未完成接线

Runtime H6双启动存在框架接线缺口：第二次预约发生在前一slot尚未Complete时，且逐run只有一个launch的列表不应使用整轮index取第二项。该缺口属于尚未运行的续作测试框架，不是核心Runtime产品回归；在H6真实执行前必须独立修复并用fake双启动回归验证。

本轮仅修复launcher的check-only路径，使其在条件满足时仍提前返回、绝不启动真实入口。旧冻结登记未改；脚本哈希改变与目录归档均要求重新绑定，不能沿用旧登记宣称可运行。
