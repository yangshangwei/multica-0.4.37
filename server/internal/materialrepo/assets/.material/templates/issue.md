---
schema_version: 1
id: "<issue-id>"
type: issue
title: "<问题标题>"
category: unknown
discovered_release: null
affected_releases: []
tested_repositories: []
tested_builds: []
requirement_refs: []
---

# <问题标题>

> 模板：发布前替换占位符并补充真实依据。以下不是已完成或已验收的业务记录。

## 现象与影响
<保留原始报告；推测与已确认事实分开>

## 复现
<前置条件、操作、预期与实际>

## 被测版本与证据
<发现版本、确认影响版本、完整代码SHA、构建、环境和原始证据引用>

计划修复范围由 releases/<version>/release.yaml 记录；已验证修复版本由 resolution.md 与复测证据记录。未知版本不推断为最新版本。
