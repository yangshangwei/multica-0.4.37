# 内网 MCP 配置与运行准备

新增的七个内置模板直接启动智能体机器上已安装的程序，不使用 `npx`、`uvx` 或自动安装命令。API 服务器只保存配置，不会替智能体安装程序或探测内部服务。以下版本已核对上游说明和配置源码；真实服务连通性还取决于内网地址、账号权限、证书和运行环境。

## 本地程序

| 市场条目 | 核对版本 / 软件包 | 智能体 PATH 中的命令 |
| --- | --- | --- |
| GitLab MCP | `@zereight/mcp-gitlab` 2.1.69 | `zereight-mcp-gitlab` |
| MCP Atlassian | `mcp-atlassian` 0.23.1 | `mcp-atlassian` |
| Grafana MCP | Grafana MCP 2.0.0 | `mcp-grafana` |
| Kubernetes MCP | Kubernetes MCP Server **0.0.67** | `kubernetes-mcp-server` |
| MongoDB MCP | `mongodb-mcp-server` 3.0.5 | `mongodb-mcp-server` |
| Redis MCP | `redis-mcp-server` 0.5.1 | `redis-mcp-server` |
| ClickHouse MCP | `mcp-clickhouse` 0.7.0 | `mcp-clickhouse` |

安装准备在能获取安装材料的机器上完成，再将完整材料通过组织认可的方式传入内网：

- Node.js 程序需要兼容的 Node 版本及完整依赖目录。只复制 `npm pack` 的顶层包通常不够。准备环境与运行机器的操作系统、CPU 架构及原生依赖应匹配。
- Python 程序各用独立环境。准备包含全部传递依赖的 wheelhouse，在目标机器用 `pip --no-index --find-links` 安装；不要直接搬运不可重定位或跨系统的虚拟环境。
- Grafana、Kubernetes 使用对应平台的官方独立二进制，并按发行资料校验完整性。
- Kubernetes 使用 v0.0.67 配方中的 `--read-only` / `--kubeconfig` 参数。更新后的上游 main 已迁移到 TOML；更换版本前必须重新核验，不应直接替换成任意新版。
- 将程序加入智能体启动环境的 PATH，或者提供同名本地启动脚本。内网根 CA 加入相应信任链；模板不关闭 TLS 证书校验。

## 添加时填写什么

| 条目 | 配置输入 |
| --- | --- |
| GitLab | 完整 API 地址，如 `https://gitlab.internal/api/v4`；访问令牌 |
| Atlassian | Jira 地址与 PAT，或 Confluence 地址与 PAT；可同时配置，至少完整填写一组 |
| Grafana | 内网 Grafana 地址，可含部署子路径；服务账号令牌 |
| Kubernetes | 智能体机器上 kubeconfig 文件的绝对路径；其中引用的证书、密钥与 exec 认证插件也需预先准备 |
| MongoDB | `mongodb://` 或 `mongodb+srv://` 连接串；SRV 模式需要可用的内网 DNS |
| Redis | 不含凭证的 `redis://host:6379/0` 或 `rediss://host:6379/0`；用户名、密码单独填写 |
| ClickHouse | HTTP(S) 接口地址，如 `http://clickhouse.internal:8123` 或 `https://clickhouse.internal:8443`；用户名、密码；可选本地 CA 文件 |

ClickHouse 表单地址会由应用拆成固定的主机、端口和 TLS 环境变量，不是上游的连接串配置。省略端口时按 URL 标准使用 HTTP 80 / HTTPS 443；直接连接常用的 8123 / 8443 端口请明确填写。不要填写原生 TCP 接口 9000 / 9440。允许空密码，但运行配置仍会传入空密码值。

Redis 地址不接受凭证或查询参数，避免密码进入进程参数。需要额外 TLS 客户端证书等配置时，使用现有自定义配置入口；证书验证应保持开启。用户名、密码允许留空，按内部 Redis 的认证策略处理。

Redis 使用本地 ACL 认证时，启动环境及工作目录的 `.env` 中不得设置 `REDIS_ENTRAID_AUTH_FLOW`。该上游版本会将它解释为 Azure 认证选项；需要删除该变量，设为空字符串也不能禁用。模板会固定 TLS 证书验证为 `required`，避免继承关闭验证的设置。

凭证字段掩码显示，保存后不会从工作空间或智能体配置列表返回。配置保存不会自动分配智能体；需在下一步明确选择。

## 内网与权限行为

- GitLab：只读模式，关闭启动版本检查，固定 stdio。
- Atlassian：Server/Data Center 的 PAT 配置；只读模式。未配置的 Jira 或 Confluence 不要求额外账号。避免运行目录内其他 `.env` 文件覆盖上游 Jira/Confluence 环境变量。
- Grafana：关闭写工具、公网文档及匿名使用统计；仅保留内网常用的搜索、数据源、Dashboard、Prometheus、Loki、告警工具类别。
- Kubernetes：核对版本启用只读工具模式。集群 RBAC 仍应限制为实际需要的权限。
- MongoDB：启用只读，关闭遥测，禁用 Atlas、Atlas Local、在线助手及切换连接工具。
- Redis：在线文档服务地址被显式设为空，不发起文档 HTTP 请求。上游仍会列出该文档工具，调用时返回未配置提示。**Redis 没有 MCP 只读开关，需使用只读或受限 ACL 账号。**
- ClickHouse：只读，禁止写入和 DROP，关闭额外 chDB 功能。

只读工具筛选不能代替服务端账号授权。以上关闭项覆盖已核对版本中已知的公网功能；最终内网验收应在阻断公网的实际运行环境中执行。

## 验收

依次确认：本地程序可执行、MCP 初始化成功、工具列表正确、访问目标内网服务的代表性查询成功、越权写入被拒绝。使用测试资源完成验证，不向市场模板写入生产凭证或假连接成功标记。此功能不包含第三方软件包分发器，也不预置用户内部服务。

上游来源：

- [GitLab 2.1.69](https://github.com/zereight/gitlab-mcp/tree/v2.1.69)
- [Atlassian 0.23.1](https://github.com/sooperset/mcp-atlassian/tree/v0.23.1)
- [Grafana 2.0.0](https://github.com/grafana/mcp-grafana/tree/v2.0.0)
- [Kubernetes 0.0.67](https://github.com/containers/kubernetes-mcp-server/tree/v0.0.67)
- [MongoDB 3.0.5](https://github.com/mongodb-js/mongodb-mcp-server/tree/v3.0.5)
- [Redis 0.5.1](https://github.com/redis/mcp-redis/tree/0.5.1)
- [ClickHouse 0.7.0](https://github.com/ClickHouse/mcp-clickhouse/tree/v0.7.0)
