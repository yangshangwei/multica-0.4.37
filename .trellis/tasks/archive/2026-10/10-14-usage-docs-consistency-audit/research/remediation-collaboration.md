# 协作与教程组修订记录

日期：2026-10-14。D08、D09、D10、D11、D18 的源文档修订完成，待主会话统一生成嵌入包并进行集成验证。原始 `findings.json` 与审计报告保持为历史证据。

## 文件范围

- `apps/docs/content/docs/comments.zh.mdx`
- `apps/docs/content/docs/members-roles.zh.mdx`
- `apps/docs/content/docs/squads.zh.mdx`
- `apps/docs/content/docs/tutorial.zh.mdx`
- 本记录。

未修改业务代码、测试、图片、生成包、spec 或任务元数据。`members-tab.tsx` 正由其他会话修改，本组只读取它的当前实现。

## 逐项关闭记录

| 发现 | 修订位置 | 关闭依据 |
|---|---|---|
| D08 | `members-roles.zh.mdx:27`、`tutorial.zh.mdx:335` | 分开说明邮箱定向邀请与分享链接；两者仅授予管理员或成员。邮箱邀请固定 7 天、可撤销，管理列表无复制按钮；分享链接可查看、复制、撤销，页面默认 7 天，可选 24 小时、30 天或永久有效。重新生成使旧链接失效，撤销不移除已加入成员。纯密码账号需先注册、登录并完成账号设置或强制改密，分享链接不绕过注册限制。中文角色展示采用当前 locale，首次出现保留 schema 标识符对照。 |
| D09 | `comments.zh.mdx:32` | `@all` 自身不入队，只阻止隐式路由；同条显式 `@智能体` / `@小队` 仍按目标处理。补充 `/note` 首词和取消触发预览目标的静默方法。 |
| D10 | `comments.zh.mdx:48` | 明确正文编辑会取消原评论作为触发来源或合并输入的未完成 task，包含正在运行的执行；剩余有效评论与新正文重新路由。只改附件、正文不变时不走该路径。删除描述为“尝试取消”，不承诺回滚已产生的修改；建议以新评论补充要求来保留当前执行。 |
| D11 | `squads.zh.mdx:65`、`:79`，`tutorial.zh.mdx:246` | 删除“智能体结果里提及其他智能体仍会额外唤醒队长”的例外。区分显式目标、成员或全员提及、线程承接、顶层评论和工作智能体结果的队长兜底；限定队长身份的防自触发。需要队长参与时明确提及队长或当前小队。 |
| D18 | `tutorial.zh.mdx:43`、`:269` | 导航改为“资源与连接 → 运行时”和“资源与连接 → 技能库”；用相邻正文说明旧英文截图与当前入口的对应关系，图片替代文本注明旧版英文界面。 |

## 当前实现证据

- D08：`packages/views/settings/components/members-tab.tsx` 的 `ShareLinkRow`（312）、有效期初值（411）、`handleCreateShareLink`（784）和创建区域（980）；`packages/views/locales/zh-Hans/settings.json:1482` 的角色与有效期标签。`server/cmd/server/router.go:1772` 对创建、查看、撤销分享链接设置所有者/管理员门禁；`server/internal/handler/invitation.go:59`、`:453` 处理邮箱、角色、7 天有效期与受邀账号匹配；`server/internal/handler/share_link.go:68`、`:200`、`:264` 以及 `server/pkg/db/queries/share_link.sql` 处理角色、轮换、撤销、登录加入、到期与次数限制。`server/internal/handler/auth_password.go:128` 检查注册开关，`server/internal/auth/password_session.go:142` 和 `server/internal/middleware/password_auth.go:88` 阻止尚未完成设置/改密的会话加入。
- D09：`server/internal/handler/comment.go:2637` 先检查接纳条件和 `/note`，再于 2673 处理显式智能体/小队提及，之后才处理 `@all` 与成员提及；`isNoteComment` 在 1941。已阅读 `comment_trigger_preview_test.go:1109` 和 `:1139` 的显式智能体/小队与 `@all` 组合用例，未在本组运行这些数据库测试。
- D10：`server/internal/handler/comment.go:3286`、`:3338`、`:3381` 区分带版本约束的正文编辑与旧客户端路径；现代正文编辑成功更新与取消在同一事务中，旧路径先取消再更新，失败时重建存活输入。`:3433` 在正文确有变化时重算，`:3470` 删除前调用单个评论 ID 的取消，`:3581` 重建其他仍有效输入。`server/pkg/db/queries/agent.sql:754` 的范围为 `trigger_comment_id` 或 `coalesced_comment_ids` 包含该评论，状态为 `queued`、`dispatched`、`running`、`waiting_local_directory`、`deferred`。删除的取消失败只记录日志并继续删除，因此文档没有声称保证取消整条讨论中所有回复触发的执行。已阅读 `daemon_comment_delivery_test.go:384`、`:406`、`:433`、`:455` 的取消和批次修复用例，未执行。
- D11：`server/internal/handler/comment.go:2673` 显式目标分支立即返回；`:2683` 的智能体结果路径仅在没有上述提及时考虑已分配小队的队长；`:2702` 起的人类回复路径先走父评论作者/线程承接。`:2932` 的队长兜底检查运行时绑定、归档、Access 与去重；`server/internal/handler/squad.go:1393` 按最近执行的小队身份约束防自触发。`:3021` 起显式目标解析不追加原负责人。
- D18：`packages/views/layout/app-sidebar.tsx:175` 的 `resourceNav` 及 `:931` 的分组；`packages/views/locales/zh-Hans/layout.json:12`、`:13`、`:61` 分别定义运行时、技能库、资源与连接。

行号对应本组最后核对时的工作区；并行会话可能继续移动源码行号，应结合上述函数名定位。

## 截图核对

通过本地图片查看工具检查了以下已有文件，未生成或修改图片：

| 图片 | 观察结果 |
|---|---|
| `tutorial-runtimes-list.webp`、`tutorial-runtime-detail.webp` | 旧英文界面明确显示 `Configure → Runtimes`，已在教程运行时段落说明当前入口。 |
| `tutorial-skills-list.webp`、`tutorial-skill-detail.webp` | 旧英文界面明确显示 `Configure → Skills`，已在 skill 段落说明当前入口。详情图仍有历史 GitHub 来源标签，不能据此推定内网可在线导入。 |
| `tutorial-skill-new-modal.webp` | 显示英文 `New skill` 及四种创建方式，背景导航模糊，未据此辨认分组文字。其替代文本明确为旧版截图，同节正文给出当前技能库入口。 |

这次核对只证明图片内容与说明的对应关系，没有验证当前应用的实际渲染，也没有声称截图已更新。

## 已执行检查与交接

- 对四个源文件运行 `git diff --check -- <四个路径>`，退出码 0，无空白错误；逐段审阅最终 diff 并对照上述源码。
- `git diff --name-only -- <上述五个图片路径>` 无输出，图片没有工作区差异。
- 本组没有新增仅断言文案字符串的测试，也没有运行或宣称通过 docs、MDX、类型检查或 Go 测试。
- 主会话继续统一生成 `server/internal/docs/content/`，执行 docs 测试、typecheck、MDX、Go docs、链接/锚点/资源与包字节一致性检查，再由独立检查代理复核。
