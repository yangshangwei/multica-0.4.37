# 隔离测试环境

本文件仅记录环境准备与只读就绪检查，未执行业务闭环测试。

- 来源提交：`4d454bb2c2c945664fe73947ecaccf053909d55a`，产品源码哈希见 `source-hashes.json`。
- 私有快照：`/Volumes/artisan/code/2026/qa-private-collaboration-20261109-bff677f2/source`。workspace 模块链接已验证指向此快照，没有指向原 checkout。
- 独立数据库：`multica_qa_20261109_4f1375f76bad`，创建前确认不存在，创建后通过 pg 和 psql 检查 `current_database()`。
- 迁移：全新库执行到 765，退出 0。
- 集群连接：准备时已用 38 / 100，API 池上限 8、下限 1。
- API：http://127.0.0.1:58765，PID 67745，password 认证、托管安装支持已验证。
- Web：http://127.0.0.1:58766，production build `3iIFVPCnhVORPEid8eW4f`，PID 67892。
- Renderer：http://127.0.0.1:58767，PID 67746，HTTP 正文与本次构建 index.html 一致。
- 运行时 API/WS 地址已在登录页 HTML 核对，详见 allocation.json。
- 三项标准工具和 closure/memory Provider 均从本次源码编译，哈希见 allocation.json。未使用商业模型账号。
- 新增主线测试 discovery 成功：1 条测试；尚未执行。
- 环境配置仅存在私有目录的 environment.json（0600），未复制到报告目录。

## 运行

从原仓库目录使用：

```sh
python3 .trellis/tasks/11-09-collaboration-multi-account-qa/research/environment/run.py --provider closure env QA_RUN_ID=run-01 pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --config playwright.qa.config.ts --workers=1 --retries=0
```

该 launcher 在私有快照中执行，用 JSON 数据加载明确配置，不载入原仓库 .env。`--provider host` 供 squad/switch 用例；`--provider memory` 供上下文专项。

每次完整重跑必须使用新的 QA_RUN_ID。Playwright 配置保留 JSON 结果、截图，关闭 trace 以避免记录认证值。根代理可以将本次新测试文件更新到快照，但不得改变已构建产品源文件而不重建并记录。

## 停止

```sh
python3 .trellis/tasks/11-09-collaboration-multi-account-qa/research/environment/services.py verify
python3 .trellis/tasks/11-09-collaboration-multi-account-qa/research/environment/services.py stop
```

stop 只对 allocation 记录且仍匹配进程组身份的进程发送 SIGTERM，保留数据库、日志与快照。停止后应再次检查端口释放与 fixture gate 恢复。

## 已知准备情况

- 首次 psql 前置检查误将连接串放在 PGDATABASE，检查拒绝，未执行迁移；随后使用明确 PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE，并验证期望库名后迁移成功。
- Web 构建退出 0，包含现有 CSS `::highlight` 优化器警告；不视为浏览器业务测试结果。
- 准备期间原仓库 HEAD 与此前规划读取的 HEAD 不同。本环境固定于上列实际提交与文件哈希，未 reset 或覆盖原目录；同步新测试时检查产品源码零漂移。
