# 独立整合复核

Trellis check 只读审阅：无 P1/P2 阻塞。

- 暂存168文件等于批准的六组并集；各产品/测试blob与最终源提交一致。
- client.ts 除PAT三个方法和对应导入外，与main7906c6e93逐字节一致。
- 无协作路径、运行符号、服务、handler或迁移；迁移上限570。
- Query权限边界、秘密隔离、双端滚动接口、下载发布次序和seed依赖完整。
- desktop spec索引累计条目正确。
- 构建生成next-env.d.ts的变更将在进程停止后还原。

后续E2E补充：批准两处旧文本断言对齐已移入的英文产品文案，未削弱创建权限、执行者或请求体断言。此补充仅修改e2e/autopilot-template.spec.ts。
