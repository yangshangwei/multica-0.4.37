# V0.8.8 正式服务端运行及公开回执复核

日期：2026-11-26（Asia/Shanghai）。结论：**PASS：正式归档的运行验收具有实际执行证据，公开摘要可用于交付。** 本结论不代替待完成的 Release CI/线上全部资产/最终总校验门槛。

归档 SHA-256：eedd38f3ef0237f202bffc1c7ca555c94d2d757eba297c663fa6c4f090254e64，源码 d2c9240eaf9f768446faba93cac625bd63e9cf53。此前静态复核的同一包和唯一 CI feed 进入本次运行，非预览替代。

## 实际运行证据

读取全部12阶段命令回执，exit0、完整源码、归档、harness SHA绑定相同，日志摘要逐一重算相同。审阅 acceptance.py 的实际断言，而非仅接受 passed 字段：

- password/device 基线均为真实v0.6.7包：594旧账本；升级后877完整版本名，新增283。原594行applied_at逐行保留。
- 两模式 PostgreSQL 实际 SHOW server_version_num 从170011到170012；对应旧/新镜像manifest及同一持久卷。由原升级脚本和Compose触发，没有手动重建PG伪造结果。
- /healthz、旧session、新password/device登录、原用户/工作空间/问题/附件内容、显式迭代关闭、旧列删除均有断言。真实运行容器13项认证/密钥/UUID/新配置逐字段等于私有fixture，DATABASE_URL等于实际Compose解析值；原overlay顺序、模板、命名卷和bind保留。
- 升级前dump恢复至隔离空库，核对旧594账本、问题和删除列哨兵，再以真实旧backend启动并验证原session/登录/问题。不是旧镜像连接新schema冒充回滚。
- 三部署实际restart记录StartedAt变化；账本含时间戳、schema和登录保留。三份最终DDL原字节SHA均4e33496977c8ef2d63d8b5b7593d39b15b566564722a6b52872263514a9f29a9，877版本完整，invalid/unready/notlive索引为空。归一化仅移除pg_dump注释和随机restrict标记，不丢业务DDL。
- fresh关闭注册，初始化凭据0600/单链接，重复初始化不改凭据；初次必须改密、相同临时密码拒绝、改密后旧密码拒绝，admin role/overview/users/installations实际响应通过。embedded feed与正式包一致。
- 原入口注入错误SQL真实exit1，日志有注入异常且没有Starting server，故障版本未记账。移除仅测试overlay错误后healthz恢复、877账本时间戳不变；单独删除001_init并直接启动API时/health可达、/healthz503，实际账本876。
- 112条SQL observation的query/result哈希均逐条核验。实际扩展为pg_trgm1.6、pgcrypto1.3、plpgsql1.0，故障演练原迁移日志也保留pg_bigm/pg_cron不可用及371条件跳过。不能称所有可选扩展分支都执行。

## 清理和公开投影

在上述证据核验后通知父会话允许清理3个专属formal项目。后续cleanup命令exit0、日志摘要匹配，仅password/device/fresh各自容器/网络/卷删除；私有文件证据保留，没有清理其他会话资源。

公开server-verification.json SHA-256：8125cf2d158a12bc03fe5ac33fb97310346cd8007bf3513dfacdd6a2b56f7ae3。14份引用证据摘要逐个匹配。公开字段经过allowlist，无本机路径/测试UUID；在不输出值的前提下比对41个真实fixture凭据/身份值均未出现。仅公开配置字段名+相等布尔，不含环境值、备份、私有原日志或管理员密码。

## 上传助手复核

resume-parent/publish-assets.py使用显式文件计划，验证完整source、approved标记、唯一basename、禁止symlink、大小与SHA；全部已有同名远端资产先做大小+digest比较，有冲突在上传前失败。gh upload不带clobber，并对每个文件进行完整下载及hash/size核验、最后再读取远端digest。没有覆盖异内容资产的路径。

助手只核查Release targetCommitish，实际tag解引用仍由父会话在执行前单独确认；已提醒保持这条门槛。助手无源码编辑，本轮未运行它或修改远端。

## 验证范围

本轮只读保存的原始回执、源码harness与日志摘要，未重复Docker/build/DB/全仓测试。相同源码lint/typecheck/main CI通过证据此前已核验。环境是Mac ARM64 Docker Desktop amd64仿真，不是客户物理Linux内网；Namespace、商业provider、可选扩展分支仍保留原限制。

机器记录：主目录.artifacts/release-v0.8.8/resume-review/formal-runtime-review.json。等待最终dist目录和远端附件完成后复核最终交付状态。
