# M1-C1 有效委派即时回执消费

## 目标
使队长与CLI明确消费已有comment API的真实trigger_outcomes，不把评论保存成功当成目标已派发。不另建队列、不改变业务验收。

## 验收
1. 队长在JSON响应中逐一按预期 target_type/target_id 对照。queued/coalesced/deferred仅代表已被接纳；blocked/missing/unknown不能声称委派成功。
2. CLI comment add/update保持完整stdout JSON、成功写入退出语义，针对blocked/unknown/畸形回执在stderr提示；正常旧服务端无字段仍兼容。只执行一次写，不自动重试/重复派发。
3. 部分成功明确显示；失去响应要求核对已有评论，不能盲重发。保持每轮单评论、人工done、owner/guest状态规则。
4. fake HTTP CLI回归 + 注入协议回归保留失败/通过；独立审查，无模型/生产/远程操作。

## 不包含
持久回执、幂等普通comment POST与server-derived squad evaluation另立子任务；本交付只证明工具输出和队长指令合同，不证明真实模型一定遵从。
