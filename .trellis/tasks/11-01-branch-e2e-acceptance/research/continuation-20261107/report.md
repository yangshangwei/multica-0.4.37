# 当前分支核心业务续验报告

**本轮续验完成，完整验收仍未通过。** 产品固定于`4c817dee9397a42ac903f419af7804b11d2d863e`，只修补验收夹具；旧822c/1a32结果与当前结果分别保留。

| 检查 | 本轮实际结果 |
| --- | --- |
| 核心业务Web/Electron定向E2E | 68用例：66PASS、2FAIL，已无条件跳过 |
| 非Mobile单元测试 | 13,320PASS、4FAIL，1,074文件、5包 |
| 非Mobile lint/typecheck | 15任务通过；30警告、0错误；UI导出检查通过 |
| Go针对性race回归 | 396顶层＋618子用例通过，14包，0失败/跳过 |
| Go全量构建与vet | 通过 |
| 验证脚本 | 101个Node测试结点通过，Go wrapper shell通过 |

E2E是多轮归并结果，包含67个仓库用例和1个独立的小队手动创建提交用例；全量收集441个仓库用例只证明入口可加载，另外374个本轮未执行。原任务15项失败均已有当前候选PASS；两个安装版用例本轮未构建/配置当前构件，仍为BLOCKED，不能借用旧包通过记录。见`original-case-current-status.md`。

## 当前未解决问题

1. **QA-20261107-001：小队创建路由缺诊断映射。** manual/template路由落入通配遮罩，core单测1FAIL；独立整文件复现14PASS/1FAIL。影响诊断页面归类，未证明身份泄漏。
2. **QA-20261107-003：进展报告Skill默认描述与本地化目录不同步。** 后端收窄为当前项目，展示目录仍保留旧文案，英文界面回退显示中文。views单测3FAIL；独立整文件复现106PASS/3FAIL。
3. **QA-20261107-004：详情切换作用域后的滚动恢复不稳定。** 完整Web/Desktop原始旅程保持2FAIL。独立诊断中Web按原620断言PASS，Desktop先恢复620后落到476，最大可滚动值仍大于620；具体布局变化原因未关闭，未降低断言或认定修复。

全部首败、复验和根因边界见`defects-current.md`。R08限定本任务交付验收和缺陷，不修改产品逻辑。原15项失败通过不代表本候选没有新增回归。

## 修补及简化

共修改15个E2E文件，复用共享控件helper，没有新依赖或产品抽象：

- `e2e/fixtures/collaboration-disclosures.ts`：修正计数按钮正则转义，统一真实Mention展开与可见性前置。
- `e2e/collaboration-{account-desktop,boundaries-web,delivery-web,leave-web,memory-web,records-web,workbench-web}.spec.ts`：按模态抽屉、Mention和More actions的实际交互操作；权限负断言先确认对应区域可访问。
- `e2e/collaboration-knowledge.spec.ts`：刷新后通过可见Needs attention提示进入恢复，保留原请求身份，并明确断言没有新增POST。
- `e2e/fixtures/collaboration-{control-gaps,detail-journey,knowledge,mention-journeys,resource-offers,retained-attachments}.ts`：补齐可见控件、评论编辑完成和抽屉关闭前置，保持预览、授权、精确文件字节、版本、恢复及持久化断言。
- `.trellis/spec/web/frontend/e2e-run-environment.md`：记录可见性前置、权限负断言、提醒入口恢复与证据保留规则。

知识恢复最初按旧的自动展开假设失败；原始DOM显示状态仍保留且提示可见。当前设计要求通过提示进入，复验及独立trace审计确认原身份GET恢复、刷新后零POST，因此没有把旧交互失配误报为数据丢失。

原始中断批次规范化为7PASS/2FAIL/2条件SKIP/42未执行，其中42包括1个框架中断skip和41个无结果用例；原始框架数为7PASS/2FAIL/3SKIP/41无结果。完整首次夹具修补批次37PASS/7FAIL/9SKIP，后续逐项补证，不拼成单轮全绿。首次私有helper收集错误、缺CLI和所有未通过尝试也独立留档。自动重试始终为0。

## 来源、范围与清理

生产Web为本候选build/start，Electron使用同源码renderer/preload harness。8,169个源文件按既有Next生成导入规范化规则审计，仅声明15个测试文件变化，494个renderer/preload产物字节未变；所有运行绑定API/Web配置、构建及进程身份。最终测试补丁SHA256为`a3e173f02860547f7368b7ec9c7c71e77f66f62a19d694a6644db129ee2d5f39`，已独立复核并按哈希守卫同步主工作区。

本轮不覆盖其他374个仓库用例、安装包完整Main、Linux/Windows/iOS实际UI、签名/自动更新、真实商业模型或完整性能门。真实Provider调用0；旧候选的性能例外未扩展到本候选。工具链为macOS arm64、Node22.23.2、pnpm10.28.2、Go1.27.0；Go版本与CI1.26存在差异。

5个自有浏览器环境端口均已关闭，3个浏览器专库和8个后端专库连接为0，自有进程已退出。保留专库、构件和私有原始证据用于复现；未清理其他任务资源。私有配置、Cookie和trace不进入提交。任务保持in_progress，等待产品修复候选及剩余验收门。

证据入口：`browser-report.md`、`browser-coverage.csv`、`browser-results.json`、`browser-provenance.json`、`browser-cleanup.json`；`frontend-result.json`与独立`frontend-review.json`；`backend-final.json`、`runtime-closure-supplement.json`及协调者raw审计；`drawer-failure-analysis.md`与最终证据复核。所有旧结果均保留。
