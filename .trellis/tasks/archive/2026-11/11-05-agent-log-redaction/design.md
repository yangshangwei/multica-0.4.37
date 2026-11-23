# 设计

行为位于 pkg/redact 与 daemon 工具结果上报调用。保持 InputMap/Text 公共签名；私有敏感字段谓词统一 map 与 JSON key 行为，按完整词/明确后缀匹配，不以 contains(token) 扩大。

JSON 使用有界、保留原文的成员 value 区间替换；优先标准 encoding/json 解码 key/RawMessage 确定 value，避免重写整个文档与数字。敏感 value 超预算/截断时遮蔽剩余敏感片段，不将原文作为 fallback。正常文本仍用既有 patterns。实现需保持线性或有明确预算、最大深度与无输入修改。

daemon.go 现有 toolOutputPreview(msg.Output) 在唯一工具结果路径改为先 redact.Text；不新增平行 preview 抽象。纯 preview 函数继续只负责 UTF8/长度；集成测试通过 fake backend 验证真实调用顺序。

凭据格式：jwt.go 的 mul/mdt/mat +40 lowercase hex；plugin_token.go mpi/mpc +43 base64url；mip_ 前缀 JWT。边界不得吞掉分隔符而漏相邻 token。mcn_ 无权威长度暂不补猜测规则。

文件：server/pkg/redact/redact.go、redact_test.go；server/internal/daemon/daemon.go、tool_output_preview_test.go（可按既有 fake helper 适当放相邻测试）。风险：过度遮蔽、扫描耗时、跨截断泄露。回滚：独立提交；旧日志不自动重写，持续泄露风险需单独处理。

验证发现 pkg/agent/codex_test.go 的 TestCodexRawItemMCPToolCall 固定旧厂商占位符；本批允许仅同步该断言为字段语义占位符，并保留秘密未出现/原输入不变检查。Provider 产品代码不变；原失败记录保留。
