# PAY-241 S3 执行与协作恢复

## 目标

证明协作在超时、失败、取消、断线与并发修改后可以恢复原意图，且不重复派发、评论、附件、结果或通知；恢复重新核对当前权限，保留原始失败与历史身份。

## 背景与边界

父任务为 `../../../11-13-pay-241-collaboration-acceptance`。用户已授权 S3 规划、实现、测试、本地提交和独立检查后归档。当前先开展不依赖 S2 职业规则的研究与验证准备；不会据此跳过 S2 或关闭父任务。已有 outbox、workflow operation、run/command history、Stop/proof/Resume 复用，不重建恢复机制。

## 要求与验收

- [x] R01 / TC14：真实启动受控进程并触发超时、失败及取消竞争，保留原 run/attempt/request 身份，明确终结证明。timeout-proof、受控运行 R03 和输出 R03 的来源及未变化宿主算法已在[最终检查](research/ordinary-ui-cas-root-final-check-20261119.md)核对；真实模型质量/账户调用另列，父 TC-14 保持其更大范围。
- [x] R02 / TC14：未知响应、重复请求、迟到结果和显式重试均不能重复派发或重复保存评论/附件/结果；新成功不改写旧失败。原 T/A/N/受控运行证据加上 [Agent 输出 R03](research/agent-output-runtime-r03-independent-check-20261119.md) 已独立接受本包所需受控合同：2 顶层/16 子场景、10 Provider/12 输出 CLI、10 Agent 文件/20 引用、4 已记录结果及零恢复启动；原 R01/R02 失败保留。真实模型质量仍属父任务独立范围。
- [x] R03 / TC15：双端各 55 条离线评论与 2 条对照，经两次实际断线/重认证后核对持久 ID、完整分页、顺序及最终界面；[批次独立验收](research/comment-batch-r01-independent-check-20261118.md)。
- [x] R04 / TC16：负责人和优先级同版本竞争明确拒绝冲突；协作 R06 与[普通 UI GREEN R02](research/ordinary-ui-cas-green-r02-independent-check-20261119.md)分别独立接受。普通双端两种顺序、原 R 的 200/409、败方字段保持和显式 R+1 续接通过；版本缺失/异构批量和旧无版本 API 保留兼容边界，未推定通用强制版本。
- [x] R05 / TC21：断线、排队和恢复期间撤权后，任务、附件、原运行和间接读取均不得泄漏受保护内容。（定义的当前路径已验收：Extras-A R05的人/来源撤权与独立参与权对照，加上[受控R03](research/controlled-runtime-r03-independent-check-20261119.md)四原运行的40授权读取/80原凭据精确拒绝、等待输入不消费、实际恢复和原证明。未来新增输出能力须验证自己的权限入口；完整S3仍开放。）
- [x] R06 / TC22：人员提及、Agent提及和订阅通知目标准确，以原事件身份去重；重放/重连不产生第二通知或执行。（定义的通知链路已验收：Extras-A人员/Agent提及、普通评论listener原事件去重、[普通订阅R05](research/ordinary-subscription-r05-independent-check-20261119.md)与[受控R03](research/controlled-runtime-r03-independent-check-20261119.md)的成功/失败/Stop原事件/收件人/通知/运行恢复。其他泛化事件、混合writer和自动ordinary composer幂等保持各自范围。）
- [x] R07 / TC23：阻塞原因、责任人、解除条件与预计解除时间完整；解除记录实际人、时间和恢复状态。[R08 双端验收与只读核账](research/block-runtime-r08-20261118.md)；不据此核销全 S3。
- [x] R08 / TC20：所有本包恢复动作有来源版本、当前身份与权威证据链接；失败保留，不能把未知写成未执行后重新发出。[最终检查](research/ordinary-ui-cas-root-final-check-20261119.md)核对既有动作清单及新 UI 原 R/冲突/fresh 意图；父 TC-20 的外部系统审计范围仍独立。
- [x] R09：独立新DB与源码匹配的API/production Web/Desktop验证，原执行gate恢复；原生Trellis[最终检查](research/ordinary-ui-cas-root-final-check-20261119.md)通过，21 整合路径、29 领域文件、95 宿主文件、43 输出文件及38项资源证据核对。主树 typecheck 9/9、lint 6/6；功能验收完成，归档仍等待历史所属代码提交整理。

## 不纳入通过结论

真实模型超时/质量、支付和履约幂等、生产发布/回滚、100协作者性能不由本包测试推断。S3局部断言只能推进父矩阵对应子断言，原24例未满足部分继续开放。
