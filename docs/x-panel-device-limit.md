# x-panel (xeefei/x-panel) 设备限制功能分析

> 本文档整理了 x-panel 的设备限制(IP限制)相关逻辑代码和接口，供后续修改 3x-ui IP 限制功能参考。

## 目录

1. [架构概览](#架构概览)
2. [数据模型](#数据模型)
3. [核心任务：CheckDeviceLimitJob](#核心任务checkdevicelimitjob)
4. [封禁/解封机制](#封禁解封机制)
5. [观察期防误封逻辑](#观察期防误封逻辑)
6. [TTL 过期清理](#ttl-过期清理)
7. [遗留任务：CheckClientIpJob](#遗留任务checkclientipjob)
8. [前端 UI](#前端-ui)
9. [主程序启动与依赖注入](#主程序启动与依赖注入)
10. [关键日志路径](#关键日志路径)
11. [与 3x-ui 的差异总结](#与-3x-ui-的差异总结)

---

## 架构概览

x-panel 有两套 IP 限制机制并行运行：

| 任务 | 来源 | 执行方式 | 核心思路 |
|------|------|----------|----------|
| `CheckDeviceLimitJob` | 新增 | `main.go` 中 goroutine + 10s Ticker | 内存跟踪活跃 IP，超限通过 Xray API 替换 UUID 封禁 |
| `CheckClientIpJob` | 遗留(同 3x-ui) | cron 每 10s | 解析 access.log，超限 IP 写入 Fail2ban 日志 |

**CheckDeviceLimitJob 工作流程（每 10 秒一次）：**

```
Run()
  ├─ 1. cleanupExpiredIPs()   // 清理 3 分钟不活跃的 IP
  ├─ 2. parseAccessLog()      // 增量读取 access.log，更新活跃 IP 表
  └─ 3. checkAllClientsLimit() // 检查所有用户，超限封禁，恢复解封
```

---

## 数据模型

**源文件：** `database/model/model.go`

### Inbound 结构体（新增字段）

```go
type Inbound struct {
    // ... 原有字段 ...

    // 设备限制字段，per-inbound 级别（不是 per-client）
    DeviceLimit int `json:"deviceLimit" form:"deviceLimit" gorm:"column:device_limit;default:0"`
}
```

- `device_limit > 0` 表示该入站规则启用了设备限制
- 这是**入站级别**的限制，不是客户端级别的

### Client 结构体

```go
type Client struct {
    ID         string `json:"id"`
    Security   string `json:"security"`
    Password   string `json:"password"`
    SpeedLimit int    `json:"speedLimit" form:"speedLimit"` // KB/s，0=不限速
    Flow       string `json:"flow"`
    Email      string `json:"email"`
    LimitIP    int    `json:"limitIp"`     // 遗留字段，Fail2ban 用
    TotalGB    int64  `json:"totalGB"`
    ExpiryTime int64  `json:"expiryTime"`
    Enable     bool   `json:"enable"`
    TgID       int64  `json:"tgId"`
    SubID      string `json:"subId"`
    Comment    string `json:"comment"`
    Reset      int    `json:"reset"`
}
```

### InboundClientIps（与 3x-ui 相同）

```go
type InboundClientIps struct {
    Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
    ClientEmail string `json:"clientEmail" gorm:"unique"`
    Ips         string `json:"ips"` // JSON 数组字符串
}
```

### 内存状态结构

```go
// 活跃 IP 跟踪（TTL 机制）
// map[用户email] -> map[IP地址] -> 最后活跃时间
var ActiveClientIPs = make(map[string]map[string]time.Time)
var activeClientsLock sync.RWMutex

// 用户封禁状态跟踪
// map[用户email] -> 是否被封禁(true/false)
var ClientStatus = make(map[string]bool)
var clientStatusLock sync.RWMutex
```

---

## 核心任务：CheckDeviceLimitJob

**源文件：** `web/job/check_client_ip_job.go`

### 结构体

```go
type CheckDeviceLimitJob struct {
    inboundService    service.InboundService
    xrayService       *service.XrayService
    xrayApi           xray.XrayAPI
    lastPosition      int64                    // access.log 增量读取位置
    telegramService   service.TelegramService  // TG 通知（可为 nil）
    violationStartTime map[string]time.Time    // 观察期开始时间
    triggerLock        sync.Mutex              // 保护 violationStartTime
}
```

### 构造函数

```go
func NewCheckDeviceLimitJob(xrayService *service.XrayService, telegramService service.TelegramService) *CheckDeviceLimitJob
```

### Run() 主循环

```go
func (j *CheckDeviceLimitJob) Run() {
    if !j.xrayService.IsXrayRunning() {
        return
    }
    j.cleanupExpiredIPs()
    j.parseAccessLog()
    j.checkAllClientsLimit()
}
```

### cleanupExpiredIPs() — 清理过期 IP

- TTL 窗口：**3 分钟**
- 超过 3 分钟未出现的 IP 被删除
- 用户所有 IP 都过期后，用户条目也从 map 中移除

```go
const activeTTL = 3 * time.Minute
for email, ips := range ActiveClientIPs {
    for ip, lastSeen := range ips {
        if now.Sub(lastSeen) > activeTTL {
            delete(ActiveClientIPs[email], ip)
        }
    }
    if len(ActiveClientIPs[email]) == 0 {
        delete(ActiveClientIPs, email)
    }
}
```

### parseAccessLog() — 增量解析日志

- 使用 `file.Seek(j.lastPosition, 0)` 实现增量读取
- 正则提取 email 和 IP：
  ```go
  emailRegex := regexp.MustCompile(`email: ([^ ]+)`)
  ipRegex := regexp.MustCompile(`from (?:tcp:|udp:)?\[?([0-9a-fA-F\.:]+)\]?:\d+ accepted`)
  ```
- 忽略 `127.0.0.1` 和 `::1`
- 读取完毕后记录当前位置；如果文件被截断（当前位置 < 上次位置），重置为 0

### checkAllClientsLimit() — 核心检查逻辑

```go
// 查询启用了设备限制且正在运行的入站
db.Where("device_limit > 0 AND enable = ?", true).Find(&inbounds)

// 获取 Xray API 端口
apiPort := j.xrayService.GetApiPort()
j.xrayApi.Init(apiPort)
defer j.xrayApi.Close()
```

**第一步：处理在线用户**
- 遍历 `ActiveClientIPs`
- 通过 `inboundService.GetClientTrafficByEmail(email)` 关联到入站
- 检查活跃 IP 数 vs `device_limit`
- 超限 → 进入观察期逻辑 → 封禁
- 恢复 → 解封

**第二步：处理已封禁但已下线的用户**
- 遍历 `ClientStatus`
- 已封禁但不在 `ActiveClientIPs` 中的用户 → 解封

---

## 封禁/解封机制

### banUser() — 封禁（UUID 替换）

```go
func (j *CheckDeviceLimitJob) banUser(email string, activeIPCount int, info *struct{...}) {
    // 1. 从数据库获取原始客户端信息
    _, client, err := j.inboundService.GetClientByEmail(email)

    // 2. 发送 Telegram 通知（异步 goroutine）
    go func() {
        j.telegramService.SendMessage(tgMessage)
    }()

    // 3. 从 Xray-Core 中删除该用户
    j.xrayApi.RemoveUser(info.Tag, email)

    // 4. 等待 5 秒，解决竞态条件
    time.Sleep(5000 * time.Millisecond)

    // 5. 创建临时客户端，替换 UUID/Password
    tempClient := *client
    if tempClient.ID != "" { tempClient.ID = RandomUUID() }
    if tempClient.Password != "" { tempClient.Password = RandomUUID() }

    // 6. 用错误的 UUID/Password 添加回去 → 客户端无法通过验证
    j.xrayApi.AddUser(string(info.Protocol), info.Tag, clientMap)

    // 7. 标记为已封禁
    ClientStatus[email] = true
}
```

### unbanUser() — 解封（恢复原始 UUID）

```go
func (j *CheckDeviceLimitJob) unbanUser(email string, activeIPCount int, info *struct{...}) {
    // 1. 从数据库获取原始客户端信息
    _, client, err := j.inboundService.GetClientByEmail(email)

    // 2. 删除封禁用的临时用户
    j.xrayApi.RemoveUser(info.Tag, email)

    // 3. 等待 5 秒
    time.Sleep(5000 * time.Millisecond)

    // 4. 用原始正确的 UUID/Password 添加回去
    j.xrayApi.AddUser(string(info.Protocol), info.Tag, clientMap)

    // 5. 移除封禁标记
    delete(ClientStatus, email)
}
```

### RandomUUID() — 生成随机 UUID

```go
func RandomUUID() string {
    uuid := make([]byte, 16)
    rand.Read(uuid)
    uuid[6] = (uuid[6] & 0x0f) | 0x40
    uuid[8] = (uuid[8] & 0x3f) | 0x80
    return hex.EncodeToString(uuid[0:4]) + "-" + hex.EncodeToString(uuid[4:6]) + "-" +
           hex.EncodeToString(uuid[6:8]) + "-" + hex.EncodeToString(uuid[8:10]) + "-" +
           hex.EncodeToString(uuid[10:16])
}
```

### 关键依赖接口

| 接口 | 说明 |
|------|------|
| `j.inboundService.GetClientByEmail(email)` | 从数据库获取客户端原始配置（含 UUID/Password） |
| `j.xrayApi.RemoveUser(tag, email)` | 通过 gRPC 从 Xray-Core 移除用户 |
| `j.xrayApi.AddUser(protocol, tag, clientMap)` | 通过 gRPC 向 Xray-Core 添加用户 |
| `j.xrayService.GetApiPort()` | 获取 Xray API 端口号 |
| `j.xrayService.IsXrayRunning()` | 检查 Xray 是否运行中 |
| `j.telegramService.SendMessage(msg)` | 发送 Telegram 通知 |

---

## 观察期防误封逻辑

**目的：** 解决用户切换网络时产生临时双 IP 导致误封的问题。

```
场景 A：用户设备数超限，且当前未被封禁
├─ 首次发现超限 → 记录时间，进入 3 分钟观察期，不封禁
├─ 观察期内仍超限但未满 3 分钟 → 继续观察
└─ 观察期满 3 分钟仍超限 → 确认封禁

场景 B：用户恢复正常（IP 数 ≤ 限制）
├─ 之前在观察名单中 → 移除观察记录，皆大欢喜
└─ 之前被封禁 → 执行解封
```

核心代码：

```go
if activeIPCount > info.Limit && !isBanned {
    startTime, exists := j.violationStartTime[email]
    if !exists {
        // 首次超限，开始观察
        j.violationStartTime[email] = time.Now()
        continue
    }
    if time.Since(startTime) < 3*time.Minute {
        // 还在观察期，暂不封禁
        continue
    }
    // 观察期结束，确认封禁
    delete(j.violationStartTime, email)
    j.banUser(email, activeIPCount, &info)
}
```

---

## TTL 过期清理

- **活跃窗口：** 3 分钟
- 每 10 秒执行一次清理
- IP 在 `ActiveClientIPs` 中的 `lastSeen` 时间超过 3 分钟则删除
- 用户所有 IP 被清理后，用户条目也移除
- 被清理的已封禁用户在 `checkAllClientsLimit` 第二步中会被解封

---

## 遗留任务：CheckClientIpJob

**源文件：** `web/job/check_client_ip_job.go` (lines 416-714)

与 3x-ui 的实现完全一致：

1. 解析 access.log，提取每个 email 的所有 IP
2. 与数据库中 `InboundClientIps` 记录对比
3. 超过 `LimitIP` 的 IP 写入 `3xipl.log`
4. 依赖 Fail2ban 读取日志进行 iptables 封禁
5. 每小时清理 access.log

此任务由 cron 调度，与 `CheckDeviceLimitJob` 独立运行。

---

## 前端 UI

**源文件：** `web/html/form/client.html`

### 入站级别

`DeviceLimit` 字段不在 client 表单中显示，而是在入站配置中设置（具体 UI 未在提供的文件中）。

### 客户端级别

| 字段 | 行号 | 说明 |
|------|------|------|
| `client.limitIp` | 108 | IP 数量限制（遗留，Fail2ban 用） |
| `client.speedLimit` | 85-92 | 独立限速，单位 KB/s，0=不限速 |
| `client._totalGB` | 150 | 总流量限制 |
| `client._expiryTime` | 179-182 | 过期时间 |
| `client.reset` | 193 | 续期天数 |

---

## 主程序启动与依赖注入

**源文件：** `main.go`

### 服务初始化（runWebServer 函数）

```go
// 1. 创建服务实例
xrayService := service.XrayService{}
settingService := service.SettingService{}
serverService := service.ServerService{}
inboundService := service.InboundService{}

// 2. 创建 Xray API 实例并注入
xrayApi := xray.XrayAPI{}
xrayService.SetXrayAPI(xrayApi)
inboundService.SetXrayAPI(xrayApi)

// 3. 初始化 Telegram Bot（如已启用）
if tgEnable {
    tgBot := service.NewTgBot(...)
    tgBotService = tgBot
}

// 4. 注入 Telegram 服务
serverService.SetTelegramService(tgBotService)
inboundService.SetTelegramService(tgBotService)
```

### 设备限制定时任务启动

```go
go func() {
    time.Sleep(10 * time.Second)  // 等待面板和 Xray 稳定

    ticker := time.NewTicker(10 * time.Second)  // 每 10 秒执行
    defer ticker.Stop()

    // 创建 Telegram 服务（可为 nil）
    var tgBotService service.TelegramService
    if tgEnable {
        tgBotService = new(service.Tgbot)
    }

    // 创建任务实例
    checkJob := job.NewCheckDeviceLimitJob(&xrayService, tgBotService)

    // 无限循环
    for {
        <-ticker.C
        checkJob.Run()
    }
}()
```

---

## 关键日志路径

| 路径 | 说明 |
|------|------|
| `config.GetLogFolder() + "/3xipl.log"` | IP 限制日志（遗留 Fail2ban 用） |
| `config.GetLogFolder() + "/3xipl-banned.log"` | 封禁日志 |
| `config.GetLogFolder() + "/3xipl-ap.log"` | 持久化访问日志 |
| Xray access log（配置中指定） | 用户连接日志，设备限制解析源 |
| `config.GetBinFolderPath() + "/core_crash_*.log"` | 崩溃报告 |

---

## 与 3x-ui 的差异总结

| 特性 | 3x-ui | x-panel |
|------|-------|---------|
| IP 限制级别 | per-client (`LimitIP`) | per-inbound (`DeviceLimit`) + per-client 遗留 |
| 封禁方式 | Fail2ban + iptables | Xray API UUID 替换 |
| 活跃 IP 跟踪 | 无（全量日志分析） | 内存 map + 3 分钟 TTL |
| 误封防护 | 无 | 3 分钟观察期 |
| 解封机制 | Fail2ban unban | 恢复原始 UUID |
| 通知 | 无 | Telegram Bot 集成 |
| 限速 | 无 | per-client `SpeedLimit` (KB/s) |
| 调度方式 | cron 10s | goroutine + Ticker 10s |
| 依赖 | Fail2ban, iptables | Xray gRPC API |
