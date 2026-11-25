# 前端与移动数据检查结果

基线：`main@103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`。
运行：`main-post-merge-e2e-20261124-080504`。
范围：前端静态、组件/单元测试、独立移动数据测试和 iOS 启动脚本契约；本文件不代表浏览器、原生移动 UI 或安装包已验收。

## 结论

七个检查步骤均有完整通过记录。非移动五包实际执行 15,059 个命名测试，移动包实际执行 196 个，合计 **1,190 个文件、15,255 个命名测试通过**。Turborepo 检查使用 `--force`，静态阶段 15/15 项成功、0 缓存；测试阶段也没有重放缓存结果。

服务器恢复后没有重跑任何产品测试。独立证据核对验证了七个退出码、原始日志 SHA、包/文件/命名结果数量、桌面脚本子集和当前来源，见 [recovery-review-01.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/frontend/recovery-review-01.json)。

| 包 | 测试文件 | 命名通过数 |
| --- | ---: | ---: |
| `@multica/core` | 381 | 5,503 |
| `@multica/docs` | 10 | 64 |
| `@multica/views` | 631 | 7,856 |
| `@multica/web` | 36 | 322 |
| `@multica/desktop` | 104 | 1,314 |
| `@multica/mobile` | 28 | 196 |
| **合计** | **1,190** | **15,255** |

## 实际检查与证据

| 步骤 | 实际命令/语义 | 结果 |
| --- | --- | --- |
| 01 | `pnpm exec turbo run lint typecheck --filter=!@multica/mobile --concurrency=1 --force` | PASS，15/15 项成功、0 缓存 |
| 02 | `pnpm check:ui-exports` | PASS |
| 03 | `pnpm exec turbo run test --filter=!@multica/mobile --concurrency=1 --force -- --maxWorkers=2 --reporter=verbose` | PASS，五包 1,162 文件/15,059 用例 |
| 04 | `pnpm --filter @multica/mobile typecheck` | PASS |
| 05 | `pnpm --filter @multica/mobile lint` | PASS |
| 06 | `pnpm --filter @multica/mobile exec vitest run --maxWorkers=2 --reporter=verbose` | PASS，28 文件/196 用例 |
| 07 | 在 `apps/mobile` 执行 `bash scripts/ios-run.test.sh` | PASS，原脚本报告所有断言通过 |

移动测试脚本原本为 `vitest run && bash scripts/ios-run.test.sh`；为保留命名结果，实际拆成 06/07 两步，仍只在 Vitest 成功后执行 shell。`verbose` 仅改变显示方式，没有修改测试集合、断言或超时。

完整命令、时间、退出码、环境和日志摘要见 [result.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/frontend/result.json)。实际命名终端及重复显示名的顺序保留在 [named-inventory-final.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/frontend/named-inventory-final.json)。参数化测试中相同显示名没有去重或丢弃。

## 来源与环境核对

- 七个步骤执行前均对初始 32,724 个跟踪文件核对来源、文件种类、权限和 SHA。执行结束的来源快照 PASS，无意外修改，也没有使用 Next 生成声明例外。
- 恢复时再次核对同一基线的 32,724 个跟踪文件，仍 PASS、无意外修改，见 [source-recovered-01.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/frontend/source-recovered-01.json)。父会话后续构建的来源/资产终审仍由父会话负责。
- 私有 HOME/TMP/GOTMPDIR 在 Git 外可信目录中；记录了已安装工具、缓存目录和七条实际工作区依赖边，均解析到当前 checkout。
- 已记录工具：Node 22.23.2、pnpm 10.28.2、Vitest 4.1.0、TypeScript 5.9.3、ESLint 9.39.4、Turbo 2.9.5。
- 原前端 runner 进程已结束；保留的命名总账没有记录残留进程组成员。恢复收尾只读取记录，没有启动浏览器、SQL、生产 Main 或模型账号。

## 桌面脚本的正确入口

`apps/desktop/vitest.config.ts:14` 显式包含 `scripts/**/*.test.mjs`。脚本首轮把八个文件投递到 Node test runner，这些入口错误独立保留；八个文件已由本次完整桌面 Vitest 实际执行，共 **212 个 PASS**，见 [desktop-vitest-script-results.json](/Volumes/artisan/code/2026/multica-0.4.37/artifacts/qa/main-post-merge-e2e-20261124-080504/frontend/desktop-vitest-script-results.json)。

这 212 个测试已包含在桌面 1,314 个及总计 15,255 个测试中，不另计新增覆盖，也没有补跑已通过的桌面整套测试。

## 未修复事项与验证边界

- 静态检查保留 30 个既有 lint warnings（views 27、ui 3），0 errors。本任务没有为消除警告修改产品代码。
- **P2 依赖声明债务**：`apps/mobile/eslint.config.js:2` 直接 `require('eslint/config')`，而 `apps/mobile/package.json` 仅声明 `eslint-config-expo`，没有直接声明 `eslint`。当前依赖提升使实际 lint 可解析 ESLint 9.39.4；这违反根 `CLAUDE.md` 的工作区直接依赖规则。此项属于依赖清单调整，按本任务保持产品基线/不加依赖的范围保留，未修复。
- 移动 28 个文件为纯 Node/数据语义测试，不证明 React Native renderer、Hermes、键盘/手势、AppState 前后台或真实设备网络恢复。
- iOS shell 契约通过不代表 Xcode/模拟器或原生应用启动通过。本机缺少原生 iOS 前置条件，见平台研究。
- UI 组件/平台接线测试与实际浏览器、生产 Electron Main、安装器和真实 daemon/provider 链路分开报告；这些后续验收不由本文件的 PASS 推断。

## 本次恢复新增文件

只新增任务研究报告、私有证据核对器和恢复 JSON。没有修改产品源码、已有测试、依赖、锁文件或既有原始运行记录。
