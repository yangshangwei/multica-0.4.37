# Extras-A 附件抽屉修正 R05

当前结论为 **STATIC_AUTHOR_GREEN_ONLY**。实际 API／production Web／Desktop 的新 R05 测试体尚未执行，完整 S3 继续开放。

主会话 `/root` 授予 `/root/s3_extras_drawer_implement` 唯一 fixture 写权限：`e2e/fixtures/collaboration-recovery-extras.ts`。其他任务 WIP、Extras spec、共享 recovery helper、产品代码、配置、数据库、构建和进程未改动。

## 原失败与修正

R04 完整六顶层运行有 2 PASS／1 FAIL／3 NOT_RUN。Web 通知和上传顶层通过；Web 撤权的 `participant_last_basis_offline_and_delayed_read`、`source_only_membership_loss` 两子例通过，`source_loss_participation_survives` 失败。未将不同轮次通过项合并。

真实失败截图 `web-extras-revocations-failure-3.png` 显示 Attachments 资源抽屉仍打开，Comments 位于模态背景；原 fixture 在关闭文件预览后直接调用背景 Preview delivery，等待 15 秒超时。当前 `detail-auxiliary.tsx` 使用独立的资源 Dialog 和 Close resources；文件预览是另一 Dialog。根 E2E 规范要求背景操作前真实关闭抽屉并等待隐藏。

唯一代码差异为一个旧关闭表达式替换成六行：取得并关闭实际文件预览、等待该预览隐藏；取得实际 Attachments Dialog、点击 Close resources、等待该抽屉隐藏。随后沿用原 preview／Send comment、持久评论、权限、原 native capability 拒绝及新 capability 身份断言。没有新 helper、强制点击、睡眠、时间预算增加或业务断言删除。

R04 的 fixture、spec、共享 helper、原始报告、截图及数据库证据均保留；R05 不追溯改变原失败。

## 静态验证

私有 coherent harness 来自不可变 `harness-extrasA-r04`，只复制修正后的 fixture；`node_modules` 和相对 `packages` 均指向冻结的 `source-r01`。243 个 harness 文件在检查前后字节／模式一致。新静态 runner 通过既有 no-DB wrapper 和封闭 `nodb-client-tests-r01.json` 配置运行，不继承环境；NODE_ENV=test，DATABASE_URL／MULTICA_TOKEN 不存在。

- 严格 TypeScript：exit0，解析 638 个源条目，完整源图保留；ROOT 产品包／ROOT E2E 引用均为零。
- ESLint：实际两文件、no-ignore、max-warnings0，exit0，零错误／警告／消息。
- Playwright --list：exit0，实际六顶层，源码每顶层三个子例，共18个 planned subcases，retry0。
- 三命令完整源码与工具／配置 before-after snapshot 均零漂移。命令、PID、cwd、工具和配置 SHA、原始 stdout／stderr 留存。
- 新 API／浏览器／Provider 测试体、数据库、构建或进程操作：0。

私有证据目录：`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/verification/extras-a-drawer-author-r05`。相邻 JSON 封存源码、patch、harness manifest、解析图、静态报告和原失败身份。

## 后续验收

主会话分配新 R05 专库和 API 配置，将本 fixture 与匹配的现有 production Web／Desktop 绑定，重新完整运行六顶层／十八子例。仍需核对真实持久身份、原 gate、连接和进程恢复，再独立检查、更新矩阵与提交。此作者静态检查不核销任何 S3 业务要求或父矩阵条目。
