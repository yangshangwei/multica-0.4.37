# V0.8.8 source-preflight-03 独立证据审查

日期：2026-11-26。目标SHA：`7a99271341910b58c35a99d5f38ffe1ae75c6959`。

证据根目录：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/source-preflight-03/`。实际脚本为此目录内的 `source-preflight.py`，不是其父目录同名文件。

## 结论与边界

**接受该次源码级迁移预验在已记录配置下通过。** 本审查读取脚本、重解析全部15份迁移原始日志、重算文件摘要、比较完整集合与DDL，并检查执行/失败历史，没有只采用 `result.json` 的布尔值。未发现需要修改产品代码的缺陷。

该结果不是正式Linux离线包验收。实际执行器是Darwin/arm64 Go1.27迁移二进制，数据库为Docker Desktop上的Linux amd64 PostgreSQL17.11；未运行正式API、Web、容器entrypoint或认证流程。“second_process_idempotent”证明重新启动迁移器进程，不证明backend容器重启。故障非零证明migrator退出，不证明容器入口已阻止API启动。后两项及healthz/登录、真实旧包password/device升级仍必须由正式包验收完成。

## 来源绑定

- 二进制实际SHA-256：`f9d279eab7bdeac7f68a752a6c023aa9d1deed8cd9703662e4b0436e37f4544a`；独立执行 `go version -m` 与binding记录逐字一致。
- 实际buildinfo为 `vcs.revision=53df2dbb60fcb29acebb507f4a6041802bfb27dc`、`vcs.modified=true`、GOOS=darwin、GOARCH=arm64、CGO_ENABLED=1。不能称它是7a992713的clean正式构件。
- 独立 `git diff 53df2db... 7a992713... -- server` 无差异；两提交仅差桌面/Release workflow及其长路径契约测试。4996条server Go/SQL/go.mod/go.sum清单的路径集合等于目标提交tracked集合，逐文件哈希与当前源码一致，聚合SHA `6f5e3ccfa3d97850096190a46a32c7236abc4fb45478fb80b42738b15a885c96` 与binding/result相同。01尝试保存的源码清单也完全相同。
- 初次编译的完整命令执行回执未保存在03的commands数组；build.log为空且其空文件摘要匹配，构建命令是binding声明。因此这里接受已披露构建戳及不变server源码的预验关系，不把它提升为clean发布构件来源证明。
- final-migrations的1754个文件名/哈希与目标源码完全一致；四组baseline目录逐文件匹配不可变tag清单。组内合并依据完整SQL文件哈希相等，不依据最大编号。v0.6.7目录还与已交付包SQL清单一致。
- 保存的数据库manifest明确为linux/amd64，digest `sha256:cd4ccfdaf62cbfeb5c6ee19ef0dd663406d4ed740c320b324d016a90e85385b3`；记录的PostgreSQL版本为17.11 x86_64。不是从宿主ARM默认image inspect推断架构。

## 执行顺序与完整集合

脚本先核验HEAD/干净server差异、保存清单/复制SQL、逐条校验历史SQL、确认端口和资源未占用，再创建独占带owner标签的数据库容器。TCP pg_isready成功且 `SELECT current_database()` 匹配后，先建fresh，再逐组old→upgrade→repeat，之后故障及恢复。97条通用命令回执退出码均0；迁移进程另存15个专用JSON/日志，故障进程按预期为1。没有把 `commands.json` 全0误说成所有子进程都0。

| 实际SQL起点组 | old up | upgrade up | upgrade已有skip | repeat up/skip |
| --- | ---: | ---: | ---: | --- |
| v0.6.5 / v0.6.6 / v0.6.7 | 594 | 283 | 594 | 0 / 877 |
| v0.5.5 | 535 | 342 | 535 | 0 / 877 |
| v0.5.6 | 559 | 318 | 559 | 0 / 877 |
| v0.6.0 / v0.6.1 | 590 | 287 | 590 | 0 / 877 |

fresh为877 up/0已有skip。所有applied/skip数组从原始日志重新解析，逐项匹配其JSON、目标差集和基线集合，均无重复/遗漏。所有15个log SHA匹配；不是仅核对计数或889最大前缀。

这些旧schema由最终迁移器执行不可变历史SQL生成，不是逐个历史版迁移二进制/旧安装包运行；四组结果不能冒充7个真实旧部署的完整业务升级。

## Schema、账本与条件对象

- fresh、四个升级schema、四个repeat-schema共9份规范化DDL均为436032字节，SHA-256均为 `4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9`。
- 脚本只过滤pg_dump注释行和随机restrict/unrestrict标记，其余DDL严格比较，没有忽略某类表/列/约束来求通过。
- 每个升级及repeat快照包含完整877条 version+applied_at，集合精确等于目标列表；每组repeat与升级快照账本逐字相同，schema/index列表也相同。
- 各快照924个索引包含243个pg_toast内部索引；跨数据库TOAST名字包含对象OID，不能要求它们逐字相同。681个非TOAST索引定义在fresh及所有升级库完全相同；各库内repeat前后全部924项相同。invalid/unready/not-live集合均为空。DDL摘要已独立包含全部用户schema。
- 371在fresh及各baseline执行中明确记录“preferred index public.idx_comment_content_bigm is unavailable or unusable”，但仍入账；没有把该up记录当成DROP SQL真正执行。快照证实bigm索引不存在，`public.idx_comment_content_trgm`仍为 `USING gin (lower(content) gin_trgm_ops)`，符合371源码条件与fallback契约。
- 日志另保留SQL内部对pg_bigm/pg_cron缺失的NOTICE；`condition_skips`数组只代表runner显式跳过，不是所有DO块分支的完整数量。实际扩展仅pg_trgm1.6、pgcrypto1.3、plpgsql1.0。此结果覆盖fallback配置，不覆盖安装pg_bigm/pg_cron后的对象分支。

## 数据保留、备份与失败门槛

- v0.6.7组在旧schema插入唯一用户/工作空间/显式关闭迭代设置和等候名单哨兵；升级前pg_dump中确有这些数据及完整594条旧账本。
- 独立解析备份的schema_migrations COPY数据，594条旧applied_at与升级后快照完全一致。因此该组的旧时间戳保留可直接复核，不只依赖True摘要。
- 脚本在升级/重复迁移之后断言用户名称、迭代false、等候名单列确实删除；随后新建空 `release088_restore`，用psql ON_ERROR_STOP导入旧dump，核对旧594集合及原等候邮箱。执行回执及通过脚本表明恢复动作发生在空库，不是对新schema降级灌dump。
- 故障副本仅新增 `99999_release_preflight_failure.up.sql` 的明确RAISE EXCEPTION；原日志含SQLSTATE P0001，退出码1，up0/skip877。脚本在故障后比较完整账本与fresh原快照，然后切回原目录再启迁移器，退出0/up0/skip877。没有删生产迁移或改源码求通过。
- 证据粒度限制：其他三组升级前的完整applied_at、升级后哨兵查询输出、restore查询输出以及故障后的账本未单独保存为原始快照；对应断言在已保存脚本中且成功流程到达result写入，但不能称独立审查重新查询过这些已删除数据库。建议正式包验收分别保存这些前后快照及非秘密查询结果，避免只剩内存断言。当前结论保留此限制，不要求重跑本次源码预验。

## 失败尝试与清理

- 01：failure.json/run.log/last-command.stderr保留，11条通用命令的末条SQL退出2，未执行迁移、无result.json。readiness当时探测Unix socket，临时初始化实例退出窗口与后续SQL发生竞争。03改为TCP readiness；不把01改写为成功。
- 02：failure.json/run.log保留，只有HEAD检查回执；旧期望53df2db与已更新HEAD不符，断言拒绝启动。未创建新数据库/容器、未迁移，无result.json。
- 03：97条通用命令、15份迁移日志、进度/结果和cleanup.json均保留；cleanup记录仅移除owner匹配的03容器及卷。独立只读 `docker ps -aq --filter name=^/multica-release-088-source-preflight-` 与对应volume查询均为空，不重启数据库或触碰其他资源。

## 核心审计文件摘要

| 文件（相对03目录） | SHA-256 |
| --- | --- |
| source-preflight.py | c110041342dfb55dd2f23726c10b99d0b6ecc3b479db2c15e7618968704737b0 |
| result.json | 8f1326538b9694f5b0c449da80543b0c8670f654fd5285584bb391865b6fce14 |
| binding.json | 12b1c7b677105bbd21ee9d6eb55ff554d459e7262cd13079ad80cb78c4141151 |
| commands.json | 73396f7ef3ab57f649dd408a7ae5449c617fd422b02dcc0df2837eb089f85bb8 |
| baseline-seeded-backup.sql | 191d620e9f5457c7154797f8f605aa00427768b363a8518507cd7a0f52617790 |
| failure-gate.log | 9a2559eee4d086565bc6cabada5440ec03e23f002657a360f791098593c44728 |
| failure-recovery.log | 7befb82cef0fa67f5165f1eb8e99465e80bc9541bf09ce43f92f7d82482b3303 |

## 审查操作

只读文件/日志、Git对象差异、哈希重算、数组/DDL比较、go buildinfo读取及资源列表检查；没有重跑迁移或重型测试、没有改产品或原始证据。仅新增本报告。Lint/typecheck不适用于本次证据审计，未运行也未声称通过。
