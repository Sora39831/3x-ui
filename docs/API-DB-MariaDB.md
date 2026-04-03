# 3x-ui MariaDB 迁移 API 文档

> 本文档说明如何将 3x-ui 面板的数据库从 SQLite 迁移到 MariaDB，涵盖所有数据库相关的 API 接口、数据模型映射、SQL 差异及兼容性改造方案。

---

## 目录

- [1. 架构概览与迁移总览](#1-架构概览与迁移总览)
- [2. 环境准备与依赖](#2-环境准备与依赖)
- [3. 数据库连接层改造](#3-数据库连接层改造)
- [4. 数据模型与 MariaDB 表结构映射](#4-数据模型与-mariadb-表结构映射)
- [5. SQLite 特有 SQL 的 MariaDB 替代方案](#5-sqlite-特有-sql-的-mariadb-替代方案)
- [6. 入站管理 API（数据库层）](#6-入站管理-api数据库层)
- [7. 客户端管理 API（数据库层）](#7-客户端管理-api数据库层)
- [8. 流量管理 API（数据库层）](#8-流量管理-api数据库层)
- [9. IP 记录管理 API（数据库层）](#9-ip-记录管理-api数据库层)
- [10. 面板配置 API（数据库层）](#10-面板配置-api数据库层)
- [11. 用户管理 API（数据库层）](#11-用户管理-api数据库层)
- [12. 数据库导入导出（MariaDB 方案）](#12-数据库导入导出mariadb-方案)
- [13. 订阅服务的数据库查询改造](#13-订阅服务的数据库查询改造)
- [14. 数据库维护操作（MariaDB 对应）](#14-数据库维护操作mariadb-对应)
- [15. 迁移实施步骤](#15-迁移实施步骤)

---

## 1. 架构概览与迁移总览

### 1.1 当前架构（SQLite）

```
┌─────────────┐     ┌──────────┐     ┌───────────────┐
│  Web Panel  │────▶│  GORM    │────▶│   SQLite DB   │
│  (Gin)      │     │ v1.31.1  │     │  x-ui.db      │
└─────────────┘     └──────────┘     └───────────────┘
                         │
                    sqlite driver
                  (mattn/go-sqlite3)
```

### 1.2 目标架构（MariaDB）

```
┌─────────────┐     ┌──────────┐     ┌───────────────┐
│  Web Panel  │────▶│  GORM    │────▶│   MariaDB     │
│  (Gin)      │     │ v1.31.1  │     │  Server       │
└─────────────┘     └──────────┘     └───────────────┘
                         │
                    mysql driver
               (go-sqlite3 → go-mysql-driver)
```

### 1.3 迁移范围

| 组件 | SQLite 特有 | MariaDB 兼容 | 需改造 |
|---|---|---|---|
| `database/db.go` 连接初始化 | `sqlite.Open()` | `mysql.Open()` | **是** |
| GORM 标准 CRUD | `db.Where/First/Create/Save/Delete` | 相同 | 否 |
| `gorm.Expr` 原子递增 | `gorm.Expr("up + ?", val)` | 相同 | 否 |
| `JSON_EACH()` 原始 SQL | SQLite 特有 | **无等价函数** | **是** |
| `PRAGMA wal_checkpoint` | SQLite 特有 | `FLUSH TABLES` | **是** |
| `PRAGMA integrity_check` | SQLite 特有 | `CHECK TABLE` | **是** |
| `VACUUM` | SQLite 特有 | `OPTIMIZE TABLE` | **是** |
| `IsSQLiteDB()` | 检查 SQLite 文件头 | 不适用 | **是** |
| `JSON_EXTRACT()` / `JSON_TYPE()` | SQLite JSON1 | MariaDB 原生支持 | 否 |
| `IFNULL()` | SQLite | MariaDB `IFNULL()` (也支持 `COALESCE`) | 否 |
| `GROUP_CONCAT()` | SQLite | MariaDB 支持 | 否 |
| `LIKE` 模糊查询 | SQLite | MariaDB 支持 | 否 |
| `AutoMigrate` | GORM 自动生成 | GORM 自动生成 | 否 |

---

## 2. 环境准备与依赖

### 2.1 Go 依赖替换

在 `go.mod` 中：

**移除：**
```
gorm.io/driver/sqlite v1.6.0
github.com/mattn/go-sqlite3 v1.14.38
```

**新增：**
```
gorm.io/driver/mysql v1.5.7
```

### 2.2 环境变量

| 环境变量 | 说明 | 示例 |
|---|---|---|
| `XUI_DB_DRIVER` | 数据库驱动类型 | `mysql`（默认 `sqlite`） |
| `XUI_DB_HOST` | MariaDB 主机地址 | `127.0.0.1` |
| `XUI_DB_PORT` | MariaDB 端口 | `3306` |
| `XUI_DB_NAME` | 数据库名 | `xui` |
| `XUI_DB_USER` | 数据库用户 | `xui` |
| `XUI_DB_PASSWORD` | 数据库密码 | `secret` |
| `XUI_DB_CHARSET` | 字符集 | `utf8mb4` |
| `XUI_DB_FOLDER` | SQLite 数据库文件夹（保留向后兼容） | `/etc/x-ui` |

### 2.3 MariaDB 数据库初始化

```sql
CREATE DATABASE xui CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'xui'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON xui.* TO 'xui'@'localhost';
FLUSH PRIVILEGES;
```

---

## 3. 数据库连接层改造

> 原始文件：`database/db.go`

### 3.1 连接初始化（InitDB）

**当前 SQLite 实现（第 141 行）：**
```go
db, err = gorm.Open(sqlite.Open(dbPath), c)
```

**MariaDB 改造：**
```go
import (
    "gorm.io/driver/mysql"
)

func InitDB(dbDriver, dsn string) error {
    var dialector gorm.Dialector

    switch dbDriver {
    case "mysql":
        dialector = mysql.Open(dsn)
    default:
        // 保持 SQLite 向后兼容
        dialector = sqlite.Open(dsn)
    }

    db, err = gorm.Open(dialector, c)
    // ... 后续逻辑不变
}
```

### 3.2 DSN 连接字符串格式

```
xui:password@tcp(127.0.0.1:3306)/xui?charset=utf8mb4&parseTime=True&loc=Local
```

### 3.3 不再需要的功能

| 函数 | 原用途 | MariaDB 处理 |
|---|---|---|
| `IsSQLiteDB()` | 验证 SQLite 文件头 | MariaDB 下返回 `false` 或跳过 |
| `ValidateSQLiteDB()` | `PRAGMA integrity_check` | 改用 `CHECK TABLE` 或 `mysqladmin check` |
| `Checkpoint()` | `PRAGMA wal_checkpoint` | 改用 `FLUSH TABLES` |

---

## 4. 数据模型与 MariaDB 表结构映射

> 原始文件：`database/model/model.go`, `xray/client_traffic.go`

GORM 的 `AutoMigrate` 在 MariaDB 下会自动创建表。以下是每个模型到 MariaDB 表结构的映射。

### 4.1 `users` 表

**Go 结构体：**
```go
type User struct {
    Id       int    `gorm:"primaryKey;autoIncrement"`
    Username string
    Password string
}
```

**MariaDB DDL：**
```sql
CREATE TABLE users (
    id       INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(255) NOT NULL DEFAULT '',
    password VARCHAR(255) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.2 `inbounds` 表

**Go 结构体：**
```go
type Inbound struct {
    Id                   int    `gorm:"primaryKey;autoIncrement"`
    UserId               int
    Up                   int64
    Down                 int64
    Total                int64
    AllTime              int64  `gorm:"default:0"`
    Remark               string
    Enable               bool   `gorm:"index:idx_enable_traffic_reset,priority:1"`
    ExpiryTime           int64
    TrafficReset         string `gorm:"default:never;index:idx_enable_traffic_reset,priority:2"`
    LastTrafficResetTime int64  `gorm:"default:0"`
    Listen               string
    Port                 int
    Protocol             Protocol
    Settings             string  -- LONGTEXT (JSON)
    StreamSettings       string  -- LONGTEXT (JSON)
    Tag                  string  `gorm:"unique"`
    Sniffing             string  -- LONGTEXT (JSON)
}
```

**MariaDB DDL：**
```sql
CREATE TABLE inbounds (
    id                     INT AUTO_INCREMENT PRIMARY KEY,
    user_id                INT NOT NULL DEFAULT 0,
    up                     BIGINT NOT NULL DEFAULT 0,
    down                   BIGINT NOT NULL DEFAULT 0,
    total                  BIGINT NOT NULL DEFAULT 0,
    all_time               BIGINT NOT NULL DEFAULT 0,
    remark                 VARCHAR(255) NOT NULL DEFAULT '',
    enable                 TINYINT(1) NOT NULL DEFAULT 0,
    expiry_time            BIGINT NOT NULL DEFAULT 0,
    traffic_reset          VARCHAR(255) NOT NULL DEFAULT 'never',
    last_traffic_reset_time BIGINT NOT NULL DEFAULT 0,
    listen                 VARCHAR(255) NOT NULL DEFAULT '',
    port                   INT NOT NULL DEFAULT 0,
    protocol               VARCHAR(50) NOT NULL DEFAULT '',
    settings               LONGTEXT,
    stream_settings        LONGTEXT,
    tag                    VARCHAR(255) NOT NULL DEFAULT '',
    sniffing               LONGTEXT,
    UNIQUE KEY idx_tag (tag),
    INDEX idx_enable_traffic_reset (enable, traffic_reset)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**MariaDB 关键差异：**
- `bool` → `TINYINT(1)` (GORM 自动处理)
- JSON 字段存储为 `LONGTEXT`（MariaDB 10.2+ 也可用原生 `JSON` 类型做校验）
- `unique` 索引在 MariaDB 中正常工作

### 4.3 `client_traffics` 表

**Go 结构体：**
```go
type ClientTraffic struct {
    Id         int    `gorm:"primaryKey;autoIncrement"`
    InboundId  int
    Enable     bool
    Email      string `gorm:"unique"`
    Up         int64
    Down       int64
    AllTime    int64
    ExpiryTime int64
    Total      int64
    Reset      int    `gorm:"default:0"`
    LastOnline int64  `gorm:"default:0"`
}
```

**MariaDB DDL：**
```sql
CREATE TABLE client_traffics (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    inbound_id  INT NOT NULL DEFAULT 0,
    enable      TINYINT(1) NOT NULL DEFAULT 0,
    email       VARCHAR(255) NOT NULL DEFAULT '',
    up          BIGINT NOT NULL DEFAULT 0,
    down        BIGINT NOT NULL DEFAULT 0,
    all_time    BIGINT NOT NULL DEFAULT 0,
    expiry_time BIGINT NOT NULL DEFAULT 0,
    total       BIGINT NOT NULL DEFAULT 0,
    reset       INT NOT NULL DEFAULT 0,
    last_online BIGINT NOT NULL DEFAULT 0,
    UNIQUE KEY idx_email (email),
    INDEX idx_inbound_id (inbound_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**注意：** `UUID` 和 `SubId` 字段标记了 `gorm:"-"`，不会持久化到数据库。

### 4.4 `outbound_traffics` 表

**MariaDB DDL：**
```sql
CREATE TABLE outbound_traffics (
    id    INT AUTO_INCREMENT PRIMARY KEY,
    tag   VARCHAR(255) NOT NULL DEFAULT '',
    up    BIGINT NOT NULL DEFAULT 0,
    down  BIGINT NOT NULL DEFAULT 0,
    total BIGINT NOT NULL DEFAULT 0,
    UNIQUE KEY idx_tag (tag)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.5 `settings` 表

**MariaDB DDL：**
```sql
CREATE TABLE settings (
    id    INT AUTO_INCREMENT PRIMARY KEY,
    `key` VARCHAR(255) NOT NULL DEFAULT '',
    value LONGTEXT,
    INDEX idx_key (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**注意：** `key` 是 MariaDB 保留字，需要反引号包裹。

### 4.6 `inbound_client_ips` 表

**MariaDB DDL：**
```sql
CREATE TABLE inbound_client_ips (
    id           INT AUTO_INCREMENT PRIMARY KEY,
    client_email VARCHAR(255) NOT NULL DEFAULT '',
    ips          LONGTEXT,
    UNIQUE KEY idx_client_email (client_email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.7 `history_of_seeders` 表

**MariaDB DDL：**
```sql
CREATE TABLE history_of_seeders (
    id           INT AUTO_INCREMENT PRIMARY KEY,
    seeder_name  VARCHAR(255) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 5. SQLite 特有 SQL 的 MariaDB 替代方案

### 5.1 `JSON_EACH()` — 最大阻塞点

`JSON_EACH()` 是 SQLite 特有的表值函数，用于展开 JSON 数组。**MariaDB 没有等价函数。**

以下代码中使用了 `JSON_EACH`，必须改造：

#### 5.1.1 `web/service/inbound.go:144-156` — `getAllEmails()`

**当前 SQLite SQL：**
```sql
SELECT JSON_EXTRACT(client.value, '$.email')
FROM inbounds,
    JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
```

**MariaDB 替代方案（应用层解析）：**
```go
func (s *InboundService) getAllEmails() ([]string, error) {
    db := database.GetDB()
    var inbounds []model.Inbound
    err := db.Model(model.Inbound{}).Select("settings").Find(&inbounds).Error
    if err != nil {
        return nil, err
    }
    var emails []string
    for _, inbound := range inbounds {
        clients, _ := s.GetClients(&inbound)
        for _, c := range clients {
            if c.Email != "" {
                emails = append(emails, c.Email)
            }
        }
    }
    return emails, nil
}
```

#### 5.1.2 `web/service/inbound.go:1313-1323` — `MigrationRemoveOrphanedTraffics()`

**当前 SQLite SQL：**
```sql
DELETE FROM client_traffics
WHERE email NOT IN (
    SELECT JSON_EXTRACT(client.value, '$.email')
    FROM inbounds,
        JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
)
```

**MariaDB 替代方案（应用层）：**
```go
func (s *InboundService) MigrationRemoveOrphanedTraffics() {
    db := database.GetDB()
    var allEmails []string
    var inbounds []model.Inbound
    db.Model(model.Inbound{}).Select("settings").Find(&inbounds)
    for _, inbound := range inbounds {
        clients, _ := s.GetClients(&inbound)
        for _, c := range clients {
            if c.Email != "" {
                allEmails = append(allEmails, c.Email)
            }
        }
    }
    if len(allEmails) > 0 {
        db.Where("email NOT IN ?", allEmails).Delete(xray.ClientTraffic{})
    } else {
        db.Where("1 = 1").Delete(xray.ClientTraffic{})
    }
}
```

#### 5.1.3 `web/service/inbound.go:2057-2082` — `GetClientTrafficByID()`

**当前 SQLite SQL：**
```sql
SELECT ... FROM client_traffics WHERE email IN (
    SELECT JSON_EXTRACT(client.value, '$.email') as email
    FROM inbounds,
        JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
    WHERE JSON_EXTRACT(client.value, '$.id') IN (?)
)
```

**MariaDB 替代方案：**
```go
func (s *InboundService) GetClientTrafficByID(id string) ([]xray.ClientTraffic, error) {
    db := database.GetDB()
    // 先从 inbounds 的 settings JSON 中按应用层查出 email
    var inbounds []model.Inbound
    db.Model(model.Inbound{}).
        Where("JSON_EXTRACT(settings, '$.clients[*].id') LIKE ?", "%"+id+"%").
        Find(&inbounds)

    var emails []string
    for _, inbound := range inbounds {
        clients, _ := s.GetClients(&inbound)
        for _, c := range clients {
            if c.ID == id && c.Email != "" {
                emails = append(emails, c.Email)
            }
        }
    }
    if len(emails) == 0 {
        return nil, nil
    }
    var traffics []xray.ClientTraffic
    err := db.Model(xray.ClientTraffic{}).Where("email IN ?", emails).Find(&traffics).Error
    // ... 后续 reconcile 逻辑不变
    return traffics, err
}
```

#### 5.1.4 `sub/subService.go:115-130` — `getInboundsBySubId()`

**当前 SQLite SQL：**
```sql
SELECT DISTINCT inbounds.id
FROM inbounds,
    JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
WHERE protocol in ('vmess','vless','trojan','shadowsocks')
    AND JSON_EXTRACT(client.value, '$.subId') = ? AND enable = ?
```

**MariaDB 替代方案：**
```go
func (s *SubService) getInboundsBySubId(subId string) ([]*model.Inbound, error) {
    db := database.GetDB()
    var allInbounds []*model.Inbound
    err := db.Model(model.Inbound{}).Preload("ClientStats").
        Where("enable = ? AND protocol IN (?)", true,
            []string{"vmess", "vless", "trojan", "shadowsocks"}).
        Find(&allInbounds).Error
    if err != nil {
        return nil, err
    }
    // 应用层过滤 subId
    var result []*model.Inbound
    for _, inbound := range allInbounds {
        clients, _ := s.inboundService.GetClients(inbound)
        for _, c := range clients {
            if c.Enable && c.SubID == subId {
                result = append(result, inbound)
                break
            }
        }
    }
    return result, nil
}
```

#### 5.1.5 `sub/subService.go:141-162` — `getFallbackMaster()`

**当前 SQLite SQL：**
```sql
SELECT * FROM inbounds
WHERE JSON_TYPE(settings, '$.fallbacks') = 'array'
AND EXISTS (
    SELECT * FROM json_each(settings, '$.fallbacks')
    WHERE json_extract(value, '$.dest') = ?
)
```

**MariaDB 替代方案：**
```go
func (s *SubService) getFallbackMaster(dest string, streamSettings string) (string, int, string, error) {
    db := database.GetDB()
    var inbounds []*model.Inbound
    // 使用 MariaDB 的 JSON_TYPE 和 JSON_SEARCH
    err := db.Model(model.Inbound{}).
        Where("JSON_TYPE(JSON_EXTRACT(settings, '$.fallbacks')) = 'ARRAY'").
        Find(&inbounds).Error
    if err != nil {
        return "", 0, "", err
    }
    // 应用层查找匹配 dest 的 fallback
    for _, inbound := range inbounds {
        var settings map[string]any
        json.Unmarshal([]byte(inbound.Settings), &settings)
        if fallbacks, ok := settings["fallbacks"].([]any); ok {
            for _, fb := range fallbacks {
                if f, ok := fb.(map[string]any); ok {
                    if f["dest"] == dest {
                        // 找到匹配，返回主入站信息
                        var stream, masterStream map[string]any
                        json.Unmarshal([]byte(streamSettings), &stream)
                        json.Unmarshal([]byte(inbound.StreamSettings), &masterStream)
                        stream["security"] = masterStream["security"]
                        stream["tlsSettings"] = masterStream["tlsSettings"]
                        stream["externalProxy"] = masterStream["externalProxy"]
                        modifiedStream, _ := json.MarshalIndent(stream, "", "  ")
                        return inbound.Listen, inbound.Port, string(modifiedStream), nil
                    }
                }
            }
        }
    }
    return "", 0, "", fmt.Errorf("fallback master not found for dest: %s", dest)
}
```

### 5.2 `PRAGMA` 和 `VACUUM` 替代

#### `Checkpoint()` — `database/db.go:195-202`

```go
// SQLite:
db.Exec("PRAGMA wal_checkpoint;")

// MariaDB 替代:
func Checkpoint() error {
    sqlDB, err := db.DB()
    if err != nil {
        return err
    }
    _, err = sqlDB.Exec("FLUSH TABLES")
    return err
}
```

#### `ValidateSQLiteDB()` — `database/db.go:207-228`

```go
// SQLite:
db.Raw("PRAGMA integrity_check;").Scan(&res)

// MariaDB 替代:
func ValidateDB() error {
    var tables []string
    db.Raw("SHOW TABLES").Scan(&tables)
    for _, table := range tables {
        var result string
        db.Raw("CHECK TABLE " + table).Scan(&result)
        // 检查 result 中是否包含 "error"
    }
    return nil
}
```

#### `VACUUM` — `web/service/inbound.go:2213`

```go
// SQLite:
db.Exec(`VACUUM "main"`)

// MariaDB 替代（逐表 OPTIMIZE）:
var tables []string
db.Raw("SHOW TABLES").Scan(&tables)
for _, table := range tables {
    db.Exec("OPTIMIZE TABLE " + table)
}
```

### 5.3 兼容函数（可直接使用）

| 函数 | SQLite | MariaDB | 备注 |
|---|---|---|---|
| `IFNULL()` | 支持 | 支持 | 也可用 `COALESCE()` |
| `JSON_EXTRACT()` | 支持 | 支持 | 语法相同 |
| `GROUP_CONCAT()` | 支持 | 支持 | MariaDB 功能更丰富 |
| `COALESCE()` | 支持 | 支持 | |
| `INSTR()` | 支持 | 支持 | |
| `REPLACE()` | 支持 | 支持 | |
| `LIKE` | 支持 | 支持 | 大小写敏感性不同 |

### 5.4 布尔值差异

- SQLite：`bool` 存储为 `0/1`
- MariaDB：GORM 使用 `TINYINT(1)`，同样存储为 `0/1`
- **无代码改动**：GORM 自动处理两种驱动的布尔值序列化

### 5.5 `IFNULL` vs `COALESCE`

代码中使用了 `COALESCE`（如 `inbound.go:999`）：
```go
gorm.Expr("COALESCE(all_time, 0) + ?", traffic.Up+traffic.Down)
```
也使用了 `IFNULL`（如 `inbound.go:2224`）：
```sql
SET all_time = IFNULL(up, 0) + IFNULL(down, 0)
```
**两者在 MariaDB 和 SQLite 中都支持，无需修改。**

---

## 6. 入站管理 API（数据库层）

### `GET /panel/api/inbounds/list`

**数据库操作：** GORM 标准查询，可移植。

```go
db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Find(&inbounds)
```

**MariaDB 无改动。**

---

### `GET /panel/api/inbounds/get/:id`

**数据库操作：** GORM `First`，可移植。

```go
db.Model(model.Inbound{}).First(inbound, id)
```

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/add`

**数据库操作：** GORM `Save` + `Create`，事务，可移植。

```go
tx := db.Begin()
tx.Save(inbound)
tx.Create(&clientTraffic)
tx.Commit()
```

**MariaDB 无改动。** InnoDB 事务支持良好。

---

### `POST /panel/api/inbounds/del/:id`

**数据库操作：** GORM `Delete` + 级联删除客户端流量，可移植。

```go
db.Where("inbound_id = ?", id).Delete(xray.ClientTraffic{})
db.Delete(model.Inbound{}, id)
```

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/update/:id`

**数据库操作：** GORM `Save` + 事务，可移植。

**MariaDB 无改动。** 需注意 `updateClientTraffics` 内部的 `GetClients` 是应用层 JSON 解析，不涉及数据库特有 SQL。

---

### `POST /panel/api/inbounds/import`

**数据库操作：** 接收 JSON 数据后通过 `AddInbound` 写入。

**MariaDB 无改动。**

---

## 7. 客户端管理 API（数据库层）

### `GET /panel/api/inbounds/getClientTraffics/:email`

**数据库操作：** GORM 标准查询。

```go
db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics)
```

**MariaDB 无改动。**

---

### `GET /panel/api/inbounds/getClientTrafficsById/:id`

**数据库操作：** **使用 `JSON_EACH`，需改造。**

改造方案见 [5.1.3](#513-webserviceinboundgo2057-2082--getclienttrafficbyid)。

---

### `POST /panel/api/inbounds/addClient`

**数据库操作：** GORM `Save` 更新 `inbound.Settings` JSON 字段 + `Create` 客户端流量记录。

**MariaDB 无改动。** 客户端信息存储在 `inbounds.settings` 的 JSON 中，应用层解析。

---

### `POST /panel/api/inbounds/:id/delClient/:clientId`

**数据库操作：** GORM `Delete` 删除客户端流量记录 + `Save` 更新入站 Settings。

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/updateClient/:clientId`

**数据库操作：** GORM `Updates` 更新客户端流量 + `Save` 更新入站 Settings。

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/:id/delClientByEmail/:email`

**数据库操作：** GORM `Delete` + `Save`，可移植。

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/delDepletedClients/:id`

**数据库操作：** 使用 `GROUP_CONCAT` 和条件查询。

```go
db.Model(xray.ClientTraffic{}).
    Where(whereText+" and ((total > 0 and up + down >= total) or ...)", id, now).
    Select("inbound_id, GROUP_CONCAT(email) as email").
    Group("inbound_id").
    Find(&depletedClients)
```

**MariaDB 兼容：** `GROUP_CONCAT` 在 MariaDB 中工作正常，**无需改动**。

---

## 8. 流量管理 API（数据库层）

### `POST /panel/api/inbounds/:id/resetClientTraffic/:email`

**数据库操作：** GORM `Updates`，可移植。

```go
db.Model(xray.ClientTraffic{}).
    Where("email = ?", email).
    Updates(map[string]any{"enable": true, "up": 0, "down": 0})
```

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/resetAllTraffics`

**数据库操作：** GORM `Updates`，可移植。

```go
db.Model(model.Inbound{}).Where("user_id > ?", 0).
    Updates(map[string]any{"up": 0, "down": 0})
```

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/resetAllClientTraffics/:id`

**数据库操作：** GORM 事务 + `Updates`，可移植。

```go
db.Transaction(func(tx *gorm.DB) error {
    tx.Model(xray.ClientTraffic{}).Where(whereText, id).
        Updates(map[string]any{"enable": true, "up": 0, "down": 0})
    tx.Model(model.Inbound{}).Where(inboundWhereText, id).
        Update("last_traffic_reset_time", now)
    return nil
})
```

**MariaDB 无改动。** InnoDB 事务支持良好。

---

### `POST /panel/api/inbounds/updateClientTraffic/:email`

**数据库操作：** GORM `Updates`，可移植。

```go
db.Model(xray.ClientTraffic{}).Where("email = ?", email).
    Updates(map[string]any{"up": upload, "down": download})
```

**MariaDB 无改动。**

---

### `addInboundTraffic` — 原子递增

```go
tx.Model(&model.Inbound{}).Where("tag = ?", traffic.Tag).
    Updates(map[string]any{
        "up":       gorm.Expr("up + ?", traffic.Up),
        "down":     gorm.Expr("down + ?", traffic.Down),
        "all_time": gorm.Expr("COALESCE(all_time, 0) + ?", traffic.Up+traffic.Down),
    })
```

**MariaDB 兼容：** `gorm.Expr` 直接生成 SQL，`COALESCE` 在 MariaDB 中支持。**无需改动。**

---

### 自动禁用逻辑 — `disableInvalidClients` / `disableInvalidInbounds`

```go
// Join 查询
tx.Table("inbounds").
    Select("inbounds.tag, client_traffics.email").
    Joins("JOIN client_traffics ON inbounds.id = client_traffics.inbound_id").
    Where("((client_traffics.total > 0 AND ...) OR ...) AND client_traffics.enable = ?", now, true).
    Scan(&results)
```

**MariaDB 兼容：** 标准 SQL JOIN。**无需改动。**

---

## 9. IP 记录管理 API（数据库层）

### `POST /panel/api/inbounds/clientIps/:email`

**数据库操作：** GORM `First` 查询。

```go
db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps)
```

**MariaDB 无改动。**

---

### `POST /panel/api/inbounds/clearClientIps/:email`

**数据库操作：** GORM `Update`。

```go
db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).Update("ips", "")
```

**MariaDB 无改动。**

---

## 10. 面板配置 API（数据库层）

### `POST /panel/setting/all`

**数据库操作：** GORM `Find` 查询 settings 表，应用层通过反射映射到 `AllSetting` 结构体。

```go
db.Model(model.Setting{}).Not("key = ?", "xrayTemplateConfig").Find(&settings)
```

**MariaDB 无改动。** `key` 字段在 MariaDB 中是保留字，但 GORM 会自动加反引号。

---

### `POST /panel/setting/update`

**数据库操作：** 逐字段 `saveSetting`（`First` → `Save` 或 `Create`）。

**MariaDB 无改动。**

---

## 11. 用户管理 API（数据库层）

### `POST /panel/setting/updateUser`

**数据库操作：** GORM `First` + `Updates`。

```go
db.Model(model.User{}).Where("username = ?", username).First(user)
db.Model(model.User{}).Where("id = ?", id).
    Updates(map[string]any{"username": username, "password": hashedPassword})
```

**MariaDB 无改动。**

---

### 认证与 LDAP

`CheckUser` 中的数据库查询使用 GORM 标准 API，LDAP 认证不涉及数据库。**MariaDB 无改动。**

---

## 12. 数据库导入导出（MariaDB 方案）

### `GET /panel/api/server/getDb`

**当前 SQLite 实现：** 直接复制 `x-ui.db` 文件提供下载。

**MariaDB 替代方案：**

```go
func (s *ServerService) GetDb(c *gin.Context) error {
    db := database.GetDB()
    sqlDB, _ := db.DB()

    // 方案 A：mysqldump 逻辑（推荐）
    // 导出所有表的 SQL dump
    cmd := exec.Command("mysqldump",
        "-u", dbUser, "-p"+dbPassword,
        "-h", dbHost, "-P", dbPort,
        "--single-transaction",
        "--result-file=/tmp/xui-backup.sql",
        dbName,
    )
    if err := cmd.Run(); err != nil {
        return err
    }
    c.FileAttachment("/tmp/xui-backup.sql", "xui-backup.sql")

    // 方案 B：JSON 导出（应用层）
    // 逐表读取数据并序列化为 JSON
    dump := map[string]any{}
    var inbounds []model.Inbound
    db.Find(&inbounds)
    dump["inbounds"] = inbounds
    // ... 其他表
    c.JSON(200, dump)
}
```

### `POST /panel/api/server/importDB`

**当前 SQLite 实现：** 接收 `.db` 文件，覆盖当前数据库文件。

**MariaDB 替代方案：**

```go
func (s *ServerService) ImportDB(file *multipart.FileHeader) error {
    // 方案 A：SQL dump 导入
    cmd := exec.Command("mysql",
        "-u", dbUser, "-p"+dbPassword,
        "-h", dbHost, "-P", dbPort,
        dbName,
        "-e", "source /tmp/xui-import.sql",
    )
    return cmd.Run()

    // 方案 B：JSON 导入
    // 解析 JSON，逐表 truncate 后重新插入
}
```

---

## 13. 订阅服务的数据库查询改造

> 原始文件：`sub/subService.go`

### `getInboundsBySubId()` — 需改造

详见 [5.1.4](#514-subsubservicego115-130--getinboundsbysubid)。

### `getFallbackMaster()` — 需改造

详见 [5.1.5](#515-subsubservicego141-162--getfallbackmaster)。

### 其他订阅逻辑

链接生成、流量统计、HTML 渲染等均为应用层逻辑，不涉及 SQLite 特有 SQL。**MariaDB 无改动。**

---

## 14. 数据库维护操作（MariaDB 对应）

| SQLite 操作 | 用途 | MariaDB 等价操作 |
|---|---|---|
| `PRAGMA wal_checkpoint` | 刷写 WAL 日志 | `FLUSH TABLES` |
| `PRAGMA integrity_check` | 数据完整性检查 | `CHECK TABLE table_name` |
| `VACUUM` | 回收空间/碎片整理 | `OPTIMIZE TABLE table_name` |
| `PRAGMA journal_mode=WAL` | 日志模式 | 无需设置（InnoDB 自带崩溃恢复） |
| 文件复制备份 | 备份数据库 | `mysqldump --single-transaction` |
| 文件替换恢复 | 恢复数据库 | `mysql < backup.sql` |

### InnoDB 原生优势

MariaDB 的 InnoDB 引擎自带以下特性，无需额外操作：
- **崩溃恢复**：通过 redo/undo 日志自动恢复
- **行级锁**：并发性能远优于 SQLite 的库级锁
- **事务隔离**：支持 READ COMMITTED、REPEATABLE READ 等级别

---

## 15. 迁移实施步骤

### 步骤 1：安装 MariaDB 并创建数据库

```bash
# Ubuntu/Debian
apt install mariadb-server
mysql_secure_installation

# 创建数据库和用户
mysql -u root -p <<EOF
CREATE DATABASE xui CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'xui'@'localhost' IDENTIFIED BY 'your_secure_password';
GRANT ALL PRIVILEGES ON xui.* TO 'xui'@'localhost';
FLUSH PRIVILEGES;
EOF
```

### 步骤 2：修改 Go 依赖

```bash
go get gorm.io/driver/mysql
# 可选：移除 SQLite 依赖（如完全不再需要）
# go mod tidy
```

### 步骤 3：改造 database/db.go

支持双驱动（通过环境变量切换），保持向后兼容。

### 步骤 4：改造原始 SQL 查询

按照 [第 5 节](#5-sqlite-特有-sql-的-mariadb-替代方案) 的方案，改造所有使用 `JSON_EACH`、`PRAGMA`、`VACUUM` 的代码。

### 步骤 5：数据迁移

```bash
# 从 SQLite 导出数据（应用层）
# 然后导入到 MariaDB

# 或使用工具
apt install sqlite3
sqlite3 /etc/x-ui/x-ui.db .dump > xui-data.sql
# 手动调整 SQL 兼容性后导入
mysql xui < xui-data.sql
```

### 步骤 6：测试验证

- [ ] 启动面板，确认 `AutoMigrate` 正确创建所有表
- [ ] 登录认证正常
- [ ] CRUD 入站/客户端正常
- [ ] 流量统计正确累加
- [ ] 订阅链接生成正常
- [ ] 客户端在线状态跟踪正常
- [ ] 数据库导出/导入正常
- [ ] 并发写入无锁冲突

---

## 附录 A：完整代码改动清单

| 文件 | 改动类型 | 说明 |
|---|---|---|
| `go.mod` | 依赖替换 | `sqlite` → `mysql` driver |
| `database/db.go` | 连接层改造 | 支持双驱动初始化 |
| `database/db.go` | 删除/改造 | `IsSQLiteDB`, `ValidateSQLiteDB`, `Checkpoint` |
| `config/config.go` | 新增环境变量 | `XUI_DB_DRIVER/HOST/PORT/NAME/USER/PASSWORD` |
| `web/service/inbound.go:144` | SQL → 应用层 | `getAllEmails()` |
| `web/service/inbound.go:1313` | SQL → 应用层 | `MigrationRemoveOrphanedTraffics()` |
| `web/service/inbound.go:2057` | SQL → 应用层 | `GetClientTrafficByID()` |
| `web/service/inbound.go:2206` | SQL → MariaDB | `MigrationRequirements()` 中的 `VACUUM` |
| `sub/subService.go:115` | SQL → 应用层 | `getInboundsBySubId()` |
| `sub/subService.go:141` | SQL → 应用层 | `getFallbackMaster()` |
| `web/controller/server.go` | 导入导出改造 | `getDb` / `importDB` |

## 附录 B：风险评估

| 风险项 | 等级 | 缓解措施 |
|---|---|---|
| `JSON_EACH` 无等价函数 | **高** | 改用应用层解析，可能影响大数据量性能 |
| JSON 字段查询性能 | 中 | MariaDB 原生 JSON 类型 + 虚拟列索引 |
| 数据迁移完整性 | 中 | 先导出 JSON 备份，再逐表验证行数 |
| 并发写入锁行为变化 | 低 | InnoDB 行级锁，实际上优于 SQLite |
| 大小写敏感性差异 | 低 | 统一使用 `utf8mb4_unicode_ci` 排序规则 |
