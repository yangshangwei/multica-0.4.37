# 实现
1. 独立计划审查后start。trellis-implement直接实现，禁止递归派遣。
2. 所有权仅cmd/multica/cmd_issue.go及相邻receipt helper/test、internal/handler/squad_briefing.go及协议test、internal/daemon/execenv/runtime_config_sections.go对应提及段及窄test、builtin_skills/multica-mentioning与multica-working-on-issues对应SKILL/source maps。不改comment.go、daemon.go、db/generated或角色模板（另一lane拥有）。主会话任务/spec/git。
3. 先真实缺陷回归（fake HTTP response警告缺失），再实现；记录red/green。测试命令限定不调用真实agent、不读用户配置；DB handler测试须主会话分配独立库。
4. gofmt、go build、go vet、CLI/execenv定向测试；handler用独立库协议测试。
5. trellis-check独立检查，无阻断再主会话spec和Lore原子提交。
