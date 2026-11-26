# 剩余验收阻塞续修报告

基线为本次实际拉取的main `9ffa37281184ae605bcbc0bc7a09caf3df60d180`。新隔离worktree `main-acceptance-root-causes`、分支 `codex/main-acceptance-root-causes`。旧worktree及失败数据库保持不变；原546项publication清单摘要逐项匹配。未提交、推送、合并或发布。

## 已证实的根因与修改

1. **知识测试跨用例处理历史任务**：新增回归在修改前证实fixture初始化会推进其他用例保留的扫描/提示。改为fixture事务保留已有pending行，由原生产SKIP LOCKED语义隔离，清理时回滚释放。没有删历史、改生产worker、放宽NOWAIT或权限。独立复核修正回归测试自身最后排空旧任务的问题；20顶层29命名测试通过，完整Linuxnormal4631PASS0FAIL0SKIP，耗时1009.99秒；race亦4631PASS/1891顶层/0FAIL/0SKIP，实际测试2066.422秒，无遗漏、原40分钟不变。启动器含冷编译耗时不混作测试超时；4个随机收件人动态名称差异单独记录，固定测试形状一致。历史55P03持锁者仍未知，这个fixture缺陷不是其已确认原因。

2. **旧CLI查询等待主要发生于宿主机/VM访问路径**：实测273查询客户端16.904秒，相关PG执行记录约0.674秒；同时简单SELECT经宿主机映射端口中位80.18ms，容器内同库0.016ms。关闭查询追踪/时长日志的完整原旅程也240秒失败，排除追踪为必要原因。将API移至现有默认Docker网络访问同一任务PG后，原10操作87.787秒PASS，所有权限/状态/历史/无副作用断言不变。查询次数是放大因素，未推断具体Docker实现缺陷、未扩大池或延时。独立复核接受，最初缺少后端来源记录的事实及后续完全相同API二进制重建补证一并保留。

3. **Host服务端尝试响应与客户端实际结果混淆**：产品日志新增request_context固定none/canceled/deadline，不改响应和级别。实际取消回归及完整DB+Redis middleware normal/race各143PASS0FAIL0SKIP，vet通过。完整order02/memory02/order03均通过，浏览器实际收到5xx为0；大部分服务端503对应浏览器取消，带安全诊断的memory7条/order33条未取消能力读取均返回collaboration_busy。其为原三次冲突事务策略拒绝，未放宽守卫；额外两个真实事务证明：持有同账号LockPasswordSession排他锁，能力读取三次在LockArtifactUsers遇到55P03后busy，释放后摘要一致成功；具体某条历史记录的持锁者仍未证明。历史320/116记录不被回填。

4. **默认知识ON契约**：新独立完整用例与原OFF共享原旅程，ON要求3次启动不同session/workdir且无resume/persistence。原OFF断言保留；当前main生产Web/Desktop构建成功且来源未变。真实MacON03完整56.379秒PASS：三次launch及Stop/重启/Resume/全部历史断言通过，来源/包未变、清理通过。原OFF完整57.117秒PASS，原会话/目录复用断言保留且通过；两次运行各自的协议基线恢复、客户端/API清理均通过。准备失败、错误选例产生的2FAIL及协议状态变动均单独保留。

5. **普通评论附件契约已明确**：用户选择保留成员可上传、上传者独立归属。规范确认现行行为；新正反handler回归normal/race各5PASS，包含原协作私有流程拒绝。没有修改权限实现，也不宣称历史观察被漏洞修复。

## 来源范围

service完整包使用自己的冻结候选。相对最终工作区，后续只新增middleware日志及其测试、独立handler附件契约测试；service实现与fixture未再变化。middleware完整143项normal/race和handler5项normal/race分别验证后续文件。API/生产UI/Native使用各自明确的来源和制品绑定，不能合称一次所有文件相同的全产品候选。

## 保留限制

- 原400PASS/138SKIP、117PASS/21FAIL、114PASS/24FAIL及旧11PASS/9BLOCKED/1FAIL映射不变。新映射为12PASS/9policyBLOCKED/0FAIL；其中11项旧PASS仍绑定旧来源/配置，不是当前main全产品验收。
- Namespace9项需已有安全策略允许实际非特权namespace控制器的Linux宿主；当前fixture的privileged/unconfined不获允许，未执行。
- 新真实匿名GitHubtree403且remaining0；在其声明reset之后的16:16:22Z新响应仍remaining0并给出下一窗口。未借个人token或增加导入重试；完整导入缺少当前匿名可用额度，历史缺头403仍未知。
- Linuxservice结果不能改写Darwin失败或当作Darwin完整运行。历史锁持有者、VM边界内部具体实现原因和历史启动许可失败仍有证据边界。
- macOS默认/注册集合多次出现来源未知变化，准备/每次运行重新捕获当时状态；仅恢复任务新增项到该次可观察基线，不重建消失旧项或重置外部默认。

## 验证与清理待办

完整service normal/race、MacON/OFF均完成；独立复核已完成源码、全部测试清单和最终资源核对，无剩余可操作代码发现。所有新API/子容器按owner核验后清理；4个基础/runner容器停止保留，26个新数据库保留，父级25个已登记端口关闭；独立复核补查CLI端口后共26个端口关闭，自有进程候选0。最终只读协议快照与MacOFF02该次基线一致；不追认整个任务最初状态。资源清单见artifacts/qa/main-root-causes-20261125/integration/global-resource-final-audit.json。

**发布建议仍为HOLD**：现有环境无法满足9个Namespace身份的安全运行前置，匿名公共GitHub完整导入缺额度；原Darwin经Docker数据库发布端口的整包未在该路径重新通过。当前受支持Linux整包、旧CLI与Mac知识ON/OFF的可验证范围已闭合，不将无法重建的历史持锁者视为一个新发现的产品缺陷，也不把这些分范围通过当作当前main全产品发布验收。

## 变更文件与简化

- `server/internal/service/collaboration_knowledge_invalidation_test.go`：删除全局历史排空初始化，复用现有SKIP LOCKED事务语义隔离fixture并验证释放。
- `server/internal/middleware/request_logger.go` / `request_logger_test.go`：仅新增固定上下文状态观察及回归，保留响应与日志级别，修复测试日志捕获的legacy logger恢复。
- `server/internal/handler/file_comment_membership_test.go`：用户确认契约的正反回归，复用testutil、现有multipart与storage工具，无权限实现变更。
- `e2e/collaboration-host-installed.spec.ts`：抽取同一完整旅程体，保留原OFF并增加独立默认ON的会话隔离断言。
- 三份server规范、原父子任务续修报告、精选证据和工作日志：记录精确行为、来源范围、失败保留及资源边界。

没有新产品依赖、权限缓存、连接池扩大、重试扩张、预算增长或安全策略降低。最终代码未提交；提交、推送、合并均等待用户后续明确指令。

独立最终复核：[root-causes-independent-continuation-review.md](root-causes-independent-continuation-review.md)；其原始审计记录包含来源限定、完整测试清单和最终资源观测。
