# M1-C2 关键智能体路径 PR smoke

## 目标
PR验证必须真正执行固定关键路径并核对库存，复用现有真实API/浏览器+SimulatedDaemon，不使用付费模型或宿主agent。支持main与codex/cross-workspace-collaboration目标分支。

## 验收
- 固定执行现有G1-01/G1-02/G6/G8（源文件和完整title受测试固定），不复制业务断言；production Web、单浏览器worker、零重试。
- PG17/pgvector专有数据库与独立API/Web，先确认DB身份再迁移/fixture；不能默认连接/复用开发环境。显式synthetic legacy auth，禁用对外邮件/模型/插件配置。
- 缺失/skip/fail/timeout/重试过关/空报告/重复项目库存均失败；真实4个named pass才通过。
- 保存JSON报告、API/Web日志、源SHA与执行配置（无凭据）、失败trace/screenshot；保留首次失败。
- PR workflow和launcher/inventory验证通过；至少一次本地真实4路径运行，Hosted GitHub与真实模型明确未测。

无新依赖、无远程触发/推送、无生产操作。该子任务是确定性用户路径烟测，不是完整容量/恢复或模型行为评测。
