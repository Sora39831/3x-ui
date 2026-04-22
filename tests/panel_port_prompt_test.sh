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

assert_not_contains() {
    local file="$1"
    local pattern="$2"
    if grep -Fq "$pattern" "$file"; then
        echo "unexpected pattern in $file: $pattern" >&2
        return 1
    fi
}

assert_contains "install.sh" "请输入面板端口（留空将随机生成）："
assert_contains "install.sh" "if [[ -z \"\${config_port}\" ]]; then"
assert_contains "install.sh" "已生成随机端口："
assert_contains "install.sh" "无效端口，请输入 1-65535 之间的数字。"
assert_not_contains "install.sh" "是否要自定义面板端口？（否则将使用随机端口）[y/n]："

echo "panel port prompt flow looks correct"
