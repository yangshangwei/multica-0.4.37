# 评论表情操作术语统一

轻量文案规划；来源 `11-18-security-remediation-v2-template/design.md` 14.2 L5-a，2026-11-20 按用户批准建议独立跟踪。

## Goal

消除评论页、收件箱、协作界面“表情回应/表情反应”混用，让产品说明与实际按钮、通知词汇一致。

## Requirements

- 先统计 zh-Hans 文案、中文 MDX、内置 skill 与术语表中的用法，明确统一词及英文 reaction 对应。
- 同一次变更更新 `developers/conventions*.mdx`、受影响文案及中文生成文档包；不改历史记录、API 字段、评论权限与 revision 语义。
- 安全小队的放行警告与链接保留。

## Acceptance Criteria

- [ ] 当前用户可见面采用已定术语，残留检索列出仅限历史例外。
- [ ] 生成包无漂移，相关文档/目录测试、lint/typecheck 通过，无新增 key 漏译。

本轮不决定全站术语，不实施文案批量替换。
