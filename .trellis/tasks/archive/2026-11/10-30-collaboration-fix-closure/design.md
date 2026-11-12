# 首轮协作修复交付闭环设计

## 目标和边界

在不改变协作权限模型的前提下，把集成分支的 8 个修复收尾为一个可审计、可回退的最终 HEAD。首轮只处理已证实的附件预算、迁移命名、恢复文案/界面证据和实验入口/记录问题；不实现新的 handoff envelope、评论分页或知识候选。

## 基线

- 集成修复链：7bb077987、e6ed3d1c3、fd3836da2、d84f217a9、b34701756、0ef4c7858、4c2140b54、4b0cbfde7，当前集成 HEAD 4b0cbfde7。
- 当前主分支另有 85b98c3bb，涉及 core collaboration action timeout；实施分支须保留它并再合入 8 个修复。
- 主工作区的未提交改动不属于本任务，实施使用独立 worktree。

## 方案

1. 迁移：若 745_owner_reconsent 尚未发布，分配下一个空闲数字；运行 migration lint、空库/已有库升级、重复执行和 sqlc 漂移检查。已发布迁移禁止重命名。
2. 附件：选择器接收具体 runtime limits。全部引用先验证且必须保留；引用超限返回 runtime_preparation_limits。未引用历史按既有新到旧顺序逐个尝试，超出可选预算就省略。promotion、claim、freeze、never-launched refusal 共用选择器。
3. B/C 收尾：恢复文案遵循 server can_consent 和 Switch-only 原因；接收方 handoff e2e 增加正文/请求人可见断言，组件测试只补正常 status select 值；保留现有 role/step、authorizer、local-link、escaped-comment 回归。
4. 验证：所有命令记录 HEAD、环境、数据库、测试名和退出码；DB 测试必须确认实际测试运行而非不可达静默退出。
5. 实验：先在 fake provider 下硬化未提交实验框架，再运行正式 TestHostProviderAccountSmoke，最后按 E1→E6 串行运行；环境故障与产品/模型失败分开。

## 回退

迁移错误停止后续工作；未发布迁移可恢复原文件并重新编号。附件选择器通过 feature flag/回退分支关闭可选补位，但不能删除引用。实验只保存证据，不改生产数据。
