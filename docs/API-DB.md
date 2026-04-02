# 3x-ui 数据库相关接口

> 以下接口涉及数据库的读写操作（增删改查）或数据库文件的导入导出。

---

## 目录

- [1. 入站管理](#1-入站管理)
- [2. 客户端管理](#2-客户端管理)
- [3. 流量管理](#3-流量管理)
- [4. IP 记录管理](#4-ip-记录管理)
- [5. 面板配置](#5-面板配置)
- [6. 用户管理](#6-用户管理)
- [7. 数据库导入导出](#7-数据库导入导出)

---

## 1. 入站管理

### `GET /panel/api/inbounds/list`

查询数据库，获取当前用户的所有入站记录。

**响应 (`obj`)：** `[]Inbound`

---

### `GET /panel/api/inbounds/get/:id`

根据 ID 从数据库查询单条入站记录。

**URL 参数：** `:id`（int）

**响应 (`obj`)：** `Inbound` 对象。

---

### `POST /panel/api/inbounds/add`

向数据库写入一条新的入站记录。

**请求体：** `Inbound` 对象（JSON 或表单）。

**响应 (`obj`)：** 创建的 `Inbound` 对象。

---

### `POST /panel/api/inbounds/del/:id`

从数据库删除指定入站记录及其关联的客户端流量数据。

**URL 参数：** `:id`（int）

**响应 (`obj`)：** 被删除的入站 ID（int）。

---

### `POST /panel/api/inbounds/update/:id`

更新数据库中指定入站记录。

**URL 参数：** `:id`（int）

**请求体：** `Inbound` 对象（JSON 或表单）。

**响应 (`obj`)：** 更新后的 `Inbound` 对象。

---

### `POST /panel/api/inbounds/import`

通过 JSON 数据导入，向数据库写入一条新的入站记录。

**请求体**（表单）：

| 字段 | 类型 | 必填 |
|---|---|---|
| `data` | string (JSON) | 是 |

`data` 字段为 JSON 序列化的 `Inbound` 对象。

**响应 (`obj`)：** 创建的 `Inbound` 对象。

---

## 2. 客户端管理

### `GET /panel/api/inbounds/getClientTraffics/:email`

根据邮箱从数据库查询客户端流量记录。

**URL 参数：** `:email`（string）

**响应 (`obj`)：** `[]ClientTraffic`

---

### `GET /panel/api/inbounds/getClientTrafficsById/:id`

根据客户端 ID 从数据库查询流量记录。

**URL 参数：** `:id`（string）

**响应 (`obj`)：** `[]ClientTraffic`

---

### `POST /panel/api/inbounds/addClient`

向数据库写入新客户端记录，更新入站的 `Settings` 字段。

**请求体：** 包含新客户端信息的 `Inbound` 对象。

**响应：**

```json
{ "success": true, "msg": "Client added successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/:id/delClient/:clientId`

从数据库删除指定客户端记录，更新入站的 `Settings` 字段。

**URL 参数：** `:id`（int）、`:clientId`（string）

**响应：**

```json
{ "success": true, "msg": "Client deleted successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/updateClient/:clientId`

更新数据库中指定客户端的配置。

**URL 参数：** `:clientId`（string）

**请求体：** 包含更新后客户端设置的 `Inbound` 对象。

**响应：**

```json
{ "success": true, "msg": "Client configuration updated successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/:id/delClientByEmail/:email`

根据邮箱从数据库删除客户端记录。

**URL 参数：** `:id`（int）、`:email`（string）

**响应：**

```json
{ "success": true, "msg": "Client deleted successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/delDepletedClients/:id`

批量删除数据库中指定入站下所有流量耗尽的客户端记录。

**URL 参数：** `:id`（int）

**响应：**

```json
{ "success": true, "msg": "Depleted clients deleted successfully", "obj": null }
```

---

## 3. 流量管理

### `POST /panel/api/inbounds/:id/resetClientTraffic/:email`

将数据库中指定客户端的上行、下行流量重置为 0。

**URL 参数：** `:id`（int）、`:email`（string）

**响应：**

```json
{ "success": true, "msg": "Client traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/resetAllTraffics`

将数据库中所有入站的上行、下行流量重置为 0。

**响应：**

```json
{ "success": true, "msg": "All traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/resetAllClientTraffics/:id`

将数据库中指定入站下所有客户端的上行、下行流量重置为 0。

**URL 参数：** `:id`（int）

**响应：**

```json
{ "success": true, "msg": "All client traffic reset successfully", "obj": null }
```

---

### `POST /panel/api/inbounds/updateClientTraffic/:email`

手动修改数据库中指定客户端的流量数值。

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

## 4. IP 记录管理

### `POST /panel/api/inbounds/clientIps/:email`

从数据库查询客户端关联的 IP 地址记录。

**URL 参数：** `:email`（string）

**响应 (`obj`)：**

- `[]string`，格式为 `"IP (YYYY-MM-DD HH:MM:SS)"`（含时间戳时）
- `[]string`，纯 IP 字符串（旧格式）
- `"No IP Record"`（无数据时）

---

### `POST /panel/api/inbounds/clearClientIps/:email`

清除数据库中指定客户端的 IP 记录。

**URL 参数：** `:email`（string）

**响应：**

```json
{ "success": true, "msg": "Log cleanup successful", "obj": null }
```

---

## 5. 面板配置

### `POST /panel/setting/all`

从数据库查询所有面板配置项。

**响应 (`obj`)：** `AllSetting` 对象。

---

### `POST /panel/setting/update`

将配置写入数据库（批量更新面板设置）。

**请求体：** `AllSetting` 对象（JSON 或表单）。

**响应：**

```json
{ "success": true, "msg": "Settings modified successfully", "obj": null }
```

---

## 6. 用户管理

### `POST /panel/setting/updateUser`

修改数据库中的管理员用户名和密码。

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

## 7. 数据库导入导出

### `GET /panel/api/server/getDb`

导出整个 SQLite 数据库文件（`x-ui.db`）。

**响应：** 二进制文件下载（`application/octet-stream`，文件名 `x-ui.db`）。不使用 `Msg` 包装格式。

---

### `POST /panel/api/server/importDB`

导入数据库备份文件，覆盖当前数据库。导入后自动重启 Xray 服务。

**请求体：** multipart 文件上传（字段名 `db`）。

**响应 (`obj`)：** `"Database imported successfully"`
