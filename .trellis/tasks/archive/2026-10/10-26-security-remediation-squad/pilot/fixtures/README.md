# 合成安全试点实验室

这些文件全部为本地编写的合成材料，不对应真实包、公告、账号或客户数据。仅需 Python 3.10+ 标准库；无需安装依赖、启动服务或联网。`artifacts.invalid` 是占位地址，不应访问。

## 基线

从本目录运行：

```bash
python3 -B validate_fixture.py
python3 -B -m unittest -v test_dependency test_authorization
```

基线实测：结构校验退出码为 `0`；契约测试运行 `6` 项，其中 `4` 项通过、`2` 项失败，退出码为 `1`。失败项有意保留：

- `test_label_over_limit_is_rejected`：锁定的虚构依赖未拒绝超过八个字符的标签。
- `test_other_workspace_is_denied_before_read_audit`：另一工作空间的合成记录被返回，并产生了读取审计。

不能删除或放宽这两项断言来获得通过。它们是后续修复的失败基线，不是已完成的安全验收。

## 案例材料

| 案例 | 入口 | 验证目标 |
|---|---|---|
| 依赖公告 | `advisory.json`、`dependency.lock.json`、`application.py` | 确认版本和实际调用路径；一次只处理 `pilot-labelkit`；保留应用的字符串返回契约 |
| 代码授权 | `workspace_reader.py`、`test_authorization.py` | 当前工作空间可读；其他工作空间不可读且不能产生读取审计；不存在的记录无副作用 |
| 供应链缺证 | `supply-chain.json` | 来源摘要、生产者证明和组件清单缺失时保留 `hold/unknown`，交给 `pilot-build-owner` 补证 |

`vendor/` 提供虚构包的两个本地版本。公告明确记录了固定版本的返回字段变化，必须让迁移审查员评估调用方兼容性和回退限制。文件摘要只能验证本地字节一致，不能证明生产者可信，也不能代替供应链证明。

结构校验只检查合成标识、本地依赖摘要、锁文件一致性、Python 语法及缺证案例仍然保持阻塞。它不执行网络或构建脚本，也不授予安全结论。

## 运行与证据边界

只复制本目录到专用非生产容器、虚拟机或专用操作系统用户的独立仓库；不挂载个人 HOME、生产凭据或产品数据库。目录复制和 Python 命令本身不建立操作系统沙箱。

`baseline-source-sha256.json` 记录初始文件字节，供审查前后对比。实际试点另行记录仓库 ref、角色/skill 版本、运行身份、任务评论、activity、执行结果及独立 QA。实现者应在试点副本中修改必要文件；本目录保留原始失败基线。

独立 QA 应从最终补丁副本重跑同样的结构校验和契约测试，并分别回读锁文件来源、依赖调用契约和跨工作空间拒绝结果。依赖与代码两例修复后契约测试必须全部通过；供应链案例即使测试通过也仍为 `hold/unknown`，直到其 owner 提供可审查证据。任何生产事件只演练人工上报和 incident/release 交接。
