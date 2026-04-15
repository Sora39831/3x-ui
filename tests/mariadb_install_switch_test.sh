#!/usr/bin/env bash
set -euo pipefail

assert_contains() {
    local file="$1"
    local pattern="$2"
    if ! grep -Fq "$pattern" "$file"; then
        echo "missing pattern in $file: $pattern" >&2
        return 1
    fi
}

assert_contains "install.sh" "本地 MariaDB"
assert_contains "install.sh" "远程 MariaDB"
assert_contains "install.sh" "业务数据库名"
assert_contains "install.sh" "ensure_local_mariadb_ready"
assert_contains "install.sh" "远程 MariaDB 端口无效，请输入 1-65535 之间的数字"

assert_contains "x-ui.sh" "本地 MariaDB"
assert_contains "x-ui.sh" "远程 MariaDB"
assert_contains "x-ui.sh" "ensure_local_mariadb_ready"
assert_contains "x-ui.sh" "ensure_mariadb_database_and_user"
assert_contains "x-ui.sh" "远程 MariaDB 端口无效，请输入 1-65535 之间的数字"
assert_contains "x-ui.sh" "是否删除数据库并卸载本机 MariaDB？"
assert_contains "x-ui.sh" "remove_local_mariadb_data"
assert_contains "x-ui.sh" "uninstall_local_mariadb_packages"

echo "mariadb install/switch prompts look correct"
