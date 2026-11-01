# 证据索引

被测源固定822c15785；原始数据在本目录保留，凭据只在private目录，不写入报告。下列均可从当前工作区重现或核查。

| 证据 | 内容 |
| --- | --- |
| `baseline/source.json`、`baseline/commits.txt`、`diff-name-status.txt` | 原始HEAD、远端基线、完整业务差异和工作树 |
| `baseline/fix-ancestry.json` | 当前版本与其他工作树修复的祖先关系 |
| `environment/*running.json`、`*-config.json` | API/Web生产构建、PID、指纹、模式、配置 |
| `scope.md`、`change-risk-test-mapping.csv` | 需求/变更→风险→测试；领域关联局限已声明 |
| `coverage.csv`、`coverage-results.json` | 每个原用例最终结果、首轮状态、次数、证据 |
| `executions.csv`、`executions.json` | 每次执行原始状态，未丢弃失败/中断 |
| `additional-cases.json`、`evidence/ENV1/*` | 新增P0回归、SQL/API事实和安全截图 |
| `go-summary.json`、`go-tests.csv`、`go-skips.json` | Go包/顶层/子用例与逐项跳过理由 |
| `runs/typescript-*`、`runs/unit-admin-filter-reproduction` | 全部TS首次结果及管理日期失败复现 |
| `runs/<run>/execution.json`、`output.log`、`results.json` | 精确命令、时间、退出码、JSON结果及附件 |
| `environment/macos-package.json`、`runs/macos-packaged-main-host` | 固定.app/executable/ASAR/CLI哈希、真实完整Main旅程 |
| `environment/namespace-mode-diagnosis.json`、`runs/namespace-*` | 明确v2/v3模式和Linux双进程生命周期 |
| `environment/assist-provider*.json`、`assist-provider*.mjs` | 仅固定JSON/SSE的测试HTTP Provider来源及版本 |
| `environment/cleanup.json`、`runs/*services-stop`、`runs/restore-owned-gates` | 专属服务停止、端口检查、gate恢复与失败数据保留 |

`private/`含测试数据库连接和合成账号秘密，不是可发布证据。raw trace/截图可能含合成会话数据，保持本地；对外传递应使用本报告和已脱敏ENV1摘录。哈希清单不包含private内容。

协作使用仓库 [trellis-channel](../../../.agents/skills/trellis-channel/SKILL.md) 所述边界和原生代理/线程消息；仅向同仓统筹回报授权的资源状态，无邮件、Slack、外部发布或真实Provider重启。

哈希：原证据目录 `sha256-manifest.txt`；API/构建和结果以原执行目录为准。

独立终审：`research/final-evidence-review.md`；后续文档同步核对：`research/final-review-resolution.md`。另时工具版本和独立静态执行位于原证据 `review/`，不替代初始测量。
