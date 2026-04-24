Task Record: Detect stale mariadb service file when server package is missing

Date: 2026-04-23
Related Module: install.sh, x-ui.sh — has_local_mariadb_service()
Change Type: Fix

Background

用户在安装过程中 MariaDB 重启失败。排查发现 systemd unit 文件存在于 `systemctl list-unit-files` 输出中，但 `mariadb-server` 包实际已被卸载，服务文件是残留状态。`has_local_mariadb_service()` 只检查了 unit 文件是否存在，未验证包是否已安装，导致跳过了服务器重新安装。

Changes

- `has_local_mariadb_service()`（install.sh + x-ui.sh）：在检测到 unit 文件后，追加 `dpkg -s mariadb-server`（Debian/Ubuntu）或 `rpm -q mariadb-server`（RHEL/Fedora）验证包是否已安装。包不存在时返回 1，触发重新安装。

Impact

- install.sh: `has_local_mariadb_service()`
- x-ui.sh: `has_local_mariadb_service()`
- 不影响正常安装流程，仅在包被卸载但 unit 文件残留时触发重新安装
- 无 API、数据库、配置变更

Verification

- `bash -n install.sh` — syntax OK
- `bash -n x-ui.sh` — syntax OK
- `bash tests/mariadb_install_switch_test.sh` — PASS
- `bash tests/mariadb_admin_empty_password_test.sh` — PASS
- `bash tests/install_uninstall_resilience_test.sh` — PASS

Risks And Follow-Up

- 无风险。仅增加包安装状态检查，不影响已有逻辑
- Arch/Alpine 等非 dpkg/rpm 发行版保持原行为（仅检查 unit 文件）
