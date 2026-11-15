# 本机 Codex 开发工具范围与现有任务交接

用户澄清后已改为直接调用本机 Codex CLI 做开发，不再要求开发工具经过 Multica SDK。第4次调用已实际退出0并生成非空交接记录；保留前两次超时和一次空输出，不把exit0/空输出当作有效结果。当前调用使用配置中的gpt-6.1-sol，单次reasoning_effort=low；全局地址/模型/凭据未修改。该结果只证明这一开发CLI调用可用，不提升产品原生集成验收状态。

HEAD：`45ec006d96445b41d02c5d26acd5b8c574ea0289`。工作区`git status --short`为417条（包含大量既有改动/删除/未跟踪资料），全部保留。父任务及三个既有子任务均in_progress；未建新任务、未归档。

| 产品验收线 | 已用/总额 | 剩余 | 原处置与阻塞 |
| --- | --- | --- | --- |
| Runtime | 12/25 | 13 | H3a-t2 incomplete/exit143保留；历史停止例外和真实续作授权保留，当前源码/Claude/配置/合法原生网关/独占窗口未重绑；H3a-t3→H3b→H5→H6未启动 |
| E5 | 19/27 | 8 | trial1仍failed-with-valid-evidence；trial2/3与E6未运行，不自动重跑 |
| Knowledge | 2/17 | 15 | 两次E0b 403保留，K1/K2/K3未运行；产品登记仍固定旧CLI/配置，不能用开发CLI通路替代E0b；合法恢复后E0b→K1→K2→K3 |

真实runnable、allow_build、allow_check、allow_run均false。开发CLI进程4次单独记录，产品验收新启动0次。

本轮在范围纠正前完成8,312文件源码快照、配置候选、0.167原生版本/字节、签名元数据、schema静态比较、启动配置元数据校验、两份Go构建及四个新专库迁移（781条ledger，最新793_delivery_vcs_trust，初始issue/task/run均0）。没有迁移或清理旧库。已有API/Desktop/daemon属于其他会话，未停止，未宣称机器级独占窗口。

多余SDK迁移三文件候选只在`/Volumes/artisan/code/2026/mc-knowledge-rebind-20261112`；未合入主工作区，动态普通/race控制未运行，独立SDK审查中止为incomplete。主工作区修改仅限现有父/知识task.json、知识progress和本目录证据；没有改产品源码或依赖。保留隔离副本、新数据库和全部原失败资料。

命令与结果：Trellis context/phase/packages、Git状态/空白检查、原始SHA复算；codex --version/help/login status；doctor网络诊断exit0但HTTP401仅可达、不作准入；原生generate-json-schema与print-startup-config仅元数据；go build迁移/CLI exit0；四库CREATE/迁移up exit0；直接codex exec的四次完整命令、配置hash、起止、退出与输出hash见各direct-coding-cli JSON。SDK worker的tagged go test -c只是编译，不执行TestMain或测试体。没有产品真实Provider执行。

新增/复算哈希入口：[scope-resolution.json](scope-resolution.json)、[preparation-quality-check.json](preparation-quality-check.json)、[source-snapshot.json](source-snapshot.json)、[database-allocation.json](database-allocation.json)、[native-installation-binding.json](native-installation-binding.json)。H3a-t2 public result `9a4b9405…628ce`、历史invocation `909ac2da…ddd3ed`、approval `7d743b5e…4f53c`、真实授权 `1f56add2…8f580`均重新匹配，历史字节未改。

未完成：现有三条产品真实验收及其当前绑定/合法原生网关/独占窗口。当前工具用法已澄清，开发CLI使用无需新的业务决定；没有把用户允许CLI开发扩大为产品评分/预算/停止规则变更。
