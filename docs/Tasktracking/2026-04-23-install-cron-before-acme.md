Task Record: Install cron before acme.sh for all distros

Date: 2026-04-23
Related Module: install.sh — cron 安装
Change Type: Fix

Background

acme.sh 依赖 cron 来执行证书自动续期，但在部分发行版（RHEL/Fedora/CentOS/Arch/openSUSE/Alpine）上，cron 服务可能未预装。acme.sh 安装时如果找不到 cron，会静默失败或报错，导致证书续期不生效。

Changes

- `install.sh`:
  - 在 `install_base()` 中新增 cron 包安装逻辑
  - RHEL/Fedora/CentOS/Arch/openSUSE: 安装 `cronie` 包
  - Alpine: 安装 `dcron` 包
  - 安装后确保 crond 服务启用并启动（`enable --now`）
  - 将 cron 安装移到 acme.sh 安装之前，确保依赖顺序正确

Impact

- `install.sh`: `install_base()` 函数
- 不影响已有安装流程，仅在 cron 未安装时补充安装
- 不影响数据库、API、前端

Verification

- `bash -n install.sh` — syntax OK
- 在 Ubuntu/Debian 上验证（cron 通常已预装，无副作用）
- 需要在 RHEL/Alpine 等发行版上验证 cron 安装逻辑

Risks And Follow-Up

- 无风险。仅增加缺失包的安装，不影响已有逻辑
- 如果用户手动禁用了 cron，证书续期仍会失败（非本次修复范围）
