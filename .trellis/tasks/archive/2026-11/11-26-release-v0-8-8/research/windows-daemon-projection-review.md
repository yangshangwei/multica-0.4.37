# Windows daemon 私有诊断最小投影审查

日期：2026-11-26。**最终PASS：新增投影两文件与此前home两文件可一并提交并运行新native，未发现剩余阻断问题。** 范围仅diagnostics.mjs与config.node-test增量；native home两文件沿用已通过审查，逐哈希确认未变。

## 安全结构与空值语义

投影只遍历已有failure对象的message/stack，不从raw stdout/stderr或任意附件扫描JSON。只接受一行开头精确marker `Installed daemon diagnostic: `后的单个JSON对象；failure16项、每文本128Ki字符、每文本128行、marker行64Ki字符限制。非对象/null/数组/解析失败/超长返回固定invalid；无合格marker返回unavailable，第一次合格marker后停止，不合并未知内容。

从零构造固定字段，而非redact或spread原对象：expected/actual state白名单、profile/server/auth/managed/installation/capture的严格boolean或null，以及固定config/owner/pid/log/err_log五项文件的exists/outside_home/read_failed严格boolean或null。缺失/类型错误返回null，不用Boolean(value)把未知变false；显式false保留false。原profile名、PID、路径、用户名、token、密码、config正文、文件内容、错误message/stack、任意key都不公开。

已直接核对嵌套实际daemonFailureDiagnostic字段。capture_failed/timed_out没有actual/profileFiles时仍可返回对应true，其余unknown/null，不假称daemon stopped或文件不存在。保留现有公开源码坐标与generic错误，不扩controller协议、timeline或workflow上传目录。

## 发现与修正请求

初始state集合漏掉实际诊断支持的installing_cli、cli_not_found、recovery_paused、auth_expired，却含实际投影前不会返回的error；会把这些最相关的失败状态变unknown。已要求实现者对齐现有8种实际状态并补每项保留断言；不扩为任意字符串。

## 独立验证

diagnostic config+workflow窄测试15PASS/0FAIL/0SKIP，含秘密注入/未知字段/伪marker/畸形/超限。额外无副作用调用真实投影函数：capture flag保留、0与字符串true输出null、第17个failure/第129行/128Ki文本之后marker均不可读。node --check与git diff --check通过。

本机未运行Windows安装或daemon，也不据此证明d746 metadata失败由home导致；这是下一轮定位能力。home两文件摘要与此前报告一致，子孙进程测试不重跑。


## 最终修正复核

实施者已将state集合精确对齐真实诊断的8种枚举，删除不在来源集合的error；测试逐项验证8种保留，同时error/未知文本/boolean/null均变unknown。独立再次运行窄测试16PASS/0FAIL/0SKIP，syntax与diff检查通过。原发现已解决，无需扩大公开字段或降低断言。

四文件最终摘要如下，home两项与此前审查相同：

| 文件 | SHA-256 |
| --- | --- |
| `apps/desktop/scripts/windows-collaboration-diagnostics.mjs` | `b992252e20de7823ab3bd184c96d60d926d61b9544f846280345a95ad3876b1a` |
| `apps/desktop/scripts/windows-collaboration-config.node-test.mjs` | `fb8941bcdf670a6ac44720bf9106f74e38fe449fbc1496349cdcc0ea0eebd354` |
| `e2e/fixtures/collaboration-installed-main.ts` | `bbcc4b97b73f7ff829b527853f85cdf943310248e9f89db8511d936cc1bea7ad` |
| `e2e/fixtures/collaboration-installed-main.node-test.mjs` | `7b394a05a6e5d3dfe0b7a6b5058a3bc6e3fb090e9258c047454ac0f481bd819d` |

本审查只写报告，无产品/测试修改；未运行全仓lint/typecheck或Windows原生验收。提交可行不等于协作或Release通过。
