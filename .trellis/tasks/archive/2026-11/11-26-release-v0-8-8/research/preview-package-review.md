# V0.8.8 PRIVATE PREVIEW 完整包独立审计

日期：2026-11-26。唯一证据根：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/preview-acceptance/`。源码 `5a48e91b385967309e53f0ceb211931eecdef873`，预览外包SHA-256 `675f8f97cd2983d0f83e9f4413c065d5e269329d39aaaf6fce94cd1387159b57`。

## 结论

**接受8项回执在该私人预览配置下的通过结果，附下述数据库镜像范围限制。** 本审查读取实际harness、归档、镜像元数据、SQL原始查询与结果、恢复/重启/故障日志，逐项重算而非只采用summary。没有重跑或修改原始证据。

feed含1条Unreleased记录，明确不是正式CI feed。此包不是正式交付或发布批准，正式feed、最终SHA与构件仍需独立验收。Linux运行环境为macOS ARM64的Docker Desktop amd64仿真；不等于客户物理Linux/内网、Windows安装、Namespace或商业模型验证。

## 来源与镜像

- 外包实际计算SHA匹配。23个归档文件与本验收target解包文件逐个字节摘要一致；路径均在唯一预期根，无穿越/链接/设备/AppleDouble/真实.env/管理员凭据文件。
- README.md与操作文档.md均与5a48 Git对象中的源手册逐字一致。MANIFEST记录完整SHA、版本、amd64、三镜像及内部镜像包摘要。
- 内部docker-save包含且仅包含backend/web/pgvector三镜像；实际config均linux/amd64。独立检查saved manifest的内容地址、config摘要、rootfs.diff_ids，与image-verification记录及password/device运行manifest一致。不能把宿主架构当作镜像证明。
- backend manifest `sha256:cfe530c7273f723e1e8a804d8d016f6d155e822803cb16e5e8692f73cbecbc2c`；Web `sha256:ecf217acbb9c327c8345d97624e2c7b9dd9cb95c8b00b9fccd2c82fccca5dee6`；数据库 `sha256:0a547a6067fdea4eec4cfb0887c4ddae2e91b1932cecc071541c14ee318312ce`。
- 1754个从实际backend容器复制的SQL文件逐一匹配input-binding/image-verification以及5a48 Git对象，包含877up/877down。完整目标version集合唯一，旧594集合与新增283差集匹配。

### 数据库镜像范围限制（需带入正式验收设计）

harness先load旧包再load新包，两包共用可变标签 `pgvector/pgvector:pg17`。新包覆盖该标签后才建立旧应用基线，因此password/device的旧backend/Web是在**新PG17.12**上建立594条schema。原v0.6.7包数据库config为 `151aa1c2e849e7828ed8aef2183ae15f692cbe887d2f28921d8b44b26e154d7e`、PG17.11；新包config为 `8bce63b3aa8181e9c040468016e7df9c82fb9400c40df9e731752bed2965d8ef`、PG17.12。私有基线mount记录已证实基线数据库manifest就是新包0a547…。

所以本次证明实际旧应用镜像和旧schema升级、数据库卷保留，**没有证明旧PG17.11数据卷由PG17.12二进制打开的切换**。包中 `offline-upgrade.sh:237` 显式up backend/frontend，未给postgres版本固定/重建独立流程；不能把随包有新DB镜像直接解释为现有数据库已升级。正式验收应显式绑定旧PG digest建立基线，并核实实际脚本保留或重建postgres的行为，记录最终运行digest；如要求DB二进制升级，另在隔离副本演练并说明步骤。此限制不撤销当前schema/应用验收，也不能被summary的卷保留True覆盖。

## 原始数据与升级顺序

- 97项evidence-manifest文件的长度/摘要全部匹配，8项passed回执均存在；104条sql-observations逐条query/result摘要与总文件摘要一致，采集时间有序。
- harness调用解包的原offline-upgrade.sh，带同一deployment、指定新备份目录和两层ordered override。password/device原命令、日志和exit文件均为0。没有替换成裸image load加手工宣告成功。
- 旧应用先通过health/healthz并核对v0.6.7源码，真实API注册password或device账号、建workspace/issue、上传附件；升级后真实API验证旧token、重新登录返回原用户、原workspace/issue和附件完整摘要。Web/login实际HTTP200，healthz实际DB+migrations都ok；这些HTTP结果由harness断言及回执记录，未保存全量原始HTTP trace，避免把私有token/密码输出为公共证据。
- 独立解析两模式前后sentinel快照：用户UUID/name保留，workspace/issue/iteration完整JSON相同，显式enabled=false/revision1保留，745移除的两列确实消失。
- 两模式baseline594条version+applied_at与upgraded877条对比，594旧时间戳全部保留，新增正好283。升级前实际database.sql中的594条COPY账本也逐项等于baseline快照。
- 私有值只在内存比较、不输出：原.env备份与fixture精确相同；override和模板摘要、前后mount路径/卷名/RW相同（仅规范化Docker Desktop的/host_mnt前缀）；管理员初始文件摘要与升级前相同。运行容器13项env与fixture相等、DATABASE_URL与resolved Compose相等由harness读取真实docker inspect验证，公开回执只含字段名/布尔结果。

## 恢复与管理员

恢复流程是CREATE DATABASE新空库→导入升级脚本生成的旧dump→读取旧594账本、原issue、旧等候邮箱→在该库上启动实际v0.6.7 backend并核对旧commit/healthz、旧token和原password/device重新登录→移除临时API→drop恢复库。两模式各3项独立记录的恢复SQL及结果已复核，restore日志未被当作schema降级；未将旧镜像连到新877schema冒称回滚。

fresh的signup关闭仍通过同包初始化工具建立独立管理员，重复初始化保持文件字节、0600/单链接，首次改密被要求、旧密码及相同密码变更拒绝、改密后admin role/overview/users/installations通过。password旧管理员保留，device配置不启用后台、不生成管理员文件或初始化记录。这些断言均在实际API调用后才写passed回执。

## Schema与真实容器重启

三模式各before-restart/after-restart/final共9份DDL均436032字节，SHA `4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9`；独立重算相同。仅去pg_dump注释和随机restrict标记，没有删除DDL类别求相等。三模式完整877账本在重启前后包括时间戳逐字不变，无invalid/unready/not-live索引；有效comment搜索为pg_trgm fallback。

3份container-restart回执明确记录真实 `docker compose ... restart backend`，同container ID的StartedAt/Pid发生变化，随后healthz、schema和原身份/管理员登录再次验证。比源码预验的“重复migrator进程”更强，此次可称真实backend容器重启幂等。

## 失败门槛、保留与清理

fault-attempt-03先建空故障库，确认schema_migrations不存在，再给真实v0.8.8镜像只读挂载迁移副本。副本仅追加故障SQL，docker wait返回1；container_state为exited/ExitCode1，原日志有明确注入异常且无Starting server标记。执行过的合法877条完整记账，故障99999不入账。

仅移除副本的故障文件，重建同镜像后healthz成功，完整877条时间戳与失败后一致。随后停止容器，在一次性故障库删除最早001_init账本，绕过migrate直接启动同镜像/app/server；实际/health可达，/healthz返回503、db=ok/migrations=out_of_date，独立快照只有876条且唯独缺001。证明健康门槛检查完整成员集合，不是只看最大889。

01原尝试已通过故障非零/无API及恢复，但在直接server启动轮询遇RemoteDisconnected，harness未捕获；其trace/exit1/账本与日志仍保留。02是fault_root局部变量初始化笔误，在DB/container启动前失败，trace保留。03只修harness startup polling/路径，未改应用、迁移或镜像；首次失败仍为失败，不计作产品失败，也不追认整个01已通过。

cleanup记录与三份原始Compose移除日志保留。独立查询preview前缀容器/卷/网络均为空；临时故障/恢复容器移除检查owner，主项目按精确Compose label核对后移除。未重启或删除其他资源。

## 核心摘要

| 文件 | SHA-256 |
| --- | --- |
| preview-verification.json | 3db7fe9f700a2caf61920565c38e7b9af2e06bc6fe2f76f3fbb1891a3bf036ef |
| evidence-manifest.json | 3d4a1d9b0d1e4793067780dc82789dab5906763a8157fb845985c35d0c3072b4 |
| sql-observations.jsonl | 6526610db2114e0b337d1d773f12cfe53f0a099195bbe9978d9f8314a281a651 |
| acceptance.py | caae6de096b61c1588d0f03f5ccbe21e5de5d0602d0389967a0a87d2d3f46cb1 |
| input-binding.json | 315c29a9eb5b99c8571db4a7e5c544ae6b6da642971b17ad3d431a0501066dde |
| image-verification.json | 59390206ec9967ce901a343c27835560f27086b33367bb5d1592d6b87d439233 |
| fault-attempt-03/fault-entrypoint.log | 6c0f7a3f3ef8b44786ce1ec8848d7fcf27ae493133001274e8fdba6f5a2caf8e |
| fault-attempt-03/fault-missing-migration-health.json | fd97c1589a0a16d6b36123e15849a6b55153ef0246e0499bf6ca4b1c0841b562 |

本轮不运行lint/typecheck或重型测试；是只读证据审计。只新增本报告，原预览资料和失败均未变更。私有目录含.env、凭据和备份，不能整体上传；公共交付只选经审查的脱敏说明/清单。
