# 实现与验证

1. 编写 runtime 三种上下文的交付/验收合同回归、leader 和 squad 分别覆盖六角色的断言；先证明旧代码失败。
2. 修正文本、角色版本和摘要；同步两项 skill/source-map、双语 docs 与 locale，保持 JSON key 不变。
3. agent CLI guard + Git外 TMPDIR，选定 execenv 相关测试；只选 service 纯模板/skill测试，禁止整个 service suite 或实际 provider。
4. 验证 TestBuiltinSkillsConformToTemplate、TestWorkingOnIssuesSkillCoversIssueLoopContracts、TestSquadsSkillCoversLeaderRoutingContract、TestSquadTemplates_LifecycleEvidenceRouting、TestReleaseGateGovernanceDrill_Contract 以及新增一致性与相关 role 模板测试；检查实际已有测试名称再组 filter。
5. 检查双语 JSON 解析/key parity，gofmt/vet（适用包）、git diff --check。后端 barrier 逻辑不变，不为仅文案修复引入新 DB fixture；如修改后端则必须先报告范围。
6. 向主会话记录命令、red/green证据、现有实例升级限制；不自行提交、不扩展其他小队正文。
