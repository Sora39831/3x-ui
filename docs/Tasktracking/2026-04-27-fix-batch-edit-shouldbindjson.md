# Task Record

Date: 2026-04-27
Related Module: web/controller/inbound.go
Change Type: Fix

## Background
批量编辑（batch edit）功能提示 "入站连接已成功更新 (invalid character 'i' looking for beginning of value)"，操作实际未生效。

根因：前端全局 axios 配置将 POST Content-Type 设为 `application/x-www-form-urlencoded`，请求拦截器用 `Qs.stringify` 将数据转为 URL-encoded 格式。但 `batchUpdateInboundClients` 处理器使用了 `c.ShouldBindJSON`，它只接受 JSON 格式。form-encoded 数据（如 `inboundId=123&...`）以 `i` 开头，导致 Go JSON 解析器报 `invalid character 'i' looking for beginning of value`。

项目中其他所有 handler（`addInbound`、`updateInbound`、`addClient` 等）均使用 `c.ShouldBind`（根据 Content-Type 自动选择绑定方式），仅在 batchUpdateInboundClients 中使用了 `ShouldBindJSON`，导致不一致。

## Changes
- `web/controller/inbound.go:481`: `c.ShouldBindJSON(&request)` → `c.ShouldBind(&request)`
- `web/controller/inbound.go:477-479`: 为匿名结构体字段添加 `form` 标签，确保 form 绑定能正确映射字段名（`shouldBind` 使用 `form` 标签而非 `json` 标签进行 form 字段映射）

## Impact
- `ShouldBind` 会根据 Content-Type 自动选择 form 绑定（浏览器请求）或 JSON 绑定（其他客户端），与项目其他 handler 一致
- 向后兼容：JSON 请求依然能被正确解析（`json` 标签仍保留）
- 不影响数据库、API、配置

## Verification
- `gofmt -l -w .` — 无格式问题
- `go vet ./web/controller/...` — 通过

## Risks And Follow-Up
- 低风险，与项目现有代码风格一致（model 中所有 struct 字段均同时携带 `json` 和 `form` 标签）
