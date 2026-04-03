# 3x-ui API 文档

> 基础路径: `{basePath}`（可配置，默认 `/`）
> Web 框架: Gin (Go)
> 所有 JSON API 响应（除特别说明外）使用统一格式: `{ "success": bool, "msg": string, "obj": any }`

---

## 目录

- [1. 认证接口](#1-认证接口)
- [2. 面板页面](#2-面板页面)
- [3. 面板设置](#3-面板设置)
- [4. Xray 设置](#4-xray-设置)
- [5. 入站 API](#5-入站-api)
- [6. 服务器 API](#6-服务器-api)
- [7. WebSocket](#7-websocket)
- [8. 订阅服务](#8-订阅服务)
- [数据模型](#数据模型)

---

## 1. 认证接口

### `GET /`

根路径。已认证时重定向到 `/panel/`，否则显示登录页面。

---

### `POST /login`

用户登录认证。

**请求体**（JSON 或表单）：

```json
{
  "username": "string",
  "password": "string",
  "twoFactorCode": "string"  // 可选，开启双因素认证时必填
}
```

**成功响应：**

```json
{ "success": true, "msg": "Successfully logged in", "obj": null }
```

**错误响应：**

- `success: false, msg: "Username cannot be empty"`
- `success: false, msg: "Password cannot be empty"`
- `success: false, msg: "Wrong username or password"`

---

### `GET /logout`

清除会话并重定向到基础路径。

---

### `POST /getTwoFactorEnable`

查询是否开启双因素认证。

**响应：**

```json
{ "success": true, "msg": "", "obj": true }
```

---

## 2. 面板页面

> `/panel` 下的所有路由需要认证（`checkLogin` 中间件）。

### `GET /panel/`

面板主页面（HTML）。

### `GET /panel/inbounds`

入站管理页面（HTML）。

### `GET /panel/settings`

面板设置页面（HTML）。

### `GET /panel/xray`

Xray 配置页面（HTML）。

---

## 3. 面板设置

> `/panel/setting` 下的所有路由需要认证。

### `POST /panel/setting/all`

获取所有面板配置。

**响应 (`obj`)：** `AllSetting` 对象（详见 [数据模型](#allsetting)）。

---

### `POST /panel/setting/defaultSettings`

获取默认配置（根据请求 Host 头生成）。

**响应 (`obj`)：** 默认 `AllSetting` 对象。

---

### `POST /panel/setting/update`

更新面板配置。

**请求体：** `AllSetting` 对象（JSON 或表单）。

**响应：**

```json
{ "success": true, "msg": "Settings modified successfully", "obj": null }
```

---

### `POST /panel/setting/updateUser`

修改管理员用户名和密码。

**请求体**（JSON 或表单）：

```json
{
  "oldUsername": "string",
  "oldPassword": "string",
  "newUsername": "string",
  "newPassword": "string"
}
```

**成功响应：**

```json
{ "success": true, "msg": "User modified successfully", "obj": null }
```

**错误响应：**

- `msg: "User modification failed: original username/password incorrect"`
- `msg: "User modification failed: username and password cannot be empty"`

---

### `POST /panel/setting/restartPanel`

重启面板（3 秒后重启）。

**响应：**

```json
{ "success": true, "msg": "Panel restart successful", "obj": null }
```

---

### `GET /panel/setting/getDefaultJsonConfig`

获取默认 Xray JSON 配置。

**响应 (`obj`)：** Xray 配置 JSON 对象。

---

## 4. Xray 设置

> `/panel/xray` 下的所有路由需要认证。

### `POST /panel/xray/`

获取当前 Xray 配置及元数据。

**响应 (`obj`)：**

```json
{
  "xraySetting": { /* Xray 配置 JSON */ },
  "inboundTags": ["tag1", "tag2"],
  "outboundTestUrl": "https://www.google.com/generate_204"
}
```

---

### `GET /panel/xray/getDefaultJsonConfig`

获取默认 Xray JSON 配置。

**响应 (`obj`)：** Xray 配置 JSON 对象。

---

### `GET /panel/xray/getOutboundsTraffic`

获取出站流量统计。

**响应 (`obj`)：** `OutboundTraffics` 对象数组。

---

### `GET /panel/xray/getXrayResult`

获取当前 Xray 服务运行状态。

**响应 (`obj`)：** Xray 服务状态字符串。

---

### `POST /panel/xray/warp/:action`

管理 Cloudflare Warp 集成。

**URL 参数：**

| 参数 | 可选值 |
|---|---|
| `:action` | `data`、`del`、`config`、`reg`、`license` |

**请求体**（表单，取决于 action）：

| Action | 字段 |
|---|---|
| `reg` | `privateKey`、`publicKey` |
| `license` | `license` |
| 其他 | 无 |

**响应 (`obj`)：** Warp 数据/配置字符串。`del` 操作返回 `obj: null`。

---

### `POST /panel/xray/update`

更新 Xray 配置。

**请求体**（表单）：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `xraySetting` | string (JSON) | 是 | Xray 配置 JSON |
| `outboundTestUrl` | string | 否 | 默认: `https://www.google.com/generate_204` |

**响应：**

```json
{ "success": true, "msg": "Settings modified successfully", "obj": null }
```

---

### `POST /panel/xray/resetOutboundsTraffic`

重置出站流量统计。

**请求体**（表单）：

| 字段 | 类型 | 必填 |
|---|---|---|
| `tag` | string | 是 |

**响应 (`obj`)：** `""`

---

### `POST /panel/xray/testOutbound`

测试出站连通性。

**请求体**（表单）：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `outbound` | string (JSON) | 是 | 要测试的出站配置 |
| `allOutbounds` | string (JSON 数组) | 否 | 所有出站配置（用于解析 dialerProxy 依赖） |

**响应 (`obj`)：** 测试结果，包含延迟/响应时间。

---

## 5. 入站 API

> `/panel/api` 下的所有路由需要通过 `checkAPIAuth` 中间件认证（未认证时返回 404 以隐藏接口存在）。

### `GET /panel/api/inbounds/list`

获取当前用户的所有入站列表。

**响应 (`obj`)：** `[]Inbound`

---

### `GET /panel/api/inbounds/get/:id`

根据 ID 获取指定入站。

**URL 参数：** `:id`（int）

**响应 (`obj`)：** `Inbound` 对象。

---

### `GET /panel/api/inbounds/getClientTraffics/:email`

根据邮箱获取客户端流量记录。

**URL 参数：** `:email`（string）

**响应 (`obj`)：** `[]ClientTraffic`

---

### `GET /panel/api/inbounds/getClientTrafficsById/:id`

根据客户端 ID 获取流量记录。

**URL 参数：** `:id`（string）

**响应 (`obj`)：** `[]ClientTraffic`

---

### `POST /panel/api/inbounds/add`

添加新入站。

**请求体：** `Inbound` 对象（JSON 或表单，详见 [数据模型](#inbound)）。

**响应 (`obj`)：** 创建的 `Inbound` 对象。通过 WebSocket 广播更新。

---

### `POST /panel/api/inbounds/del/:id`

删除入站。

**URL 参数：** `:id`（int）

**响应 (`obj`)：** 被删除的入站 ID（int）。通过 WebSocket 广播更新。

---

### `POST /panel/api/inbounds/update/:id`

更新入站。

**URL 参数：** `:id`（int）

**请求体：** `Inbound` 对象（JSON 或表单）。

**响应 (`obj`)：** 更新后的 `Inbound` 对象。通过 WebSocket 广播更新。

---

### `POST /panel/api/inbounds/clientIps/:email`

获取客户端关联的 IP 地址。

**URL 参数：** `:email`（string）

**响应 (`obj`)：**

- `[]string`，格式为 `"IP (YYYY-MM-DD HH:MM:SS)"`（含时间戳时）
- `[]string`，纯 IP 字符串（旧格式）
- `"No IP Record"`（无数据时）

---

### `POST /panel/api/inbounds/clearClientIps/:email`

清除客户端的 IP 记录。

**URL 参数：** `:email`（string）

**响应：**

```json
{ "success": true, "msg": "Log cleanup successful", "obj": null }
```

---

### `POST /panel/api/inbounds/addClient`

向入站添加客户端。

**请求体：** `Inbound` 对象，新客户端信息在其 `Settings` JSON 中。

**响应：**

```json
{ "success": true, "msg": "Client added successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/:id/delClient/:clientId`

从入站删除客户端。

**URL 参数：** `:id`（int）、`:clientId`（string）

**响应：**

```json
{ "success": true, "msg": "Client deleted successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/updateClient/:clientId`

更新客户端配置。

**URL 参数：** `:clientId`（string）

**请求体：** 包含更新后客户端设置的 `Inbound` 对象。

**响应：**

```json
{ "success": true, "msg": "Client configuration updated successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/:id/resetClientTraffic/:email`

重置指定客户端的流量统计。

**URL 参数：** `:id`（int）、`:email`（string）

**响应：**

```json
{ "success": true, "msg": "Client traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/resetAllTraffics`

重置所有流量统计。

**响应：**

```json
{ "success": true, "msg": "All traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/resetAllClientTraffics/:id`

重置入站下所有客户端的流量。

**URL 参数：** `:id`（int）

**响应：**

```json
{ "success": true, "msg": "All client traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/delDepletedClients/:id`

删除入站中所有流量耗尽的客户端。

**URL 参数：** `:id`（int）

**响应：**

```json
{ "success": true, "msg": "Depleted clients deleted successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/import`

导入入站配置。

**请求体**（表单）：

| 字段 | 类型 | 必填 |
|---|---|---|
| `data` | string (JSON) | 是 |

`data` 字段为 JSON 序列化的 `Inbound` 对象。

**响应 (`obj`)：** 创建的 `Inbound` 对象。通过 WebSocket 广播更新。

---

### `POST /panel/api/inbounds/onlines`

获取当前在线客户端。

**响应 (`obj`)：** 在线客户端标识列表。

---

### `POST /panel/api/inbounds/lastOnline`

获取所有客户端的最后在线时间。

**响应 (`obj`)：** 客户端邮箱到最近在线时间戳的映射。

---

### `POST /panel/api/inbounds/updateClientTraffic/:email`

手动更新客户端流量统计。

**URL 参数：** `:email`（string）

**请求体**（JSON）：

```json
{
  "upload": 0,
  "download": 0
}
```

**响应：**

```json
{ "success": true, "msg": "Client configuration updated successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/:id/delClientByEmail/:email`

根据邮箱删除客户端。

**URL 参数：** `:id`（int）、`:email`（string）

**响应：**

```json
{ "success": true, "msg": "Client deleted successfully", "obj": null }
```

---

### `GET /panel/api/backuptotgbot`

向 Telegram Bot 管理员发送数据库备份。

**响应：** 空 `200 OK`。

---

## 6. 服务器 API

> `/panel/api/server` 下的所有路由需要通过 `checkAPIAuth` 认证。

### `GET /panel/api/server/status`

获取服务器状态（CPU、内存、磁盘、网络、运行时间）。

**响应 (`obj`)：** 服务器状态对象（每 2 秒刷新）。

---

### `GET /panel/api/server/cpuHistory/:bucket`

获取 CPU 使用历史。

**URL 参数：** `:bucket`（int）—— 每个数据点的秒数。

| 允许值 | 说明 |
|---|---|
| `2` | 每 2 秒一个点（最近 120 秒） |
| `30` | 每 30 秒一个点（最近 30 分钟） |
| `60` | 每 1 分钟一个点（最近 1 小时） |
| `120` | 每 2 分钟一个点（最近 2 小时） |
| `180` | 每 3 分钟一个点（最近 3 小时） |
| `300` | 每 5 分钟一个点（最近 5 小时） |

**响应 (`obj`)：** CPU 历史数据点数组（最多 60 个采样点）。

---

### `GET /panel/api/server/getXrayVersion`

获取可用的 Xray 版本列表。

**响应 (`obj`)：** `[]string` —— 版本列表（缓存 60 秒）。

---

### `GET /panel/api/server/getConfigJson`

获取当前 Xray 配置 JSON。

**响应 (`obj`)：** Xray 配置 JSON。

---

### `GET /panel/api/server/getDb`

下载 SQLite 数据库文件。

**响应：** 二进制文件下载（`application/octet-stream`，文件名 `x-ui.db`）。不使用 `Msg` 包装格式。

---

### `GET /panel/api/server/getNewUUID`

生成新的 UUID。

**响应 (`obj`)：** UUID 字符串。

---

### `GET /panel/api/server/getNewX25519Cert`

生成新的 X25519 密钥对。

**响应 (`obj`)：** X25519 密钥对数据。

---

### `GET /panel/api/server/getNewmldsa65`

生成新的 ML-DSA-65 密钥。

**响应 (`obj`)：** ML-DSA-65 密钥数据。

---

### `GET /panel/api/server/getNewmlkem768`

生成新的 ML-KEM-768 密钥。

**响应 (`obj`)：** ML-KEM-768 密钥数据。

---

### `GET /panel/api/server/getNewVlessEnc`

生成新的 VLESS 加密密钥。

**响应 (`obj`)：** VLESS 加密密钥数据。

---

### `POST /panel/api/server/stopXrayService`

停止 Xray 服务。通过 WebSocket 广播状态变更。

**响应：**

```json
{ "success": true, "msg": "Xray service stopped successfully", "obj": null }
```

---

### `POST /panel/api/server/restartXrayService`

重启 Xray 服务。通过 WebSocket 广播状态变更。

**响应：**

```json
{ "success": true, "msg": "Xray service restarted successfully", "obj": null }
```

---

### `POST /panel/api/server/installXray/:version`

安装/切换到指定 Xray 版本。

**URL 参数：** `:version`（string）

**响应：**

```json
{ "success": true, "msg": "Xray version switched", "obj": null }
```

---

### `POST /panel/api/server/updateGeofile`

### `POST /panel/api/server/updateGeofile/:fileName`

更新 GeoIP/Geosite 数据文件。

**URL 参数：** `:fileName`（string，可选）—— 指定要更新的文件名。省略时更新所有文件。

**响应：**

```json
{ "success": true, "msg": "Geo file update result", "obj": null }
```

---

### `POST /panel/api/server/logs/:count`

获取应用日志。

**URL 参数：** `:count`（int）—— 日志行数。

**请求体**（表单）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `level` | string | 日志级别过滤 |
| `syslog` | string | 系统日志过滤 |

**响应 (`obj`)：** 日志内容字符串。

---

### `POST /panel/api/server/xraylogs/:count`

获取 Xray 服务日志。

**URL 参数：** `:count`（int）—— 日志行数。

**请求体**（表单）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `filter` | string | 日志过滤关键词 |
| `showDirect` | string | 显示直连日志 |
| `showBlocked` | string | 显示被阻止的日志 |
| `showProxy` | string | 显示代理日志 |

**响应 (`obj`)：** 过滤后的 Xray 日志内容字符串。

---

### `POST /panel/api/server/importDB`

导入数据库备份。导入后自动重启 Xray。

**请求体：** multipart 文件上传（字段名 `db`）。

**响应 (`obj`)：** `"Database imported successfully"`

---

### `POST /panel/api/server/getNewEchCert`

生成新的 ECH（Encrypted Client Hello）证书。

**请求体**（表单）：

| 字段 | 类型 | 必填 |
|---|---|---|
| `sni` | string | 是 |

**响应 (`obj`)：** ECH 证书数据。

---

## 7. WebSocket

### `GET {basePath}/ws`

WebSocket 端点，用于面板实时更新（服务器状态、入站变更、Xray 状态、通知）。

---

## 8. 订阅服务

> 运行在独立端口（可配置）。独立的 Gin 服务器。

### `GET {subPath}:subid`

获取客户端订阅链接。

**URL 参数：** `:subid`（string）—— 订阅 ID。

**查询参数：**

| 参数 | 说明 |
|---|---|
| `html=1` 或 `view=html` | 强制渲染 HTML 页面 |

**响应（订阅客户端）：**

- Content-Type: `text/plain`
- Body: 每行一个代理分享链接（`vmess://`、`vless://`、`trojan://`、`ss://`）
- 若开启 `subEncrypt`：body 经过 base64 编码

**响应（浏览器 / `?html=1`）：**

- Content-Type: `text/html`
- HTML 页面，展示流量统计、到期时间、代理链接

**响应头：**

| 响应头 | 说明 |
|---|---|
| `Subscription-Userinfo` | `upload=N; download=N; total=N; expire=N`（字节数，Unix 时间戳） |
| `Profile-Update-Interval` | 重新拉取间隔（分钟，默认 `10`） |
| `Profile-Title` | Base64 编码的配置标题（已配置时） |
| `Support-Url` | 支持页面 URL（已配置时） |
| `Profile-Web-Page-Url` | 配置网页 URL |
| `Announce` | Base64 编码的公告（已配置时） |
| `Routing-Enable` | `"true"` 或 `"false"` |
| `Routing` | 自定义路由规则 JSON（已配置时） |

**错误：** `400 Bad Request`，body 为 `"Error!"`

---

### `GET {jsonPath}:subid`

获取 JSON 格式订阅配置（仅在 `subJsonEnable` 为 true 时注册此路由）。

**URL 参数：** `:subid`（string）—— 订阅 ID。

**响应：**

- Content-Type: `text/plain`
- Body: JSON 字符串，包含完整客户端配置（分片、噪声、多路复用、路由规则）
- 响应头与订阅链接接口相同

**错误：** `400 Bad Request`，body 为 `"Error!"`

---

## 数据模型

### 统一响应格式 (`Msg`)

```json
{
  "success": true,
  "msg": "string",
  "obj": null
}
```

### Inbound

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 主键 |
| `up` | int64 | 上行流量（字节） |
| `down` | int64 | 下行流量（字节） |
| `total` | int64 | 总流量限制（字节） |
| `allTime` | int64 | 累计总流量（字节） |
| `remark` | string | 入站备注/名称 |
| `enable` | bool | 是否启用 |
| `expiryTime` | int64 | 过期时间戳（毫秒，0 = 永不过期） |
| `trafficReset` | string | 流量重置周期（默认 `"never"`） |
| `lastTrafficResetTime` | int64 | 上次流量重置时间戳 |
| `clientStats` | []ClientTraffic | 客户端流量统计 |
| `listen` | string | 监听地址 |
| `port` | int | 监听端口 |
| `protocol` | string | 协议：`vmess`、`vless`、`trojan`、`shadowsocks`、`http`、`mixed`、`wireguard`、`tunnel` |
| `settings` | string (JSON) | 协议相关设置 |
| `streamSettings` | string (JSON) | 传输/流设置 |
| `tag` | string | 唯一的 Xray 入站标签 |
| `sniffing` | string (JSON) | 探测配置 |

### ClientTraffic

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 主键 |
| `inboundId` | int | 所属入站 ID |
| `enable` | bool | 是否启用 |
| `email` | string | 客户端邮箱（唯一） |
| `uuid` | string | 客户端 UUID（非持久化） |
| `subId` | string | 订阅 ID（非持久化） |
| `up` | int64 | 上行流量（字节） |
| `down` | int64 | 下行流量（字节） |
| `allTime` | int64 | 累计总流量（字节） |
| `expiryTime` | int64 | 过期时间戳（毫秒） |
| `total` | int64 | 总流量限制（字节） |
| `reset` | int | 流量重置计数器 |
| `lastOnline` | int64 | 最后在线时间戳 |

### Client

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 客户端 ID |
| `security` | string | 加密方式 |
| `password` | string | 客户端密码 |
| `flow` | string | VLESS flow 类型 |
| `email` | string | 客户端邮箱 |
| `limitIp` | int | IP 限制（0 = 不限制） |
| `totalGB` | int64 | 流量限制（字节） |
| `expiryTime` | int64 | 过期时间戳（毫秒） |
| `enable` | bool | 是否启用 |
| `tgId` | int64 | Telegram 用户 ID |
| `subId` | string | 订阅 ID |
| `comment` | string | 客户端备注 |
| `reset` | int | 流量重置计数器 |

### User

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 主键 |
| `username` | string | 登录用户名 |
| `password` | string | 登录密码 |

### OutboundTraffics

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 主键 |
| `tag` | string | 出站标签（唯一） |
| `up` | int64 | 上行流量（字节） |
| `down` | int64 | 下行流量（字节） |
| `total` | int64 | 总流量限制（字节） |

### InboundClientIps

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 主键 |
| `clientEmail` | string | 客户端邮箱（唯一） |
| `ips` | string | IP 地址（JSON 字符串） |

### AllSetting

**Web 服务器：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `webListen` | string | 监听地址 |
| `webDomain` | string | 域名 |
| `webPort` | int | 端口 |
| `webCertFile` | string | TLS 证书文件路径 |
| `webKeyFile` | string | TLS 私钥文件路径 |
| `webBasePath` | string | 基础路径 |
| `sessionMaxAge` | int | 会话最大有效期（天） |

**界面：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `pageSize` | int | 分页大小 |
| `expireDiff` | int | 到期提醒天数差 |
| `trafficDiff` | int | 流量提醒差值 |
| `remarkModel` | string | 备注显示模式 |
| `datepicker` | string | 日期选择器格式 |

**Telegram Bot：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `tgBotEnable` | bool | 是否启用 Telegram Bot |
| `tgBotToken` | string | Bot Token |
| `tgBotProxy` | string | Bot 代理地址 |
| `tgBotAPIServer` | string | Bot API 服务器地址 |
| `tgBotChatId` | string | 管理员 Chat ID |
| `tgRunTime` | string | 定时任务执行时间 |
| `tgBotBackup` | bool | 是否启用自动备份 |
| `tgBotLoginNotify` | bool | 是否启用登录通知 |
| `tgCpu` | int | CPU 告警阈值 |
| `tgLang` | string | Bot 语言 |

**安全：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `timeLocation` | string | 时区 |
| `twoFactorEnable` | bool | 是否开启双因素认证 |
| `twoFactorToken` | string | 双因素认证令牌 |

**订阅服务：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `subEnable` | bool | 是否启用订阅服务 |
| `subJsonEnable` | bool | 是否启用 JSON 订阅 |
| `subTitle` | string | 订阅标题 |
| `subSupportUrl` | string | 支持页面 URL |
| `subProfileUrl` | string | 配置页面 URL |
| `subAnnounce` | string | 公告内容 |
| `subEnableRouting` | bool | 是否启用路由规则 |
| `subRoutingRules` | string | 自定义路由规则 |
| `subListen` | string | 订阅服务监听地址 |
| `subPort` | int | 订阅服务端口 |
| `subPath` | string | 订阅路径 |
| `subDomain` | string | 订阅服务域名 |
| `subCertFile` | string | 订阅服务 TLS 证书路径 |
| `subKeyFile` | string | 订阅服务 TLS 私钥路径 |
| `subUpdates` | int | 客户端更新间隔（分钟） |
| `subEncrypt` | bool | 是否加密订阅内容 |
| `subShowInfo` | bool | 是否显示服务器信息 |
| `subURI` | string | 订阅 URI |
| `subJsonPath` | string | JSON 订阅路径 |
| `subJsonURI` | string | JSON 订阅 URI |
| `subJsonFragment` | string | TLS 分片配置 |
| `subJsonNoises` | string | WebSocket/HTTP 噪声配置 |
| `subJsonMux` | string | 多路复用配置 |
| `subJsonRules` | string | 自定义路由规则 |
| `externalTrafficInformEnable` | bool | 是否启用外部流量通知 |
| `externalTrafficInformURI` | string | 外部流量通知 URI |

**LDAP：**

| 字段 | 类型 | 说明 |
|---|---|---|
| `ldapEnable` | bool | 是否启用 LDAP |
| `ldapHost` | string | LDAP 服务器地址 |
| `ldapPort` | int | LDAP 端口 |
| `ldapUseTLS` | bool | 是否使用 TLS |
| `ldapBindDN` | string | 绑定 DN |
| `ldapPassword` | string | 绑定密码 |
| `ldapBaseDN` | string | 基础 DN |
| `ldapUserFilter` | string | 用户过滤器 |
| `ldapUserAttr` | string | 用户属性 |
| `ldapVlessField` | string | VLESS 字段映射 |
| `ldapSyncCron` | string | 同步周期（Cron 表达式） |
| `ldapFlagField` | string | 标志字段 |
| `ldapTruthyValues` | string | 真值列表 |
| `ldapInvertFlag` | bool | 是否反转标志 |
| `ldapInboundTags` | string | 关联入站标签 |
| `ldapAutoCreate` | bool | 是否自动创建客户端 |
| `ldapAutoDelete` | bool | 是否自动删除客户端 |
| `ldapDefaultTotalGB` | int | 默认流量限制（GB） |
| `ldapDefaultExpiryDays` | int | 默认有效天数 |
| `ldapDefaultLimitIP` | int | 默认 IP 限制 |

---

## 向后兼容重定向

以下重定向自动处理（301）：

| 原路径 | 重定向到 |
|---|---|
| `/panel/API/*` | `/panel/api/*` |
| `/xui/API/*` | `/panel/api/*` |
| `/xui/*` | `/panel/*` |
