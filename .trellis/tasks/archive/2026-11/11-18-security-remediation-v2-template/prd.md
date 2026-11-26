# 安全漏洞修复小队 v2：指令、契约测试与文档同步

父任务：`.trellis/tasks/11-18-security-remediation-squad-v2`。需求（R1–R13）、背景事实（B1–B8）和决策（K1–K8）以父任务 `prd.md` 为准，本文只按编号引用，并写明本子任务的范围、依赖和验证形式。技术方案见 `design.md`，执行计划见 `implement.md`。

## Goal

交付内置小队 `security-remediation` 的 v2：让流程在审查员结果不回流的现行平台上不再静默停住，补上修复前的独立复现环节，并把分诊记录扩展到能支撑定级、“不受影响”结论、变体和回移。只改指令文字、版本号、测试、文案和文档。

## Requirements

- 实现父任务 R1–R11 与 R13 的全部条目，遵守 K1–K7。
- R12：本子任务的证据只有契约测试和脚本模拟两类；每条行为结论标明类型。
- 改动范围：
  - 两份指令正文与两处版本号（小队、队长角色各升到 2）。
  - 服务层契约测试、处理器层测试。
  - 目录文案及其在测试中的副本。
  - 产品文档（中英文，含本轮授权的 comments.mdx/comments.zh.mdx）、生成的文档包、内置产品 skill 说明及其 source map、`.trellis/spec/server/builtin-templates.md`。
- 不改：花名册（仍是 v1 的 7 个角色）、角色模板与角色 skill、任何处理器或服务的产品逻辑、数据库、前端组件逻辑。

## Constraints

- 不运行真实模型，不解析或执行用户安装的智能体 CLI（仓库测试规则；父任务 K3）。
- 新增的数据库测试用 `server/internal/testutil` 的夹具和 `testutil.Call`，不手写 `INSERT` 与请求样板。
- 指令正文为中文；代码注释为英文。编辑 `packages/views/locales/` 与中文文档前先读 `apps/docs/content/docs/developers/conventions.zh.mdx`。
- v1 模板只存在于当前分支：`main` 落后 704 个提交且不含该模板（2026-11-18，`git rev-list --left-right --count main...HEAD` 输出 `0 704`）。实现基线是当前分支的提交，不是 `main`。
- 工作区里有其他会话的大量未提交改动。本子任务只触碰自己的文件，提交时按路径暂存。规划时本子任务要改的文件都没有未提交改动。

## Dependencies

- 上游：无。启动条件是用户批准规划。
- 下游：子任务二 `11-18-security-remediation-v2-real-model-acceptance` 依赖本子任务合入。

## Acceptance Criteria

对应父任务 AC1–AC8，验证形式如下。

- [x] AC1 `server/internal/service/builtin_security_remediation_templates_test.go` 中：新增契约组在两份正文上各自通过；否定断言确认 v1 的回执与重试表述已不在；共用小节逐字一致的测试通过。
- [x] AC2 同一文件：最小安全复现的四条边界、`code` 路由里复现步骤先于实现步骤、补丁作者不得修改复现测试。
- [x] AC3 同一文件：v1 的 13 行分诊字段是 v2 分诊块的前缀；12 个追加字段及其枚举逐行存在；期限表四档；提交绑定与变体规则。
- [x] AC4 v1 的五组契约标记（triage、routing、handoff、stop、evidence）不改一字，全部通过；父任务状态小节的测试通过。
- [x] AC5 `design.md` 第 9.3 节的表里每一行都有测试钉住：已有的规范测试，或本任务新增的处理器层测试。新增的测试确实运行而不是被跳过。
- [x] AC6 处理器层四个脚本场景通过；`design.md` 第 11 节的对照表在实现结束时补上每项的实际证据位置。
- [x] AC7 契约测试钉住“安全负责人”“security owner”“漏洞验证”“vulnerability verification”“任务创建者”“非独立复现”；注册表中本小队仍是 7 个成员座位；中英文文档含试点指令范本与入座步骤。
- [x] AC8 `pnpm generate:docs-bundle` 后生成目录无差异；`implement.md` 列出的命令全部通过，未运行的逐条写明原因。

## 本轮授权与边界（2026-11-20）

用户明确批准19项设计定稿、14.2推荐及试点前修订、独立产品任务规划、本地提交与归属清理。最终设计见 design.md 第14节；版本规则见父任务 K12。真实模型调用归子任务二，子任务一本身仍不运行真实模型。
