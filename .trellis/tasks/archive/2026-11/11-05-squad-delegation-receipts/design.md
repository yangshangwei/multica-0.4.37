# 设计
既有comment.go产出真实逐目标结果，CLI原样透传，Web已有警告处理。复用状态词汇，新增窄CLI结果提示helper；处理不可信JSON条目时不得panic，不输出目标名称或额外隐私。正常状态不警告；未知状态与畸形条目保守提示。旧服务端字段缺失不报命令失败；队长掌握预期目标，必须自己核对缺项。

动态squadOperatingProtocolHeader在delegation之后明确检查JSON回执；action评价仍是既有“acted/delegated”语义，失败目标记录failed/解释，部分成功明确结果，不改存储语义。Stop-after-dispatch要求处理回执后退出；不可产生额外评论循环。runtime mention段修正绝对“新run”说明，同步相关built-in skills/source-map。

API/DB无修改，不新增依赖。JSON响应保持原样和退出码0避免已提交写被调用方自动重试。回退此原子提交即可。
