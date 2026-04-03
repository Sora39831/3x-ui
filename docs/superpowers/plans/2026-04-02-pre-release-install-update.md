# Pre-release Install/Update Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users choose between the latest Stable or Pre-release when installing or updating 3x-ui.

**Architecture:** Replace the hardcoded `/releases/latest` API call with a `/releases` call that parses both stable and pre-release tags. Add an interactive prompt in both `install.sh` and `update.sh` so users pick which version to install. Functions are duplicated across files (matching existing conventions — no shared library).

**Tech Stack:** Bash, GitHub REST API, grep/sed/awk for JSON parsing (no jq).

---

### Task 1: Add `get_releases` helper + prompt to `install.sh`

**Files:**
- Modify: `install.sh:874-911`

- [ ] **Step 1: Add `get_releases` function before `install_x-ui()`**

Insert this function before `install_x-ui()` (around line 874). It fetches all releases and parses out the latest stable and pre-release tags:

```bash
get_releases() {
    local releases_json
    releases_json=$(curl -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases")
    if [[ -z "$releases_json" ]]; then
        echo -e "${yellow}正在尝试通过 IPv4 获取版本...${plain}"
        releases_json=$(curl -4 -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases")
        if [[ -z "$releases_json" ]]; then
            echo -e "${red}获取 x-ui 版本失败，可能是 GitHub API 限制，请稍后重试${plain}"
            exit 1
        fi
    fi

    # Parse first non-prerelease tag_name
    latest_stable=$(echo "$releases_json" | tr '{' '\n' | grep '"prerelease":false' | head -1 | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    # Parse first prerelease tag_name
    latest_prerelease=$(echo "$releases_json" | tr '{' '\n' | grep '"prerelease":true' | head -1 | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$latest_stable" && -z "$latest_prerelease" ]]; then
        echo -e "${red}获取 x-ui 版本失败${plain}"
        exit 1
    fi
}

select_version() {
    if [[ -n "$latest_stable" && -n "$latest_prerelease" ]]; then
        echo ""
        echo -e "${green}请选择要安装的版本：${plain}"
        echo -e "${green}1)${plain} 最新稳定版: ${latest_stable}"
        echo -e "${green}2)${plain} 最新预发布版: ${latest_prerelease}"
        read -rp "请输入选择 [1-2]: " version_choice
        while [[ "$version_choice" != "1" && "$version_choice" != "2" ]]; do
            read -rp "无效输入，请重新输入 [1-2]: " version_choice
        done
        if [[ "$version_choice" == "1" ]]; then
            tag_version="$latest_stable"
        else
            tag_version="$latest_prerelease"
        fi
    elif [[ -n "$latest_stable" ]]; then
        tag_version="$latest_stable"
    else
        tag_version="$latest_prerelease"
    fi
}
```

- [ ] **Step 2: Replace the no-argument release fetch block in `install_x-ui()`**

Replace lines 879-888:

```bash
        tag_version=$(curl -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            echo -e "${yellow}正在尝试通过 IPv4 获取版本...${plain}"
            tag_version=$(curl -4 -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
            if [[ ! -n "$tag_version" ]]; then
                echo -e "${red}获取 x-ui 版本失败，可能是 GitHub API 限制，请稍后重试${plain}"
                exit 1
            fi
        fi
        echo -e "获取到 x-ui 最新版本：${tag_version}，开始安装..."
```

With:

```bash
        get_releases
        select_version
        echo -e "获取到 x-ui 版本：${tag_version}，开始安装..."
```

- [ ] **Step 3: Verify the script still works for the specific-version path**

Read the full `install_x-ui()` function and confirm the `else` branch (lines 894-911, where `$1` is provided) is untouched.

- [ ] **Step 4: Commit**

```bash
git add install.sh
git commit -m "feat(install): add pre-release version selection prompt"
```

### Task 2: Add `get_releases` helper + prompt to `update.sh`

**Files:**
- Modify: `update.sh:748-767`

- [ ] **Step 1: Add `get_releases` and `select_version` functions before `update_x-ui()`**

Insert the same two functions before `update_x-ui()` (around line 748). Identical logic to install.sh except the prompt text says "更新" (update) instead of "安装" (install):

```bash
get_releases() {
    local releases_json
    releases_json=$(${curl_bin} -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases" 2>/dev/null)
    if [[ -z "$releases_json" ]]; then
        echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
        releases_json=$(${curl_bin} -4 -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases" 2>/dev/null)
        if [[ -z "$releases_json" ]]; then
            _fail "ERROR: Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later"
        fi
    fi

    latest_stable=$(echo "$releases_json" | tr '{' '\n' | grep '"prerelease":false' | head -1 | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    latest_prerelease=$(echo "$releases_json" | tr '{' '\n' | grep '"prerelease":true' | head -1 | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$latest_stable" && -z "$latest_prerelease" ]]; then
        _fail "ERROR: Failed to fetch x-ui version"
    fi
}

select_version() {
    if [[ -n "$latest_stable" && -n "$latest_prerelease" ]]; then
        echo ""
        echo -e "${green}Which version do you want to update to?${plain}"
        echo -e "${green}1)${plain} Latest Stable: ${latest_stable}"
        echo -e "${green}2)${plain} Latest Pre-release: ${latest_prerelease}"
        read -rp "Please enter your choice [1-2]: " version_choice
        while [[ "$version_choice" != "1" && "$version_choice" != "2" ]]; do
            read -rp "Invalid input, please re-enter [1-2]: " version_choice
        done
        if [[ "$version_choice" == "1" ]]; then
            tag_version="$latest_stable"
        else
            tag_version="$latest_prerelease"
        fi
    elif [[ -n "$latest_stable" ]]; then
        tag_version="$latest_stable"
    else
        tag_version="$latest_prerelease"
    fi
}
```

Note: `update.sh` uses `${curl_bin}` instead of bare `curl` — the helper respects this.

- [ ] **Step 2: Replace the release fetch block in `update_x-ui()`**

Replace lines 760-768:

```bash
    tag_version=$(${curl_bin} -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ ! -n "$tag_version" ]]; then
        echo -e "${yellow}Trying to fetch version with IPv4...${plain}"
        tag_version=$(${curl_bin} -4 -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$tag_version" ]]; then
            _fail "ERROR: Failed to fetch x-ui version, it may be due to GitHub API restrictions, please try it later"
        fi
    fi
    echo -e "Got x-ui latest version: ${tag_version}, beginning the installation..."
```

With:

```bash
    get_releases
    select_version
    echo -e "Got x-ui version: ${tag_version}, beginning the installation..."
```

- [ ] **Step 3: Verify the rest of `update_x-ui()` is unchanged**

Confirm lines 769+ (download, cleanup, install) remain intact.

- [ ] **Step 4: Commit**

```bash
git add update.sh
git commit -m "feat(update): add pre-release version selection prompt"
```
