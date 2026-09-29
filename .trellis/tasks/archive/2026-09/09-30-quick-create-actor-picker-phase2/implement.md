# 第二期执行计划

用户已授权设计和开发，并确认点击触发推荐。先审查本设计，再激活本任务；不重复询问已确认的触发方式。

## 进度

- [x] 建立任务、记录用户选择及代码调研。
- [x] PRD／技术方案／执行计划与真实上下文。
- [x] 独立方案审查通过；补充小目录片段、响应向前兼容、人类成员接口边界。
- [x] 激活并在独立工作区实施。
- [x] Core默认、预选、API与mutation。
- [x] 服务端权限、检索、受控LLM及确定性测试。
- [x] UI默认／项目／推荐采用及过期保护。
- [x] 全面验证、规范、仅本期增量回传和归档。

## 1 基线与分工

保留第一期代码和现有未提交修改。使用新的独立worktree，从第一期功能分支建立，再带入当前目录相关状态；用临时index保存基线，最终仅回传第二期增量。

Trellis implement/check指引适用，原生子agent明确写入范围，禁止互相覆盖；实现者不提交、不重新规划或递归派生。

三个可并行边界：backend handler/auth/tests；core preference/seed/API/tests；views picker/project/recommendation及locale。共享契约按design.md固定，父表单与UI同一负责人。Leader负责环境、端到端、规范、集成和最终验证。

## 2 Core（测试先行）

先补默认字段归一化／设清／登录和workspace隔离、5级预选和pending-vs-invalid回归；看见失败后实现。API先补workspaceUUID／AbortSignal转发、无害新字段兼容、非法响应与重复推荐拒绝，再实现新请求和mutation。复用旧工具，不新增存储层或React Query数据镜像。

## 3 Backend（测试先行）

先为原调用权限判定补对照测试，再抽出供批量使用的纯判定。推荐测试采用现有dbfx/testutil，人类认证、拒绝agent凭证且不调用模型、无可用候选、私有admin、外workspace、失效队长、超长／未知字段、短名单尾部命中、无证据、假ref/证据、取消／超时／并发饱和和生成后权限变化均需明确覆盖。

通过本地httptest fake LLM断言出站只含允许字段、≤40候选、命中片段来自完整描述，且没有创建任何任务。不得启动真实模型CLI或调用付费provider。文档中的新出站用途与llm outbound-contract测试同批更新。

## 4 UI与表单（测试先行）

先补纯项目分组／8项预算／去重矩阵；默认footer和项目视图使用真实PropertyPicker测试。推荐hook/panel覆盖手动点击、取消、文字/项目/actor/session变化、迟到结果、失效候选、失败重试、采用只改actor、焦点恢复。

移除“创建者”歧义，使用“创建助手”及职责说明；更新第一期E2E和组件断言的可访问名称。保留第一期所有Esc、输入法、上传、草稿、权限与CLI门槛回归。不得把AI结果自动设为默认或最近。

## 5 验证

窄检查后做相关全包检查，命令以package.json/Makefile实际支持为准：

```sh
pnpm --filter @multica/core exec vitest run issues api
pnpm --filter @multica/views exec vitest run modals/quick-create-actor-picker-model.test.ts modals/quick-create-actor-picker.test.tsx modals/quick-create-issue.test.tsx locales/parity.test.ts
pnpm typecheck
pnpm lint
pnpm knip
pnpm check:ui-exports
```

Go在server工作目录，使用显式隔离DATABASE_URL：`go test ./internal/handler -run 'Test.*(Recommend|Creator|Invoke)' -count=1`，及`go test ./pkg/llm`、`go vet`涉及包。新测试不允许静默因无数据库被跳过后宣称通过。

浏览器用独立API和确定性fake provider：真实「帮我选」请求和手动采用、默认恢复、项目变更不改actor、迟到结果拒绝、未配置AI可继续手动、窄屏与真实Electron。截图每轮先visual-verdict，证据放research并写`.omx/state/quick-create-actor-picker-phase2/ralph-progress.json`。规模case至少550候选且验证尾部相关描述入选；检查服务端payload上限和无数据库写入。

验证报告逐项映射AC01–AC15；复验原目录既存core `/mcp`诊断与Knip问题，不混入修复。无新失败后仅回传本期diff，保留其他工作；按Lore格式留存功能提交，归档时检查CLI自动提交只含任务文件。

最终验收见 verification.md；功能分支包含最小可选弹窗扩展支持，原目录保留既有实现。
