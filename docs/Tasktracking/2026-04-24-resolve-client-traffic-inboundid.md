Task Record: Resolve client traffic InboundId from DB in shared mode

Date: 2026-04-24
Related Module: web/service/traffic_flush.go, web/job/xray_traffic_job.go — 共享模式流量收集
Change Type: Fix

Background

共享模式（MariaDB 多节点）下，Xray gRPC Stats API 返回的客户端流量只包含 email，不包含 InboundId（始终为 0）。`Collect()` 函数直接使用了这个 `InboundId: 0`，导致流量无法正确关联到 inbound，写入数据库时违反外键约束或写入错误的 inbound。

Changes

- `web/service/traffic_flush.go`:
  - `Collect()` 新增 `emailToInboundID` 映射：在处理客户端流量前，先从 `client_traffics` 表查询所有 email 对应的 `inbound_id`
  - 用查询到的真实 `InboundId` 替换 Xray API 返回的 `InboundId: 0`
  - 未知 email（数据库中无对应记录）跳过并记录 warning 日志
  - 新增测试用例：`TestCollectResolvesInboundIdFromDB`、`TestCollectSkipsUnknownEmail`、`TestCollectClampsNegativeResidualAndLogsDetailedWarning`
- `web/job/xray_traffic_job.go`:
  - 共享模式下跳过 `addClientTraffic()`（因为 `Collect()` 已处理），改为手动计算并设置在线客户端列表
- `web/service/inbound.go`:
  - 新增 `SetOnlineClients()` 和 `GetOnlineClients()` 方法，供共享模式设置在线状态
- `x-ui.sh`:
  - 节点配置菜单增加 `trafficFlushInterval` 输入提示

Impact

- `web/service/traffic_flush.go`: Collect() 逻辑变更，影响所有共享模式节点的流量收集
- `web/job/xray_traffic_job.go`: 共享模式的在线客户端检测逻辑
- 不影响非共享模式（SQLite/单节点 MariaDB）

Verification

- `go test ./web/service/ -run TestCollect -v` — PASS
- `go test ./web/service/ -run TestTraffic -v` — PASS

Risks And Follow-Up

- 如果 `client_traffics` 表为空（首次部署），所有客户端流量都会被跳过，直到第一个 inbound 被创建并产生 `client_traffics` 行
- 旧的 `inboundId: 0` 残留 delta 仍可能存在于 `traffic-pending.json` 中（后续 commit 修复）
