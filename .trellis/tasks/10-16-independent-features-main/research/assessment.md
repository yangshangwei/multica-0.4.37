# 可独立合并的功能评估

评估日期：2026-10-16。
源分支：codex/cross-workspace-collaboration，ff6bb36e3。
目标：本地 main，7906c6e93。远端 origin/main 为4797e6d97，本地额外包含认证与桌面修复；本报告的独立应用结论以本地 main 为准。

结论：可提取3个产品改进包、2个配套包，以及1个小型测试修复包。均不依赖未完成的跨工作区协作。此结论是依赖及移植可行性评估，不代表已经合并或完成拆分结果的全部回归。

## 建议分组

| 包 | 最小提交来源与顺序 | 文件数（含测试/规范） | 模拟结果 |
| --- | --- | ---: | --- |
| 内网桌面下载页 | 35cc68ca2 | 13 | 无冲突 |
| 自动化模板创建与返回焦点 | e5fe7b909 → c98c14ad4的产品/测试/规范路径 | 15 | 无冲突 |
| 设置页失败恢复及个人令牌管理 | f3e1161a4 → 6ccd5e643 | 80 | client.ts一处导入上下文冲突；定向处理后其余全部无冲突 |
| 使用文档与文档导航 | 4d7399b74 → 5d6966b4b | 52 | 无冲突 |
| 本地QA样本数据工具 | ede5ae202的源码/测试/规范路径 | 6 | 无冲突 |
| 既有页面测试修复 | 14642c42f | 3 | 无冲突 |

模拟使用独立 GIT_INDEX_FILE，以 main 初始化后逐提交三方应用。没有修改原工作区索引、源码或分支引用。所有包均排除历史任务记录、日志及截图；必要规范保留。完整结果见 application-results.json；设置冲突处理结果见 settings-resolved-with_specs.json。

## 产品包细节

### 内网桌面下载

提供内网静态下载页、Windows x64安装器的download.json投影、管理员业务服务器配置和发布脚本，用户可以自行下载并完成引导。

依赖的更新/安装脚本基线与 main 完全一致。不依赖协作表、协作权限或设置恢复组。不新增安装器发布物，不自动发布新版本。管理员仍需填写download-config.json.serverUrl。

候选补丁：`download.patch`（来源路径见 slices.json）。

### 自动化模板交互

保留模板分类、滚动与键盘焦点，创建成功才清除来源焦点；修复Desktop先更新URL、后提交路由时丢失返回焦点的问题。Web/Desktop共用接口和各自适配都须包含。

e5fe7b909全部15个路径一起提取。c98c14ad4只更新其中3个路径：

- packages/views/autopilots/components/template-create-autopilot-page.tsx
- packages/views/autopilots/components/template-create-autopilot-page.test.tsx
- .trellis/spec/views/frontend/automation-template-discovery.md

c98c14ad4其余验收截图和任务记录不属于运行依赖。整提交搬运会带入之前确认的图片预算问题。

候选补丁：`autopilot.patch`（来源路径见 slices.json）。

### 设置恢复与个人令牌

覆盖瞬时错误保留编辑、权限拒绝清除缓存、失败动作重试、保存结果判断、集成列表分页、令牌响应校验及秘密不进入Query/Mutation缓存。

两个提交需要一起提取。6ccd5e643依赖f3e1161a4新增的withQueryAccessBoundary、PAT schema和token queries；还包含Agent集成页与Desktop设置面板适配，不能仅提取settings目录。

不需要协作API/schema、571–619迁移或collaboration包导出。client.ts冲突来自首个import hunk的协作代码上下文。模拟只添加PAT import、替换三个PAT方法，保留main其他客户端实现；再应用6ccd5e643成功。严禁整文件覆盖HEAD，否则会带入协作部分。

候选补丁：`settings.patch`（来源路径见 slices.json）。其client.ts部分已在私有索引定向处理，未写入工作区。

## 配套包

文档4d7399b74应与5d6966b4b一起提取：前者新增/auth-setup#开启密码登录链接，后者才提供该标题。包括中文说明、缺失的迭代/分诊页面、文档导航样式及内置文档bundle。相关产品能力已在main。
候选补丁：`docs.patch`（来源路径见 slices.json）。

QA工具ede5ae202增加seed命令、真实业务样本和回归测试；21张显式写入表及pg依赖均已在main。它需要本地数据库与已有工作空间，不提供跨账号协作功能。
候选补丁：`seed.patch`（来源路径见 slices.json）。

14642c42f只修复3个既有测试的ApiError mock、listitem选择器及Base UI点击时序，无运行代码依赖，可以先合。
候选补丁：`view_test_alignment.patch`（来源路径见 slices.json）。

f90c3fe13是额外的历史交付文档候选：记录0.6.6、源码4797e6d、迁移570及当次产物证据。可独立保留历史记录，但不能作为新版本发版或当前分支验收证明。与下载功能一起的净文档补丁为download_release_docs.patch，不建议把整套归档材料都当作新功能带入。

## main已有，无需重复移植

- MCP大列表选择与分类筛选：main与HEAD相关产品/测试文件相同。
- CSV编码兼容：e0a028cd8与main的4797e6d97补丁等价。
- 私有服务器登录体验：7436bc810与main的53563fe78补丁等价。
- 服务器不可用时桌面恢复：0c6afec4c与main的f14da8a30补丁等价。
- 桌面键盘与核心工作流修复：bb2d9f481与main的5c6c2a0a0补丁等价。

认证与核心桌面修复只存在于本地main领先远端的提交中；若将来改以当前origin/main为摘取起点，需要先补齐这些前置提交。

## 暂缓的部分

cc85172d9、57f6163f4、c29e05c8f及其协作schema/迁移/服务/core契约构成尚未完成的跨账号协作链路。创建和共享执行仍保持关闭，不建议作为“已完成、可独立开放的功能包”移入main。不要单摘路由、权限校验或撤权处理而遗漏其数据与事务依赖。

历史任务归档、journal、截图与构建日志不构成功能依赖，尤其不要通过c98c14ad4或后续归档提交重新带入超限PNG。

## 验证界限与建议顺序

上一轮针对当前完整分支的lint/typecheck、11731项TypeScript测试、726项关键Go race回归及SQL生成检查已通过。本轮没有新增源代码变更，执行了逐功能私有索引模拟、提交基线比对、符号/表依赖审查和已合入补丁等价检查；没有对每个拆分树重新执行完整测试。

建议先合小型测试修复；下载、自动化和设置分别形成独立PR。文档与QA工具可独立推进。真正合入前，各候选补丁需在最终目标main上重跑相应回归及CI，设置组重点复核client.ts的定向移植。

