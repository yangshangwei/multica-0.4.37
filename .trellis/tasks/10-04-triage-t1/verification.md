# T1 acceptance ledger

Status: not implemented. Baseline: core 201 suites / 2,403 tests passed; isolated PostgreSQL migrations through 511 applied.

| Requirement | Scenario | Expected | Evidence | Status |
| --- | --- | --- | --- | --- |
| TRI-AC-01 | 默认工作空间尚未启用 | 普通任务流程保持原样，无分拣导航 | Pending | Not started |
| TRI-AC-02 | admin 启用分拣 | 入口出现，历史正式任务不回填 | Pending | Not started |
| TRI-AC-03 | member 修改开关 | 操作被拒绝且原设置不变 | Pending | Not started |
| TRI-AC-04 | 成员导入 CSV | 有效行进入待分拣，每条有稳定编号，来源记录导入批次和行号 | Pending | Not started |
| TRI-AC-05 | 未授权来源提交，或无任务权限的成员导入 | 不因开关开启而成功创建 | Pending | Not started |
| TRI-AC-06 | 导入行带 `in_progress` 状态、迭代、智能体负责人和 @提及 | 仍待分拣，状态和迭代报告未采用，智能体只作候选；不加入迭代、不执行 | Pending | Not started |
| TRI-AC-07 | 同一导入批次确认后网络中断并重试 | 每行只有一条任务，批次只有一条汇总通知 | Pending | Not started |
| TRI-AC-08 | 成员从项目普通创建任务 | 保持既有流程，不无条件被拦入分拣 | Pending | Not started |
| TRI-AC-09 | 优先级要求开启，priority 为 none | 接受失败并定位优先级字段 | Pending | Not started |
| TRI-AC-10 | 合法普通接受带项目和负责人 | 准入与字段同时生效，项目计数增加，不启动执行 | Pending | Not started |
| TRI-AC-11 | 接受时目标项目失效 | 不接受、不保存半套属性，保留用户输入 | Pending | Not started |
| TRI-AC-12 | 接受并执行合法且连续重试 | 只准入一次、只启动一次执行 | Pending | Not started |
| TRI-AC-13 | 已接受后执行启动失败 | 保持已接受，显示未启动和重试执行入口 | Pending | Not started |
| TRI-AC-14 | 重复目标是自身或其他空间任务 | 不允许提交，不泄漏目标内容 | Pending | Not started |
| TRI-AC-15 | 标记为正式已结束任务的重复输入 | 可关联且保留原输入，不复制评论附件、不改目标 | Pending | Not started |
| TRI-AC-16 | 拒绝原因只有空格 | 拒绝未提交，提示填写原因 | Pending | Not started |
| TRI-AC-17 | 正常拒绝后查看普通项目进度 | 不计为项目正式范围或交付 | Pending | Not started |
| TRI-AC-18 | 任务稍后至未来时刻 | 所有成员默认队列隐藏，可在稍后列表看到 | Pending | Not started |
| TRI-AC-19 | 稍后到期 | 恢复队列，不改变截止日期和原始进入时间 | Pending | Not started |
| TRI-AC-20 | 稍后期间任务已被接受 | 到期不重新进入分拣 | Pending | Not started |
| TRI-AC-21 | 稍后期间收到新评论 | 保留稍后状态，评论通知与审核结果独立 | Pending | Not started |
| TRI-AC-22 | 收件箱通知被归档或稍后处理 | 分拣状态和任务稍后时间不变 | Pending | Not started |
| TRI-AC-23 | 责任人被指定或离开 | 不替换执行负责人；离开后任务仍可由成员处理 | Pending | Not started |
| TRI-AC-24 | 批量接受部分任务发生冲突 | 展示逐项结果，重试只处理失败项 | Pending | Not started |
| TRI-AC-25 | 已拒绝任务重新审核 | 新轮次待分拣，旧理由和操作者仍可追溯 | Pending | Not started |
| TRI-AC-26 | 已接受任务请求重新分拣 | 第一阶段不支持，不暗中撤销执行 | Pending | Not started |
| TRI-AC-27 | 两成员同时处理 | 只有一个生效结果，没有重复通知或覆盖 | Pending | Not started |
| TRI-AC-28 | 待分拣任务经 CLI、旧客户端或@提及启动 | 服务端拒绝并说明待审核原因 | Pending | Not started |
| TRI-AC-29 | 全部未决任务均在稍后列表，admin 关闭 | 仍阻止关闭，并显示未决数量 | Pending | Not started |
| TRI-AC-30 | 最后一条处理成功或队列筛选无结果 | 空队列与筛选无结果文案可区分，键盘焦点合理 | Pending | Not started |
| TRI-AC-31 | 用户在描述输入框按数字或 J／K | 不触发审核快捷键 | Pending | Not started |
| TRI-AC-32 | 已失去访问权限的用户打开结果通知 | 无法读取任务、目标或附件内容 | Pending | Not started |
| TRI-AC-37 | 分拣台已关闭，从历史链接、API 或旧客户端重新审核或主动投递 | 拒绝产生新的待分拣任务，保留原历史结果并提示先启用 | Pending | Not started |
| TRI-AC-38 | 导入文件含缺标题、日期非法和越权项目的行 | 预览逐行标出原因且不写入任务；确认后只创建有效行，越权项目留空并标明，失败行可下载 | Pending | Not started |
| TRI-AC-39 | 导入行的外部编号已导入过，或标题与正式任务相似 | 外部编号相同默认跳过并标明；标题相似只提示重复候选，不静默丢弃 | Pending | Not started |
| TRI-AC-40 | 文件超出行数或大小上限，或不是 UTF-8 编码 | 不进入预览，说明上限或编码要求，不产生任何任务 | Pending | Not started |
| TRI-AC-41 | 分拣台未启用时通过页面或 API 导入 | 不显示导入入口，API 拒绝并提示先启用 | Pending | Not started |

Additional mandatory checks: package lint/typecheck, Go vet and tests, safe migration rollback rehearsal, boundary scan, malformed API response, two-client race, CSV limits baseline, Web/Desktop browser evidence.

## FR-only proof obligations

| Requirement | Required proof | Status |
| --- | --- | --- |
| TRI-FR-04 | Every specified filter, stable oldest/newest/priority tie ordering and independent global counts | Not started |
| TRI-FR-10 | Duplicate selection by pasted link and formal closed task; deleted target keeps reference | Not started |
| TRI-FR-12/13 | One hour/tomorrow/next week/custom presets display actual instant+timezone; cancel/reset snooze audit | Not started |
| TRI-FR-15/18 | none/notify/assign modes, human reviewer identity, reassignment and current-member notification dedup | Not started |
| TRI-FR-16 | Both batch endpoints reject every non-whitelisted action, explicit valid-only selection and categorized failures | Not started |
| TRI-FR-17/26 | Global action/round history survives reopen and contains CSV batch summaries | Not started |
| TRI-FR-03/09/26 | Consumed keys survive issue deletion; terminal task and deleted imported row cannot be recreated by retries | Not started |
| TRI-FR-19 | Delayed pending-era mention never executes after acceptance; legacy agent identity cannot review | Not started |
