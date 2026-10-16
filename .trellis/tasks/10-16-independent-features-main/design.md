# 整合设计

在sibling worktree从main创建codex/independent-features-main，保持原工作区不切分支。按六组净补丁提取源码、测试、spec，排除旧任务记录和证据。

顺序：测试→下载→自动化→设置→文档→seed。实现代理逐组应用并记录各步staged tree；主会话最终验证后按树提交，确保可逐功能回滚。重叠spec索引累计保留所有有效条目。

settings/client.ts仅移入PAT schema import和三个PAT方法，保持main其余内容，禁止从HEAD整文件覆盖。c98c14ad4只取组件、对应测试和automation规范三个路径。无新增数据库迁移；MDX与内置bundle同步。

独立审阅检查main基线、跨包依赖和协作排除。所有功能提交和验证完成后在隔离工作区切换main并--ff-only合并。若main变动则重新整合验证，禁止覆盖。原工作区与其未跟踪文件不动。
