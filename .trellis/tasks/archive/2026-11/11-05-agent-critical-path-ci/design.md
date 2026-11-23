# 设计
采用单独agent-critical-path.yml workflow与窄launcher，避免在现有CI中重复所有测试。PR main和integration分支以及相关server/web/packages/e2e/scripts/config变化触发，workflow自己的修改也触发；工作流提供独立命名status，branch protection需远程管理因此本地不宣称required gate配置。

复用scripts/dev-env.sh的environment registry、isolated resource allocation、production start/stop和health/commit ownership。允许CI服务PG runner临时库，但launcher必须明确expected DB并校验current_database。只为本smoke分配/启动资源，不读取用户.env或私密部署设置，不整仓check.sh。API和Web均需显式loopback URLs，legacy auth使用合成验证code，UPLOAD_DIR/task state在私有临时目录。

可把固定selection放独立Playwright config，引用原两份spec且grep四个完整title；使用既有chromium项目。JSON输出文件必须传入inventory validator。严格解析测试树，只接受四个固定case，预期状态expected，单次result passed/retry0，无skip/interrupted/unexpected/global errors；不能只看进程exit0或summary。

小而明确的launcher必须记录启动与清理归属，trap仅停自身pid/registry组件；真实失败目录保留，不自动抹掉。已有实际环境启动函数优先复用，不造第二部署平台。本地运行由主会话分配独立数据库/配置，child不得借用共享DB。

## 独立审查修订
launcher source dev-env.sh只复用底层端口分配/manifest/health/start/stop函数，不调用cmd_up/ensure_database（会改变预分配DB或启动共享容器）。主会话/CI service预分配确切数据库，launcher校验expected name/loopback并记录，之后直接migrate_database/start_api/start_web。环境由窄allowlist重建，拒绝仓库root/.env、.env.worktree和apps/web/.env*非example文件，防e2e/fixtures的独立env导入污染；不修改原fixtures。
固定完整title取自源：G1-01 assigning an agent queues one task that the daemon runs to a live completed reply；G1-02 a failed run notifies the creator, retries exactly once and can be stopped before it runs；G6 a plain comment re-engages the assignee, an @mention engages another agent and /note engages none；G8 a running task streams its transcript, leaves the issue status alone and stays stopped after a late completion。validator另验spec文件和chromium项目身份。生产build期间source需冻结，最后核对source/config一致；禁止把在其他源码上跑的结果归此提交。
