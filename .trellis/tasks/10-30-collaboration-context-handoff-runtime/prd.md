# 可靠上下文交接与召回

## Goal
把停止/撤权保留、成员/队长接力、评论分页和权限可信度从模型自觉提升为服务端可审计的结构化输入。

## In scope
bounded checkpoints、versioned handoff envelope、reader/result/wakeup provenance、comment cursor/byte budget/truncation、prompt trust boundary、Web/Desktop evidence。

## Out of scope
自动知识库、服务端总结模型、技能层写入、跨 Server 和 Mobile。

## Acceptance
接手 Agent 可从 envelope 复述当前 handler、意图、决定/理由、member results、next step 和证据；权限失效后的缺失范围可见；分页截断显式；非 SubmitCommand 评论不驱动执行；旧客户端兼容。
