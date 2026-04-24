# install.sh 逻辑文档

## 概述

`install.sh` 是 3x-ui 面板的安装脚本，负责在 Linux 服务器上完成以下工作：

1. 安装系统依赖包
2. 下载并解压 3x-ui 发行版
3. 配置 systemd / OpenRC 服务
4. 生成随机凭据（用户名、密码、端口、Web 路径）
5. 配置 SSL 证书（Let's Encrypt 域名证书、IP 证书、或自定义证书）
6. 显示安装结果和访问信息

---

## 全局配置

### 颜色变量

| 变量    | 值             | 用途       |
|---------|----------------|------------|
| `red`   | `\033[0;31m`   | 红色文本   |
| `green` | `\033[0;32m`   | 绿色文本   |
| `blue`  | `\033[0;34m`   | 蓝色文本   |
| `yellow`| `\033[0;33m`   | 黄色文本   |
| `plain` | `\033[0m`      | 重置颜色   |

### 路径变量

| 变量            | 默认值                  | 说明                     |
|-----------------|-------------------------|--------------------------|
| `xui_folder`    | `/usr/local/x-ui`       | x-ui 安装目录            |
| `xui_service`   | `/etc/systemd/system`   | systemd 服务文件目录     |

可通过环境变量 `XUI_MAIN_FOLDER` 和 `XUI_SERVICE` 覆盖。

---

## 入口流程

```
install.sh 被执行
  ├─ 检查 root 权限
  ├─ 检测操作系统发行版
  ├─ 检测 CPU 架构
  ├─ install_base()          ← 安装系统依赖
  └─ install_x-ui($1)        ← 主安装逻辑（$1 为可选的版本号）
```

---

## 函数详解

### 1. root 权限检查（第 14-15 行）

检查 `$EUID` 是否为 0。非 root 用户直接退出并提示使用 root 权限。

### 2. 操作系统检测（第 17-28 行）

读取 `/etc/os-release` 或 `/usr/lib/os-release`，将 `$ID` 赋值给 `release` 变量。

支持的发行版：

| 包管理器 | 发行版 |
|----------|--------|
| `apt`    | ubuntu, debian, armbian |
| `dnf`    | fedora, amzn, virtuozzo, rhel, almalinux, rocky, ol |
| `yum`    | centos 7 |
| `pacman` | arch, manjaro, parch |
| `zypper` | opensuse-tumbleweed, opensuse-leap |
| `apk`    | alpine |

### 3. `arch()` — CPU 架构检测（第 30-41 行）

通过 `uname -m` 映射到标准架构标识：

| `uname -m` 输出        | 返回值   |
|------------------------|----------|
| x86_64, x64, amd64     | `amd64`  |
| i*86, x86              | `386`    |
| armv8*, arm64, aarch64 | `arm64`  |
| armv7*, arm            | `armv7`  |
| armv6*                 | `armv6`  |
| armv5*                 | `armv5`  |
| s390x                  | `s390x`  |
| 其他                   | 退出报错 |

### 4. IP/域名验证函数（第 46-57 行）

| 函数          | 逻辑                                              |
|---------------|---------------------------------------------------|
| `is_ipv4()`   | 正则匹配 `数字.数字.数字.数字` 格式              |
| `is_ipv6()`   | 检查字符串是否包含 `:`                            |
| `is_ip()`     | 调用 `is_ipv4` 或 `is_ipv6`                       |
| `is_domain()` | 正则匹配标准域名格式（含国际化域名 `xn--` 支持） |

### 5. `is_port_in_use()` — 端口占用检测（第 60-74 行）

按优先级尝试三种方式：

1. `ss -ltn` — 检查监听端口
2. `netstat -lnt` — 回退方案
3. `lsof -nP -iTCP:端口 -sTCP:LISTEN` — 最后手段

任一命中即返回 0（端口被占用）。

### 6. `install_base()` — 安装基础依赖（第 76-104 行）

根据 `$release` 使用对应的包管理器安装以下公共依赖：

```
curl, tar, tzdata, socat, ca-certificates, openssl
```

额外安装 `cron`（用于 acme.sh 自动续期，仅 apt 系列）。

- CentOS 7 使用 `yum`，其他版本使用 `dnf`
- 未识别的发行版默认回退到 `apt-get`

### 7. `gen_random_string(length)` — 随机字符串生成（第 106-111 行）

```
openssl rand -base64(length*2) → 过滤 a-zA-Z0-9 → 截取前 length 个字符
```

用于生成用户名、密码、Web 路径等随机值。

### 8. `install_acme()` — 安装 acme.sh（第 113-124 行）

```bash
curl -s https://get.acme.sh | sh
```

安装到 `~/.acme.sh/` 目录。失败返回 1。

---

## SSL 证书管理

### 9. `setup_ssl_certificate(domain, server_ip, port, webBasePath)` — 域名 SSL（第 126-191 行）

**用途**：为域名签发 Let's Encrypt 证书。

**流程**：

```
检查 acme.sh 是否已安装
  ├─ 未安装 → 调用 install_acme()
  └─ 已安装 → 继续

创建证书目录：/root/cert/${domain}/

签发证书：
  acme.sh --set-default-ca --server letsencrypt
  acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80
  ↳ 失败 → 清理并返回 1

安装证书：
  acme.sh --installcert
    --key-file     /root/cert/${domain}/privkey.pem
    --fullchain-file /root/cert/${domain}/fullchain.pem
    --reloadcmd    "systemctl restart x-ui"

启用自动续期：acme.sh --upgrade --auto-upgrade

设置文件权限：
  privkey.pem  → 600（仅所有者可读）
  fullchain.pem → 644

配置面板证书路径：
  x-ui cert -webCert fullchain.pem -webCertKey privkey.pem
```

**前提条件**：80 端口必须可从外网访问。

### 10. `setup_ip_certificate(ipv4, ipv6)` — IP 证书（第 195-343 行）

**用途**：为 IP 地址签发 Let's Encrypt 短期证书（约 6 天有效期）。

**流程**：

```
检查 acme.sh
验证 IPv4 地址格式

创建证书目录：/root/cert/ip/

选择 HTTP-01 监听端口：
  └─ 默认 80，用户可自定义
  └─ 循环检测端口占用，被占用则提示换端口

签发证书：
  acme.sh --issue
    -d ${ipv4} [-d ${ipv6}]
    --standalone
    --server letsencrypt
    --certificate-profile shortlived
    --days 6
    --httpport ${WebPort}

安装证书：
  acme.sh --installcert
    --key-file     /root/cert/ip/privkey.pem
    --fullchain-file /root/cert/ip/fullchain.pem
    --reloadcmd    "systemctl restart x-ui || rc-service x-ui restart"
  ↳ 通过检查文件是否存在（而非退出码）判断成功

启用自动续期
设置文件权限
配置面板证书路径
```

**关键特性**：

- 使用 `--certificate-profile shortlived` 配置文件，证书有效期约 6 天
- acme.sh cron 任务会在到期前自动续期
- 不依赖退出码判断安装成功（因为 reloadcmd 失败会导致非零退出）
- 支持 IPv4 + IPv6 双栈

### 11. `ssl_cert_issue()` — 手动 SSL 证书签发（第 346-509 行）

**用途**：交互式域名证书签发，提供更多自定义选项。

**流程**：

```
读取当前面板的 webBasePath 和 port

检查 acme.sh（不存在则安装）

获取并验证用户输入的域名：
  └─ 循环直到输入有效域名
  └─ 检查是否已存在该域名的证书

创建证书目录：/root/cert/${domain}/

选择端口（默认 80）

临时停止面板（释放端口）

签发证书：
  acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort}

设置 reloadcmd（证书续期后执行的命令）：
  ├─ 默认：systemctl restart x-ui || rc-service x-ui restart
  ├─ 选项 1：systemctl reload nginx ; systemctl restart x-ui
  ├─ 选项 2：自定义命令
  └─ 选项 0：保持默认

安装证书并启用自动续期

启动面板

询问是否将证书应用到面板：
  └─ 是 → x-ui cert -webCert ... -webCertKey ...
  └─ 否 → 跳过
```

**特点**：

- 签发前会停止面板以释放端口
- 支持自定义 reloadcmd（例如先 reload nginx 再重启 x-ui）
- 签发失败会自动重新启动面板

### 12. `prompt_and_setup_ssl(panel_port, web_base_path, server_ip)` — SSL 选择菜单（第 513-638 行）

**用途**：安装时的统一 SSL 配置入口，提供三种选择。

**菜单**：

```
1. Let's Encrypt 域名证书（90 天有效期，自动续期）
   └─ 调用 ssl_cert_issue()
   └─ 从 acme.sh 列表提取域名作为 SSL_HOST

2. Let's Encrypt IP 证书（6 天有效期，自动续期）  ← 默认选项
   └─ 可选输入 IPv6 地址
   └─ 停止面板释放 80 端口
   └─ 调用 setup_ip_certificate(server_ip, ipv6)
   └─ SSL_HOST = server_ip

3. 自定义 SSL 证书（指定已有文件路径）
   └─ 输入域名
   └─ 循环验证证书文件（存在、可读、非空）
   └─ 循环验证私钥文件（存在、可读、非空）
   └─ x-ui cert -webCert ... -webCertKey ...
   └─ 提示用户自行管理续期
```

**全局变量**：设置 `SSL_HOST` 供后续显示访问地址使用。

---

## 安装后配置

### 13. `config_after_install()` — 安装后配置（第 640-760 行）

**用途**：首次安装后的凭据生成、端口设置、Web 路径生成、SSL 配置。

**流程图**：

```
读取当前面板设置：
  - hasDefaultCredential（是否为默认凭据）
  - webBasePath
  - port
  - cert（证书路径）

获取服务器公网 IP：
  └─ 依次尝试 6 个 API：
     1. api4.ipify.org
     2. ipv4.icanhazip.com
     3. v4.api.ipinfo.io/ip
     4. ipv4.myexternalip.com/raw
     5. 4.ident.me
     6. check-host.net/ip

判断 webBasePath 是否足够长（≥4 字符）：

  ┌─ webBasePath 过短
  │
  │  ├─ hasDefaultCredential == true（首次安装）
  │  │   ├─ 生成随机 webBasePath（18 位）
  │  │   ├─ 生成随机用户名（10 位）
  │  │   ├─ 生成随机密码（10 位）
  │  │   ├─ 询问是否自定义端口
  │  │   │   ├─ 是 → 用户输入端口
  │  │   │   └─ 否 → 随机生成 1024-62000 范围端口
  │  │   ├─ 应用设置：x-ui setting -username ... -password ... -port ... -webBasePath ...
  │  │   ├─ prompt_and_setup_ssl()  ← 必需
  │  │   └─ 显示完整凭据和访问地址
  │  │
  │  └─ hasDefaultCredential != true（非首次安装）
  │      ├─ 生成新 webBasePath
  │      ├─ 检查是否有证书：
  │      │   ├─ 无 → prompt_and_setup_ssl()（推荐）
  │      │   └─ 有 → 显示 HTTP 访问地址
  │      └─ 结束
  │
  └─ webBasePath 正常（≥4 字符）
  
     ├─ hasDefaultCredential == true
     │   ├─ 生成随机用户名和密码
     │   ├─ 应用新凭据
     │   └─ 显示凭据
     │
     └─ hasDefaultCredential != true
         └─ 提示凭据已正确设置

     再次检查证书：
     ├─ 无证书 → prompt_and_setup_ssl()（推荐）
     └─ 有证书 → 跳过

最后执行：x-ui migrate（数据库迁移）
```

---

## 主安装逻辑

### 14. `install_x-ui(version)` — 主安装函数（第 762-958 行）

**参数**：`$1` 可选，指定安装版本号（如 `v2.3.5`）。

**流程**：

```
cd /usr/local/

┌─ 无版本参数（安装最新版）
│   ├─ 从 GitHub API 获取最新版本号
│   │   └─ IPv4 失败时重试 curl -4
│   └─ 下载：x-ui-linux-${arch}.tar.gz
│
└─ 有版本参数
    ├─ 验证版本号 ≥ v2.3.5
    └─ 下载指定版本

同时下载 x-ui.sh 到 /usr/bin/x-ui-temp

停止已有 x-ui 服务并删除旧安装目录

解压 tar.gz，设置执行权限

ARM 架构特殊处理：
  armv5/armv6/armv7 → 重命名为 xray-linux-arm

安装 x-ui.sh 到 /usr/bin/x-ui
创建日志目录 /var/log/x-ui/

调用 config_after_install()  ← 生成凭据 + SSL

etckeeper 兼容：
  └─ 如果 /etc/.git 存在，将 x-ui.db 加入 .gitignore

┌─ Alpine Linux
│   ├─ 下载 OpenRC 脚本 x-ui.rc → /etc/init.d/x-ui
│   ├─ rc-update add x-ui（启用开机自启）
│   └─ rc-service x-ui start
│
└─ 其他系统（systemd）
    ├─ 优先使用 tar.gz 中的服务文件
    │   ├─ x-ui.service          ← 通用
    │   ├─ x-ui.service.debian   ← Ubuntu/Debian
    │   ├─ x-ui.service.arch     ← Arch/Manjaro
    │   └─ x-ui.service.rhel     ← 其他（CentOS/Fedora 等）
    │
    ├─ 如果 tar.gz 中没有，从 GitHub 下载对应文件
    │
    └─ 配置服务：
        chown root:root x-ui.service
        chmod 644 x-ui.service
        systemctl daemon-reload
        systemctl enable x-ui
        systemctl start x-ui

显示安装完成信息和子命令用法
```

**子命令列表**（安装完成后显示）：

| 命令              | 功能               |
|-------------------|--------------------|
| `x-ui`            | 打开管理菜单       |
| `x-ui start`      | 启动面板           |
| `x-ui stop`       | 停止面板           |
| `x-ui restart`    | 重启面板           |
| `x-ui status`     | 查看状态           |
| `x-ui settings`   | 查看当前设置       |
| `x-ui enable`     | 设置开机自启       |
| `x-ui disable`    | 取消开机自启       |
| `x-ui log`        | 查看日志           |
| `x-ui banlog`     | 查看 Fail2ban 日志 |
| `x-ui update`     | 更新               |
| `x-ui legacy`     | 安装旧版本         |
| `x-ui install`    | 安装               |
| `x-ui uninstall`  | 卸载               |

---

## 调用关系总结

```
install.sh
  │
  ├─ install_base()
  │     └─ 根据发行版安装 curl, tar, tzdata, socat, ca-certificates, openssl
  │
  └─ install_x-ui($1)
        ├─ 下载 x-ui 发行版和 x-ui.sh
        ├─ 解压、设置权限
        ├─ config_after_install()
        │     ├─ gen_random_string() × 3（用户名/密码/Web路径）
        │     ├─ 获取公网 IP
        │     ├─ prompt_and_setup_ssl()
        │     │     ├─ [选项1] ssl_cert_issue()
        │     │     │     ├─ install_acme()
        │     │     │     └─ acme.sh 签发/安装/续期域名证书
        │     │     ├─ [选项2] setup_ip_certificate()
        │     │     │     ├─ install_acme()
        │     │     │     └─ acme.sh 签发/安装/续期 IP 短期证书
        │     │     └─ [选项3] 用户提供自定义证书路径
        │     └─ x-ui migrate
        └─ 配置系统服务（systemd 或 OpenRC）
```

---

## 关键设计决策

1. **强制 SSL**：首次安装时必须配置 SSL 证书（三种方式选一），确保面板通过 HTTPS 访问。

2. **随机化安全**：用户名、密码、端口、Web 路径全部随机生成，避免使用默认凭据。

3. **多 OS 兼容**：通过 `case` 语句适配 7 大包管理器体系，Alpine 使用 OpenRC，其余使用 systemd。

4. **IP 证书支持**：利用 Let's Encrypt 的 shortlived profile，为无域名场景提供 SSL 支持（6 天有效期，自动续期）。

5. **优雅降级**：
   - GitHub API 失败时用 `curl -4` 重试
   - `ss` 不可用时回退到 `netstat`，再回退到 `lsof`
   - tar.gz 中无服务文件时从 GitHub 下载
   - acme.sh reloadcmd 失败不阻止证书安装

6. **etckeeper 兼容**：自动将数据库文件加入 `/etc/.gitignore`，避免 etckeeper 追踪频繁变化的数据库。
