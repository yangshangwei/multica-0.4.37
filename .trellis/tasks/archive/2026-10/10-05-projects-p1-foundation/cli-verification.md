# P1 CLI 显式版本编辑兼容

工作树：`/Volumes/artisan/code/2026/multica-projects-p1`。范围仅 `server/cmd/multica/cmd_project.go`、`cmd_project_test.go`；用户文档及 built-in skill 示例由父任务更新。

## 行为

- `project update` 新增 `--expected-description-revision` 和可选 `--expected-revision`，作为 JSON 数值发送对应 snake_case 字段。两者均为正 int64，并限制在 JSON/TypeScript 安全整数 `1..9007199254740991`。
- 描述版本必须搭配实际 `--description`，包括 `--description ""` 清空。版本参数是前置条件，不算更新字段；单独传版本仍报无字段更新。
- 未传参数的旧调用不补 token，由后端决定是否以 428 拒绝。显式版本完全来自调用者；CLI 不读取最新版本代填，也不在冲突后刷新或重试。现有短 ID 解析仅解析身份，列表返回的 revision 不用于更新 token。
- 保留 title/status/icon/lead/date 原参数。project CLI 没有既有 `status_reason` 参数，本次不扩展它。
- 核查发现共享 CLI 格式化器会把 428 显示成“An unexpected error occurred.”。仅在 project update 使用现有 `cli.WithUserMessage` 给出非 debug 模式可见的 HTTP 428 / `--expected-description-revision` 指引；保留底层 `HTTPError` 和退出分类，不改共享错误框架。

## TDD 与验证

- `/tmp/p1-cli-red.log`：新增参数注册、实际请求 body、非法数值本地拒绝测试在修改前失败；旧代码漏发两个版本字段，并把非法参数静默忽略后发出请求。
- `/tmp/p1-cli-428-red.log`：新增正常输出断言证明旧 428 只显示泛化错误；加入命令级提示后通过。
- 使用真实 `httptest.Server` 捕获请求：非空/空描述、两 token、旧调用无 token、保留 title/status/date、最大安全整数、零/负数/小数/非数字/溢出/超安全范围、description token 缺 description、token-only 请求、400/409/428 无自动 GET/重试/成功输出。
- 另覆盖短 ID 解析：列表故意返回最新 description_revision=42/revision=99，显式旧 token 3/7 保持原样；旧调用即使解析看到了版本仍不发送 token。

最终命令（2026-10-05 18:44 Asia/Shanghai）：

```sh
bash scripts/go-test-with-agent-cli-guard.sh go -C server test -race ./cmd/multica -run 'Test.*Project|Test.*ResourceRef' -count=1 -timeout=120s -json
# cwd=server
go vet ./cmd/multica
```

结果：**12 个顶层测试、57 条含子测试 PASS，0 FAIL，无 race，1.790s**（`/tmp/p1-cli-final.jsonl`）；`go vet`、`gofmt`、本范围 `git diff --check` 通过。测试只访问 httptest 本地服务器，不访问真实后端，不调用真实 agent CLI。

本项未新增依赖、自动 revision GET、自动请求重试或新 CLI 子命令。未用真实用户服务器做发布验证；全 P1 验收和部署仍由父任务整合。
