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

assert_contains "install.sh" "MariaDB 管理员密码（可留空）"
assert_contains "install.sh" "if [[ -n \"\$pass\" ]]; then"
assert_contains "install.sh" "if [[ -n \"\$LOCAL_MARIADB_ADMIN_PASS\" ]]; then"
assert_contains "install.sh" "test_mariadb_server_connection \"127.0.0.1\" \"\$port\" \"root\" \"\""
assert_contains "install.sh" "LOCAL_MARIADB_JUST_INSTALLED=\"1\""

assert_contains "x-ui.sh" "MariaDB 管理员密码（可留空）"
assert_contains "x-ui.sh" "if [[ -n \"\$pass\" ]]; then"
assert_contains "x-ui.sh" "if [[ -n \"\$LOCAL_MARIADB_ADMIN_PASS\" ]]; then"
assert_contains "x-ui.sh" "test_mariadb_server_connection \"127.0.0.1\" \"\$port\" \"root\" \"\""
assert_contains "x-ui.sh" "LOCAL_MARIADB_JUST_INSTALLED=\"1\""

echo "mariadb admin empty-password flow looks correct"
