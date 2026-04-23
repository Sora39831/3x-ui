Task Record: Resolve shared-mode traffic flush blocked by stale inboundId=0 delta

Date: 2026-04-24
Related Module: web/service/traffic_flush.go, web/web.go, web/job/xray_traffic_job.go — 流量刷盘
Change Type: Fix

Background

共享模式下流量统计始终为 0，MariaDB 的 `client_traffics` 表从未被写入。排查发现 `traffic-pending.json` 中存在一个残留的 `inboundId: 0` 客户端流量 delta（在 InboundId 解析修复前产生）。`flushToDatabase()` 尝试将其写入 `client_traffics` 时，违反外键约束 `fk_inbounds_client_stats`（`inbounds` 表不存在 `id=0`），导致整个事务回滚，所有流量永远无法写入。

此外，`NewXrayTrafficJob()` 和 `startTrafficFlushLoop()` 各自创建了独立的 `TrafficPendingStore` 实例，指向同一个 `traffic-pending.json` 文件但使用独立的 `sync.Mutex`，存在数据竞争风险。

Changes

- `web/service/traffic_flush.go`:
  - `flushToDatabase()` 循环开头新增 `InboundID == 0` 检查，跳过无效 delta 并记录 warning 日志
- `web/job/xray_traffic_job.go`:
  - `NewXrayTrafficJob()` 改为接受 `*service.TrafficPendingStore` 参数，不再自行创建 store
  - 移除 `config` 包依赖
- `web/web.go`:
  - `Server` struct 新增 `trafficStore *service.TrafficPendingStore` 字段
  - `Start()` 中统一创建一个 `TrafficPendingStore` 实例
  - `startTask()` 和 `startTrafficFlushLoop()` 共享同一个 store 实例，消除双实例竞争
- `web/service/traffic_flush_test.go`:
  - 新增 `TestFlushOnceSkipsZeroInboundIdDelta` 测试

Impact

- `web/service/traffic_flush.go`: flushToDatabase() 跳过无效 delta
- `web/web.go`: Server 启动流程变更，store 统一创建
- `web/job/xray_traffic_job.go`: 构造函数签名变更
- 修复后需要删除残留的 `traffic-pending.json` 文件才能生效

Verification

- `go test ./web/service/ -run TestTraffic -v` — PASS
- `go test ./web/service/ -run TestFlushOnceSkipsZeroInboundIdDelta -v` — PASS

Risks And Follow-Up

- 部署时必须删除 `/etc/x-ui/traffic-pending.json`，否则残留的 `inboundId: 0` delta 仍会被跳过（不影响功能，但会产生 warning 日志）
- `TrafficPendingStore` 的文件级锁已通过共享实例解决，但如果未来有多个进程访问同一文件，仍需考虑进程级锁
