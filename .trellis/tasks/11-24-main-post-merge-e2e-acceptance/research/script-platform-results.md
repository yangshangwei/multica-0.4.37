# 脚本与平台契约检查结果

基线：`main@103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。
运行：`main-post-merge-e2e-20261124-080504`。
范围：现有 Node、Vitest 脚本、shell、Python 和平台入口契约；不代表实际部署、SQL 数据链路、安装器、生产 Electron Main 或商用模型已验收。

## 结论

声明了 **57 个入口**。按完整保留的初轮与配置补充记录核对，**53 个本机可运行入口最终 PASS，4 个因缺少 Helm/PowerShell 未运行**。完整声明范围的结果为 **PARTIAL**。

合法 Node 结果为 **366 个命名终端：361 PASS、5 SKIP**。另外有三份 Python unittest 文件合计 **39 PASS**、九个 shell 入口 PASS。八个桌面 Vitest 脚本的 **212 PASS** 已包含在前端总账中，不重复计入新增测试。

恢复后只运行证据核对器，没有重跑产品测试。核对器实际退出 **2**，明确表示 PARTIAL，见 [recovery-review-01.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts/recovery-review-01.json)。

| 验证类别 | 入口/文件 | 实际结果 |
| --- | ---: | --- |
| Node 原生测试 | 33 文件 | 361 PASS / 5 SKIP，共 366 命名终端，含 Node 统计的嵌套用例 |
| 桌面 Vitest 脚本 | 8 文件 | 212 PASS，关联本次前端完整运行；不重复计数 |
| Shell | 10 入口 | 9 PASS，Helm 契约 1 NOT_RUN |
| Python | 3 文件 | 17 + 16 + 6 = 39 PASS |
| PowerShell | 3 入口 | 3 NOT_RUN |
| **声明入口合计** | **57** | **53 PASS / 4 NOT_RUN** |

## 原始失败与补充处置

首轮记录完整保留在 [commands.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts/commands.json)。它包含 **43 PASS、10 FAIL、4 NOT_RUN** 的入口结果。首轮 Node 原报告为 364 tests、342 PASS、9 FAIL、13 SKIP；其中八个失败是错误 runner 的文件级终端，不能计为八个产品用例。

| 初轮问题 | 实际处置与最终证据 |
| --- | --- |
| 八个 desktop `*.test.mjs` 需要 Vitest，却进入 `node --test` | 保留原始 Node 失败；按 `apps/desktop/vitest.config.ts:14` 的正确入口，关联本次完整桌面 Vitest 的八文件/212 PASS；没有重复运行它们 |
| `collaboration-stage-execution.node-test.mjs` 缺少独立 capture/command 前置输入 | 新建 `stage-execution-configured-01`，当前 test/source/control SHA 与新私有 capture 均保留；实际 AF_UNIX frame/ack/capture 1/1 PASS，验证原 120 秒 metric-window 注册参数。它不证明真实 120 秒运行或完整协作执行 |
| `selfhost-config.test.sh` 在私有 HOME 下找不到已安装 Compose，退出 125 | 新私有 Docker config 只指向 Docker.app 现有 cli-plugin 目录；Compose 5.0.2 preflight PASS，不复制用户认证、不安装、不启动服务；`compose-configured-01-1` PASS |
| 八个 Compose availability 顶层分支在初轮跳过 | 仅选择这八个原跳过名称，两个现有文件的配置补充实际执行 8 个顶层 + 10 个新子例，共 18 PASS；没有重跑原先已通过的名称 |

相关补充记录分别保留在 `stage-execution-configured-01/`、`compose-client-preflight-01/` 和 `compose-configured-01-{1,2,3}/`，原失败没有被覆盖。

## 退出码与验收语义

原私有 `run_scripts.py` 完成记录工作后退出 0，尽管其 entry receipts 包含 10 个 FAIL。因此 [driver-exit.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts/driver-exit.json) 的 0 **只证明记录器完成，不能单独证明测试通过**。

恢复收尾保留原程序和 inputs 的字节，新增 [recover_evidence.py](/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/scripts/recover_evidence.py)，依据当前源码、完整命名终端、日志摘要、每入口状态和补充处置决定结果。核对器验证了没有未解决的本机可运行入口失败，并对四个未运行入口给出 PARTIAL/退出 2。它没有把首轮退出 0 或单次诊断 PASS 提升为完整验收。

## 剩余五个条件跳过

这些是仍未执行的具体命名场景，保留为 SKIP，不算 PASS。原 TAP 未附文字原因，以下前提来自当前源码。

| 场景 | 当前源码与前提 | 本次处置 |
| --- | --- | --- |
| real Compose preserves every literal path character after installation and reinstallation | `scripts/install-changelog.test.mjs:166`，`MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1`；该个体实际只使用 Compose config 与私有文件 | 默认 opt-in 保持关闭；可由后续明确选择的配置子验收补足 |
| actual loaded frontend image installs with UID permissions and durable Compose dotenv escaping | `scripts/offline-changelog.test.mjs:556`，相同 Docker smoke opt-in 和明确的加载镜像 | 未启动真实 Docker smoke/加载镜像 |
| real shell entry rejects CR/LF directory values before mkdir or any bind mount | `scripts/offline-changelog.test.mjs:586`，相同 opt-in | 未运行该实际 Docker 命令链 |
| real shell entry preserves quotes and commas in every Docker bind source | `scripts/offline-changelog.test.mjs:623`，相同 opt-in | 未运行该实际 Docker 命令链 |
| PostgreSQL fixture isolation, idempotency and reset guards | `scripts/seed-rich-test-data.test.mjs:79–80`，明确 `SEED_TEST_DATABASE_URL` 的已迁移任务隔离数据库 | 本脚本 lane 没有 SQL 授权/分配，不继承默认数据库 |

这些集成前置条件应由父会话与后续运行分配、配置和完整验收对齐，不借用其他工作区数据库、旧结果或隐式默认端点。

## 五个集成场景的后续执行交接

以下是准入后的执行形状，**本次恢复没有执行这些命令**。继续使用本次记录的 Node、`go-test-with-agent-cli-guard.sh`、干净子进程环境、Git 外 HOME/TMP/GOTMPDIR 和唯一的新输出目录，保存原始 TAP、实际资源与资产身份。不要重跑已经 PASS 的名字。

| 场景组 | 实际会使用/创建的资源 | 必需的当前源资产与隔离条件 |
| --- | --- | --- |
| install-changelog 的 real Compose 场景 | 仅创建私有 fixture 文件并调用已安装 `docker compose ... config --environment`；不会 build/pull/up/run 容器，也不使用 PG | 当前 `scripts/install-changelog.mjs`、`install-changelog.test.mjs` 与 helper SHA；本次私有 `DOCKER_CONFIG` 仅指向已安装 cli-plugin，不借用户认证 |
| offline-changelog 的 loaded frontend + quoting 场景 | fixture 在源码定义的 `.changelog-build/smoke-*`、`shell-boundary-*`、`mount-csv-*` 创建随机共享目录；安装脚本使用实际 Docker 临时执行环境及 bind mounts，之后 Compose config 解析；不启动应用 API/Web/PG；测试注册目录清理 | 当前 `offline-bundle.sh`、`install-changelog.sh`、发布/helper 源码与输出 bundle；显式设置 `MULTICA_CHANGELOG_SMOKE_IMAGE` 为已记录 digest/当前源 provenance 的前端镜像，不能无证据采用默认 `multica-web:dev`；需要父会话准入随机共享目录前缀和临时 Docker 资源、验证清理 |
| offline-changelog 的 CR/LF 拒绝场景 | 构造同类 bundle 与私有 deployment；若实现正确，非法目录在 mkdir/bind 前被拒绝，trace 不含 `--mount`；测试仍会查找真实 Docker CLI | 同组当前 shell/bundle 源码、私有 Docker config；完整保留原 env/feed/trace，不能把“只检测到了参数错误”当作正确拒绝链 |
| PostgreSQL seed 场景 | 一个独立、可丢弃、已迁移的本次数据库；向随机 workspace 写入 fixture、sentinel、活动迭代和临时引用，验证回滚/幂等/reset，最后关闭 client；不需要运行 Agent 或应用服务器 | 数据库名必须匹配源码的 `^multica_(rich_seed_verify_|seed_test_)`，例如 `multica_seed_test_<本次唯一后缀>`；使用当前已哈希 migrate/schema；测试开始前该新库必须有一个本次合成 fixture owner（源码 `:90` 读取首个 user），不能克隆已用 API/Go 测试数据或借真实账号 |

Docker 三个真实 shell 场景为了 Docker Desktop 共享而在源码固定的 `.changelog-build` 下建随机子目录；它们不是本次全局 HOME/TMP。父会话应在执行前登记其仅限随机前缀的写范围、镜像和临时容器归属，执行后核对自清理，不放宽其他测试的 Git 外状态要求。

第一组可独立只选一个原 SKIP 的顶层场景：

```sh
node --test --test-concurrency=1 --test-reporter=tap \
  --test-name-pattern='^real Compose preserves every literal path character after installation and reinstallation$' \
  scripts/install-changelog.test.mjs
```

该子进程明确设置 `MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1` 和本次私有 `DOCKER_CONFIG`；虽然共享 opt-in 为 1，名字选择仅允许运行这一原跳过场景。

Docker 实际 shell 组只选以下三个原 SKIP 名称，使用相同 opt-in、私有 Docker config 和已登记的显式镜像：

```sh
node --test --test-concurrency=1 --test-reporter=tap \
  --test-name-pattern='^(actual loaded frontend image installs with UID permissions and durable Compose dotenv escaping|real shell entry rejects CR/LF directory values before mkdir or any bind mount|real shell entry preserves quotes and commas in every Docker bind source)$' \
  scripts/offline-changelog.test.mjs
```

PG 场景应先通过本次 PG 的 container/label/port 身份确认，创建上表新名字的空数据库并用当前 migrate/schema 迁移，在它里面准备一个合成 owner。若 owner 通过任务 API 建立，Node 检查前先停止该自有 API，避免共享池/后台 worker 干扰。将仅该库 URL 从私有 allocation 注入 `SEED_TEST_DATABASE_URL`，在同一子进程配置先验证 `current_database()` 与登记名称相等，且 URL 不含路由覆盖。只选择原跳过的顶层场景：

```sh
node --test --test-concurrency=1 --test-reporter=tap \
  --test-name-pattern='^PostgreSQL fixture isolation, idempotency and reset guards$' \
  scripts/seed-rich-test-data.test.mjs
```

PG 失败时保留该新库和原日志，后续完整复跑分配另一个新库；不使用被 API/Go 整包测试消费过的库，不删除 protected history 来取得 PASS。这五个顶层场景会按实际运行展开新的子例，最终以命名终端对账，不能预先填入预计 PASS 数。

## 未运行平台入口

| 入口 | 缺少的实际前提 |
| --- | --- |
| `scripts/helm-config.test.sh` | 已安装 Helm 不可用 |
| `apps/desktop/scripts/accept-windows-installer.test.ps1` | PowerShell 不可用，且本机不是 Windows native runner |
| `apps/desktop/scripts/windows-update-acceptance.test.ps1` | 同上 |
| `scripts/install.ps1.test.ps1` | 同上 |

没有新增依赖，也没有把配置/跨平台静态测试当作 Windows 安装器、ACL/Job Object 或真实升级通过。

## 来源、权限与进程边界

- 原脚本 lane 使用干净 allowlist 环境、Git 外私有 HOME/TMP/GOTMPDIR、已安装工具与显式 Go 缓存；所有命令经现有 `go-test-with-agent-cli-guard.sh` 包裹。
- 默认路径没有启动 production Main、真实模型账号、部署或 SQL。少数 fixture 使用 `/private/tmp` 下随机 0700 的短 AF_UNIX 路径，保留其受控传输与自清理边界。
- 全部 57 个选中测试文件的当前 SHA 仍匹配 preflight，原最终快照没有 tracked diff。恢复全仓来源核对仍 PASS，见前端 `source-recovered-01.json`。
- 原 runner 进程已结束；恢复只新增证据核对器、JSON 和本研究报告。完整 API、browser、daemon、安装器和终审资源归属仍由父会话另行验证。
- 完整原始日志、命令、时间、退出码、源 SHA 和每个命名结果均保留；八个 runner 错误、stage 前提失败、Compose 发现失败与后续 PASS 分开记录。
