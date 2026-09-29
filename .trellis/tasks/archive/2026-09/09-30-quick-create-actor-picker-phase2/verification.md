# 第二期验收记录

## 结果

完成默认创建助手、项目相关小队与手动触发的“帮我选”。建议只提供当前有权调用的对象及已保存职责依据，采用才更新助手；不自动设置默认、不记录最近使用、不创建或执行任务。

## 检查结果

| 检查 | 最终证据 |
| --- | --- |
| Views全量 | 460个测试文件、5636个测试通过 |
| 第一／二期Web与Electron | 8/8通过，46.0秒；英文、中文375px、实际Electron与第一期回归 |
| Core全量 | 2144个测试通过；1个原有 /mcp 路由诊断覆盖失败，第一期已在原目录复现 |
| Go后端 | 推荐／调用权限、假引用与假依据、生成后变化、取消、超时、准入、序列化预算及550项尾部检索通过；显式使用隔离数据库 |
| LLM出站合同与静态检查 | pkg/llm测试、go vet涉及handler/server包通过 |
| 全仓类型检查 | 9个任务通过；后续独立提交树core/views类型检查通过 |
| 全仓lint | 6个任务通过，保留已有警告，未新增lint错误 |
| Knip | 与第一期同样的9个闲置文件、1个依赖和1个开发依赖报告，未混入清理 |
| UI exports／文档 | 62个UI导出检查通过；环境变量说明与生成文档同步 |
| 视觉 | 95/pass；默认／项目弹层、英文与中文窄屏、Electron截图见research |
| 性能回归 | 550个对象、20次已加载目录输入到渲染采样，P95=32.2ms；Next开发模式，目标100ms |
| 原目录回传复验 | Views定向71个、Core定向52个测试通过 |
| 独立提交树 | core/views类型检查、76个组件测试、后端推荐测试通过；最终弹窗复验5个测试通过，Electron生产构建通过 |
| 独立代码审查 | APPROVE；见research/implementation-review.md |

Go测试命令（从工作区根目录，通过隔离环境执行）：

```sh
bash scripts/dev-env.sh exec actor-picker-phase2 -- sh -c 'cd server && go test ./internal/handler -run "TestRecommendCreators|TestCreatorShortlist|TestLoadedInvocation|TestCanInvokeAgent" -count=1 -v'
```

端到端使用本地确定性provider，无真实模型、智能体或守护进程执行：

```sh
PLAYWRIGHT_BASE_URL=http://localhost:13390 NEXT_PUBLIC_API_URL=http://localhost:18470 \
ACTOR_RECOMMENDATION_PROVIDER_URL=http://127.0.0.1:54004 \
CHANGELOG_ELECTRON_RENDERER_URL=http://127.0.0.1:54180 CHANGELOG_E2E_API_URL=http://localhost:18470 \
pnpm exec playwright test e2e/quick-create-actor-picker-phase2.spec.ts e2e/quick-create-actor-picker.spec.ts --workers=1
```

## 验收映射

| 标准 | 覆盖 |
| --- | --- |
| AC01–AC03 | 默认设／清／恢复、5级预选、未知数据等待、非法actor类型、workspace/login隔离的core测试；真实API创建后默认优先于最近 |
| AC04–AC06 | plural/legacy现有项目合同、configured可调用小队投影、8项去重预算、完整项目浏览、未知项目不伪装为空；浏览器切项目不改变当前助手 |
| AC07–AC09 | 点击才请求、重复点击防重、显式采用、文字／项目／助手／workspace／登录代次变化后取消和丢弃迟到结果；采用前读取实时编辑器和描述快照 |
| AC10–AC12 | 服务端human gate、成员与私有调用权限、队长／运行时、引用／证据校验和生成后重读；完整目录尾部文字检索与payload上限 |
| AC13 | 未配置503、繁忙429、超时504、模型错误502、无结果200[]分离；可取消／重试且创建按钮不受阻 |
| AC14 | 真实嵌套弹层键盘、默认footer、项目视图、中文／英文窄屏和完整创建按钮可见性；第一期E2E继续通过 |
| AC15 | 实际API+假LLM链路和Electron；配置注释、LLMconsumer清单、双语说明、嵌入文档同步 |

## 发现并修复

1. 全局mutation缓存：gcTime:0不足以清理仍被观察的mutation。真实QueryClient测试先复现，再验证结算／取消／清除／卸载时reset；旧请求不能reset新请求。
2. 存储里未知actor类型：不能默认当squad处理。纯预选测试先失败，修复后跳过非法类型／空ID。
3. 职责改变：即使旧片段仍存在，描述增加否定等内容也不能继续使用旧建议。服务端和客户端均校验完整描述快照。
4. 窄屏窗口：初始440px高度会裁切创建按钮（浏览器实际可见比例约0.82）。复用onNeedsSpace扩展窗口，限制建议区高度；中文／英文浏览器现在断言创建按钮可见比例为1、编辑器至少一半可见。
5. 项目未知状态：项目查询失败时不展示“没有项目小队”的确定性空态；类型筛选空结果使用项目专属文案。

测试脚本修正：Markdown会转义带下划线的控制标记，因此假provider测试改用纯字母标记；窄屏侧栏默认折叠，中文测试先打开创建窗口再切到窄屏。截图等待弹层动画结束。以上未改变产品语义。

一次高并发全量运行出现无关chat-queue菜单时序失败，原目录和隔离目录定向测试均通过；最终maxWorkers=2的全量460文件通过，未改聊天代码。

## 独立提交与工作区保护

开发在独立快照中完成；临时Git index计算本期增量，原目录其他未提交改动保持原样。

当前原目录已有未提交的通用弹窗扩展接口。功能分支仅补入最小AgentCreatePanel可选回调及agent模式600px扩展支持，保留来源预览展开的优先级；未把其他未提交的描述编辑器改造带入分支。独立提交树验证：core/views类型检查、76个相关组件测试、后端推荐测试通过；最终弹窗支持复验5个测试通过，Electron生产构建通过（renderer 3.72秒）。原目录回传后Views定向71个、Core定向52个测试通过。

`e2e/issue-assist-flow.spec.ts`是原目录已有未跟踪测试，本次只修正创建助手的可访问名称；该局部工作区调整不把整份他人测试收入功能提交。

## 限制

默认偏好按本地工作区保存，退出登录沿用现有清理规则，不跨设备同步。推荐使用现有LLM配置；大目录的文字检索可能漏掉同义／跨语言匹配，仍可手动搜索。性能为开发模式、已加载目录测量。没有执行真实智能体、生产部署或移动App验证。
