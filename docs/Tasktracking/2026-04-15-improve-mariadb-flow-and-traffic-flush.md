Task Record: Improve MariaDB flow, DB settings init, and traffic flush

Date: 2026-04-15
Related Module: install.sh, x-ui.sh, config, database, web/service — MariaDB 安装/切换/流量刷盘
Change Type: Feature

Background

MariaDB 安装和切换流程存在多个问题：安装脚本逻辑分散、数据库设置初始化不完整、流量刷盘服务存在 bug。本次提交对整个 MariaDB 相关流程进行了大规模重构和修复。

Changes

- `install.sh`（+598/-行重构）:
  - 重构 MariaDB 安装/切换流程，统一本地和远程 MariaDB 配置路径
  - 新增 MariaDB 业务用户/数据库创建逻辑
  - 改进卸载流程，支持部分安装状态下的清理
  - 新增 `tests/mariadb_install_switch_test.sh` 测试覆盖
- `x-ui.sh`（+422/-行重构）:
  - 重构数据库管理菜单，支持本地/远程 MariaDB 切换
  - 新增 MariaDB 端口校验、远程访问管理等菜单项
- `config/config.go`:
  - 新增 `GetDBConfigFromJSON()` 读取数据库连接配置
  - 新增 `readGroupedString()`/`readGroupedInt()` 通用配置读取辅助函数
  - 配置别名映射支持多分组查找
- `database/shared_state.go`:
  - 改进版本号操作的事务安全性
- `web/service/traffic_flush.go`:
  - `Collect()` 新增 inbound-only 残留流量 delta 计算（inbound 总量 - 客户端总量）
  - `flushToDatabase()` 改进 UPSERT 逻辑，支持 MariaDB 的 `ON DUPLICATE KEY UPDATE`
  - 新增 `ReconcileSharedTrafficState()` 调用（auto-renew/disable 过期客户端）
- `web/service/traffic_pending.go`:
  - 改进 `Merge()` 语义，支持按 `(kind, inboundId, email)` 键去重合并
- `main.go`:
  - 新增 `NodeConfig` 启动校验入口
  - 改进 MariaDB 连接初始化流程
- `update.sh`: 更新脚本适配新的安装流程

Impact

- `install.sh`: 大规模重构，影响所有 MariaDB 安装/切换/卸载路径
- `x-ui.sh`: 数据库管理菜单重构
- `config/config.go`: 新增配置读取辅助函数
- `web/service/traffic_flush.go`: 流量刷盘逻辑改进
- `web/service/traffic_pending.go`: delta 队列合并语义改进
- `main.go`: 启动流程增加节点配置校验

Verification

- `go test ./config/ -v` — PASS
- `go test ./database/ -v` — PASS
- `go test ./web/service/ -run TestTraffic -v` — PASS
- `go test ./main_test.go -v` — PASS
- `bash -n install.sh` — syntax OK
- `bash -n x-ui.sh` — syntax OK
- `bash tests/mariadb_install_switch_test.sh` — PASS

Risks And Follow-Up

- 安装脚本改动量大，需要在多种发行版上验证
- 流量刷盘的 `inboundId: 0` 残留问题尚未处理（后续 commit 修复）
- 配置读取辅助函数的分组别名映射需要与 JSON 结构保持同步
