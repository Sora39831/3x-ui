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

assert_contains "install.sh" "timeout 30 \${xui_folder}/x-ui migrate"
assert_contains "install.sh" "数据库迁移未在 30 秒内完成或执行失败"
assert_contains "x-ui.sh" "if [[ -x \"\${xui_folder}/x-ui\" || -d /etc/x-ui || -d \"\${xui_folder}\" ]]; then"

echo "install/uninstall resilience checks look correct"
