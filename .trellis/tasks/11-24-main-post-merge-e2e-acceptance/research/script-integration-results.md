# 五个脚本集成条件跳过的补充验收

基线：`main@103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。本次运行：`main-post-merge-e2e-20261124-080504`。资源 owner：`main-e2e-script-integration-7874d8d24698`。

## 实际结论

原报告保留的五个条件跳过全部完成补充执行，**5 个顶层 / 23 个命名终端 / 18 个嵌套 / 19 个叶用例全部 PASS**。三个选定文件组均退出 0；原测试、断言、超时和名称选择保持不变。没有重跑此前已经 PASS 的产品测试名。

叶用例包含 18 个嵌套用例，以及 loaded-image 那个没有子用例的顶层测试。将 18 标为 leaf 是术语错误；本次原始结果和计数没有修改。

| 原始 SKIP 身份 | 本次实际入口 | 顶层 / 命名 / 嵌套 / 叶 | 结果与实际边界 |
| --- | --- | --- | --- |
| real Compose preserves every literal path character after installation and reinstallation | `scripts/install-changelog.test.mjs` 的单个原名称 | 1 / 6 / 5 / 5 | PASS；真实 Compose 解析，五种特殊路径各安装两次，保持 unrelated dotenv 字节并防止美元展开；没有容器启动 |
| actual loaded frontend image installs with UID permissions and durable Compose dotenv escaping | `scripts/offline-changelog.test.mjs` 的原名称 | 1 / 1 / 0 / 1 | PASS；真实 Docker 中的 Node 以本机 UID/GID 执行当前脚本，写入任务新目录，再由真实 Compose 核对 durable dotenv |
| real shell entry rejects CR/LF directory values before mkdir or any bind mount | 同文件原名称 | 1 / 4 / 3 / 3 | PASS；三个非法路径各被实际无挂载 Node 验证容器拒绝；目录、旧 feed 和 dotenv 均无受保护副作用，trace 没有 bind mount |
| real shell entry preserves quotes and commas in every Docker bind source | 同文件原名称 | 1 / 3 / 2 / 2 | PASS；包目录、输出和 deployment 的双引号/逗号经实际 Docker CSV 解析与安装，再由真实 Compose 核对 |
| PostgreSQL fixture isolation, idempotency and reset guards | `scripts/seed-rich-test-data.test.mjs` 的单个原名称 | 1 / 9 / 8 / 8 | PASS；新空库迁移、只读预览、回滚、幂等保留、人为编辑/删除、外部软引用、运行时与 marker 拒绝、开关关闭后的 scoped cleanup 均实际执行 |

完整命令、时间、退出码、TAP、结构化 Node events、每个命名终端及源文件行列保存在本次三个独立目录中：

- [Compose 补充 inventory](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/compose-01/inventory.json)
- [真实 Docker shell inventory](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/shell-01/inventory.json)
- [PostgreSQL seed inventory](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/seed-01/inventory.json)
- [严格补充账本](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/completion-ledger.json)

账本另外直接核对了每个原始 `test:pass` 的实际文件路径、类型、完整名称集合、顶层/嵌套数、命令退出码和日志 SHA；没有未选择名称执行、缺失、重复、SKIP、失败或命名终端之外的文件级失败。三个原始 events 与 TAP 都保留，不能用一个汇总退出码替代。

对照原脚本报告的合法 Node **361 PASS / 5 SKIP**，本次补齐五个原顶层名称并首次展开 18 个嵌套名称。按这个命名粒度，合并处置后为 **384 PASS / 0 个未闭合的这类集成 SKIP**；原始 5 SKIP 仍保留并链接本补充。前端中已经计数的 212 个 desktop Vitest 结果没有重复添加。原报告四个 Helm/PowerShell NOT_RUN 没有变化。

## 当前资产镜像与证明范围

本机缓存没有 `node:22-alpine`，当前 Web 生产输出没有 standalone。补充在 Git 外私有 `image-mirror` 中复制冻结生产输出、1,328 个 trace 文件及 42 个相对目录 symlink，再调用已安装 Next 16.2.6 的 canonical `copyTracedFiles`，完成 2,292 文件/links 的独立 standalone 组装。helper 退出 0、stderr 为 0；required-server-files、BUILD_ID 和其他主要 manifests 的原始字节均保持一致。

随后从明确缓存的 ARM64 Node22-bookworm 字节生成一个任务镜像：不运行包安装、网络下载或原 Dockerfile 的 build 阶段，Docker build 无 RUN、无 pull、network none。镜像为：

`sha256:68718f8e505250173023cc96f01c69a79a07d6c7357594b669ec3b537d2159f7`

它有实际当前 candidate/owner 标签；默认用户 `1001:1001`。测试运行时原脚本继续显式使用本机 UID/GID。复制给 Docker 的整个 context payload 与 canonical standalone 的全部文件/links 同 hash。原 `apps/web/.next/standalone` 始终不存在，当前 `.next`、Desktop `out` 和产品源码没有被这次组装改写。

该镜像证明**当前生产资产的 runtime assembly 和这五个脚本调用链**。它没有启动 Web app，不证明原 `Dockerfile.web` 的全构建、frontend app、真实 installer/离线部署、发布包或升级验收。macOS 构建所追踪的 native 依赖也没有被提升为 Linux native app 兼容性结论。

对应来源证据：

- [独立资产组装](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/asset-assembly.json)，SHA `e0a13cdc86531972e8da6381c5b77427f1e0588056e7a0f8e4615837d97fdce5`
- [镜像组装](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/image-assembly.json)，SHA `61478cb8f725913f71a849b0860a528eefef96793decf264ac57d3fb05fd1ffc`
- [完整 context 对账](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/context-binding-01.json)，SHA `f89d2522e7fe8374be860427ead28072f1413263d69021e6f963503d3d496efd`

## 独立准入与编排失败保留

执行前的不可变 [plan.json](/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/scripts-integration/plan.json) SHA 为 `613f11a9eb86a3a86093c398dd4847cddd70aaf9a7c63ab3a503853e184965de`。父会话独立复核当前 source、PG/Redis/container/volume、payload/context/image、owner/architecture/user、私有 runner 和镜像 receipts，发布匹配的 accepted/blocking0 [root-review.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/root-review.json) 与 clearance 后，才执行五个产品测试身份和 SQL。

实际执行沿用当前 Node22、Bash、psql、私有无认证 Docker config 与既有 Agent CLI guard。新增 Docker guard 仅允许 Compose 解析和原两种 no-network/pull-never/Node/user argv，验证同一新建 fixture 的 bind 角色和真实 canonical 路径，并加入任务 owner 标签/cidfile。Node 的 `-e` 只允许出现在镜像之后；未知授权、额外 network、旧镜像、外部/既有目录和 symlink escape 均拒绝。全部原 argv 与实际标签后的 argv 分开保留。

20 个任务私有 admission/inventory 反例与真实 Node reporter/名称过滤 probe 的 3 个命名测试 PASS，均不计入产品覆盖。

首个 preparation 把合法 NFT 目录 symlink 错误要求为普通文件，因而失败；当时 SQL、产品测试、容器启动和镜像 build 都是 0。错误版本源码与当时私有目录原字节保留在 [preparation-failed01.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/preparation-failed01.json) 指向的记录中。后续按 canonical helper 的 readlink 语义支持这 42 个相对目录 link，并完整绑定真实目标；没有修改产品测试或将此编排失败归为产品缺陷。

## 数据库和最终资源审计

Seed 使用 backend 已准入、端口 49359 的任务自有 PG，新增空数据库 `multica_seed_test_924fea80cfce_7874d8d24698`，从 `template0` 创建后用当前源码编译的 migrate 全迁移，再只插入一个新的合成 owner。它没有克隆被 API 或 Go 测试使用过的数据，没有共享池/后台 API/daemon，没有修改 backend 输入、allocation、template、PG 设置或任何既有数据库。每次 SQL 都验证同一实际容器、loopback 和 volume；URL/密码仅在私有子进程环境使用。

最终 [资源审计](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/scripts-integration/final-audit.json) 确认：

- 全部 32,724 tracked 源文件、工具、trace 输入和当前生产资产绑定保持；无产品/现有测试修改。
- 9 个实际 Docker Node 容器均通过原 `--rm` 清除，没有自有残留容器。6 个退出 0，3 个退出 1 对应 CR/LF 拒绝断言，是预期负控制。
- `.changelog-build` 中每个新随机 fixture 自行清理，原目录集合完全保留。
- 专用 seed 数据库剩余客户端 0。数据库、当前任务镜像与 base alias 留存为证据资产，等待父会话统一收尾；没有停止或断开其他会话资源。

私有 `audit` CLI 单独退出 0 时不会因组集合不完整而失败。**不能把这个退出码单独视为完整验收**。本次 completion ledger 额外严格核对三个组全部存在且 PASS、5 top / 23 named / 18 nested / 19 leaf、raw events、source/assets、seed clients0 和 container remaining0。父会话仍负责整个 T01–T25/L0–L10 campaign 的最终严格 gate。

## 变更和剩余边界

新增内容仅为本任务 `scripts-integration` 的计划、runner、Docker/SQL 限界、canonical asset assembler、结构化 reporter、私有反例、独立 receipts 和本研究报告。复用既有产品测试、Next helper、backend ownership/source helpers 与 Agent CLI guard，没有新增依赖，也没有扩展为产品修复。

这五个集成 SKIP 已闭合；原 Helm/PowerShell 未运行入口、真实 frontend Docker 构建、原生 Windows/移动/桌面 Main、全离线部署/升级以及真实模型仍属于其他 lane 的独立结论。本补充不改变它们的状态。
