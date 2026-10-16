#!/usr/bin/env node

import { createHash } from "node:crypto";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";
import pg from "pg";

export const FIXTURE_KEY = "multica.rich-desktop.v1";
export const MARKER = `<!-- fixture:${FIXTURE_KEY} -->`;
export const DEFAULT_WORKSPACE = "desktop-qa-1008-1743";
export const STATUSES = ["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"];
export const PRIORITIES = ["urgent", "high", "medium", "low", "none"];

const help = `Populate an existing local workspace with desktop business scenarios.

DATABASE_URL must explicitly select a local PostgreSQL database.
  pnpm seed --workspace ${DEFAULT_WORKSPACE} --dry-run
  make seed ARGS="--workspace ${DEFAULT_WORKSPACE}"

Options:
  --workspace <slug>  Existing workspace (default: ${DEFAULT_WORKSPACE})
  --dry-run           Read-only preview; never reserves issue numbers
  --reset             Delete only this fixture; does not reseed
  --help              Show this help

Repeat runs preserve edits, read cursors and deleted child records. Root rows
must retain their fixture marker. Reset refuses references from other records.
No agent CLI, daemon, credential, notification delivery or model is started.
`;

export function optionsFrom(args, environment = process.env) {
  const { values } = parseArgs({ args, options: {
    workspace: { type: "string", default: DEFAULT_WORKSPACE },
    "dry-run": { type: "boolean", default: false },
    reset: { type: "boolean", default: false },
    help: { type: "boolean", default: false },
  } });
  if (values.help) return { help: true };
  if (!values.workspace.trim()) throw new Error("--workspace must be a nonempty slug");
  if (!environment.DATABASE_URL) throw new Error("DATABASE_URL is required; select the current local development database explicitly");
  let url;
  try { url = new URL(environment.DATABASE_URL); }
  catch { throw new Error("DATABASE_URL must be a PostgreSQL URL"); }
  if (!["postgres:", "postgresql:"].includes(url.protocol) || !url.pathname.slice(1)) {
    throw new Error("DATABASE_URL must select a PostgreSQL database");
  }
  if (!["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)) {
    throw new Error("This development fixture requires a loopback PostgreSQL host");
  }
  if ([...url.searchParams.keys()].some((key) => ["host", "hostaddr", "dbname", "service"].includes(key))) {
    throw new Error("DATABASE_URL must not override its database or host through query parameters");
  }
  return { workspace: values.workspace, dryRun: values["dry-run"], reset: values.reset,
    connectionString: environment.DATABASE_URL };
}

export function fixtureId(workspaceId, key) {
  const bytes = createHash("sha1").update(`${FIXTURE_KEY}/${workspaceId}/${key}`).digest().subarray(0, 16);
  bytes[6] = (bytes[6] & 0x0f) | 0x50;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = bytes.toString("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

const json = JSON.stringify;
const marked = (text) => `${text}\n\n${MARKER}`;
const quote = (identifier) => `"${identifier.replaceAll('"', '""')}"`;
const primaryKeys = {
  issue_to_label: ["issue_id", "label_id"],
  issue_subscriber: ["issue_id", "user_type", "user_id"],
  iteration_participation: ["workspace_id", "iteration_id", "issue_id"],
  issue_triage: ["issue_id"],
};
const keysFor = (table) => primaryKeys[table] ?? ["id"];
const recordKey = (table, data) => `${table}:${keysFor(table).map((key) => data[key]).join(":")}`;

// Planned membership mirrors iteration/membership.go. Closed snapshots and
// current iteration statistics remain owned by the ordinary product workflow.
export function buildFixture(context) {
  const { workspace, ownerId, memberId = ownerId, now = new Date(), timezone = "Asia/Shanghai", iterationEnabled = true, triageEnabled = false } = context;
  const id = (key) => fixtureId(workspace.id, key);
  const dateParts = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(now);
  const datePart = (type) => dateParts.find((part) => part.type === type).value;
  const today = `${datePart("year")}-${datePart("month")}-${datePart("day")}`;
  const day = (offset) => new Date(Date.parse(`${today}T12:00:00Z`) + offset * 86400000).toISOString().slice(0, 10);
  const at = (minutes) => new Date(now.getTime() + minutes * 60000).toISOString();
  const rows = [];
  const add = (table, key, data, { marker, anchor } = {}) => {
    const row = { table, key, data: { ...(keysFor(table).includes("id") ? { id: id(key) } : {}), ...data }, marker, anchor };
    rows.push(row);
    return row.data.id;
  };

  const projectSpecs = [
    ["delivery", "客户交付", "in_progress", "urgent", -10, 3, "推进桌面协作发布，完成核心流程回归和交付验收。"],
    ["experience", "平台体验", "planned", "high", 7, 21, "规划搜索、通知和快捷操作，确认下一阶段可交付范围。"],
    ["migration", "数据迁移", "paused", "medium", -20, -2, "等待上游字段确认；保留已完成工作和风险证据。"],
    ["auth", "身份认证", "completed", "low", -35, -7, "登录、权限与会话恢复已经完成验收。"],
    ["legacy", "旧方案归档", "cancelled", "none", -40, -12, "经过评审取消旧方案，保留取消原因与历史记录。"],
  ];
  const projects = Object.fromEntries(projectSpecs.map(([key, title, status, priority, start, due, description]) => [key,
    add("project", `project/${key}`, { workspace_id: workspace.id, title: `桌面场景 · ${title}`,
      description: marked(`## 目标\n\n${description}\n\n## 验收范围\n\n- 列表、详情和状态筛选显示一致\n- 负责人、日期和任务关联可追溯\n- 更改后刷新保持一致`),
      status, priority, lead_type: "member", lead_id: ownerId, start_date: day(start), due_date: day(due),
      in_progress_since: status === "in_progress" ? at(-10 * 1440) : null,
      in_progress_since_source: status === "in_progress" ? "transition" : null,
    }, { marker: "description" })]));
  const labels = [
    ["bug", "缺陷", "#ef4444"], ["feature", "功能", "#3b82f6"],
    ["review", "待验证", "#f59e0b"], ["edge", "边界场景", "#8b5cf6"],
  ].map(([key, name, color]) => add("issue_label", `label/${key}`, {
    workspace_id: workspace.id, name: `桌面场景 · ${name}`, color, description: marked("手测分类与筛选"),
  }, { marker: "description" }));
  const runtimeId = add("agent_runtime", "runtime/offline", { workspace_id: workspace.id,
    name: "桌面场景 · 离线演示设备", runtime_mode: "local", provider: "codex", status: "offline",
    daemon_id: null, owner_id: ownerId, visibility: "private", device_info: "测试数据；未注册 daemon；不会执行任务",
    metadata: json({ fixture_key: FIXTURE_KEY, synthetic: true }),
  }, { marker: "metadata" });
  const agentId = add("agent", "agent/reviewer", { workspace_id: workspace.id,
    name: "桌面场景 · 回归助手", description: marked("展示任务、执行记录与离线状态；成员可查看，只有拥有者可调用。"),
    runtime_mode: "local", runtime_id: runtimeId, runtime_config: json({ fixture_key: FIXTURE_KEY, synthetic: true }),
    visibility: "workspace", permission_mode: "private", status: "offline", owner_id: ownerId,
    instructions: "Synthetic fixture. No executable runtime is registered.", max_concurrent_tasks: 1,
    conversation_starters: json([
      { label: "整理验收清单", prompt: "整理桌面发布前的验收清单。" },
      { label: "检查失败记录", prompt: "说明演示执行记录中的失败原因。" },
    ]),
  }, { marker: "runtime_config" });
  const quickActionId = add("quick_action", "quick-action/review", { workspace_id: workspace.id,
    name: "桌面场景 · 检查验收条件", description: marked("预设操作展示；演示智能体运行时离线。"),
    assignee_type: "agent", assignee_id: agentId, prompt: "检查当前任务的验收条件，列出尚未覆盖的边界场景。",
    visibility: "private", created_by_type: "member", created_by_id: ownerId,
  }, { marker: "description" });
  const iterationId = add("iteration", "iteration/planned", { workspace_id: workspace.id,
    name: "桌面场景 · 下一轮回归", description: marked("计划迭代；与已有活动迭代、历史快照独立。"),
    coordinator_user_id: ownerId, timezone, start_date: day(7), end_date: day(20), created_by: ownerId,
  }, { marker: "description" });
  const titles = [
    ["delivery", "backlog", "补齐桌面端断网后的提示与恢复说明"],
    ["delivery", "todo", "修复收件箱未读计数在刷新后的不同步"],
    ["delivery", "in_progress", "完成任务详情中的父子任务联动"],
    ["delivery", "in_review", "验证用户与智能体混合协作的评论线程"],
    ["delivery", "done", "统一看板与表格的任务优先级展示"],
    ["delivery", "blocked", "等待测试设备恢复后验证离线运行时"],
    ["delivery", "cancelled", "取消重复的旧版通知入口"],
    ["delivery", "backlog", "研究大量任务列表中的搜索体验"],
    ["delivery", "todo", "处理逾期任务的日期筛选与排序"],
    ["delivery", "in_progress", "补全多窗口切换后的工作区上下文"],
    ["delivery", "in_review", "复核长标题在窄窗口中的换行、截断、悬浮提示以及任务详情与列表之间的一致性，确保中文与 English 混排均可读"],
    ["delivery", "done", "完成评论回复和已解决线程展示"],
    ["delivery", "blocked", "等待接口契约确认后接入项目风险提示"],
    ["delivery", "cancelled", "取消已被统一导航取代的独立菜单"],
    ["experience", "backlog", "收集快捷创建流程中的用户反馈"],
    ["experience", "todo", "补充任务搜索的中文关键词"],
    ["experience", "backlog", "设计空项目中的下一步引导"],
    ["experience", "todo", "完善订阅与取消订阅的状态提示"],
    ["experience", "backlog", "整理快捷操作的常用验收模板"],
    ["experience", "todo", "验证无负责人任务的筛选"],
    ["experience", "backlog", "评估标签筛选的组合方式"],
    ["experience", "todo", "增加聊天历史的归档与恢复说明"],
    ["experience", "backlog", "规划未来迭代的范围检查"],
    ["experience", "todo", "核对用户偏好在设备间的展示"],
    ["migration", "blocked", "等待迁移字段的上游确认"],
    ["migration", "todo", "准备迁移失败后的回滚清单"],
    ["migration", "backlog", "分析历史数据中的缺失负责人"],
    ["migration", "blocked", "等待重复任务的业务归并结论"],
    ["migration", "todo", "验证历史标签与项目关联"],
    ["migration", "backlog", "补充跨时区日期的迁移说明"],
    ["migration", "blocked", "等待客户批准下一批数据范围"],
    ["auth", "done", "完成密码登录后的会话恢复"],
    ["auth", "done", "验证拥有者与成员的权限差异"],
    ["auth", "done", "完成退出登录后的页面保护"],
    ["auth", "done", "修复缺失用户资料时的恢复入口"],
    ["auth", "done", "核对工作区切换的权限边界"],
    ["auth", "done", "完成通知收件人的隔离检查"],
    ["auth", "done", "整理身份验证流程的验收结果"],
    ["legacy", "cancelled", "取消旧版桌面欢迎页"],
    ["legacy", "cancelled", "取消重复的项目汇总接口"],
    ["legacy", "cancelled", "撤销旧方案中的自动执行入口"],
    ["legacy", "cancelled", "归档旧版任务列表方案"],
  ];
  if (triageEnabled) titles.push(["experience", "backlog", "待分诊：补充客户反馈的验收条件"], ["experience", "backlog", "已暂缓：等待反馈方提供复现步骤"]);
  const issues = titles.map(([project, status, title], index) => {
    const key = `issue/${String(index + 1).padStart(2, "0")}`;
    const assignee = [1, 4, 5, 6].includes(index) ? agentId : [ownerId, memberId, agentId, null][index % 4];
    const pending = index >= 42;
    const inIteration = iterationEnabled && index < 24 && !["done", "cancelled", "blocked"].includes(status);
    const issueId = add("issue", key, { workspace_id: workspace.id, title: `桌面场景 · ${title}`,
      description: marked(`## 业务背景\n\n${title}。此任务用于验证真实数据在桌面列表、详情和协作入口中的显示。\n\n## 验收步骤\n\n1. 查看任务状态、优先级、负责人和关联项目\n2. 切换列表、看板与分组，确认数据一致\n3. 打开详情，检查评论、订阅与相关任务\n4. 刷新窗口，确认编辑结果仍在\n\n## 边界条件\n\n覆盖未分配任务、空日期、逾期日期和长文本；检查中文与 English 混排。\n\n> 手测可直接编辑这些任务。重复执行数据脚本会保留已保存的编辑。`),
      status, priority: PRIORITIES[index % PRIORITIES.length],
      assignee_type: pending || assignee === null ? null : assignee === agentId ? "agent" : "member", assignee_id: pending ? null : assignee,
      creator_type: "member", creator_id: ownerId, project_id: pending ? null : projects[project],
      parent_issue_id: [8, 9, 10].includes(index) ? id("issue/03") : index === 15 ? id("issue/10") : null,
      acceptance_criteria: json(["任务属性与列表显示一致", "刷新后保存结果保持一致", "无负责人和离线状态可正常展示"]),
      position: index + 1, start_date: pending || index % 3 === 0 ? null : day(-index % 10),
      due_date: pending || index % 6 === 0 ? null : day(index % 4 === 0 ? -3 : index % 4 === 1 ? 1 : 10),
      created_at: at(-1440 * (30 - index % 28)), updated_at: at(-60 * (index + 1)),
      metadata: json({ fixture_key: FIXTURE_KEY, scenario_key: key }), current_iteration_id: inIteration ? iterationId : null,
      admission_status: pending ? "pending" : "not_required",
    }, { marker: "metadata" });
    return { id: issueId, key, inIteration };
  });
  let sequence = 0;
  for (const [index, issue] of issues.entries()) {
    if (index < 42) {
      for (const labelId of [...new Set([labels[index % labels.length], ...(index % 3 === 0 ? [labels[3]] : [])])]) {
        add("issue_to_label", `${issue.key}/label/${labelId}`, { issue_id: issue.id, label_id: labelId }, { anchor: issue.key });
      }
      for (const [userId, reason] of new Map([[ownerId, "creator"], ...(index % 4 === 0 ? [[memberId, "manual"]] : [])])) {
        add("issue_subscriber", `${issue.key}/subscriber/${userId}`, { issue_id: issue.id, user_type: "member", user_id: userId, reason }, { anchor: issue.key });
      }
    } else {
      add("issue_triage", `${issue.key}/triage`, { workspace_id: workspace.id, issue_id: issue.id, reviewer_id: ownerId,
        candidate_project_id: projects.experience, first_entered_at: at(-120), entered_at: at(-120),
        snoozed_until: index === 43 ? at(1440) : null,
      }, { anchor: issue.key });
    }
    if (issue.inIteration) {
      const data = rows.find((row) => row.key === issue.key).data;
      const hasStarted = ["in_progress", "in_review"].includes(data.status);
      add("iteration_participation", `${issue.key}/participation`, { workspace_id: workspace.id, iteration_id: iterationId, issue_id: issue.id,
        first_joined_at: at(0), current_joined_at: at(0), has_started_current_participation: hasStarted,
      }, { anchor: issue.key });
      add("iteration_event", `${issue.key}/join`, { workspace_id: workspace.id, iteration_id: iterationId, issue_id: issue.id,
        sequence: ++sequence, operation_id: id("operation/iteration-scope"), kind: "planned_activity",
        actor: json({ type: "member", id: ownerId, user_id: ownerId }), occurred_at: at(0), sampled_at: at(0),
        before_facts: json(null), after_facts: json({ issue_id: issue.id, title: data.title, project_id: data.project_id,
          assignee_type: data.assignee_type, assignee_id: data.assignee_id, status_key: data.status, status_category: data.status,
          rollover_count: 0, has_started: hasStarted, source_iteration_id: null, target_iteration_id: iterationId }),
      }, { anchor: issue.key });
    }
  }
  rows.find((row) => row.table === "iteration").data.scope_revision = 1 + sequence;
  const comments = [
    [2, "member", ownerId, "请优先验证父子任务的状态和导航，验收后在此线程记录结果。", null, false],
    [2, "member", memberId, "已复核列表与详情，刷新后父子关系保持一致。", 0, false],
    [2, "agent", agentId, "建议再检查无负责人任务和取消状态。此处为合成记录，没有启动执行。", 0, false],
    [3, "member", memberId, "请确认评论回复和通知跳转是否保留上下文。", null, false],
    [3, "member", memberId, "通知已跳转到正确任务，回复线程显示正常。", 3, false],
    [11, "member", ownerId, "已验证长回复展示，可关闭此线程。", null, true],
    [11, "member", memberId, "验证完成：回复、订阅和已解决状态均已保存。", 5, false],
    [5, "member", memberId, "当前设备离线，等待恢复后再继续手工验证。", null, false],
  ];
  const commentIds = comments.map(([issueIndex, authorType, authorId, content, parentIndex, resolved], index) =>
    add("comment", `comment/${index + 1}`, { workspace_id: workspace.id, issue_id: issues[issueIndex].id,
      author_type: authorType, author_id: authorId, content, parent_id: parentIndex === null ? null : id(`comment/${parentIndex + 1}`),
      dispatch_eligible: false, created_at: at(-90 + index * 5), updated_at: at(-90 + index * 5),
      resolved_at: resolved ? at(-20) : null, resolved_by_type: resolved ? "member" : null, resolved_by_id: resolved ? ownerId : null,
    }, { anchor: issues[issueIndex].key }));
  const inboxSpecs = [
    [2, "new_comment", "info", false, false, "父子任务收到新回复", { comment_id: commentIds[1] }],
    [3, "mentioned", "action_required", false, false, "评论线程等待你的确认", { comment_id: commentIds[3] }],
    [8, "issue_assigned", "attention", false, false, "逾期任务需要跟进", { new_assignee_type: "member", new_assignee_id: ownerId }],
    [4, "status_changed", "info", true, false, "优先级展示已验收", { from: "in_review", to: "done" }],
    [11, "new_comment", "info", true, true, "已解决线程已归档", { comment_id: commentIds[5] }],
    [5, "task_failed", "attention", false, false, "演示执行失败，需要查看原因", { task_id: id("task/failed/1") }],
  ];
  for (const [index, [issueIndex, type, severity, read, archived, title, details]] of inboxSpecs.entries()) {
    add("inbox_item", `inbox/${index + 1}`, { workspace_id: workspace.id, recipient_type: "member", recipient_id: ownerId,
      issue_id: issues[issueIndex].id, type, severity, title: `桌面场景 · ${title}`, body: "用于验证未读、已读、归档和通知跳转。",
      read, archived, actor_type: "member", actor_id: memberId,
      details: json({ fixture_key: FIXTURE_KEY, ...details }), created_at: at(-80 + index * 7),
    }, { marker: "details", anchor: issues[issueIndex].key });
  }
  const chatIds = ["active", "archived"].map((status) => add("chat_session", `chat/${status}`, { workspace_id: workspace.id,
    agent_id: agentId, creator_id: ownerId, runtime_id: runtimeId, title: `桌面场景 · ${status === "active" ? "交付验收讨论" : "历史验收记录"}`,
    status, project_id: projects.delivery, created_at: at(-120), updated_at: at(-15), explicitly_created_at: at(-120),
    last_read_at: at(-100), unread_since: status === "active" ? at(-80) : null, pinned_at: status === "active" ? at(-120) : null,
  }));
  const taskSpecs = [
    ["completed", 4, null, "验收通过，优先级展示与刷新结果一致。"],
    ["failed", 5, null, "测试环境不可达，未完成验证。"],
    ["cancelled", 6, null, "旧方案取消后停止了演示任务。"],
    ["queued", 1, null, "离线演示设备上的排队记录。"],
    ["completed", null, chatIds[0], "已整理验收清单。"],
  ];
  const taskIds = taskSpecs.map(([status, issueIndex, chatId, summary], index) => add("agent_task_queue", `task/${status}/${index}`, {
    agent_id: agentId, runtime_id: runtimeId, issue_id: issueIndex === null ? null : issues[issueIndex].id,
    chat_session_id: chatId, status, priority: index, attempt: 1, max_attempts: 1,
    initiator_user_id: ownerId, originator_user_id: ownerId, accountable_user_id: ownerId, originator_source: "initiator",
    context: json({ fixture_key: FIXTURE_KEY, synthetic: true }), trigger_summary: `桌面场景 · ${summary}`,
    result: status === "completed" ? json({ summary }) : null, error: status === "failed" ? "Synthetic fixture: test environment unavailable" : null,
    failure_reason: status === "failed" ? "runtime_unavailable" : null, created_at: at(-70 + index * 5),
    queued_at: at(-70 + index * 5), queued_at_source: "transition",
    dispatched_at: status === "queued" ? null : at(-69 + index * 5), started_at: status === "queued" ? null : at(-68 + index * 5),
    completed_at: status === "queued" ? null : at(-66 + index * 5),
  }, { marker: "context", anchor: issueIndex === null ? "chat/active" : issues[issueIndex].key }));
  for (const [index, taskId] of taskIds.entries()) {
    if (index === 3) continue;
    const messages = [
      ["text", null, "开始检查验收范围（合成执行记录）。", null, null],
      ["tool_use", "Read", null, json({ path: "docs/acceptance.md" }), null],
      ["tool_result", "Read", null, null, "模拟文件内容：列表、详情、刷新和权限边界。"],
      ["text", null, taskSpecs[index][3], null, null],
    ];
    for (const [seq, [type, tool, content, input, output]] of messages.entries()) {
      add("task_message", `task-message/${index}/${seq}`, { task_id: taskId, seq: seq + 1, type, tool, content, input, output,
        created_at: at(-68 + index * 5 + seq / 10),
      }, { anchor: `task/${taskSpecs[index][0]}/${index}` });
    }
  }
  const chatMessages = [
    [0, "user", "请整理桌面发布前的验收清单。", null, []],
    [0, "assistant", "## 验收清单\n\n- 任务列表与详情一致\n- 评论线程与收件箱通知可跳转\n- 项目、计划迭代和权限边界清晰\n\n演示运行时处于离线状态，可以查看这些记录。", null, []],
    [0, "user", "再补充逾期任务和父子任务的验证步骤。", taskIds[4], []],
    [0, "assistant", "先筛选逾期任务，再检查父子关系。刷新后核对状态、负责人和评论线程是否保留。", taskIds[4], [
      { label: "查看边界清单", prompt: "补充无负责人、离线设备和长文本场景。", primary: true },
      { label: "整理交付说明", prompt: "整理一份桌面验收说明。" },
    ]],
    [1, "user", "保存上一轮验收结论。", null, []],
    [1, "assistant", "上一轮身份验证和权限检查已经完成，历史会话已归档。", null, []],
  ];
  chatMessages.forEach(([sessionIndex, role, content, taskId, actions], index) => add("chat_message", `chat-message/${index + 1}`, {
    chat_session_id: chatIds[sessionIndex], role, content, task_id: taskId, quick_actions: json(actions),
    created_at: at(-90 + index * 12), elapsed_ms: role === "assistant" ? 1200 : null,
  }, { anchor: `chat/${sessionIndex === 0 ? "active" : "archived"}` }));
  for (const [key, kind, judgment, body] of [
    ["delivery", "progress", "on_track", "列表、详情与协作流程已覆盖。下一步复核逾期任务和待评审任务。"],
    ["migration", "risk", "risk", "上游字段尚未确认，迁移阶段暂停。确认数据范围后再恢复计划。"],
  ]) {
    const updateId = add("project_update", `project-update/${key}`, { workspace_id: workspace.id, project_id: projects[key],
      author_user_id: ownerId, published_at: at(-180),
    }, { anchor: `project/${key}` });
    add("project_update_revision", `project-update-revision/${key}`, { workspace_id: workspace.id, project_id: projects[key],
      update_id: updateId, revision: 1, editor_user_id: ownerId, created_at: at(-180), kind, health_judgment: judgment,
      body: marked(body), evidence: json([]),
    }, { marker: "body", anchor: `project-update/${key}` });
  }
  for (const [index, target] of [[5, 1], [12, 24], [2, 16]]) {
    add("issue_dependency", `dependency/${index}/${target}`, { issue_id: issues[index].id, depends_on_issue_id: issues[target].id, type: "blocked_by" }, { anchor: issues[index].key });
  }
  return { rows, workspace, ownerId, memberId, agentId, runtimeId, iterationId, projects, issues, chatIds, taskIds, quickActionId,
    skippedScenarios: [...(!iterationEnabled ? ["Iteration assignment disabled; no participation added"] : []),
      ...(!triageEnabled ? ["Triage disabled; pending intake samples omitted"] : [])] };
}

function whereRecord(record) {
  const keys = keysFor(record.table);
  return { sql: keys.map((key, index) => `${quote(key)} = $${index + 1}`).join(" AND "), values: keys.map((key) => record.data[key]) };
}

function assertOwned(record, row, fixture) {
  if (row.workspace_id && row.workspace_id !== fixture.workspace.id) throw new Error(`Fixture identity collision in ${record.table} (${record.key})`);
  if (record.marker) {
    const value = row[record.marker];
    const owned = typeof value === "object" ? value?.fixture_key === FIXTURE_KEY : typeof value === "string" && value.includes(MARKER);
    if (!owned) throw new Error(`Fixture marker missing in ${record.table} (${record.key}); refusing to change this record`);
  }
  if (record.table === "agent_runtime" && (row.daemon_id !== null || row.status !== "offline")) {
    throw new Error("Fixture runtime is connected or online; refusing to manage potentially executable records");
  }
  if (record.table === "agent" && row.runtime_id !== fixture.runtimeId) throw new Error("Fixture agent has been rebound to another runtime");
  if (record.table === "agent_task_queue" && (row.runtime_id !== fixture.runtimeId || row.agent_id !== fixture.agentId)) {
    throw new Error("Fixture execution has been rebound to another agent/runtime");
  }
}

async function loadExisting(client, fixture) {
  const existing = new Map();
  for (const record of fixture.rows) {
    const where = whereRecord(record);
    const result = await client.query(`SELECT * FROM ${quote(record.table)} WHERE ${where.sql}`, where.values);
    if (result.rows[0]) { assertOwned(record, result.rows[0], fixture); existing.set(record.key, result.rows[0]); }
  }
  return existing;
}

async function referenceTables(client) {
  const result = await client.query(`SELECT c.table_name, c.column_name, c.udt_name FROM information_schema.columns c
    JOIN information_schema.tables t USING (table_schema, table_name)
    WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE'
      AND c.udt_name IN ('uuid', '_uuid', 'jsonb', 'json', 'text', 'varchar')
    ORDER BY c.table_name, c.ordinal_position`);
  const tables = new Map();
  for (const column of result.rows) { if (!tables.has(column.table_name)) tables.set(column.table_name, []); tables.get(column.table_name).push(column); }
  return tables;
}

// Scan actual columns, including soft references, JSON history and UUID arrays.
// A title prefix alone never authorizes deletion. New user records pointing at
// the fixture cause refusal, even when PostgreSQL has no FK to protect them.
async function assertNoExternalReferences(client, fixture, tables) {
  const ids = fixture.rows.map((record) => record.data.id).filter(Boolean);
  const owned = new Set(fixture.rows.map((record) => recordKey(record.table, record.data)));
  const foreign = [];
  for (const [table, columns] of tables) {
    const clauses = columns.map((column) => column.udt_name === "uuid" ? `${quote(column.column_name)} = ANY($1::uuid[])`
      : column.udt_name === "_uuid" ? `${quote(column.column_name)} && $1::uuid[]`
        : `${quote(column.column_name)}::text ~ $2`);
    const result = await client.query(`SELECT * FROM ${quote(table)} WHERE (${clauses.join(" OR ")})
      AND $1::uuid[] IS NOT NULL AND $2::text IS NOT NULL`, [ids, ids.join("|")]);
    for (const row of result.rows) if (!owned.has(recordKey(table, row))) foreign.push(`${table}:${row.id ?? row.issue_id ?? row.iteration_id ?? "reference"}`);
  }
  if (foreign.length) throw new Error(`Reset refused: ${foreign.length} non-fixture record(s) reference fixture data (${foreign.slice(0, 8).join(", ")}). Remove those references explicitly before reset.`);
}

function counts(records) {
  const result = {};
  for (const record of records) result[record.table] = (result[record.table] ?? 0) + 1;
  return result;
}

export async function runFixture(client, options) {
  await client.query(options.dryRun ? "BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY" : "BEGIN");
  try {
    await client.query("SET LOCAL statement_timeout = '15s'");
    await client.query("SET LOCAL lock_timeout = '2s'");
    const workspaceResult = await client.query(`SELECT * FROM workspace WHERE slug = $1${options.dryRun ? "" : " FOR UPDATE"}`, [options.workspace]);
    const workspace = workspaceResult.rows[0];
    if (!workspace) throw new Error(`Workspace ${options.workspace} does not exist; this script never creates workspaces`);
    const members = (await client.query(`SELECT m.user_id, m.role FROM member m JOIN "user" u ON u.id = m.user_id
      WHERE m.workspace_id = $1 ORDER BY CASE WHEN m.role = 'owner' THEN 0 ELSE 1 END, m.created_at, m.user_id`, [workspace.id])).rows;
    const ownerId = members.find((member) => member.role === "owner")?.user_id;
    if (!ownerId) throw new Error("Target workspace has no existing owner");
    const settings = (await client.query(`SELECT
      COALESCE((SELECT enabled FROM workspace_iteration_settings WHERE workspace_id = $1), true) AS iterations,
      COALESCE((SELECT enabled FROM workspace_triage_settings WHERE workspace_id = $1), true) AS triage,
      clock_timestamp() AS sampled_at`, [workspace.id])).rows[0];
    const fixture = buildFixture({ workspace, ownerId, memberId: members.find((member) => member.user_id !== ownerId)?.user_id ?? ownerId,
      now: settings.sampled_at, timezone: workspace.planning_timezone ?? "Asia/Shanghai",
      iterationEnabled: options.reset || settings.iterations, triageEnabled: options.reset || settings.triage });
    let tables;
    if (options.reset) {
      tables = await referenceTables(client);
      if (!options.dryRun) await client.query(`LOCK TABLE ${[...tables.keys()].map(quote).join(", ")} IN SHARE ROW EXCLUSIVE MODE NOWAIT`);
    }
    const existing = await loadExisting(client, fixture);
    const created = new Set();
    const wouldCreate = [];
    const preserved = [];
    if (options.reset) {
      await assertNoExternalReferences(client, fixture, tables);
      if (!options.dryRun) {
        for (const record of fixture.rows.toReversed()) {
          if (!existing.has(record.key)) continue;
          const where = whereRecord(record);
          await client.query(`DELETE FROM ${quote(record.table)} WHERE ${where.sql}`, where.values);
        }
      }
    } else {
      for (const record of fixture.rows) {
        if (existing.has(record.key)) continue;
        // The first seed is atomic. Missing records afterward were deleted by
        // a human, not left by a partial seed; preserve those deletions too.
        if (existing.size > 0 || (record.anchor && !created.has(record.anchor))) { preserved.push(record); continue; }
        wouldCreate.push(record); created.add(record.key);
      }
      if (!options.dryRun) {
        const newIssues = wouldCreate.filter((record) => record.table === "issue");
        if (newIssues.length) {
          const counter = (await client.query(`UPDATE workspace SET issue_counter = GREATEST(issue_counter,
            COALESCE((SELECT max(number) FROM issue WHERE workspace_id = $1), 0)) + $2
            WHERE id = $1 RETURNING issue_counter`, [workspace.id, newIssues.length])).rows[0].issue_counter;
          newIssues.forEach((record, index) => { record.data.number = counter - newIssues.length + index + 1; });
        }
        for (const record of wouldCreate) {
          const columns = Object.keys(record.data);
          await client.query(`INSERT INTO ${quote(record.table)} (${columns.map(quote).join(", ")}) VALUES
            (${columns.map((_column, index) => `$${index + 1}`).join(", ")})`, columns.map((column) => record.data[column]));
        }
      }
    }
    const history = (await client.query(`SELECT i.id, i.name, i.status, EXISTS(SELECT 1 FROM iteration_snapshot s
      WHERE s.iteration_id = i.id AND s.workspace_id = i.workspace_id) AS has_snapshot FROM iteration i
      WHERE i.workspace_id = $1 AND i.id <> $2 AND i.status IN ('active','completed') ORDER BY i.start_date DESC`, [workspace.id, fixture.iterationId])).rows;
    const summary = {
      fixture_key: FIXTURE_KEY, action: options.reset ? "reset" : "seed", dry_run: options.dryRun,
      database: (await client.query("SELECT current_database() AS name")).rows[0].name,
      workspace: { id: workspace.id, slug: workspace.slug }, owner_id: ownerId,
      expected: counts(fixture.rows), before: counts(fixture.rows.filter((record) => existing.has(record.key))),
      inserted: options.reset ? {} : counts(wouldCreate), deleted: options.reset ? counts(fixture.rows.filter((record) => existing.has(record.key))) : {},
      preserved_missing_children: counts(preserved), skipped_scenarios: fixture.skippedScenarios,
      ids: { projects: fixture.projects, planned_iteration: fixture.iterationId, existing_iterations: history,
        agent: fixture.agentId, offline_runtime: fixture.runtimeId, issue_with_replies: fixture.issues[2].id,
        long_title_issue: fixture.issues[10].id, chats: fixture.chatIds, executions: fixture.taskIds, quick_action: fixture.quickActionId },
      paths: ["issues", "projects", "iterations", "triage", "inbox", "agents"].map((path) => `/${workspace.slug}/${path}`),
    };
    await client.query(options.dryRun ? "ROLLBACK" : "COMMIT");
    return summary;
  } catch (error) {
    await client.query("ROLLBACK");
    throw error;
  }
}

async function main() {
  const options = optionsFrom(process.argv.slice(2));
  if (options.help) { process.stdout.write(help); return; }
  const client = new pg.Client({ connectionString: options.connectionString, application_name: FIXTURE_KEY, connectionTimeoutMillis: 5000 });
  try {
    await client.connect();
    process.stdout.write(`${JSON.stringify(await runFixture(client, options), null, 2)}\n`);
  } finally { await client.end(); }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => { process.stderr.write(`seed-rich-test-data: ${error.message}\n`); process.exitCode = 1; });
}
