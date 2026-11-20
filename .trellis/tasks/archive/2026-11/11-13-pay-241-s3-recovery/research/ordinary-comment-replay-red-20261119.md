# 普通评论原事件重复通知 RED（2026-11-19）

结论：**真实复现同一评论事件的重复 inbox 交付**。7 个顶层、22 个叶场景、29 个命名记录完整终结：6 PASS / 23 FAIL / 0 SKIP。失败是期望捕获的产品缺口；没有初始化、编译、缺名、重复终结或源码/配置漂移。父24例计数仍为4 PASS /13 PARTIAL /7 NOT_RUN。

主会话在新专库 `multica_pay241_s3n_20261119_uwkcehly_red_r02` 运行：

```text
go test -json -mod=readonly -count=1 -run '^TestNotificationCommentReplay' ./cmd/server
```

源码为已接受R08后端的新私有 `source/server-red-r01`，仅新增 `cmd/server/notification_comment_replay_test.go`。测试文件 SHA256 `5bc0b5cdad500f33127d1b2ccec80326f055c38debfdf460109b8948266474a4`；先执行的主会话闭合环境编译 `notification-red-compile-r01` exit0，无TestMain/测试体。

## 原始观察

- 两个真实订阅者首次各得到一条原评论通知，共2行；顺序重投同一事件后变成4行，`inbox:new` 同样由2次变成4次。相同UUID大小写、map载荷与重新注册bus的重放均捕获同一缺口。
- 两个发布者在真实SQL插入前会合，然后转发未替换的SQL/参数/结果；共用与独立bus均重复交付。计数之外同时观察了通知和吞掉的SQL错误/监听器panic。
- 同原评论UUID的新载荷不能保留原read/archive状态与完整原行；已有随机ID及历史重复记录也被追加新行。不同评论/收件人仍应分别交付，不能以正文去重。
- 旧无效/空/零UUID的随机UUIDv7路径、泛化状态通知writer及普通作者/退订/Agent/非订阅者/静音边界控制通过。它们不具有新的原事件去重保证。

## 原件与环境

私有证据根：`/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly`。

- `verification/notification-red-r01/{command,source-before,source-after,result}.json` 记录完整argv、PID、私有配置/工具/源码哈希与退出码。
- `verification/notification-red-r01/stdout.log` 原Go JSON SHA256：`8e0f2589a84401bc73b2f6d10fb424f291a4dc49a40f6b3acd91fcda19c074f1`。
- `audit/notification-red-diagnosis-r01.json` 保存原失败观察、完整分母和退出后核账；`audit/notification-red-after-r01.json` 保存855迁移、663有效索引及所有表计数。
- 原gate保持epoch2、creation=true、execution=false、host=false。退出后连接0、业务夹具行归零；3条真实 `collaboration_config_invalidation` 历史保留，不删历史来声称所有表为空。

原RED源码、库和失败原件保留。唯一服务端owner已接续新的 `source/server-green-r01`，修改范围仅通知producer、专用inbox查询及其sqlc生成文件；测试保持原字节与分母。后续normal/race使用另外两个新库，独立接受后才整合ROOT及更新已覆盖子断言。
