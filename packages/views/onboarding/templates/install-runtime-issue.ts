/**
 * Skip path: connect a runtime to start with the built-in assistant.
 *
 * Written to a new issue (assigned to the user themselves) by the welcome
 * hook when the user took the Skip exit on Step 3. Content is the
 * install-runtime tutorial; each supported locale can recommend the
 * quickest runtime path that best fits that audience.
 */

/**
 * Localized so users see the title in their current supported locale on the
 * board. The Runtimes page owns the follow-up Mika bootstrap once a runtime
 * appears, so this guide does not ask the member to copy an agent prompt.
 *
 * The documentation link below is a slug-less in-app path (`/docs/...`), not an
 * `https://multica.ai/docs/...` URL: an intranet deployment cannot reach the
 * public site, and this text is PERSISTED into an issue description that a
 * reader opens later, possibly from a different workspace. `openLink` prefixes
 * the reader's current workspace slug at click time for a known route segment
 * (see WORKSPACE_ROUTE_SEGMENTS in editor/utils/link-handler.ts), so baking one
 * workspace's slug in here would be wrong.
 *
 * Note: server's deprecation shim (`onboarding_shim.go:noRuntimeIssueTitle`)
 * still uses the bare English string for its title-based dedupe — that
 * codepath only runs for pre-v3 desktop builds and never overlaps with
 * the v3 frontend population, so the two title-spaces drifting is fine.
 */
export const INSTALL_RUNTIME_ISSUE_TITLE = {
  en: "Connect a runtime to start with 小阿孚",
  zh: "连接运行时，开始使用小阿孚",
} as const;

const en = `Welcome to Multica.

Agents need a runtime before they can execute work. You can still use Multica as a lightweight project-management workspace while you install one.

## Try Multica first

Before the runtime is ready, you can:

1. Create a project for your current work.
2. Create a few issues and move them across backlog, todo, in_progress, and done.
3. Add priorities, labels, comments, and subscriptions.
4. Use Inbox to track assignments and mentions.

That gives you the project-management layer first. Once a runtime is connected, agents can start working from the same issues.

## Install your first agent runtime

Full guide: /docs/install-agent-runtime

For English users, the fastest first path is Codex. This deployment has no
public internet access, so ask your administrator for the offline installer
or internal package mirror for the runtime CLI. Once it is installed:

1. Confirm your terminal can find it:
   which codex
   codex --version
2. Wait for Multica to pick it up. A running daemon re-checks for newly
   installed CLIs every couple of minutes, so no restart is normally needed.
   To apply it immediately:
   multica daemon restart
   In the desktop app, open any local runtime and click Restart. Quitting and
   reopening the app is NOT enough — the daemon keeps running in the background.
3. Return to Runtimes and refresh. You should see a Codex runtime online.
4. Open Runtimes. The page will offer **Start with 小阿孚**; use it to create 小阿孚 and open the guided first chat.

小阿孚 will turn one real goal into an issue, start it with the right agent, and suggest reusable specialists when your workflow needs them.`;

const zh = `欢迎来到 Multica。

智能体需要先连上运行时才能执行工作。运行时还没准备好时,你也可以先把 Multica 当作轻量项目管理工具体验起来。

## 先体验项目管理功能

运行时安装前,你可以先做这些事:

1. 为当前工作创建一个项目。
2. 新建几个任务,并在 backlog、todo、in_progress、done 之间流转。
3. 给任务加优先级、标签、评论和订阅。
4. 用收件箱追踪分配给你的事项和 @mention。

这样你先熟悉项目管理层。连上运行时后,智能体会直接在这些任务上开始工作。

## 安装第一个 Agent 运行时

完整文档:/docs/install-agent-runtime

中文用户建议先装 Kimi CLI。本部署无法访问公网，请向管理员获取运行时 CLI 的离线安装包或内网镜像。安装完成后：

1. 确认终端能找到 Kimi:
   kimi --version
2. 在你想让 Kimi 工作的项目目录里启动一次:
   kimi
3. 首次启动后输入 /login,按提示完成 Kimi Code 或 API key 配置。
4. 等 Multica 识别到它。运行中的守护进程每隔几分钟会重新检查一次新装的 CLI,通常不需要重启。
   想立刻生效:
   multica daemon restart
   桌面端请打开任意一个本机 runtime 并点 Restart。退出再打开 app 是不够的 —— 守护进程会继续在后台运行。
5. 回到 Runtimes 页面刷新。你应该能看到一个在线的 Kimi 运行时。
6. 打开"运行时"页面。页面会显示 **开始使用小阿孚**；点击后会创建小阿孚，并进入引导式的首次对话。

小阿孚会把一个真实目标转化为任务，交给合适的智能体启动执行，并在工作流需要时建议添加可复用的 specialist。`;

export const INSTALL_RUNTIME_ISSUE_BODY = { en, zh, } as const;
