Task Record: Fix MariaDB restart error diagnostics

Date: 2026-04-23
Related Module: install.sh, x-ui.sh — MariaDB service management
Change Type: Fix

Background

用户在安装过程中选择本地 MariaDB 后，遇到 "重启 MariaDB 失败，请检查配置文件" 错误。该错误信息过于笼统，因为 `restart_mariadb_service()` 和 `start_mariadb_service()` 函数使用 `2>/dev/null` 抑制了 stderr，导致 systemctl 返回的实际错误信息被隐藏，无法定位根因。

Changes

- `restart_mariadb_service()`（install.sh + x-ui.sh）：移除 stderr 抑制，捕获 systemctl/rc-service 输出，失败时打印实际错误信息和 `systemctl status` 诊断输出
- `start_mariadb_service()`（install.sh + x-ui.sh）：systemctl start/enable 失败时使用 `|| true` 避免 set -e 场景下脚本意外退出，保持行为一致

Impact

- install.sh: `restart_mariadb_service()` 和 `start_mariadb_service()`
- x-ui.sh: `restart_mariadb_service()` 和 `start_mariadb_service()`
- 不影响功能逻辑，仅改善错误诊断输出
- 无 API、数据库、配置变更

Verification

- `bash -n install.sh` — syntax OK
- `bash -n x-ui.sh` — syntax OK
- `bash tests/mariadb_install_switch_test.sh` — PASS
- `bash tests/mariadb_admin_empty_password_test.sh` — PASS
- `bash tests/install_uninstall_resilience_test.sh` — PASS

Risks And Follow-Up

- 无风险。改动仅影响错误输出，不改变控制流
- 用户重新运行安装脚本后，应能看到 systemctl 实际报错原因（如配置文件语法错误、端口冲突等），可据此进一步定位
