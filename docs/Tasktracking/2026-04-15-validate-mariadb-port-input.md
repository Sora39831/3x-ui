# 任务记录：validate-mariadb-port-input

- 日期：2026-04-15
- 关联模块：install script / db switch menu / test script
- 变更类型：修复

## 背景
远程 MariaDB 连接配置流程中，端口输入未做格式和范围校验，用户输入非法值时只能在后续连接阶段失败，定位不直观。

## 修改内容
- 在 `install.sh` 的远程 MariaDB 分支中新增端口校验循环。
- 在 `x-ui.sh` 的数据库切换到 MariaDB（远程）分支中新增端口校验循环。
- 在 `tests/mariadb_install_switch_test.sh` 增加断言，校验两处脚本都包含端口非法提示文本。

## 影响范围
- 影响文件：`install.sh`、`x-ui.sh`、`tests/mariadb_install_switch_test.sh`。
- 不影响数据库结构、接口协议、构建流程。
- 仅影响交互式输入阶段的参数合法性检查。

## 验证情况
- 执行 `bash -n install.sh`，通过。
- 执行 `bash -n x-ui.sh`，通过。
- 执行 `bash tests/mariadb_install_switch_test.sh`，通过。

## 风险与后续
- 当前风险较低，变更仅限输入校验逻辑。
- 后续可考虑将端口校验抽为统一函数，减少重复逻辑。
