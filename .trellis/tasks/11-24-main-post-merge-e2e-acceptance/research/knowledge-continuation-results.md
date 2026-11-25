# 知识自动化专项续测

候选：`103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。当前状态：**knowledge-02 实际 Web、Desktop 两条原始链路均 PASS，运行与清理退出0**。确定性模型与真实 production daemon 的知识流程已执行；完整 production Main、商用模型或安装升级不在这两条用例的证明范围内。

## 可执行性与准确边界

当前源码已提供所有必要 fixture：`project-knowledge-model.mjs` 是只监听 loopback 的确定性模型网关，支持 hold/release 与生成、检查两个阶段；`project-knowledge-provider.py` 是捕获真实 production daemon stdin 的假 Claude 协议进程。它们不代理外部账号或模型。当前 campaign 的真实 CLI、API、生产 Web 和 Desktop renderer/preload 已有匹配基线的构建与摘要，无须下载依赖或重建不同候选。

沿用 `e2e/project-knowledge-automation.spec.ts` 的原始 Web、Desktop 两个用例及各 480 秒期限、单 worker、零 retry。它们验证 UI 正式接受→真实后台生成/检查→自动保存候选→生产 daemon 接收准确知识输入与 v2 proof→重新打开后旧 session 拒绝→重新接受后同一候选恢复并绑定新 proof；另外验证无项目和 manual-policy 接受不创建自动工作、候选或模型调用。

该专项使用 **password、managed installations、knowledge context 开启**，但协作 execution 与 host execution gates 均关闭、creation 开启。这是普通 Agent consumer 路径与人工交付的前置条件，不是把 host control 开关当成知识开关。报告必须分别记录这些状态。

## 新准备与隔离

新私有 runner 位于 `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/knowledge-acceptance/run_knowledge.py`。它复用已独立受审的 runtime 来源、工具、数据库资源、SQL、生产 Web 与 renderer 启动/终止 helpers，以及既有后代进程监视器；没有修改原 runtime/supplement/native/performance 代码。

专门的新数据库名为 `mca_knowledge_browser_7df2120a1b5b_knowledge_02`。名称由本 campaign 实际 runtime owner 与新 run label 派生，用以合法满足当前 fixture 的 `mca_knowledge_browser_*` 守卫。已在明确自有的 PG 上克隆全新库，并在同一连接实际核对 `current_database()`；没有借用旧库、旧 task metadata、旧 allocation 或旧通过结果。

使用本任务实际 active path 生成新的 detail allocation/provenance，绑定真实 API/Web/renderer listener PID、候选提交、源码摘要、Web BUILD_ID、渲染 API/WS URL 与原始 artifact bytes。API 发布的 `daemon_app_url` 必须精确指向实际 Web。所有配置、HOME、temp、uploads、daemon workspaces、provider/model captures 都落在 Git 外0700路径；敏感文件保持私有，只导出摘要。

26 个 provider path 由当前源码枚举，全部使用绝对路径。仅 Claude 指向与当前源码字节相同的假 provider，其他25个必须不存在。假 provider 的 `python3` shebang 通过任务私有 symlink 固定到已记录的 Python；生产 daemon 命令仍包在 Agent CLI sentinel 保护下。每次启动和最终收尾都核对这些路径。真实用户 HOME、认证配置和模型账号不进入运行环境。

实际 managed 账号创建、公开 onboarding、PAT 签发、enrollment/handoff 和 `.desktop-user-id` 均由原始 fixture 执行并断言，runner 不伪造 marker 或绕过身份流程。

## 审查与证据

`knowledge-02/inputs.json` 与私有 `knowledge-02-review-template.json` 对应输入摘要：

`5f2bab9197418e13e607614a0578edb0ba7bae5e4762aa8b139467b02bf960d3`

初始模板明确是 `REVIEW_REQUIRED`。父会话已完成独立源码/测试/实际绑定检查，保存 `knowledge-02/root-review.json` 与私有 accepted review；随后取得独占服务时段并实际运行：

```sh
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/knowledge-acceptance/run_knowledge.py run --label knowledge-02
```

此命令服从原 runtime 的排他 lock，并拒绝正在运行的 supplement；不与性能或其他 API lane 并发。它仅克隆一个本任务全新数据库，不创建或停止共享 PG 容器。

预期实际输出包括原始 collection/Playwright、具名 inventory、当前 allocation、启动命令、API config/health、详细 journey captures、源码终审、私有 model/provider/profile/config 摘要清单和统一 cleanup/completion。失败数据库、原始用例错误和未运行身份都保留。清理覆盖 API/Web/renderer/model、实际 daemon/provider/浏览器后代、原 gate booleans/monotonic epoch、监听端口和数据库连接。

`knowledge-01` 只做过初次准备，未执行 SQL、服务或用例。增加 Web publication 和私有捕获摘要检查后使用全新 knowledge-02；初次 inputs 与其精确旧 runner bytes 已保留，并重新核对旧源码摘要匹配。

## 已完成验证

证据：`artifacts/qa/main-post-merge-e2e-20261124-080504/knowledge-acceptance/preparation-02.json`。

- 4 个纯离线测试 PASS，覆盖合法前缀、非法 task/owner、review input漂移和外部模型配置被本地 fixture覆盖。
- 实际 runtime inputs、原 clearance、新 extra bindings、fake/provider/interpreter bindings 全部一致。
- 两个 Python 文件 AST 与 tabnanny 均通过。没有已安装的 Python 类型检查器声明；没有因为私有 Python 准备重跑既有产品 lint/typecheck。
- 26 provider路径均已准备；SQL执行0、服务启动0、产品测试体0。

## 实际执行与最终核对

两条原始用例一次完成，共55.1秒，**2 PASS、0 FAIL、0 SKIP、0 retry、0 global error、0 missing/unexpected identity**。`completion.json` 与全部绑定JSON摘要已重新核对，最终 source fingerprint 与 campaign 初始一致。

两个平台分别形成完整 `journey-evidence.json`：

- 第一次真实生产 daemon任务状态completed，knowledge protocol version为2，准确候选内容与receipt进入实际假Claude stdin。
- 重新打开后候选paused_review，旧continuation失败且provider调用数不增加。
- 重新接受恢复同一候选，新实际任务completed；validity revision均由1增至2，绑定新submission/acceptance proof，session与旧执行不同。
- 每个平台5个确定性模型调用。无项目和manual-policy对照均为0个自动job/候选，模型调用数不变；实际workspace知识快照不变。
- 每个平台4个原始关键状态capture；Desktop使用原生renderer/preload与fixture Main，未借此声称完整包Main通过。

统一清理记录为PASS：API、生产Web、renderer、模型四个端口62784–62787均关闭，数据库客户端0，实际 daemon/provider/浏览器后代无剩余、无fallback强杀、无monitor error。原gate仍为creation=true、execution=false、host=false、epoch=2。26个provider overrides、fake/interpreter、产品源码、工具与build均无漂移。原始数据库和私有capture/profile/log继续保留，公共报告只保留私有文件摘要。

证据根为 `artifacts/qa/main-post-merge-e2e-20261124-080504/knowledge-acceptance/knowledge-02/`，重点文件为 `allocation.json`、`collection.json`、`playwright.json`、`named-inventory.json`、`cleanup.json`、`private-evidence-bindings.json`、`source-end.json`、`completion.json` 与两个 `results/*/journey-evidence.json`。

服务排他锁已释放，并明确交还runtime负责人继续安全与其余补测。这只关闭 `project-knowledge-automation.spec.ts` 两个原始跳过身份；知识startup flag的on/off实际Host输入仍是另一独立场景。
