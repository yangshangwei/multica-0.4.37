# 内置指令中文转换

`localize_agent_instructions.py` 使用 Python 标准库和正常的 Multica API，按工作区盘点、转换或回滚智能体与小队的指令。不会直接修改数据库，也不会调用模型。默认只读，写入必须同时提供 `--write` 和新的 `--journal` 文件。

## 运行前提

- 目标 API 的所有实例已经升级到支持指令更新前提的版本。工具检查单个 GET 响应中的 `X-Multica-Instructions-Precondition: 1`；这不能替代滚动部署完成检查，因为旧服务可能忽略 PUT 中的未知字段。
- 旧服务可以先做只读盘点，但升级后应重新生成执行计划，获取完整的更新时间戳。已审阅的历史译文只能在原文摘要仍一致时复用。
- 使用能管理目标记录的已有 CLI 配置。工具不会生成或提升凭据；正常 API 继续检查所有者、工作区和智能体权限。列表只覆盖该凭据可见的活动记录；完整盘点应使用合适的工作区 owner/admin 账号并核对权限范围。
- 工作区 UUID 必须显式指定。计划绑定 API 地址及工作区，不能拿到另一个环境直接执行。
- 备份和日志含团队指令。保存在受控本地目录，工具以 `0600` 创建文件，不覆盖已有文件，也不要把这些资料提交到 Git。

## 生成计划

在已中文化的 checkout 中运行，`--baseline-ref` 指向翻译前英文模板所在的 Git 提交。此次基线为 `2a1b56735`。

```bash
python3 scripts/localize_agent_instructions.py plan \
  --config "$HOME/.multica/profiles/your-profile/config.json" \
  --workspace "00000000-0000-4000-8000-000000000001" \
  --baseline-ref 2a1b56735 \
  --file /private/tmp/multica-instructions/plan.json
```

生成文件包含每条记录的原文、SHA-256、完整更新时间、候选译文、原因和 `selected`。只有同时满足以下条件才自动选中：

1. 来源 key 匹配真实模板。
2. 实例版本、基线版本、目标版本相同。
3. 已保存正文与该英文基线全文一致，包含空格和换行。

版本相同但正文改过的记录不会选中；版本不同，即使名称相同也不会自动升级。已是当前中文正文的记录保持原样。Mika 的工作区补充单独保留，其产品系统段通过后端发布更新。

对于自定义或历史版本，逐项检查 `before`，翻译这份实际原文并审阅。确认后只修改对应条目的 `after` 和 `selected: true`，不要改 `before`、摘要或更新时间。保留旧职责、权限和行为版本；工具只写正文。

## 预演和执行

```bash
# 预演：只读取并检查，绝不发送 PUT。
python3 scripts/localize_agent_instructions.py apply \
  --config "$HOME/.multica/profiles/your-profile/config.json" \
  --workspace "00000000-0000-4000-8000-000000000001" \
  --file /private/tmp/multica-instructions/plan.json

# 实际写入：另建不可覆盖的日志。
python3 scripts/localize_agent_instructions.py apply \
  --config "$HOME/.multica/profiles/your-profile/config.json" \
  --workspace "00000000-0000-4000-8000-000000000001" \
  --file /private/tmp/multica-instructions/plan.json \
  --write --journal /private/tmp/multica-instructions/applied.jsonl
```

每次 PUT 只包含 `instructions`、`expected_instructions` 和 `expected_updated_at`。服务端在同一条 SQL 中比较工作区、原文及完整时间戳，冲突返回 `409`；写前再读只是预演辅助，不是并发保护本身。成功更新沿用正常事件通知和前端缓存刷新。

输出是计数：`ready`、`applied`、`preserved`、`already_matches`、`conflict`。冲突时退出码为 1，其他错误为 2；日志记录实际写入和具体冲突记录。重复执行遇到已匹配的正文不会再次更新。

## 回滚

```bash
# 默认仍然是只读预演。
python3 scripts/localize_agent_instructions.py rollback \
  --config "$HOME/.multica/profiles/your-profile/config.json" \
  --workspace "00000000-0000-4000-8000-000000000001" \
  --file /private/tmp/multica-instructions/applied.jsonl

# 确定回滚时增加这两个参数：
# --write --journal /private/tmp/multica-instructions/rolled-back.jsonl
```

回滚只处理有成功回执的写入，先校验原文摘要和写前备份与回执一致，再按“本次写入的正文 + 写后时间戳”作条件更新。之后有人改过的记录会冲突跳过，不会覆盖新内容。

每条写入前，工具都会把完整备份刷入日志。网络中断或响应无法确认时会停止，不再处理后续记录；此类 `attempt`/`uncertain` 记录需要先与服务器核对结果。工具拒绝自动回滚不确定记录，不能把“请求失败”当作“服务器没有写入”。

## 验证工具

```bash
python3 -m unittest discover -s scripts -p test_localize_agent_instructions.py
```

API 的并发、权限和事务回归位于 `server/internal/handler/instructions_precondition_test.go`，需要使用隔离测试数据库。默认测试不访问用户已登录的真实智能体。
