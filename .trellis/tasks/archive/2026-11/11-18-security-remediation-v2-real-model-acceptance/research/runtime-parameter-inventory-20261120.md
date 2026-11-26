# 首次真实调用前的参数核对

用户2026-11-20明确授权Codex真实验收；不再询问是否允许。静态读取主机Codex配置白名单字段得到候选：

- model `api666-zzzcoding/gpt-6-astra`
- provider `magpie`
- reasoning effort `max`
- 配置位置 `~/.codex/config.toml`；当前无额外CODEX_HOME；没有profile。
- provider配置含inline bearer来源，未输出或复制其值；未从中获得可确认的付费账号身份。不能据默认模型或存在凭据推断本次预算授权。

任务原记录和本轮消息都没有最大调用次数、最长总时长和单次超时；不能借用其他任务历史额度。首次真实调用前一次性补齐：精确模型/推理档位、可用账号来源别名（无需发送密钥，可指定当前magpie配置）、最大物理模型执行次数、总运行时限和单次超时。正常四例路径估算至少31次执行，只作预算参考；app-server内API请求另计，用户若按请求数限制需再绑定网关请求上限。

本轮本地无模型Codex进程（help/schema/sandbox/app-server）与真实模型执行分开：没有turn/start，没有供应商请求，账号访问0；本地进程数见各探针原始记录，不冒称未运行Codex程序。真实注册仍被关闭，实际平台broker/完整驱动与最终隔离证据须先完成。
