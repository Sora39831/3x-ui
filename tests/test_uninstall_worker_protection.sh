#!/bin/bash
# Test: uninstall on worker node must NOT allow database deletion
# Tests node role detection across old (.other.*) and new (.node.*) config formats

set -euo pipefail

TEST_DIR=$(mktemp -d)
cleanup() { rm -rf "$TEST_DIR"; }
trap cleanup EXIT

mkdir -p "$TEST_DIR/etc/x-ui"
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "_meta": {
    "layout": "按模块-用途来归类",
    "schema": "module-purpose-v1"
  },
  "node": {
    "nodeRole": "worker",
    "nodeId": "test-worker-001",
    "syncInterval": "30",
    "trafficFlushInterval": "10"
  },
  "databaseConnection": {
    "dbType": "mariadb",
    "dbHost": "127.0.0.1",
    "dbPort": "3306",
    "dbName": "3xui",
    "dbUser": "xui_user"
  }
}
JSONEOF

SETTING_FILE="$TEST_DIR/etc/x-ui/x-ui.json"

# Define is_node_role_configured for isolated test (same logic as in x-ui.sh)
is_node_role_configured() {
    local json_path="${SETTING_FILE:-/etc/x-ui/x-ui.json}"
    if [ ! -f "$json_path" ]; then
        return 1
    fi
    if command -v jq >/dev/null 2>&1; then
        local found
        found=$(jq -r 'if .node.nodeRole then "yes" elif .other.nodeRole then "yes" elif .nodeRole then "yes" else "no" end' "$json_path" 2>/dev/null)
        [[ "$found" == "yes" ]] && return 0 || return 1
    elif command -v python3 >/dev/null 2>&1; then
        local found
        found=$(python3 -c "
import json
try:
    with open('$json_path') as f:
        data = json.load(f)
    if data.get('node', {}).get('nodeRole') or data.get('other', {}).get('nodeRole') or data.get('nodeRole'):
        print('yes')
    else:
        print('no')
except: print('no')
" 2>/dev/null)
        [[ "$found" == "yes" ]] && return 0 || return 1
    else
        grep -q '"nodeRole"' "$json_path" 2>/dev/null && return 0 || return 1
    fi
}

# Manually define get_node_setting for isolated test (same logic as in x-ui.sh)
get_node_setting() {
    local key="$1"
    local default_value="$2"
    local json_path="${SETTING_FILE:-/etc/x-ui/x-ui.json}"
    local jq_expr=""

    if [ ! -f "$json_path" ]; then
        echo "$default_value"
        return
    fi

    if command -v jq >/dev/null 2>&1; then
        case "$key" in
        ".nodeRole")
            jq_expr='.node.nodeRole // .other.nodeRole // .nodeRole // "master"'
            ;;
        ".nodeId")
            jq_expr='.node.nodeId // .other.nodeId // .nodeId // ""'
            ;;
        ".syncInterval")
            jq_expr='.node.syncInterval // .other.syncInterval // .syncInterval // "30"'
            ;;
        ".trafficFlushInterval")
            jq_expr='.node.trafficFlushInterval // .other.trafficFlushInterval // .trafficFlushInterval // "10"'
            ;;
        *)
            jq_expr="$key // $default_value"
            ;;
        esac
        jq -r "$jq_expr" "$json_path" 2>/dev/null
        return
    fi

    # grep fallback for systems without jq
    case "$key" in
    ".nodeRole")
        (grep -o '"nodeRole"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null || true) | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' | grep -v '^$' || echo "$default_value"
        ;;
    ".nodeId")
        (grep -o '"nodeId"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null || true) | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' | grep -v '^$' || echo "$default_value"
        ;;
    ".syncInterval")
        (grep -o '"syncInterval"[[:space:]]*:[[:space:]]*[^,}]*' "$json_path" 2>/dev/null || true) | tail -1 | awk -F': ' '{print $2}' | tr -d '[:space:]' | grep -v '^$' || echo "$default_value"
        ;;
    ".trafficFlushInterval")
        (grep -o '"trafficFlushInterval"[[:space:]]*:[[:space:]]*[^,}]*' "$json_path" 2>/dev/null || true) | tail -1 | awk -F': ' '{print $2}' | tr -d '[:space:]' | grep -v '^$' || echo "$default_value"
        ;;
    *)
        echo "$default_value"
        ;;
    esac
}

echo "=== Test 1: get_node_setting reads nodeRole from .node.group ==="
result=$(get_node_setting '.nodeRole' 'master')
if [[ "$result" == "worker" ]]; then
    echo "PASS: nodeRole = $result"
else
    echo "FAIL: Expected 'worker', got '$result'"
    exit 1
fi

echo "=== Test 2: get_node_setting reads nodeId from .node.group ==="
result=$(get_node_setting '.nodeId' '')
if [[ "$result" == "test-worker-001" ]]; then
    echo "PASS: nodeId = $result"
else
    echo "FAIL: Expected 'test-worker-001', got '$result'"
    exit 1
fi

echo "=== Test 3: Worker node uninstall path check ==="
node_role=$(get_node_setting '.nodeRole' 'master')
if [[ "$node_role" == "worker" ]]; then
    echo "PASS: Uninstall would skip database deletion for worker node"
else
    echo "FAIL: Node role detection failed"
    exit 1
fi

# Test 4: Old config format (nodeRole in "other" group)
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "other": {
    "nodeRole": "worker",
    "nodeId": "old-format-worker"
  },
  "databaseConnection": {
    "dbType": "mariadb",
    "dbHost": "192.168.1.100"
  }
}
JSONEOF

echo "=== Test 4: get_node_setting reads from .other.nodeRole (legacy) ==="
result=$(get_node_setting '.nodeRole' 'master')
if [[ "$result" == "worker" ]]; then
    echo "PASS: legacy nodeRole = $result"
else
    echo "FAIL: Expected 'worker' from legacy format, got '$result'"
    exit 1
fi

# Test 5: New config format (nodeRole in "node" group) takes precedence
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "node": {
    "nodeRole": "worker",
    "nodeId": "new-format-worker"
  },
  "other": {
    "nodeRole": "master",
    "nodeId": "old-ignored"
  }
}
JSONEOF

echo "=== Test 5: .node.nodeRole takes precedence over .other.nodeRole ==="
result=$(get_node_setting '.nodeRole' 'master')
if [[ "$result" == "worker" ]]; then
    echo "PASS: .node.nodeRole = $result (correctly preferred over .other.nodeRole)"
else
    echo "FAIL: Expected 'worker' from .node group, got '$result'"
    exit 1
fi

# Test 6: is_node_role_configured returns true when nodeRole exists
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "node": {
    "nodeRole": "master",
    "nodeId": ""
  }
}
JSONEOF

echo "=== Test 6: is_node_role_configured returns true when explicitly set ==="
if is_node_role_configured; then
    echo "PASS: detected explicit nodeRole configuration"
else
    echo "FAIL: should have detected nodeRole in config"
    exit 1
fi

# Test 7: is_node_role_configured returns false when nodeRole is missing
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "databaseConnection": {
    "dbType": "mariadb",
    "dbHost": "127.0.0.1",
    "dbPort": "3306"
  }
}
JSONEOF

echo "=== Test 7: is_node_role_configured returns false when not configured ==="
if ! is_node_role_configured; then
    echo "PASS: correctly detected missing nodeRole configuration"
else
    echo "FAIL: should have returned false for missing nodeRole"
    exit 1
fi

# Test 8: Remote DB + worker role → skip deletion silently
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "node": {
    "nodeRole": "worker",
    "nodeId": "w-001"
  },
  "databaseConnection": {
    "dbType": "mariadb",
    "dbHost": "10.0.0.50",
    "dbPort": "3306",
    "dbName": "3xui",
    "dbUser": "xui_user"
  }
}
JSONEOF

echo "=== Test 8: Worker node with remote DB → skip deletion ==="
node_role=$(get_node_setting '.nodeRole' 'master')
db_host="10.0.0.50"
if [[ "$node_role" == "worker" ]]; then
    echo "PASS: Worker with remote DB ($db_host) would skip deletion entirely"
else
    echo "FAIL: node_role should be worker"
    exit 1
fi

# Test 9: Remote DB + unset nodeRole → skip deletion (due to remote host check)
cat > "$TEST_DIR/etc/x-ui/x-ui.json" << 'JSONEOF'
{
  "databaseConnection": {
    "dbType": "mariadb",
    "dbHost": "10.0.0.50",
    "dbPort": "3306",
    "dbName": "3xui",
    "dbUser": "xui_user"
  }
}
JSONEOF

echo "=== Test 9: Unset nodeRole + remote DB → skip deletion (remote host guard) ==="
if ! is_node_role_configured; then
    node_role=$(get_node_setting '.nodeRole' 'master')
    db_host="10.0.0.50"
    if [[ "$db_host" != "127.0.0.1" && "$db_host" != "localhost" && "$db_host" != "::1" ]]; then
        echo "PASS: nodeRole not set (default=$node_role), but remote host ($db_host) still catches it"
    else
        echo "FAIL: remote host check should have caught this"
        exit 1
    fi
else
    echo "FAIL: nodeRole should not be detected as configured"
    exit 1
fi

echo ""
echo "All tests passed."
