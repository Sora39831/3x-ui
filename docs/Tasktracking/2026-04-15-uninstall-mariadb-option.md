# 任务记录：uninstall-mariadb-option

- 日期：2026-04-15
- 关联模块：x-ui uninstall flow / database cleanup / test script
- 变更类型：优化

## 背景
卸载流程原先只移除面板服务与文件，不处理 MariaDB 业务库、业务账号和本机 MariaDB 包，用户在希望彻底清理时需要手动处理。

## 修改内容
- 在 `x-ui.sh` 的 `uninstall()` 中新增交互项：`是否删除数据库并卸载本机 MariaDB？`。
- 当当前数据库类型为 MariaDB 且 host 为本机地址（`127.0.0.1`/`localhost`/`::1`）时：
  - 删除业务库与业务账号（`localhost`、`127.0.0.1`、`::1`）。
  - 卸载本机 MariaDB 服务与相关软件包。
- 当数据库为远程 MariaDB 时，输出提示并跳过数据库删除与卸载，避免误删远程资源。
- 新增 `remove_local_mariadb_data` 与 `uninstall_local_mariadb_packages` 两个函数。
- 更新 `tests/mariadb_install_switch_test.sh`，增加新卸载逻辑关键文本断言。

## 影响范围
- 影响文件：`x-ui.sh`、`tests/mariadb_install_switch_test.sh`。
- 不影响面板安装流程、数据库切换流程、数据库结构。
- 仅在卸载流程中新增可选数据库清理能力。

## 验证情况
- 执行 `bash -n x-ui.sh`，通过。
- 执行 `bash -n install.sh`，通过。
- 执行 `bash tests/mariadb_install_switch_test.sh`，通过。

## 风险与后续
- 用户若选择删除数据库，相关业务数据将不可恢复。
- 后续可增加二次确认，显示将删除的数据库名和用户名，以进一步降低误操作风险。
