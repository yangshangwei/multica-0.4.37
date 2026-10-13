# 桌面端自助下载页实施与验收计划

状态：实现与本地验证完成；真实 Windows 安装未执行，提交与记录按收尾流程完成。

## 0. 开始条件与责任

- [x] 用户在阅读最新规划摘要后确认实施，再执行 `task.py start`。
- [x] 使用当前任务的 PRD、design 和 context manifests；保持任务为单一交付。
- [x] 检查最新 Git 状态并记录本次文件范围；现有工作树有其他任务修改，不回退或混入那些修改。
- [x] 按项目默认流程交由 `trellis-implement` 完成实施，再交由独立 `trellis-check` 检查。主会话负责整合、决策和最终证据。
- [x] 子代理提示包含活动任务路径、明确写入范围及“不是唯一修改者，不要回退其他人的改动”。
- [x] 不新增依赖，不发布真实安装包或修改既有线上存储来做测试。

## 1. 发布投影：先测试再扩展

责任范围：`apps/desktop/scripts/update-artifacts.mjs` 及其现有测试。

- [x] 在现有合成安装包 fixture 上增加 Windows x64 投影断言，沿用现有校验真实字节的测试。
- [x] 覆盖生成版本/文件名/大小；非 x64 通道不创建或覆盖 Windows 入口；macOS/Linux 仍只作为页面待开放内容。
- [x] 覆盖坏摘要、缺包、错误平台文件名、冲突文件、伪造源 JSON 和显式预发布测试策略。
- [x] 实现 collect/publish 共用的投影生成，保持原 YAML 和安装包字节不变。
- [x] 暂存时验证所有新信息；最后原子替换投影。备份旧投影，实际写出文件出现在返回清单。
- [x] 测试发布两版后指针更新且旧包保留；配置文件不受影响。
- [x] 对最后投影写入失败测试真实结果：发布报错、旧投影仍可下载、重试可修复。不假设所有公开文件同时回滚。

## 2. 静态页面和指导

责任范围：新增 `deploy/desktop-updates/download.html`、`download-config.example.json`、`apps/desktop/scripts/download-page.test.mjs`。

- [x] 按 design 的单页结构、项目中文术语和视觉来源完成页面。
- [x] 按已确认的现有 `/download` 基线保留深色首屏和浅色行式平台列表，复用品牌与 SVG 资源，不沿用 GitHub 获取、全平台完整性选版、CLI/Cloud 或公网兜底。既有 React/Next 组件需提取为静态交付内容，不能直接复制当成静态文件。
- [x] 同源读取下载投影与独立地址配置，有限超时/体积、必要字段及安全 URL 校验；先验证 HEAD 再开放当前 Windows 下载。
- [x] 展示所有待开放平台，维持不能下载的状态。
- [x] 实现复制真实反馈与 HTTP/权限拒绝时的可选中地址路径。
- [x] 写清真实注册字段、已注册登录、有邀请加入/无邀请创建，以及完成上手引导、稍后连接运行时后进入工作台。
- [x] FAQ 依据现有行为处理具体障碍，不添加账户操作、工单功能或虚构联系方式。
- [x] 使用已有 jsdom/Vitest 运行实际 HTML，验证可观察的下载、待开放、地址复制、失败提示和静态教程保留；不复制一份页面实现作为测试对象。
- [x] 静态文案/布局不写逐字快照测试；重点断言链接目标、状态、键盘可用性和必要流程是否存在。

## 3. Nginx、部署与首次接入

责任范围：`deploy/desktop-updates/nginx.conf`、`scripts/desktop-updates.sh`、`scripts/desktop-updates.test.mjs`。

- [x] 为 `/desktop/` 提供页面，规范化 `/desktop`，保留具体文件、GET/HEAD、Range、no-cache、隐藏文件拒绝和关闭目录列表。
- [x] 生命周期脚本原子安装自有页面文件，配置只在缺失时初始化。避免覆盖用户配置或删除安装包。
- [x] 扩展已有 fake Docker/Node 脚本测试，验证最小部署文件、路径含空格、重复启动和配置保留。
- [x] 最小 Nginx 运行路径保持无 Node 要求；使用 shell 解析规则，不 source/eval 环境配置。
- [x] 更新 `docs/desktop-intranet-update-runbook.zh-CN.md`、`docs/desktop-updates-local.zh-CN.md`：入口、配置样例、离线文件清单、当前发布投影的首次补齐、手工发布顺序、Nginx 配置检查/reload、错误恢复。
- [x] 明确现有服务器若只复制新 HTML 还没有 download.json，将显示未就绪；提供可执行的 collect/publish 接入步骤。
- [x] 同步 `.trellis/spec/desktop/frontend/intranet-updates.md` 中新的首页、投影与恢复合同，保留旧兼容约束。

## 4. 定向验证命令

在仓库根目录执行；新页面测试加入后运行：

```bash
node --test scripts/desktop-updates.test.mjs
pnpm --filter @multica/desktop exec vitest run scripts/update-artifacts.test.mjs scripts/verify-updates.test.mjs scripts/download-page.test.mjs
pnpm --filter @multica/desktop lint
pnpm --filter @multica/desktop typecheck
bash -n scripts/desktop-updates.sh
git diff --check
```

- [x] 对内嵌 JavaScript 进行语法/现有 ESLint 规则检查，确保不因 HTML 文件位置漏掉静态分析。
- [x] 读取实际输出后记录通过/失败，区分既有无关问题；发生新改动或新失败再有针对性复验。
- [x] 如果改动扩大到离线整包收集等消费者，补充该消费者的现有测试；不要未检查调用方就宣称全部发布路径兼容。
- [x] 不修改 Go 业务代码时，无需为了这个静态页启动全仓服务或运行无关数据库测试。

## 5. 真实浏览器与 Nginx 验收

使用隔离临时存储、独立 Compose 项目和固定空闲端口；镜像复用已有固定版本。

- [x] 以两版校验有效的合成 EXE 测试完整路径：collect → publish → Nginx → 浏览器。合成 EXE 仅证明传输，不执行它。
- [x] 页面和 JSON MIME 正确，健康检查/元数据 HEAD 返回 200；安装包 Range 返回 206，完整下载大小/摘要与发布源一致。
- [x] 浏览器首次打开显示第一版；发布第二版后刷新页面得到第二版下载，旧文件仍可访问。
- [x] 覆盖无投影、无配置、损坏 JSON、安装包 404 和最后写入失败的恢复路径。
- [x] 拦截所有公网请求后验收页面完整加载、复制、FAQ、下载；业务 API 不参与页面渲染。
- [x] 真实 HTTP 页面和拒绝剪贴板权限的场景验证复制反馈及手动复制路径。
- [x] 键盘、窄视口和长地址验收，确认没有控制台错误、溢出或待开放假链接。
- [x] 按项目 visual-verdict 约定记录每次视觉审阅，完成后再做下一轮视觉修改。
- [x] 重启隔离容器后页面、业务地址配置与发布内容保留。

## 6. Windows 首次使用验收与收尾

- [ ] 使用真实、已验证来源的当前 Windows x64 安装包在干净 Windows 环境走完安装；记录安装包版本和来源。
- [ ] 对照页面文案完成业务连接、自助注册/已有账号登录，并分别验证邀请加入和自行创建路径到工作台。运行时工具可按现有入口稍后处理。
- [x] 真实安装条件不可用时，明确记录该项尚未执行，不能拿 DOM/HTTP 测试代替原生验收。
- [ ] 独立检查者核对 R1–R11、AC01–AC13 与证据，确认无范围漂移、无新增依赖、无更新源破坏，视觉结构符合已确认的既有页面基线。
- [x] 将新发布合同写入规范、更新任务证据；只在全部必要工作确实完成后进入后续提交/收尾流程。

## 回退点

- 页面与 Nginx 首页路由可单独撤回，安装包和原更新 URL 保留。
- 版本撤回必须配对处理该次 YAML 和 download.json；保留不可变安装包。
- 管理员配置不随代码/页面回退覆盖。
- Windows 客户端的卸载、降级与用户数据恢复不由页面回退操作自动完成。

## 本轮验证结论

166 项自动化测试通过；Desktop 类型检查通过，lint 无错误（1 条既有告警）；真实隔离 Nginx/Chromium 联合验证通过。独立检查代理在报告 lint/typecheck 通过后因外部额度中断，主会话完成剩余测试与全范围复核。Windows 原生安装及真实内网环境验收未执行，不将合成安装包的 HTTP 验证当作原生安装证据。详见 verification.md。
