# 首批共用执行约束（本批已执行）

以下保留规划时的运行与保留约束；#37、#38 已完成，[首批交付记录](./research/first-batch-20261119.md)列出实际验收范围。

1. 得到对本方案的明确确认后才启动子任务，先 #37 再 #38；父任务不代两项实现。逐项执行子任务的 RED → 最小修改 → GREEN。若涉及同一详情/路由文件，保留两项各自的红样本，合并后同时复验。若当前原用例转绿，先固定原交互顺序再定性，不为制造失败改产品；不可复现就保留为未证实。
2. 使用任务私有的源码/构建目录（含所需的当前未提交变更）、全新且有唯一运行号的本机 PostgreSQL 库、未占用的 API/Web 端口和独立 HOME/上传目录。按 `.trellis/spec/web/frontend/e2e-run-environment.md` 使用匹配源码的 API 和生产构建的 Web，核对实际提供的 CSS；不要在样式扫描器忽略的目录构建。两条原用例用邮箱验证码的 legacy 环境。显式绑定 `DATABASE_URL`、`NEXT_PUBLIC_API_URL`、`PLAYWRIGHT_BASE_URL`，交叉核对分配记录、`SELECT current_database()`、监听进程、`/readyz` 和 `/api/config`；任一不符不开跑。不能从默认端口或共享 `.env` 猜环境。
3. 新建私有 `.artifacts/e2e-fix-<唯一运行号>/`，按编号、red/green、原用例/整文件分目录。记录 HEAD、运行输入文件指纹、构建标识、认证模式、测试标题、退出码和 trace 路径，不在共享文档记录令牌。以下是确认后的命令模板；`$RUN` 是该次全新子目录的绝对路径，`$SPEC`、筛选名见子任务。先加 `--grep` 按原标题运行，最后不加过滤整文件串行复验：

   ```sh
   PLAYWRIGHT_JSON_OUTPUT_NAME="$RUN/report.json" node node_modules/@playwright/test/cli.js test "$SPEC" \
     --project=chromium --workers=1 --retries=0 --reporter=line,json --trace=on --output="$RUN/results"
   ```

4. 做受影响包的 Vitest、lint、typecheck；如改平台接线，验证实际受影响的平台。原两文件和定向回归优先，不重跑全部 52 条。前置失败时分别记环境/收集错误与未到达的断言，不能记通过。不操作 `qa-retained-1118`、`KEEP-1`、旧 `.artifacts/e2e-20261118-203130*`、原 trace、数据库导出或备份；不运行旧报告目录的启动/清理脚本，不在共享检出执行 `make up/down/destroy`、数据库重置或 git 清理。只结束本次自建进程，保留红绿证据。

每个编号的交付记录包括：新旧 trace 对应节点、当前根因（或未证实）、实际改动的测试/产品路径、各阶段结果与未跑项。只回退本任务的补丁；若目标文件有并行修改，先核对再整合。
