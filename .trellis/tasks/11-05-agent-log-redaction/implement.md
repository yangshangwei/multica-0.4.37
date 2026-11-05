# 实现与验证

1. 先增加能失败的结构化/JSON/本产品 token 回归及非敏感对照；只运行 pkg/redact，保存失败摘要。
2. 最小实现字段语义、JSON 文本与 token 边界；保持格式、深度限制、复制/幂等。
3. fake daemon 测试覆盖完整输出中的秘密跨 preview 截断边界；确认旧调用失败，再修调用。
4. 定向验证（根目录通过 agent CLI guard；测试 TMPDIR 在 Git 外，清除继承 DB 配置）：
   - go -C server test ./pkg/redact -count=1
   - go -C server test -race ./pkg/redact -count=1
   - go -C server test ./internal/daemon -run '^(TestToolOutputPreview.*|TestExecuteAndDrain_ToolOutput.*|TestExecuteAndDrain_RedactsNestedToolInputBeforeSending)$' -count=1
   - go -C server test ./pkg/agent -run '^(TestCodexPatchApply.*|TestCodexRawItemMCPToolCall.*)$' -count=1
   - go -C server vet ./pkg/redact
5. gofmt 与 git diff --check；向主会话交付精确改动、执行命令、失败/通过和已知限制，不自行提交或扩展到动态全局凭据注册。
