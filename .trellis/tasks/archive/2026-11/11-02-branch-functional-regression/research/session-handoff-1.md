# 当前进度与续接计划

任务：11-02-branch-functional-regression，状态 in_progress。固定候选7af9ab9905c98c77bb091223ebe687c639475bb4，分支codex/cross-workspace-collaboration，候选版本0.6.8-rc.20261102。

## 已完成

- 439项E2E均有当前PASS证据；非Mobile单元13,082、Mobile196通过。原失败、skip及补跑分别保留，不声称一次全命令全绿。
- Go原库存最新10,376 PASS / 1 FAIL / 6 SKIP；后续诊断与定向回归单列，不增加原库存。
- Linux ARM64 DEB真实升级、新装、移除重装/purge共87项检查通过；macOS固定旧版到候选版文件替换、真实Main/preload/配置/偏好/localStorage保留子门通过。
- 两轮实际prepared计划诊断共14,000条SQL/参数核验：缓存复用及无统计重新准备均重复扫描排序，任务内ANALYZE后改用现有sequence索引反向读取，每条仅1事件。已区分旧缓存身份与统计/重规划方向，不证明原P95尾样本根因或需要新索引。
- 读取语义及现有产品race回归：12顶层、14子例全部PASS，覆盖生成typed Scan、非空/空/缺行/既有target、首次锁等待/RC后续新快照、服务端错误/Close/回滚及真实撤权/CAS/快照失败/任务保留。
- 原始证据独立审阅、最终文档独立审阅均PASS WITH NOTES。5,949个批前历史文件及439项库存完整，全部已有源码/测试改动保留。最新清理9个任务专库连接0，无自有进程、端口、挂载、容器或协议注册残留；该结论是上批清理时点证据。

## 保持开放

- QA-PERF-001：2171ms > 2000ms，FAIL。冻结标准、原失败及日志不变，未反复重跑到绿。
- 原生Linux桌面、Windows原生安装升级、iOS真实UI、Mac真实登录/凭据迁移与自动更新、正式签名/公证、真实商业Provider门仍开放。当前31行条件矩阵为21 PASS / 1 FAIL / 6 BLOCKED / 3 NOT_RUN，诊断PASS不替代完整验收。
- 小型协议/race证据不证明未来完整优化实现；typed解码故障、延迟Rows.Err、active-target reentry及完整集调度仍须随实际实现验证。
- 没有产品SQL/索引/依赖修改，无提交、远程派发、发布或商业调用。

## 下一会话顺序

1. 完整读取根CLAUDE.md及session-prompt.md，再读任务progress/report/defects/matrix、最新两份独立审阅和cached-final-verification-1.json。核对Git/冻结测试/产品及测试差异；变化就记录新身份。
2. 先列本批可执行计划。性能优先检查是否存在新的真实症状环境/原库只读证据；当前统计不能倒推历史统计，旧计划不能冒充原尾样本。没有新增判别条件不再重复相同夹具或P95。
3. 按remaining-platform-handoff-1.md检查剩余门实际条件；具备条件且已有授权的项目直接执行，缺条件准确保留，不用容器/合成账号/普通签名替代原生桌面/真实认证/发行信任。
4. 产品优化仍需明确新增范围。可能的最小边界及回归标准已写入perf-cached-reprepare-diagnosis-1.md和read-semantics-diagnosis-1.md；不能将继续验收解释为改产品、加索引或依赖的授权。
5. 新验证用新run/任务专库/可回滚夹具，按Trellis派发边界明确的原生子代理；429或空消息保留记录并按实际落盘结果处理。协调者复核与独立审阅分开。结束时更新任务文档并重新核验自有资源清理，必需门未关闭则保持in_progress。

## 最新入口

- session-prompt.md：完整续接约束、环境和历史入口。
- research/perf-cached-plan-diagnosis-1.md、perf-cached-reprepare-diagnosis-1.md、read-semantics-diagnosis-1.md。
- research/cached-semantics-evidence-review-1.md/.json、cached-final-coordination-review-1.md/.json。
- research/cached-final-verification-1.json；evidence/next-cached-cleanup-2/run.json。
- research/remaining-platform-handoff-1.md；conditional-matrix.csv。

本次交接整理只读取Git和已有证据并更新文档；未启动测试、数据库、应用或新资源，也未把旧清理当作当前新测。
