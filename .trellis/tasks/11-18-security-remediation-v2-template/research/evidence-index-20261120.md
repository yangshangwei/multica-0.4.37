# 持久证据索引

仓库外证据根：`/Volumes/artisan/code/2026/security-remediation-v2-evidence-20261120/`，权限0700。供应商会话不在仓库，本轮未产生真实模型会话。账号配置与数据库连接只留private env，不放入本索引。

- `initial/`：两个检出的初始HEAD/status/diff/脏文件hash，原24产品路径可恢复tar，三个任务目录副本。
- `prior/manifest.json`：947个必要历史日志、复核、探针源和摘要，记录原始与脱敏后sha256；`prior/logs`、`prior/review`、`prior/work`保留implement历史索引相对结构。原始数据库URL、token、JWT内容已脱敏，业务测试名称保留。
- `work/final-product-sha256.json`：27个最终产品路径的原始字节hash。
- `work/source-provenance.json`：第一次变基来源；后续c56e2194e同步见implement记录与Linux source快照。
- `logs/*`：本轮迁移、整包/定向/race、构建、E2E、vet、lint/typecheck。原失败和超时日志保留，不覆盖为通过。
- `e2e/results/`：最终生产构建双语目录/采用用例产物；`e2e/health.json`与process记录证明运行环境。
- `work/cleanup-old-databases.jsonl`：旧32库逐项删除回执；大小及名称清单在同目录。
- `work/cleanup-amd64-databases.json`、`cleanup-native-databases.json`：本轮完成环境的精确清理回执。
- `work/stage-diagnostic/`：基线StageCompletion诊断overlay与未改动源码hash；诊断不替代正式验证。
- `linux-service/`：最后service完整验证的固定source、构建缓存与测试二进制（大体积临时物在汇总hash/日志后清理）。

仓库内独立复核：`review-20261120.md`。子任务二fake-only及原生无模型探针在它自己的research目录，模型turn/provider/account用量仍0，不能与子任务一契约或平台脚本混为真实验收。
