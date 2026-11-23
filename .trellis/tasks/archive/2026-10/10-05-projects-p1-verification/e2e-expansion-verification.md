# P1 端到端用例细化验收

2026-10-06，**61/61 通过，137.9 秒，零重试、零跳过、零 flaky**。严格 TypeScript 检查通过。逐条中文前置、操作、预期、需求编号及源码位置见 [61 条用例矩阵](e2e-case-matrix.md)，机器可读结果见 [最终结果](e2e-expanded-evidence/final-results.json)。

## 覆盖变化

| 测试域 | 独立用例 |
| --- | ---: |
| 目标与描述 | 5 |
| 统计与正式范围 | 11 |
| 风险下钻与分页 | 8 |
| 进展、验收与证据 | 15 |
| 生命周期、权限与恢复 | 15 |
| 真实 Electron | 6 |
| 双页面持续收敛 | 1 |
| **总计** | **61** |

原 8 条长流程变为 61 个独立入口，其中 25 条拆分/保留原覆盖，36 条新增或强化关键前置与边界断言。不是新增 53 项业务需求，也没有把 `test.step`、循环断言或 30 个性能样本算作独立用例。各 test 自建账户、工作空间和前置数据，失败不阻断其他 test；API/SQL 只负责前置和结果核对，命名的用户操作经过真实 UI。

本轮加强了取消/导航/重载后的草稿恢复、真正提交后的丢响应重试、并发更正、旧通知深链、来源失权后保留本人文本、删除失败重试、角色降级后的操作拒绝，以及无 WebSocket 时真实 HTTP 撤权并清除已持久化稿。五种视图和四类风险分别产出独立结果。

## 实际运行与修正

| 运行 | 结果 | 说明 |
| --- | --- | --- |
| 首轮完整 61 条 | 57 通过 / 4 失败 | [原始结果摘要](e2e-expanded-evidence/run1-results.json)，保留真实失败 |
| 针对失败和审查修正的 7 条 | 7/7，通过 14.6s | [修正后结果](e2e-expanded-evidence/fixes-results.json) |
| 最终完整 61 条 | **61/61，通过 137.9s** | 父执行 session 29506 退出 0，1 worker、0 retries |

四项首轮失败均来自测试对 UI/传输契约的假设：

- D06：原生侧栏与面包屑都有 Projects 链接，改为准确定位侧栏链接。
- L07：删除失败后确认框保持打开，背景概览不属于当前可访问模态；改为确认弹窗及服务端项目仍在，并在同一弹窗真实重试。
- L10：能力关闭后描述显示为完整 Markdown 文本块，改为断言完整原文，仍验证数据库描述未变。
- L14：真实 leave 返回空 204，不应调用 JSON parser；改为校验状态后触发实际 overview404，并验证页面和已持久化草稿清除。

独立只读审查还修正了两个风险：L04/L14 改为独立成员 browser context，消除 owner/member 初始化脚本写 token 的顺序竞争；D05 恢复已有中文进展和修订历史在 680px 原生窗口的覆盖。上述修正未改产品源码，也没有弱化服务端权限或幂等断言。

## 构建、命令与产物

基线 `421ca49eb`；产品代码保持 `31534d844`。复用本工作树专用 registry `check-20261005144412-26132` 的独立数据库，重新启动 API 并构建 production Web / Electron。API 19072、Web 13992，Web build `NeXhpciU_TdN4TwOKBeA7`。测试迭代仅更改 E2E/fixture，产品构建无需为每个测试修正重复执行。实际归属与最终测试文件 SHA256 见 [provenance](e2e-expanded-evidence/provenance.json)。

```sh
pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 \
  --module ESNext --moduleResolution bundler --esModuleInterop \
  e2e/projects-p1*.spec.ts e2e/fixtures/project-p1*.ts

pnpm exec playwright test e2e/projects-p1*.spec.ts --list

bash scripts/dev-env.sh exec check-20261005144412-26132 -- \
  env PLAYWRIGHT_JSON_OUTPUT_NAME=.omx/p1-expanded-final.json \
  pnpm exec playwright test e2e/projects-p1*.spec.ts \
  --workers=1 --retries=0 --output=.omx/p1-expanded-final-results --reporter=list,json
```

用显式 `e2e/` 路径限定文件；只用 `projects-p1.*spec.ts` 正则会匹配本工作树绝对路径中的 projects-p1，误收集其他项目测试。本轮实跑命令明确只运行七份 P1 spec。

本机完整日志：`.omx/p1-expanded-{run1,fixes,final}.log`；最终结果目录 `.omx/p1-expanded-final-results`。安全结果、30 样本及测试文件哈希随仓库保存；认证 token、数据库连接配置不进入证据目录。

## 持续收敛与范围边界

[30 条原始样本](e2e-expanded-evidence/convergence-30.json) 仍作为单个连续场景：due_date/assignee/admission 各 10 次，两个页面均展示新版本与正确数字，以较慢页面计时。总体 P50 45ms、P95 85ms、最大 86ms；三组 P95 为 85/46/86ms，均低于 5 秒。页面错误为零，未增加执行记录。

没有新增产品功能、依赖、生产部署或真实 agent 调用。本轮仅测试/fixture/文档变化，因此未重复 Go、移动端及全仓单元测试；其既有证据保留于父验收记录。I1 真实迭代历史、iOS 设备/模拟器/IPA 和可选 Redis 集成仍不属于此 E2E 细化验收。
