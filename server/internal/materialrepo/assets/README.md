# 工程物料仓

本仓保存工程定义、固定版本与交付证据引用；讨论、分派、进度和运行记录由 Multica 管理。

## 读取入口

先读 material.yaml 的系统身份与格式，再按当前任务读取相关物料及 .material/templates/ 模板。AGENTS.md 说明运行时的读取顺序和写入边界。

## 目录及产生时机

- modules/<id>/module.yaml：登记真实模块时形成，仓库使用逻辑标识关联。
- requirements/<id>/prd.md：需求首次送审形成；design.md、plan.md、test-plan.md 随实际阶段产生。
- issues/<id>/issue.md：问题报告；resolution.md 保存实际修复和复测证据。
- releases/<version>/release.yaml：产品版本的需求与计划修复范围引用；test-plan.md、acceptance.md、baseline.yaml 随测试、验收与发布形成。
- contracts/：跨模块契约，既有代码仓契约只保留固定版本引用。
- specs/：长期领域与工程规则。
- decisions/：重要决策、替代项及适用边界。
- knowledge/：已核实的经验与操作方法。

目录按内容产生，不创建空业务记录。需求和问题使用稳定 ID，完成、归档和延期不移动文件。产品版本表达交付范围，Git 提交与内容摘要表达具体修订；版本范围引用同一份正文。

## 模板与维护

.material/schemas/ 定义结构，.material/templates/ 提供待填写模板。模板中的占位符必须替换为真实信息；缺少证据的字段保持未验证，不能编造通过结论。

README.md、AGENTS.md、material.yaml 及 .material/ 的变更走独立初始化/维护审核。普通需求发布只写当前授权物料路径。已有文件保留，未知格式先检查并报告差异。仓库规则不授予 Git 凭据或人类审批权。

截图、录屏、大型日志和构建制品保存在附件或制品存储中，这里记录稳定引用与摘要。凭据留在授权运行环境，不能写入文档。
