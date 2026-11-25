# 验收执行清单

本清单用于本地测试与证据交付，产品基线不改。测试计划为 `test-plan.md`，来源/隔离契约为 `design.md`。每次命令使用本次allocation选择的环境；不直接source未知dotenv，不输出凭据。

2026-11-25 续接按 `continuation-plan.md` 推进。勾选表示该执行/对账工作完成，不表示产品通过；失败与平台缺口以 `report.md` 和最新覆盖快照为准。

## 1. 计划、研究与准入

- [x] 创建任务并落PRD，确认main基线和无tracked修改。
- [x] 研究Web现有覆盖、后端合并风险、平台执行条件。
- [x] 写全面T01–T25与L0–L10测试计划、design、执行清单（独立复核补充Chat领域）。
- [x] 审查计划/上下文，准备独立任务QA目录与运行状态目录。
- [x] 保存源码/工具/测试collection清单；核对无test.only/fixme和未声明side effects。
- [x] 若复用私有runner，复制其源代码到新目录并重新生成tools/source/inventory/secure-temp；独立review其hash、配置和资源clearance。
- [x] 配置真实本机psql路径、浏览器cache、可信临时目录与PG连接/磁盘预算。
- [x] 验证Trellis jsonl上下文并激活本任务。用户已明确要求新建、计划优先和完整本地验证；不扩展到付费模型/部署或产品改造。

## 2. 静态、单元与后端集成

下列是规范入口，实际编排可将其分阶段执行以保留各自完整退出码；总入口 `make check ENV_FILE=<本次私有配置>` 需通过符合spec的单包DB/环境适配后执行。

```sh
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm check:ui-exports
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2
pnpm --filter @multica/mobile typecheck
pnpm --filter @multica/mobile lint
pnpm --filter @multica/mobile test
bash scripts/test-go.test.sh
bash scripts/dev-env.test.sh
bash scripts/check.test.sh
```

- [x] L0非mobile lint/typecheck/UI exports和TS测试，保存raw logs/完整suite summary。
- [x] 独立mobile gates；root脚本排除mobile的事实写入账本。
- [x] 运行现有script/node regressions，覆盖安装/配置/CLI保护与workflow契约；4 个 Helm/PowerShell 平台入口保留 NOT_RUN。
- [x] 每个Go package由adapter分配新库/HOME，调用受保护 `scripts/test-go.sh --race` 或等价逐包原选择，不执行真实Agent integration tag。
- [x] 保存 `go test -json -count=1` 原始事件与stderr，并对照实际预登记包/测试清单。
- [x] `go -C server vet -p 2 ./...`；关键snooze/collaboration timestamp在UTC与非UTC配置回归，仍用新分配DB。
- [x] 将fail/skip/未启动包分开；早期gate失败不能使下游计为pass。Linux 条件跳过的本地补测另列后续队列。

## 3. 生产构建与L1默认浏览器

- [x] 构建当前Server/CLI、Desktop与production Web；docs production build单独记录。
- [x] 只启动本次API/Web/native renderer，验证health/config/commit/pid/build/实际APIURL/CORS/WS/served index。
- [x] 四条canonical agent critical-path 均在默认完整套件中首次通过；来源已对账，不宣称独立 smoke launcher 或 CI 执行。
- [x] 保存实际default Playwright collection和条件skip inventory。
- [x] 运行全量L1，workers=1/retries=0，JSON/HTML与失败trace/screenshot写新路径。

```sh
pnpm exec playwright test --list --reporter=json
pnpm exec playwright test --workers=1 --retries=0 --reporter=line,json,html
```

## 4. 认证与功能配置补充

- [x] L2 password：真实login/registration/questionnaire/change-password/binding/switch，独立actors和兼容spec选择。
- [x] L3 admin/managed/resource/observability/switches：真实synthetic admin身份与signed enrollment；资源发布原用例失败保留。
- [x] L4 device-auth：独立兼容API，实际device login/恢复。
- [x] 对default实际138个skip逐项映射；最近补测117PASS/21FAIL，未执行0；原SKIP和所有历史失败保留。

password基础文件选择示例，必须先验实际mode/config：

```sh
pnpm exec playwright test e2e/password-registration.spec.ts e2e/password-branches.spec.ts e2e/password-migration.spec.ts e2e/password-binding-desktop.spec.ts e2e/password-server-switch.spec.ts --workers=1 --retries=0 --reporter=line,json,html
```

## 5. 执行、知识与完整客户端

- [x] L5 namespace-only：compile/hash fixture binaries、验证gate与allocation，执行stage/多账号/控制/恢复；9个namespace浏览器身份在ready前置失败，后续断言未验证。
- [x] L6 host-enabled：执行/stop/超时/重连/晚到输出/immutable outcomes/知识launch与lease场景已执行；memory/order两个启动许可失败保留。
- [x] runtime-output采用当前源码＋真实历史归档task的全新compliant allocation，明确关联本任务；Web/Desktop 两项通过。
- [x] L7启动前快照NSWorkspace原multica协议默认应用精确URL和同bundle注册URL集合；只用task-owned稳定app安装路径，独立确认恢复/注销准入。
- [x] L7 macOS/Linux完整productionMain/installed实际启动登录并hash绑定，两端旧标题断言FAIL；renderer与完整Main分开，后续流程缺口保留，协议恢复通过。
- [x] CLI/daemon普通测试使用fake executable；真实账号/计费模型不启动；旧CLI03实际执行后原240秒超时FAIL、一次兜底清理FAIL保留。
- [x] 已安装额外browser的聚焦smoke；Firefox/WebKit 未安装，已登记缺口，不加新依赖。
- [x] mobile原生、Windows、签名/updater按实际条件给单独状态。

## 6. 补充风险与运营

- [x] 合法权限、撤权、跨工作区缓存/WS、原key丢响应、CSRF/Origin等已执行并对账；评论越权FAIL，Desktop延迟读取定位器FAIL及未执行后续明确保留。
- [x] empty/longUnicode/paging/partial batch/timezone/expired revision/old-enum malformed-response等按实际命名身份与所属层次对账。
- [x] 中英文/明暗/keyboard/narrow/zoom/scroll关键链路保存capture与原断言，失败不计通过，不宣称覆盖所有界面。
- [x] docs/plugin/iframe/selfhost/offline/installer本地可运行测试与生产构建已执行；未进行独立live docs浏览旅程，平台条件缺口保留。
- [x] L10具备完整manifest/100-user条件时顺序prepare→propagation→writeback；三个阶段通过，原阈值/样本保持，9 次 HTTP503 与逐次 socket 明细限制保留。

## 7. 最终检查与报告

- [x] 独立check复核source/config/tools/assets、named inventory、before/after bindings与resource ownership。
- [x] 每个T01–T25、L0–L10都有结果和证据；各suite/pass/fail/skip/missing/global errors、复跑与初始结果分开保留。
- [x] 输出 `report.md`、机器汇总、`defects.md`、覆盖缺口和不建议发布结论。
- [x] 停止本次自有process与PG；182端口关闭，其他客户端0，协议精确恢复；失败数据库/数据卷/历史证据保留。CLI历史兜底清理FAIL不改写。
- [x] macOS完整Main失败后，已注销仅本次新task路径并精确恢复原协议默认URL/既有9个注册URL，保存前后证据。
- [x] 按实际结果完成验收执行与证据交付；产品FAIL、平台/模型缺口、未验证的失败后续均明确保留。
