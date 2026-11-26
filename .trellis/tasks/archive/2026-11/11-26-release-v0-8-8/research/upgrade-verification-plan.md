# V0.8.8 实包升级验收准备

状态：**仅脚本准备及静态检查完成；没有启动容器、没有运行数据库迁移、没有发布验收通过声明。** 等主会话提供最终源码 SHA、正式外层包和唯一 CI changelog 后执行。

## 文件与输入

执行器：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/acceptance.py`。

从历史 `.omx/qa/release-0.6.7/server-acceptance/acceptance.py` 复制适配，保留其真实包升级、两种认证、管理员初始化、附件/卷/配置及原账号验证。此次私有目录不属于公开交付，不能整体归档或上传。生成脚本仅记录改写来源，执行时只调用 `acceptance.py`。

默认输入：

- 基线实际包 `dist/release/0.6.7/multica-server-upgrade-v0.6.7-linux-amd64.tar.gz`，固定 SHA-256 `cb963621b30c782cc7a7f073dde94065f4dbd43474c38d4879493004e4079d64`，源码 `9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`。
- 正式目标包 `dist/release/v0.8.8/multica-server-upgrade-v0.8.8-linux-amd64.tar.gz`，也可由 `--archive` 选择正式构建输出的精确绝对路径。
- `--commit` 必填完整 40 位 SHA；`--source` 默认新受管 worktree。拒绝 HEAD 不匹配和 server/构建相关路径已跟踪修改。
- 第一次 `extract` 固定 `input-binding.json`，后续每个动作重新核验目标包 SHA、源码 SHA、1754 文件哈希清单；新候选需新演练目录，不覆盖既有失败记录。

## 独占资源与保密

项目 `multica-release-088-{password,device,fresh}`；主数据库 `multica_release_088_{mode}`，专用用户 `release_088`。API 端口 28881/28882/28883，Web 23881/23882/23883，仅 loopback；临时恢复/故障端口 28884/28885。启动前检查端口与既有项目，拒绝复用目录或覆盖既有容器。三个 API 每个最大连接4、最小0；临时进程最多2。

不使用共享本地5432数据库，不改共享数据库限制，不关闭其他容器。临时容器带 `release.owner=v0.8.8-upgrade` 标签；cleanup 仅在 Compose project/标签一致时移除本任务容器和卷。加载实际包的镜像会注册相同原有版本标签，现有运行容器不被重建；父会话应统一安排该动作避免与其他构建争用标签。

所有测试秘密随机生成，`.env`、凭据和原始日志留在私有目录。公开汇总只从已经审查的 JSON 选择无秘密字段，不能直接公开整个 `private-*.json`、`*.env` 或原始日志。

## 运行顺序

下面 `FINAL_SHA` 只是本次已冻结完整 SHA 的占位符。每一步成功才继续下一步；首次调用 `extract` 不启动容器，`load` 加载两份实际镜像并从目标镜像复制 SQL。

```bash
python3 /绝对路径/acceptance.py extract --commit FINAL_SHA
python3 /绝对路径/acceptance.py load --commit FINAL_SHA
python3 /绝对路径/acceptance.py baseline password --commit FINAL_SHA
python3 /绝对路径/acceptance.py upgrade password --commit FINAL_SHA
python3 /绝对路径/acceptance.py restart password --commit FINAL_SHA
python3 /绝对路径/acceptance.py baseline device --commit FINAL_SHA
python3 /绝对路径/acceptance.py upgrade device --commit FINAL_SHA
python3 /绝对路径/acceptance.py restart device --commit FINAL_SHA
python3 /绝对路径/acceptance.py fresh fresh --commit FINAL_SHA
python3 /绝对路径/acceptance.py restart fresh --commit FINAL_SHA
python3 /绝对路径/acceptance.py compare-schema --commit FINAL_SHA
python3 /绝对路径/acceptance.py fault fresh --commit FINAL_SHA
python3 /绝对路径/acceptance.py history --commit FINAL_SHA
# 证据与独立复核完成后，才清理本任务资源
python3 /绝对路径/acceptance.py cleanup --commit FINAL_SHA
```

`extract` 拒绝路径穿越、额外根目录、符号链接/设备、AppleDouble 和私有部署文件；两份包内手册必须与最终源码逐字节相同。manifest 必须写完整源码SHA。

`load` 从实际目标 backend 镜像抽取 `/app/migrations`，逐个核对1754文件名/哈希/字节数，不以最大889替代。核验三个运行镜像为linux/amd64及配置/rootfs/digest；原始映像导出内容审计仍由主会话整包审计负责。

## 覆盖内容

1. **v0.6.7 实际包 password/device → V0.8.8**：完整旧594集合到新877集合、283精确增量，旧账号/会话及重新登录、工作空间/任务/附件、模板目录、真实卷、ordered overlays、JWT、认证模式、部署UUID、数据库URL和新增配置容器传递保留；Web `/login` 200和API `/healthz`同时成功。
2. **有意数据变化**：旧账号 seed 等候名单邮箱/原因，升级后断言两列确实删除；备份在独立空库恢复后邮箱仍存在。原来明确关闭的迭代设置保持关闭。
3. **备份恢复**：把升级脚本备份导入新的任务数据库，用旧0.6.7镜像启动临时API，验证旧会话、密码/device重新登录及任务。先关闭临时API再drop该空恢复库；不把旧镜像启动描述为新schema的降级。
4. **schema 完整性**：分别导出password/device升级库及fresh目标库的完整 `pg_dump --schema-only --no-owner --no-privileges`，只移除工具注释和每次随机的 `restrict/unrestrict` token。严格比较剩余DDL字节；任何差异先查原因，不能为通过而大面积忽略DDL。另记录实际extensions、全部无效/未ready/不live索引和有效comment fallback/bigram索引。
5. **重启幂等**：记录完整version+applied_at、schema hash、实际索引，重启真实backend入口后全部相同；原账号或fresh管理员登录成功。
6. **失败门槛**：新故障库+只挂载给测试容器的迁移副本追加故意失败SQL；真实入口非零退出，日志无API启动标记，失败version不入账本。只删除测试副本故障文件后恢复健康。再在该一次性库删除最早账本项，直接启动API绕过migrate，`/health`可达但`/healthz`必须503。
7. **历史源码链路**：读取 `sql-tag-manifests.json`，按完整文件哈希合并等价基线，分别从v0.5.5/v0.5.6/v0.6.0/v0.6.1/v0.6.5/v0.6.6/v0.6.7不可变SQL构建旧空schema，再迁移到本次完整SQL，与fresh DDL比较。明确这是使用最终migrator的历史SQL链路，不能宣称旧安装器/有业务数据的所有历史升级均已测。v0.6.0标签不替代其历史本地交付身份。

## 已运行检查与边界

- `python3 -m py_compile acceptance.py`：通过。
- `acceptance.py --help`：通过，明确动作和必填SHA。
- AST/只读模块导入自检：通过；没有启动容器，检查当前1754迁移、独占项目/数据库名、分离端口及删除旧“账本不变”断言。
- 静态核对旧SQL字段及迭代设置表，准备故障迁移与healthz策略，未对真实构件执行。
- 未来测试本机为macOS ARM64上Docker linux/amd64仿真，完整升级脚本通过现有Rosetta `arch -x86_64 /bin/bash` 保留其架构检查；不代表物理Linux硬件、Darwin服务部署或Namespace环境已验收。
- 没有pg_bigm扩展时仅能证明pg_trgm fallback；不伪报pg_bigm/pg_cron安装。外部模型、匿名GitHub额度及真实客户网络不在此fixture内。

## 未解除门槛

正式包、最终源码SHA、CI唯一feed尚未收到。脚本准备不证明产品升级可行；未来任何失败保留原始记录，另建重试证据，不改最终源码或构件来迎合验证。

## 标签前源码预验（2026-11-26）

`source-preflight-03` 已通过，完整结果见 `source-preflight-result.json` 和 `source-preflight-binding.json`。关联冻结提交 `7a99271341910b58c35a99d5f38ffe1ae75c6959`；迁移二进制实际在 `53df2dbb60fcb29acebb507f4a6041802bfb27dc` 编译，Git差异与全量server文件清单证明两者Go/SQL源码相同，未伪造编译提交。server清单SHA256 `6f5e3ccfa3d97850096190a46a32c7236abc4fb45478fb80b42738b15a885c96`。

| 不可变SQL基线 | 旧账本 | 新增账本 | 目标账本 | 结果 |
|---|---:|---:|---:|---|
| v0.6.5 / v0.6.6 / v0.6.7 | 594 | 283 | 877 | 通过 |
| v0.5.5 | 535 | 342 | 877 | 通过 |
| v0.5.6 | 559 | 318 | 877 | 通过 |
| v0.6.0 / v0.6.1 | 590 | 287 | 877 | 通过 |

四组均与fresh的完整schema一致，DDL规范化hash为 `4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9`；只移除pg_dump注释/随机restrict token、owner/ACL。完整version集合、旧applied_at、重复迁移进程（877 skip/0新增）、零无效索引及有效pg_trgm fallback已检查。fresh的877条记账包含371条件跳过：实际向SQL引擎提交876个文件，不能称877个文件全部执行。

v0.6.7最低数据哨兵验证账号姓名/迭代明确关闭保留，745字段删除真实发生，独立空数据库恢复备份后旧邮箱值存在。额外故意失败SQL导致迁移工具exit1、完整原账本不变；仅移除测试故障文件后重启迁移工具恢复，无新增记账。

原始01保留PostgreSQL临时Unix socket就绪竞态失败，尚未迁移；02保留源码SHA变化守卫拒绝，尚未启动DB；03改用TCP就绪并关联新冻结SHA，未删减迁移/数据/schema断言。三次记录分目录保留。03全部task-owned数据库随唯一容器和卷已清理；其他服务未变。

这是**源码预验**：使用最终migrator执行与实际v0.6.7包哈希等价的Git SQL及其他不可变标签SQL，不是旧安装器/正式目标包验收；未验证API登录、healthz、镜像entrypoint或真实客户内网。二进制为本地Go1.27 darwin/arm64，数据库为Docker linux/amd64仿真。pg_bigm/pg_cron不可用，只覆盖fallback分支。正式产物到位后仍按前述实包步骤执行。

## 私有预览完整包演练（2026-11-26）

结果：通过，**仅为PRIVATE PREVIEW**，不可冒充正式CI feed包交付。源码 `5a48e91b385967309e53f0ceb211931eecdef873`，完整包hash `675f8f97cd2983d0f83e9f4413c065d5e269329d39aaaf6fce94cd1387159b57`，feed实际为该SHA的Unreleased。

原始证据根：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/preview-acceptance/`。汇总 `preview-verification.json`，文件摘要 `evidence-manifest.json`，SQL query/result及各自摘要 `sql-observations.jsonl`（104次观察）。私有凭据和备份不对外交付。

- 实际v0.6.7 password/device包分别启动、seed并用原包脚本升级，全部594旧账本保留时间戳，加283至完整877；旧会话、重新登录、用户/工作空间/任务/附件及原迭代关闭选择保留。
- JWT、五类集成加密密钥、认证模式、部署ID、数据库URL及新增配置在真实容器中相等；原.env、有序覆盖、模板目录、挂载和卷保留。只记录秘密相等布尔，不公开值。
- 备份各自导入空隔离库，以旧0.6.7镜像验证旧账号登录和任务，745删除字段原值从备份恢复。
- password已有管理员凭据保留；device不创建凭据且返回skipped_mode_disabled；fresh关闭注册时初始化、重复初始化、强制改密、角色/总览/用户/安装列表均通过。
- 三个真实backend容器restart，保留容器ID、记录StartedAt变化；全部账本applied_at和schema不变，旧登录或管理员重新登录成功。升级两库与fresh完整DDL逐字节相同，hash仍为 `4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9`。
- 故意失败SQL使真实entrypoint退出1且没有Starting server；失败version未记账。移除仅测试overlay故障文件后恢复healthz。一次性库删除最早旧version、直接API启动后实际HTTP503：db=ok,migrations=out_of_date。
- 故障01保留启动轮询RemoteDisconnected失败（故意失败/恢复阶段已成功）；02保留路径变量笔误（DB未启动）；03新目录完整重跑通过，未放宽任何断言或改变产品/包。正式harness仅同步启动轮询ConnectionError捕获。
- 所有预览自有Compose项目、容器和卷已清理并记录cleanup.json；其他会话资源保持原样。

正式包仍需独立目录、正式CI唯一feed及SHA绑定复验。此轮为Docker Desktop linux/amd64仿真；无pg_bigm/pg_cron，只验证pg_trgm fallback；不涉及Namespace、付费模型、真实客户内网或Windows安装。

## PostgreSQL 实际镜像升级覆盖修正与补充（2026-11-26）

独立评审发现：原preview的load先旧后新，两个包都使用pgvector:pg17，导致旧版应用基线实际运行在新PG17.12。原password/device/fresh通过结果仅证明旧应用/schema→新应用，不证明旧PG数据卷升级；已在原汇总追加范围修正，未改原始回执。

独立补充目录 `server-acceptance/preview-pg-compatibility/` 真实验证通过：先重新load实际0.6.7包，要求`SHOW server_version_num=170011`；old manifest `sha256:cd4ccfdaf62cbfeb5c6ee19ef0dd663406d4ed740c320b324d016a90e85385b3`。原包offline-upgrade脚本加载新镜像后，原`docker compose up -d --pull never backend frontend`在Compose v5.0.2上确实自动recreate PostgreSQL依赖。新server_version_num为170012，manifest `sha256:0a547a6067fdea4eec4cfb0887c4ddae2e91b1932cecc071541c14ee318312ce`；容器ID/镜像/版本改变，pgdata卷名称/Source/挂载路径/RW全不变。没有加入force-recreate或显式启动postgres来替代原脚本行为。

password完整场景、旧账号会话及重新登录、任务附件、加密密钥/配置/UUID/管理员凭据、283新增/877完整账本、空库备份恢复和旧应用登录均通过；真实backend restart后完整ledger时间戳/schema/登录保留。证据`pg-compatibility-verification.json`、`password-postgres-{baseline,upgraded,upgrade-observation}.json`、SQL观察日志以及命令原始日志。退出后本补充项目容器卷全部清理，pgvector:pg17已核验恢复指向新manifest。

正式harness最小修正：每个password/device旧基线prepare前重新load旧包，断言实际17.11并保存完整manifest/卷/server版本；原升级脚本后观察并记录真实PG行为与卷保留，不强制重建。该补充只覆盖password模式，正式版两种模式仍各自演练；不能泛化到所有Compose版本、外部数据库或用户固定镜像override。

## d2c9240 最终正式包验收准备（未执行）

独立最终目录：`/Volumes/artisan/code/2026/multica-0.4.37/.artifacts/release-v0.8.8/server-acceptance/final-acceptance/`。固定源码完整SHA `d2c9240eaf9f768446faba93cac625bd63e9cf53`，默认正式包路径为用户交付根下 `dist/release/v0.8.8/multica-server-upgrade-v0.8.8-linux-amd64.tar.gz`。`windows-accepted-application-binding.json`记录该提交与5a预览应用树相同；本轮不重复源码预验，正式镜像二进制commit仍须真实报告d2c。

准备文件：`acceptance.py`、`run-final.py`、`run-config.json`、`summarize-public.py`、`preparation-checks.json`。Python bytecode/AST/import静态检查通过。未运行容器/数据库/镜像命令，没有生成正式验收或公开通过回执。当前配置状态`prepared-awaiting-parent-formal-package-ready`，包摘要/CI run/CI changelog目录留空，执行器拒绝运行。父会话确认正式包就绪后才填入：

- `status=parent-confirmed-formal-package-ready`
- 正式外层包的确认SHA256；若实际构建输出不同目录，通过config显式选绝对路径。
- 唯一Release CI run及下载的`changelog.json/changelog.md/changelog-metadata.json`目录。

正式绑定会验证metadata归属yangshangwei/multica-0.4.37、v0.8.8、published、d2c完整SHA、feed/Markdown摘要与发布条目身份；包内feed必须与该CI JSON逐字节相同，Unreleased预览无法通过。每次动作重验包hash、1754 SQL及CI三文件摘要。

流程：extract→load→password baseline/upgrade/restart→device baseline/upgrade/restart→fresh/restart→三库schema对照→真实entrypoint故障与缺旧账本503。每旧baselineprepare立即重新load旧包，要求运行PG17.11及实际oldmanifest；升级后/fresh实际PGmanifest须等于正式包load的目标manifest，保留卷身份观察。不加入任何强制PG重建参数。所有先前raw SQL快照、账本时间戳、登录/附件/备份/secret相等、真实容器StartedAt变化及失败原始记录保留。

命令在父会话通知后执行`python3 .../final-acceptance/run-final.py run`，独立评审后执行同脚本`cleanup`，最后执行`summarize-public.py`。清理前不生成公开通过摘要。运行入口逐阶段记录命令、退出码、时长、harness/log摘要，拒绝覆盖旧尝试日志。

公开汇总采取字段白名单：只输出版本/源码/包和feed摘要、实际PG版本manifest、保留布尔值、计数/schema/健康结果及证据摘要；不复制SQL原文、测试UUID、宿主绝对路径、Docker完整inspect、真实.env、私有凭据或数据库备份。生成时扫描已知fixture密码/JWT/集成密钥/token/设备身份/管理员初始密码，命中即拒绝。父会话仅取`final-acceptance/public/server-verification.json`，原始证据目录不得整体交付。
