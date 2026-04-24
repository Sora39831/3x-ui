Task Record: Multi-node shared control — Go backend foundation

Date: 2026-04-10
Related Module: config, database, web/service — multi-node architecture
Change Type: Feature

Background

需要支持多个 3x-ui 面板实例共享同一个 MariaDB 数据库，由 master 节点管理配置，worker 节点同步配置并上报流量。为此引入节点角色配置、共享元数据模型、写入保护和版本同步机制。

Changes

- `config/config.go`: 新增 `NodeRole`、`NodeConfig` 结构体，`GetNodeConfigFromJSON()` 读取节点角色/ID/同步间隔/流量刷盘间隔，`ValidateNodeConfig()` 校验配置合法性
- `database/model/node_state.go`: 新增 `NodeState` 模型，记录每个节点的心跳、同步状态、错误信息
- `database/model/shared_state.go`: 新增 `SharedState` 模型，键值对 + 版本计数器，用于缓存失效检测
- `database/shared_state.go`: `BumpSharedAccountsVersion()` 原子递增版本号，`GetSharedAccountsVersion()` 读取当前版本，`UpsertNodeState()` 更新节点状态
- `database/db.go`: MariaDB 模式下自动迁移 `SharedState` 和 `NodeState` 表，seed 版本行
- `web/service/node_guard.go`: `IsWorker()`/`IsMaster()`/`RequireMaster()`/`IsSharedModeEnabled()` 角色判断和写入保护
- `web/service/inbound.go`: 所有写操作（AddInbound/DelInbound/UpdateInbound/AddInboundClient 等）前调用 `ensureSharedWriteAllowed()`，写操作内 `bumpSharedVersion(tx)` 原子递增版本号
- `web/service/node_sync.go`: `NodeSyncService` — worker 轮询版本号变化 → 加载快照 → 缓存到本地 → 应用到 Xray；master 心跳循环
- `web/service/node_cache.go`: `SharedAccountsSnapshot` 序列化/反序列化到本地 JSON 缓存
- `web/service/traffic_flush.go`: `TrafficFlushService` — 收集流量 delta → 写入持久化队列 → 定时刷盘到 MariaDB
- `web/service/traffic_pending.go`: `TrafficPendingStore` — 基于文件的持久化 delta 队列，支持 merge 语义
- `web/job/xray_traffic_job.go`: 共享模式下走 `TrafficFlushService.Collect()` 路径，非共享模式走原有 `AddTraffic()` 路径
- `web/web.go`: 新增 `startNodeLoops()` 和 `startTrafficFlushLoop()` 启动入口
- `x-ui.sh`: 新增节点管理菜单（设置角色/ID/同步间隔/流量刷盘间隔）
- `install.sh`: 安装流程中增加节点角色配置提示
- `README.md`: 新增多节点共享控制文档

Impact

- 新增数据库表：`node_states`、`shared_states`
- 新增配置项：`nodeRole`、`nodeId`、`syncInterval`、`trafficFlushInterval`
- 修改 `inbounds` 和 `client_traffics` 写入流程，增加共享写入保护
- 新增流量持久化队列文件：`traffic-pending.json`
- 不影响非 MariaDB 模式的现有行为

Verification

- `go test ./config/ -v` — PASS
- `go test ./database/ -v` — PASS
- `go test ./web/service/ -run TestNode -v` — PASS
- `go test ./web/service/ -run TestTraffic -v` — PASS
- `bash -n install.sh` — syntax OK
- `bash -n x-ui.sh` — syntax OK

Risks And Follow-Up

- worker 节点需要配置 `nodeId` 和 `mariadb` 数据库类型，否则启动校验失败
- 流量刷盘依赖 `traffic-pending.json` 文件，磁盘故障可能导致 delta 丢失
- 后续需要处理残留 `inboundId: 0` delta 导致外键约束失败的问题（已在后续 commit 中修复）
