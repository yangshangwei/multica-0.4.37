# 交互与技术设计

完整设计见 [从模板中修改并创建 skill](../../../../../docs/plans/2026-09-12-skill-template-creation-design.md)。

采用内置目录读取、会话内编辑、最终一次创建的流程。模板使用独立类型，创建后为普通 skill。前端草稿序列化负责 metadata 与文件头一致性，模板原身份不复制到副本。

方案已实施并完成验证，见 verification.md。
