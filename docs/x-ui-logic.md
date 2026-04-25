# x-ui.sh 逻辑文档

## 概述

`x-ui.sh` 是 3x-ui 面板的管理脚本，提供 26 个交互式菜单选项和 15 个子命令，涵盖面板的安装、更新、卸载、凭据管理、服务控制、SSL 证书、防火墙、Fail2ban IP 限制、BBR 加速、Geo 文件更新等功能。

---

## 全局配置

### 颜色变量

| 变量    | 值             | 用途     |
|---------|----------------|----------|
| `red`   | `\033[0;31m`   | 红色     |
| `green` | `\033[0;32m`   | 绿色     |
| `blue`  | `\033[0;34m`   | 蓝色     |
| `yellow`| `\033[0;33m`   | 黄色     |
| `plain` | `\033[0m`      | 重置     |

### 日志函数

| 函数    | 前缀      | 用途       |
|---------|-----------|------------|
| `LOGD()` | `[调试]`  | 调试信息   |
| `LOGE()` | `[错误]`  | 错误信息   |
| `LOGI()` | `[信息]`  | 普通信息   |

### 路径变量

| 变量                    | 默认值                    | 说明                    |
|-------------------------|---------------------------|-------------------------|
| `xui_folder`            | `/usr/local/x-ui`         | x-ui 安装目录           |
| `xui_service`           | `/etc/systemd/system`     | systemd 服务文件目录    |
| `log_folder`            | `/var/log/x-ui`           | 日志目录                |
| `iplimit_log_path`      | `.../3xipl.log`           | IP 限制日志             |
| `iplimit_banned_log_path`| `.../3xipl-banned.log`  | IP 封禁日志             |

### 辅助函数

| 函数                  | 功能                                         |
|-----------------------|----------------------------------------------|
| `confirm()`           | 通用确认提示，支持自定义默认值               |
| `confirm_restart()`   | 确认后重启面板（重启 x-ui 也会重启 xray）    |
| `before_show_menu()`  | 按回车返回主菜单                             |
| `gen_random_string()` | 通过 openssl 生成指定长度的随机字母数字字符串 |
| `is_port_in_use()`    | 端口占用检测（ss → netstat → lsof）          |
| `is_ipv4/is_ipv6/is_ip/is_domain()` | IP/域名格式验证              |

---

## 入口流程

```
x-ui.sh 被执行
  ├─ 检查 root 权限
  ├─ 检测操作系统发行版和版本号
  ├─ 初始化路径和日志目录
  │
  ├─ 有命令行参数 → 执行对应子命令（不显示菜单）
  └─ 无参数 → 显示交互式菜单 show_menu()
       ├─ 显示当前状态（运行/停止/未安装 + 开机自启 + xray 状态）
       ├─ 读取用户输入 [0-26]
       └─ 根据选择调用对应功能
```

---

## 主菜单 (show_menu)

```
╔────────────────────────────────────────────────╗
│  0.  退出脚本                                  │
│────────────────────────────────────────────────│
│  1.  安装          2.  更新          3.  更新菜单  │
│  4.  安装旧版本    5.  卸载                      │
│────────────────────────────────────────────────│
│  6.  重置用户名和密码  7.  重置 Web 路径         │
│  8.  重置设置          9.  修改端口              │
│ 10.  查看当前设置                                │
│────────────────────────────────────────────────│
│ 11.  启动    12.  停止    13.  重启              │
│ 14.  重启 Xray    15.  查看状态                  │
│ 16.  日志管理                                     │
│────────────────────────────────────────────────│
│ 17.  设置开机自启    18.  取消开机自启           │
│────────────────────────────────────────────────│
│ 19.  SSL 证书管理    20.  Cloudflare SSL         │
│ 21.  IP 限制管理     22.  防火墙管理              │
│ 23.  SSH 端口转发管理                            │
│────────────────────────────────────────────────│
│ 24.  BBR 管理    25.  更新 Geo 文件              │
│ 26.  网速测试 (Speedtest)                        │
╚────────────────────────────────────────────────╝
```

大部分选项在执行前调用 `check_install`（检查面板是否已安装）或 `check_uninstall`（检查面板是否未安装），防止误操作。

---

## 状态检测函数

| 函数                   | 返回值                    | 逻辑                                      |
|------------------------|---------------------------|-------------------------------------------|
| `check_status()`       | 0=运行中, 1=未运行, 2=未安装 | Alpine 检查 init.d，其他检查 systemd    |
| `check_enabled()`      | 0=已启用, 1=未启用        | Alpine 检查 rc-update，其他检查 systemctl |
| `check_xray_status()`  | 0=运行中, 1=未运行        | ps 查找 xray-linux 进程                  |
| `check_install()`      | 前置检查                  | 未安装则提示并返回菜单                    |
| `check_uninstall()`    | 前置检查                  | 已安装则提示"勿重复安装"并返回菜单        |

---

## 菜单选项详解

### 选项 0：退出脚本

```bash
exit 0
```

直接退出，无额外逻辑。

---

### 选项 1：安装

**函数**：`install()`

```
下载并执行 install.sh（从 GitHub raw 文件）
  └─ 成功后自动调用 start()
```

- 执行 `bash <(curl -Ls https://raw.githubusercontent.com/Sora39831/3x-ui/main/install.sh)`
- 安装成功后自动启动面板

---

### 选项 2：更新

**函数**：`update()`

```
确认提示："更新所有 x-ui 组件到最新版本，数据不会丢失"
  ├─ 取消 → 返回菜单
  └─ 确认 → 执行 update.sh（从 GitHub 下载）
       └─ 成功 → "更新完成，面板已自动重启"
```

---

### 选项 3：更新菜单

**函数**：`update_menu()`

```
确认提示
  └─ 确认 → 下载最新 x-ui.sh 到 /usr/bin/x-ui
       └─ 成功 → "更新成功" 并 exit 0
```

仅更新管理脚本自身，不影响面板程序。

---

### 选项 4：安装旧版本

**函数**：`legacy_version()`

```
提示用户输入版本号（如 2.4.0）
  ├─ 空 → 退出
  └─ 有效 → 执行对应版本的 install.sh，传入版本参数
```

- 下载指定 tag 的 install.sh：`v$tag_version/install.sh`
- 传入参数 `v$tag_version` 进行安装
- install.sh 内部会验证版本 ≥ v2.3.5

---

### 选项 5：卸载

**函数**：`uninstall()`

```
确认："卸载面板？xray 也会被卸载！"（默认 n）
  ├─ 取消 → 返回菜单
  └─ 确认 →
      Alpine: rc-service stop → rc-update del → rm init.d
      其他:   systemctl stop → disable → rm service → daemon-reload → reset-failed
      删除 /etc/x-ui/ 和 ${xui_folder}/
      显示重装命令
      删除脚本自身（trap SIGTERM → rm $0）
```

---

### 选项 6：重置用户名和密码

**函数**：`reset_user()`

```
确认提示（默认 n）
  └─ 确认 →
      输入用户名（默认随机 10 位）
      输入密码（默认随机 18 位）
      询问是否禁用双因素认证
        ├─ 是 → -resetTwoFactor true
        └─ 否 → -resetTwoFactor false
      应用设置：x-ui setting -username ... -password ...
      确认后重启面板
```

---

### 选项 7：重置 Web 路径

**函数**：`reset_webbasepath()`

```
确认提示
  └─ 确认 → 生成随机 18 位字符串
      应用：x-ui setting -webBasePath ...
      重启面板
```

---

### 选项 8：重置设置

**函数**：`reset_config()`

```
确认："重置所有面板设置？账户数据不会丢失，用户名和密码不会改变"（默认 n）
  └─ 确认 → x-ui setting -reset
      重启面板
```

仅重置面板配置，不影响账户数据库。

---

### 选项 9：修改端口

**函数**：`set_port()`

```
输入端口号 [1-65535]
  ├─ 空 → 取消
  └─ 有效 → x-ui setting -port ${port}
       确认后重启面板
```

---

### 选项 10：查看当前设置

**函数**：`check_config()`

```
获取面板设置（x-ui setting -show true）
获取公网 IP（api.ipify.org → 4.ident.me）

检查是否有证书：
  ├─ 有证书 → 从证书路径提取域名，显示 https://域名:端口/路径
  └─ 无证书 →
      显示警告
      询问是否为 IP 生成 SSL 证书
        ├─ 是 → 停止面板 → ssl_cert_issue_for_ip() → 启动面板
        └─ 否 → 显示 http://IP:端口/路径，建议使用选项 19
```

---

### 选项 11：启动

**函数**：`start()`

```
检查当前状态
  ├─ 运行中 → "面板正在运行，无需重复启动"
  └─ 未运行 →
      Alpine: rc-service x-ui start
      其他:   systemctl start x-ui
      等待 2 秒后再次检查状态
        ├─ 成功 → "x-ui 启动成功"
        └─ 失败 → "面板启动失败，可能是因为启动时间超过两秒"
```

---

### 选项 12：停止

**函数**：`stop()`

```
检查当前状态
  ├─ 已停止 → "面板已停止，无需重复停止！"
  └─ 运行中 →
      Alpine: rc-service x-ui stop
      其他:   systemctl stop x-ui
      等待 2 秒后检查状态
        ├─ 成功 → "x-ui 和 xray 已停止"
        └─ 失败 → "面板停止失败"
```

---

### 选项 13：重启

**函数**：`restart()`

```
Alpine: rc-service x-ui restart
其他:   systemctl restart x-ui
等待 2 秒后检查状态
  ├─ 成功 → "x-ui 和 xray 重启成功"
  └─ 失败 → "面板重启失败"
```

---

### 选项 14：重启 Xray

**函数**：`restart_xray()`

```
systemctl reload x-ui    ← 发送 reload 信号，不重启面板本身
"已发送重启信号，请查看日志确认"
等待 2 秒 → 显示 xray 运行状态
```

与选项 13 的区别：选项 13 重启整个 x-ui 服务，选项 14 仅重载 xray-core。

---

### 选项 15：查看状态

**函数**：`status()`

```
Alpine: rc-service x-ui status
其他:   systemctl status x-ui -l
```

显示完整的 systemd/服务状态信息。

---

### 选项 16：日志管理

**函数**：`show_log()`

```
Alpine:
  1. 调试日志 → grep 'x-ui[' /var/log/messages
  0. 返回

其他 (systemd):
  1. 调试日志 → journalctl -u x-ui -e --no-pager -f -p debug
  2. 清除所有日志 → journalctl --rotate → --vacuum-time=1s → 重启面板
  0. 返回
```

---

### 选项 17：设置开机自启

**函数**：`enable()`

```
Alpine: rc-update add x-ui default
其他:   systemctl enable x-ui
```

---

### 选项 18：取消开机自启

**函数**：`disable()`

```
Alpine: rc-update del x-ui
其他:   systemctl disable x-ui
```

---

### 选项 19：SSL 证书管理

**函数**：`ssl_cert_issue_main()` — 子菜单入口

#### 子菜单

```
1. 获取 SSL（域名）
2. 吊销证书
3. 强制续期
4. 查看已有域名
5. 为面板设置证书路径
6. 为 IP 地址获取 SSL（6 天证书，自动续期）
0. 返回主菜单
```

#### 子选项 1：获取 SSL（域名证书）

**函数**：`ssl_cert_issue()`

```
检查/安装 acme.sh
按发行版安装 socat

获取并验证域名（循环直到有效）
检查是否已有该域名的证书（acme.sh --list）

创建证书目录 /root/cert/${domain}/

选择端口（默认 80）

签发证书：
  acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
  ↳ 失败 → 清理并退出

设置 reloadcmd：
  默认：x-ui restart
  可选：systemctl reload nginx ; x-ui restart
  可选：自定义命令

安装证书：
  acme.sh --installcert
    --key-file     /root/cert/${domain}/privkey.pem
    --fullchain-file /root/cert/${domain}/fullchain.pem
    --reloadcmd    ${reloadCmd}

启用自动续期：acme.sh --upgrade --auto-upgrade
设置文件权限：privkey.pem → 600, fullchain.pem → 644

询问是否为面板设置证书：
  ├─ 是 → x-ui cert -webCert ... -webCertKey ... → 重启
  └─ 否 → 跳过
```

#### 子选项 2：吊销证书

```
列出 /root/cert/ 下所有域名目录
选择域名 → acme.sh --revoke -d ${domain}
```

#### 子选项 3：强制续期

```
列出所有域名
选择域名 → acme.sh --renew -d ${domain} --force
```

#### 子选项 4：查看已有域名

```
遍历 /root/cert/ 下的域名目录
显示每个域名的 fullchain.pem 和 privkey.pem 路径
缺失文件的标记为"证书或密钥缺失"
```

#### 子选项 5：为面板设置证书路径

```
列出所有域名
选择域名 → 验证文件存在
  x-ui cert -webCert ... -webCertKey ...
  重启面板
```

#### 子选项 6：为 IP 地址获取 SSL

**函数**：`ssl_cert_issue_for_ip()`

```
获取服务器公网 IP（api.ipify.org → 4.ident.me）
询问是否包含 IPv6 地址
检查/安装 acme.sh
按发行版安装 socat

创建证书目录 /root/cert/ip/
构建域名参数：-d ${server_ip} [-d ${ipv6}]

选择 HTTP-01 监听端口（默认 80）
  └─ 循环检测端口占用，被占用则提示换端口

签发证书：
  acme.sh --issue
    -d ${server_ip} [-d ${ipv6}]
    --standalone --server letsencrypt
    --certificate-profile shortlived
    --days 6 --httpport ${WebPort} --force

安装证书（不依赖退出码，通过检查文件判断成功）
启用自动续期
设置文件权限

为面板设置证书路径 → 显示 https://IP:端口/路径 → 重启面板
```

---

### 选项 20：Cloudflare SSL 证书

**函数**：`ssl_cert_issue_CF()`

```
显示使用说明（需要：邮箱、全局 API 密钥、域名）
确认提示

检查/安装 acme.sh

输入域名 (CF_Domain)
输入 API 密钥 (CF_GlobalKey)
输入注册邮箱 (CF_AccountEmail)

设置 CA 为 Let's Encrypt
导出环境变量：CF_Key, CF_Email

签发通配符证书：
  acme.sh --issue --dns dns_cf -d ${domain} -d *.${domain} --force
  ↳ 使用 Cloudflare DNS 验证

创建证书目录 /root/cert/${domain}/

设置 reloadcmd（同域名证书流程）
安装证书（含 *.${domain} 通配符）
启用自动续期

询问是否为面板设置证书 → 同域名证书流程
```

**特点**：支持通配符证书 `*.domain.com`，不需要开放 80 端口（使用 DNS 验证）。

---

### 选项 21：IP 限制管理（Fail2ban）

**函数**：`iplimit_main()` — 子菜单入口

#### 子菜单

```
 1. 安装 Fail2ban 并配置 IP 限制
 2. 修改封禁时长
 3. 解封所有人
 4. 封禁日志
 5. 封禁指定 IP 地址
 6. 解封指定 IP 地址
 7. 实时日志
 8. 服务状态
 9. 重启服务
10. 卸载 Fail2ban 和 IP 限制
 0. 返回主菜单
```

#### 子选项 1：安装 Fail2ban

**函数**：`install_iplimit()`

```
检查 Fail2ban 是否已安装
  └─ 未安装 → 按发行版安装：
      Ubuntu ≥ 24: 额外安装 python3-pip + pyasynchat
      Debian ≥ 12: 额外安装 python3-systemd
      CentOS 7: 先装 epel-release

清除 jail 配置冲突（iplimit_remove_conflicts）
创建日志文件（3xipl.log, 3xipl-banned.log）
创建 jail 配置（create_iplimit_jails）
启动并启用 Fail2ban 服务
```

**Jail 配置详情** (`create_iplimit_jails`)：

```ini
# /etc/fail2ban/jail.d/3x-ipl.conf
[3x-ipl]
enabled=true
backend=auto
filter=3x-ipl
action=3x-ipl
logpath=/var/log/x-ui/3xipl.log
maxretry=2
findtime=32
bantime=30m          # 默认 30 分钟，可通过子选项 2 修改
```

**过滤器**：匹配 `[LIMIT_IP] Email=... || Disconnecting OLD IP=... || Timestamp=...` 格式的日志行。

**动作**：使用 iptables 封禁/解封 IP，同时写入封禁日志文件。

#### 子选项 2：修改封禁时长

```
输入新的封禁时长（分钟）
重新生成 jail 配置 → 重启 Fail2ban
```

#### 子选项 3：解封所有人

```
fail2ban-client reload --restart --unban 3x-ipl
清空封禁日志文件
```

#### 子选项 5/6：手动封禁/解封 IP

```
输入 IP 地址 → 正则验证（IPv4/IPv6）
  fail2ban-client set 3x-ipl banip/unbanip "$ip"
```

#### 子选项 10：卸载

```
选项 1：仅移除 IP 限制配置（保留 Fail2ban）
  删除 filter.d/3x-ipl.conf, action.d/3x-ipl.conf, jail.d/3x-ipl.conf
  重启 Fail2ban

选项 2：完全卸载
  删除 /etc/fail2ban
  停止服务
  按发行版卸载 fail2ban 包 + autoremove
```

---

### 选项 22：防火墙管理

**函数**：`firewall_menu()` — 子菜单入口（基于 UFW）

#### 子菜单

```
1. 安装防火墙
2. 端口列表 [带编号]
3. 开放端口
4. 删除列表中的端口
5. 启用防火墙
6. 禁用防火墙
7. 防火墙状态
0. 返回主菜单
```

#### 子选项 1：安装防火墙

**函数**：`install_firewall()`

```
检查 ufw 是否安装 → 未安装则 apt-get install ufw
检查防火墙是否激活 → 未激活则：
  ufw allow ssh
  ufw allow http
  ufw allow https
  ufw allow 2053/tcp   ← webPort
  ufw allow 2096/tcp   ← subport
  ufw --force enable
```

#### 子选项 3：开放端口

**函数**：`open_ports()`

```
输入端口（逗号分隔或范围，如 80,443,2053 或 400-500）
验证输入格式
逐个处理：
  范围 → ufw allow start:end/tcp + ufw allow start:end/udp
  单端口 → ufw allow port
确认显示已开放的端口
```

#### 子选项 4：删除端口

**函数**：`delete_ports()`

```
显示当前规则（ufw status numbered）
选择删除方式：
  1. 按规则编号删除 → ufw delete $number
  2. 按端口号删除 → ufw delete allow $port
确认显示已删除的端口
```

**注意**：原始代码中选项 4 有一个已知 bug（`firewall_wall_menu` 应为 `firewall_menu`），这会导致删除端口后不返回菜单。

---

### 选项 23：SSH 端口转发管理

**函数**：`SSH_port_forwarding()`

```
获取服务器公网 IP（多 API 轮询）
读取当前面板设置：
  - webBasePath, port, listenIP, cert, key

判断状态：
  ├─ 已有证书+密钥 → "面板已配置 SSL，安全" → 返回
  ├─ 无证书且 listenIP 为空或 0.0.0.0 → "面板不安全" 警告
  └─ listenIP 已设置且非 0.0.0.0 → 显示 SSH 转发命令

子菜单：
  1. 设置监听 IP
     ├─ 默认 127.0.0.1 或自定义
     ├─ x-ui setting -listenIP ${ip}
     └─ 显示 SSH 转发命令：
         ssh -L 2222:${listenIP}:${port} root@${server_ip}
         访问 http://localhost:2222${webBasePath}

  2. 清除监听 IP
     └─ x-ui setting -listenIP 0.0.0.0 → 重启

  0. 返回
```

**用途**：将面板绑定到 127.0.0.1，只能通过 SSH 隧道访问，提高安全性。

---

### 选项 24：BBR 管理

**函数**：`bbr_menu()` — 子菜单入口

#### 子菜单

```
1. 启用 BBR
2. 禁用 BBR
0. 返回主菜单
```

#### 启用 BBR

**函数**：`enable_bbr()`

```
检查是否已启用（tcp_congestion_control == bbr 且 default_qdisc 为 fq/cake）
  ├─ 已启用 → 直接返回
  └─ 未启用 →
      有 /etc/sysctl.d/ →
        创建 /etc/sysctl.d/99-bbr-x-ui.conf：
          net.core.default_qdisc = fq
          net.ipv4.tcp_congestion_control = bbr
        注释 sysctl.conf 中的旧设置
        sysctl --system
      无 /etc/sysctl.d/ →
        直接修改 /etc/sysctl.conf
        sysctl -p

验证：tcp_congestion_control == bbr → "BBR 已成功启用"
```

**特性**：启用前会备份当前设置（写入注释行 `#旧qdisc:旧拥塞控制`），以便禁用时恢复。

#### 禁用 BBR

**函数**：`disable_bbr()`

```
检查是否已启用 → 未启用则返回

有 99-bbr-x-ui.conf →
  读取备份的旧设置
  恢复 net.core.default_qdisc 和 net.ipv4.tcp_congestion_control
  删除配置文件
  sysctl --system

无 99-bbr-x-ui.conf →
  将 sysctl.conf 中的 fq→pfifo_fast, bbr→cubic
  sysctl -p

验证：tcp_congestion_control != bbr → "BBR 已成功替换为 CUBIC"
```

---

### 选项 25：更新 Geo 文件

**函数**：`update_geo()` — 子菜单入口

#### 子菜单

```
1. Loyalsoldier (geoip.dat, geosite.dat)
2. chocolate4u (geoip_IR.dat, geosite_IR.dat)
3. runetfreedom (geoip_RU.dat, geosite_RU.dat)
4. 全部更新
0. 返回主菜单
```

#### 数据源

| 选项 | 数据源                                | 文件                         | 用途             |
|------|---------------------------------------|------------------------------|------------------|
| 1    | Loyalsoldier/v2ray-rules-dat          | geoip.dat, geosite.dat       | 通用规则         |
| 2    | chocolate4u/Iran-v2ray-rules          | geoip_IR.dat, geosite_IR.dat | 伊朗规则         |
| 3    | runetfreedom/russia-v2ray-rules-dat   | geoip_RU.dat, geosite_RU.dat | 俄罗斯规则       |
| 4    | 以上全部                              | 全部 6 个文件                | 一键更新         |

**下载逻辑** (`update_geofiles`)：

```
每个文件：
  curl -fLRo ${xui_folder}/bin/${dat}.dat
       -z ${xui_folder}/bin/${dat}.dat    ← 仅在远程更新时下载
       https://github.com/${source}/releases/latest/download/${remote_file}.dat
```

`-z` 参数确保只有远程文件比本地新时才下载，节省带宽。

更新后自动重启面板以加载新规则。

---

### 选项 26：网速测试 (Speedtest)

**函数**：`run_speedtest()`

```
检查 speedtest 命令是否存在
  └─ 不存在 →
      有 snap → snap install speedtest
      无 snap → 按包管理器安装：
        dnf/yum → rpm 包源
        apt-get/apt → deb 包源
        curl 安装脚本 → 包管理器安装

执行 speedtest
```

---

## 子命令（命令行模式）

当脚本以参数调用时（如 `x-ui start`），跳过交互菜单直接执行：

| 子命令                 | 对应菜单 | 附加行为                      |
|------------------------|----------|-------------------------------|
| `start`                | 11       | 执行后不返回菜单              |
| `stop`                 | 12       | 执行后不返回菜单              |
| `restart`              | 13       | 执行后不返回菜单              |
| `restart-xray`         | 14       | 执行后不返回菜单              |
| `status`               | 15       | 执行后不返回菜单              |
| `settings`             | 10       | 执行后不返回菜单              |
| `enable`               | 17       | 执行后不返回菜单              |
| `disable`              | 18       | 执行后不返回菜单              |
| `log`                  | 16       | 执行后不返回菜单              |
| `banlog`               | 4(限制)  | 执行后不返回菜单              |
| `update`               | 2        | 执行后不返回菜单              |
| `legacy`               | 4        | 执行后不返回菜单              |
| `install`              | 1        | 使用 check_uninstall 前置检查 |
| `uninstall`            | 5        | 执行后不返回菜单              |
| `update-all-geofiles`  | 25-4     | 更新后自动重启                |
| 无效参数               | —        | 显示用法帮助                  |

所有子命令传递参数 `0` 给功能函数，使其执行后不调用 `before_show_menu()` 返回菜单。

---

## 调用关系总览

```
x-ui.sh
  │
  ├─ show_menu()
  │   ├─ show_status() → check_status() + show_enable_status() + show_xray_status()
  │   ├─ 0: exit
  │   ├─ 1: install() → install.sh → start()
  │   ├─ 2: update() → update.sh
  │   ├─ 3: update_menu() → 下载 x-ui.sh
  │   ├─ 4: legacy_version() → install.sh v$version
  │   ├─ 5: uninstall() → 停止服务 + 删除文件
  │   ├─ 6: reset_user() → x-ui setting -username/-password
  │   ├─ 7: reset_webbasepath() → x-ui setting -webBasePath
  │   ├─ 8: reset_config() → x-ui setting -reset
  │   ├─ 9: set_port() → x-ui setting -port
  │   ├─ 10: check_config() → x-ui setting -show + ssl_cert_issue_for_ip()
  │   ├─ 11: start() → systemctl/rc-service start
  │   ├─ 12: stop() → systemctl/rc-service stop
  │   ├─ 13: restart() → systemctl/rc-service restart
  │   ├─ 14: restart_xray() → systemctl reload
  │   ├─ 15: status() → systemctl/rc-service status
  │   ├─ 16: show_log() → journalctl/grep messages
  │   ├─ 17: enable() → systemctl/rc-update enable
  │   ├─ 18: disable() → systemctl/rc-update disable
  │   ├─ 19: ssl_cert_issue_main()
  │   │     ├─ 1: ssl_cert_issue() → acme.sh 域名证书
  │   │     ├─ 2: 吊销证书 → acme.sh --revoke
  │   │     ├─ 3: 强制续期 → acme.sh --renew --force
  │   │     ├─ 4: 查看已有域名
  │   │     ├─ 5: 设置面板证书路径
  │   │     └─ 6: ssl_cert_issue_for_ip() → acme.sh IP 短期证书
  │   ├─ 20: ssl_cert_issue_CF() → acme.sh Cloudflare DNS 通配符证书
  │   ├─ 21: iplimit_main()
  │   │     ├─ 1: install_iplimit() → install fail2ban + create_iplimit_jails()
  │   │     ├─ 2: 修改封禁时长
  │   │     ├─ 3: 解封所有人
  │   │     ├─ 4: show_banlog()
  │   │     ├─ 5/6: 手动封禁/解封 IP
  │   │     ├─ 7: tail -f fail2ban.log
  │   │     ├─ 8/9: 服务状态/重启
  │   │     └─ 10: remove_iplimit()
  │   ├─ 22: firewall_menu() → UFW 防火墙管理
  │   ├─ 23: SSH_port_forwarding() → 设置 listenIP 为 127.0.0.1
  │   ├─ 24: bbr_menu() → enable_bbr() / disable_bbr()
  │   ├─ 25: update_geo() → update_geofiles() → 下载 geoip/geosite .dat
  │   └─ 26: run_speedtest() → speedtest
  │
  └─ 子命令模式（$# > 0）
      └─ case $1 in "start"|"stop"|... → 对应函数 0
```

---

## 关键设计决策

1. **Alpine 兼容**：所有服务管理操作都区分 Alpine (OpenRC) 和其他系统 (systemd)，通过 `$release` 变量判断。

2. **操作确认**：危险操作（卸载、重置凭据等）默认为 "n"，防止误操作。安全操作（更新等）默认为 "y"。

3. **子命令模式**：支持 `x-ui start` 等非交互式调用，传递参数 `0` 抑制 `before_show_menu()` 的回车等待。

4. **状态前置检查**：大多数菜单选项先调用 `check_install` 或 `check_uninstall`，确保操作的前提条件满足。

5. **等待机制**：start/stop/restart 后等待 2 秒再检查状态，给 systemd/init.d 足够时间完成操作。

6. **Geo 文件条件下载**：使用 `curl -z` 参数，仅在远程文件比本地新时才下载，节省带宽和时间。

7. **BBR 备份恢复**：启用 BBR 前将当前设置备份到注释行中，禁用时精确恢复原始值。

8. **Fail2ban jail 隔离**：IP 限制使用独立的 `3x-ipl` jail，与系统默认 jail 分离，互不影响。
