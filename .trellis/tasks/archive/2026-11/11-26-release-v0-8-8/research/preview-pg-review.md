# PRIVATE PREVIEW PostgreSQL 17.11→17.12 补充独立审查

日期：2026-11-26。唯一补充证据根：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/preview-pg-compatibility/`。仍绑定预览源码 `5a48e91b385967309e53f0ceb211931eecdef873` 与外包 `675f8f97cd2983d0f83e9f4413c065d5e269329d39aaaf6fce94cd1387159b57`。

## 结论

**PASS：本补充实际证明password路径旧PG17.11数据卷被新PG17.12打开，并保留旧应用数据及身份，通过真实应用升级、备份空库恢复旧应用登录和backend重启。** 此结论补足上一预览审查的对应PG兼容缺口，不追改原预览matrix的较窄范围。

这是Docker Compose5.0.2、Docker Desktop Linux/amd64仿真、password单路径的私人预览证据。不能推广为所有Compose版本/自定义固定PG override、device完整PG切换或正式CI feed构件验收。正式两种认证矩阵仍需使用最终包并各自先恢复真实旧PG基线。

## 运行来源与原脚本行为

审查完整harness diff及run-pg-sequence，旧基线prepare现在先 `docker load`实际v0.6.7镜像包，保存loaded PG image记录，再创建新独占项目；启动后实际查询PG版本，baseline必须为170011。升级仍执行解包的同一个offline-upgrade.sh，没有手动postgres up/force-recreate/no-deps替换。

独立比较包内offline-upgrade.sh与5a48 Git对象逐字一致，SHA `2b618f1e372ca99bd2acc7aa9704a14481d0d6d629daae440cc9d95e8b3d317a`，原操作为 `up -d --pull never backend frontend`。原始password-upgrade.log清楚记录：备份PostgreSQL→postgres Recreate/Recreated→启动且Healthy→backend启动→等待healthz→升级完成。新镜像标签变化使该Compose实现自动重建依赖postgres，不是本次harness强制实现的结果。

因此上份报告“脚本未显式启动postgres”只是命令文本事实，不能推断实际依赖永不重建；本次日志证明当前Compose会重建。升级说明应核对实际运行digest/版本，不保证所有工具版本行为相同。

## 镜像、SQL与卷核对

| 项目 | 升级前 | 升级后 |
| --- | --- | --- |
| 实际SHOW server_version_num | 170011 | 170012 |
| 实际SHOW server_version | 17.11 (Debian 17.11-1.pgdg12+2) | 17.12 (Debian 17.12-1.pgdg12+1) |
| Manifest | cd4ccfdaf62cbfeb5c6ee19ef0dd663406d4ed740c320b324d016a90e85385b3 | 0a547a6067fdea4eec4cfb0887c4ddae2e91b1932cecc071541c14ee318312ce |
| 容器ID | dea07c272b38aa2175100fc37c7c8766b27b57daba116ac83241634e211e5c89 | 204d832f3a492679df34d52a7a779520470ebf868b712615c224153e032475a5 |

两个manifest都linux/amd64。独立打开旧/新内部镜像tar，匹配内容地址manifest→config摘要及PG_VERSION，分别config151aa1c2…与8bce63b3…；不是仅信标签名称或汇总。loaded旧PG记录与运行旧container manifest相同。前后database_identity都为独占multica_release_088_pgcompat_password。

前后卷Name/Source/Destination/RW完全相同：`multica-release-088-pgcompat-password_pgdata`挂在 `/var/lib/postgresql/data`；container和image确实变化，排除只是重新贴标签或连到不同空库。实际server_version查询在40条原始SQL观察中存在且顺序为170011→170012，query/result逐条SHA均匹配。

## 数据、恢复与重启

- 旧594条账本version+applied_at与新877逐条比较，全部旧时间戳保留、新增283；升级前database.sql COPY账本精确等于旧快照。
- 前后workspace/issue/iteration sentinel JSON一致、用户UUID/name一致，显式迭代关闭保留；745等待名单列按预期删除。备份恢复查询明确取回旧594集合、原issue与旧邮箱。
- 真实旧v0.6.7 backend先通过health/登录并建数据；升级后原token、password重新登录、workspace/issue/附件摘要通过harness实际HTTP调用。JWT/加密密钥/部署ID等只在内存与fixture比较，公开回执不含值；本审查独立比较原.env备份、override/模板与保留管理员凭据摘要，均一致，没有输出秘密。
- 恢复是CREATE DATABASE新空库→导入旧dump→实际v0.6.7 backend连接恢复库→旧会话/密码登录与issue验证→移除临时API及恢复库。恢复库此时运行在PG17.12，证明旧应用可读取恢复数据，不宣称PG数据目录物理降级或将旧dump灌入新schema。
- baseline/upgrade/restart三步回执均exit0且原log摘要匹配，顺序执行器遇非零会立即退出。升级日志及脚本调用未伪造为裸镜像验收。
- backend真实restart回执记录StartedAt/Pid变化；前后schema JSON除文件路径外完全一致，877条账本时间戳一致，无无效索引；DDL SHA仍为 `4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9`。原身份/admin登录及healthz重验。

## 清理与正式harness

40项evidence-manifest文件长度/摘要全部重算一致。独立只读查询pgcompat前缀容器、卷、网络为空；cleanup清单包括三种mode名称，但本次实际只运行password，不据此增加device/fresh测试次数。当前 `docker image inspect --platform linux/amd64 pgvector/pgvector:pg17` 为目标manifest0a547…，说明旧tag临时恢复后已回到新镜像。

只读检查主 `server-acceptance/acceptance.py`：每个非fresh prepare都重新load旧包，并有实际baseline PG170011断言；该要求已进入正式harness。正式跑每个旧password/device起点时都必须保留这些独立snapshot及最终PG观测。此review没有启动容器、迁移或修改脚本。

## 核心摘要

| 文件 | SHA-256 |
| --- | --- |
| `pg-compatibility-verification.json` | `f084abb727efc4edf7c5272c1d92d039be8b8a9eabd514a8c5e234c687c557a2` |
| `password-postgres-upgrade-observation.json` | `03aa4f50e2041078a967e63e1d4a2e2be677539c43ebb6470b92d06b1aef4471` |
| `evidence-manifest.json` | `9638d168fe3755d5895f7750bad8026703c03422dbb7919195e7af0fdcb21cbc` |
| `acceptance.py` | `5f743113fef38c518014123a4ba518d60276ec7bcef58ca5fb9e04234e419601` |
| `sql-observations.jsonl` | `108903ee4ae5b9d628bade2d134c841d2494b14ea77a6dec38a2b77f40e404c9` |
| `password-upgrade.log` | `06a024ccd51c57291dc4e19b88ac17db6a8660ad45f95e2ac32392868f2ba17e` |

未运行lint/typecheck/重型测试；本轮为只读证据审计，仅新增报告。原预览及历史失败未更改。
