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

assert_contains "x-ui.sh" "本地 MariaDB"
assert_contains "x-ui.sh" "远程 MariaDB"
assert_contains "x-ui.sh" "ensure_local_mariadb_ready"
assert_contains "x-ui.sh" "ensure_mariadb_database_and_user"

echo "mariadb install/switch prompts look correct"
