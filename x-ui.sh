#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

#添加一些基础函数
function LOGD() {
    echo -e "${yellow}[调试] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[错误] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[信息] $* ${plain}"
}

# 端口辅助函数：检测端口监听及所属进程（尽力而为）
is_port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -ltn 2>/dev/null | awk -v p=":${port}$" '$4 ~ p {found=1} END {exit(found ? 0 : 1)}'
        return
    fi
    if command -v netstat >/dev/null 2>&1; then
        netstat -lnt 2>/dev/null | awk -v p=":${port} " '$4 ~ p {found=1} END {exit(found ? 0 : 1)}'
        return
    fi
    if command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN >/dev/null 2>&1 && return 0
    fi
    return 1
}

# 域名/IP 验证简单辅助函数
is_ipv4() {
    [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] && return 0 || return 1
}
is_ipv6() {
    [[ "$1" =~ : ]] && return 0 || return 1
}
is_ip() {
    is_ipv4 "$1" || is_ipv6 "$1"
}
is_domain() {
    [[ "$1" =~ ^([A-Za-z0-9](-*[A-Za-z0-9])*\.)+(xn--[a-z0-9]{2,}|[A-Za-z]{2,})$ ]] && return 0 || return 1
}

# 检查 root 权限
[[ $EUID -ne 0 ]] && LOGE "错误：必须使用 root 权限运行此脚本！\n" && exit 1

# 检查操作系统并设置发行版变量
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "无法识别操作系统，请联系作者！" >&2
    exit 1
fi
echo "操作系统版本：$release"

os_version=""
os_version=$(grep "^VERSION_ID" /etc/os-release | cut -d '=' -f2 | tr -d '"' | tr -d '.')

# 声明变量
xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"
log_folder="${XUI_LOG_FOLDER:=/var/log/x-ui}"
mkdir -p "${log_folder}"
iplimit_log_path="${log_folder}/3xipl.log"
iplimit_banned_log_path="${log_folder}/3xipl-banned.log"

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -rp "$1 [默认 $2]：" temp
        if [[ "${temp}" == "" ]]; then
            temp=$2
        fi
    else
        read -rp "$1 [y/n]：" temp
    fi
    if [[ "${temp}" == "y" || "${temp}" == "Y" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "重启面板，注意：重启面板也会重启 xray" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}按回车键返回主菜单：${plain}" && read -r temp
    show_menu
}

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/Sora39831/3x-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "此功能将更新所有 x-ui 组件到最新版本，数据不会丢失。是否继续？" "y"
    if [[ $? != 0 ]]; then
        LOGE "已取消"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/Sora39831/3x-ui/main/update.sh)
    if [[ $? == 0 ]]; then
        LOGI "更新完成，面板已自动重启"
        before_show_menu
    fi
}

update_menu() {
    echo -e "${yellow}正在更新菜单${plain}"
    confirm "此功能将更新菜单到最新版本。" "y"
    if [[ $? != 0 ]]; then
        LOGE "已取消"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi

    curl -fLRo /usr/bin/x-ui https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.sh
    chmod +x ${xui_folder}/x-ui.sh
    chmod +x /usr/bin/x-ui

    if [[ $? == 0 ]]; then
        echo -e "${green}更新成功。面板已自动重启。${plain}"
        exit 0
    else
        echo -e "${red}更新菜单失败。${plain}"
        return 1
    fi
}

legacy_version() {
    echo -n "输入面板版本（例如 2.4.0）："
    read -r tag_version

    if [ -z "$tag_version" ]; then
        echo "面板版本不能为空，退出。"
        exit 1
    fi
    # 使用输入的面板版本号下载
    install_command="bash <(curl -Ls "https://raw.githubusercontent.com/Sora39831/3x-ui/v$tag_version/install.sh") v$tag_version"

    echo "正在下载并安装面板版本 $tag_version..."
    eval $install_command
}

# 处理脚本文件删除的函数
delete_script() {
    rm "$0" # 删除脚本自身
    exit 1
}

uninstall() {
    confirm "确定要卸载面板吗？xray 也会被卸载！" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi

    # 询问是否吊销证书
    local domains=$(find /root/cert/ -mindepth 1 -maxdepth 1 -type d -exec basename {} \; 2>/dev/null)
    if [[ -n "$domains" ]]; then
        echo ""
        echo "检测到以下证书："
        echo "$domains"
        confirm "是否要吊销所有证书？" "n"
        if [[ $? == 0 ]]; then
            for domain in $domains; do
                ~/.acme.sh/acme.sh --revoke -d "${domain}" 2>/dev/null
                LOGI "域名 $domain 的证书已吊销"
            done
            rm -rf /root/cert/
        fi
    fi

    local current_db_type=""
    local db_host=""
    local db_port=""
    local db_name=""
    local db_user=""
    local delete_db_confirmed=1
    current_db_type=$(read_json_dbtype)
    if [[ "$current_db_type" == "mariadb" ]]; then
        local json_path="/etc/x-ui/x-ui.json"
        if command -v jq >/dev/null 2>&1; then
            db_host=$(jq -r '.databaseConnection.dbHost // .other.dbHost // "127.0.0.1"' "$json_path" 2>/dev/null)
            db_port=$(jq -r '.databaseConnection.dbPort // .other.dbPort // "3306"' "$json_path" 2>/dev/null)
            db_name=$(jq -r '.databaseConnection.dbName // .other.dbName // "3xui"' "$json_path" 2>/dev/null)
            db_user=$(jq -r '.databaseConnection.dbUser // .other.dbUser // ""' "$json_path" 2>/dev/null)
        else
            db_host=$(grep -o '"dbHost"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/')
            db_port=$(grep -o '"dbPort"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/')
            db_name=$(grep -o '"dbName"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/')
            db_user=$(grep -o '"dbUser"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/')
        fi
        db_host="${db_host:-127.0.0.1}"
        db_port="${db_port:-3306}"
        db_name="${db_name:-3xui}"

        echo -e "${yellow}检测到当前数据库类型为 MariaDB (${db_host}:${db_port}/${db_name})${plain}"
        confirm "是否删除数据库并卸载本机 MariaDB？" "n"
        delete_db_confirmed=$?

        if [[ $delete_db_confirmed == 0 ]]; then
            if [[ "$db_host" == "127.0.0.1" || "$db_host" == "localhost" || "$db_host" == "::1" ]]; then
                remove_local_mariadb_data "$db_port" "$db_name" "$db_user"
                uninstall_local_mariadb_packages
            else
                echo -e "${yellow}当前 MariaDB 为远程地址 (${db_host})，跳过数据库删除与本机 MariaDB 卸载${plain}"
            fi
        fi
    fi

    if [[ $release == "alpine" ]]; then
        rc-service x-ui stop
        rc-update del x-ui
        rm /etc/init.d/x-ui -f
    else
        systemctl stop x-ui
        systemctl disable x-ui
        rm ${xui_service}/x-ui.service -f
        systemctl daemon-reload
        systemctl reset-failed
    fi

    rm /etc/x-ui/ -rf
    rm ${xui_folder}/ -rf

    echo ""
    echo -e "卸载成功。\n"
    echo "如需再次安装面板，可使用以下命令："
    echo -e "${green}bash <(curl -Ls https://raw.githubusercontent.com/Sora39831/3x-ui/master/install.sh)${plain}"
    echo ""
    # 捕获 SIGTERM 信号
    trap delete_script SIGTERM
    delete_script
}

remove_local_mariadb_data() {
    local db_port="$1"
    local db_name="$2"
    local db_user="$3"
    local sql=""
    local account_host=""

    if ! has_local_mariadb_service && ! has_mariadb_cli; then
        echo -e "${yellow}未检测到本机 MariaDB 服务或客户端，跳过数据库删除${plain}"
        return 0
    fi

    if ! ensure_mariadb_client_ready; then
        echo -e "${yellow}MariaDB 客户端未就绪，跳过数据库删除${plain}"
        return 0
    fi
    if ! ensure_local_mariadb_admin_access "${db_port}"; then
        echo -e "${yellow}无法获取本机 MariaDB 管理权限，跳过数据库删除${plain}"
        return 0
    fi

    if [[ "$db_name" =~ ^[A-Za-z0-9_.-]+$ ]]; then
        sql="${sql} DROP DATABASE IF EXISTS \`${db_name}\`;"
    else
        echo -e "${yellow}数据库名不符合安全规则，跳过删除业务库: ${db_name}${plain}"
    fi

    if [[ -n "$db_user" ]]; then
        if [[ "$db_user" =~ ^[A-Za-z0-9_.-]+$ ]]; then
            for account_host in "localhost" "127.0.0.1" "::1"; do
                sql="${sql} DROP USER IF EXISTS '${db_user}'@'${account_host}';"
            done
        else
            echo -e "${yellow}业务用户名不符合安全规则，跳过删除业务账号: ${db_user}${plain}"
        fi
    fi

    if [[ -z "$sql" ]]; then
        echo -e "${yellow}无可执行的数据库删除语句，跳过数据库删除${plain}"
        return 0
    fi

    if run_local_mariadb_admin_sql "$sql"; then
        echo -e "${green}本机 MariaDB 业务库/业务账号删除完成${plain}"
    else
        echo -e "${yellow}本机 MariaDB 业务库/业务账号删除失败，继续执行卸载流程${plain}"
    fi
}

uninstall_local_mariadb_packages() {
    echo -e "${green}正在卸载本机 MariaDB...${plain}"

    if command -v systemctl >/dev/null 2>&1; then
        systemctl stop mariadb 2>/dev/null || true
        systemctl disable mariadb 2>/dev/null || true
        systemctl stop mysql 2>/dev/null || true
        systemctl disable mysql 2>/dev/null || true
    elif [[ $release == "alpine" ]]; then
        rc-service mariadb stop 2>/dev/null || true
        rc-update del mariadb 2>/dev/null || true
    fi

    case "${release}" in
    ubuntu | debian | linuxmint | armbian)
        apt-get remove -y mariadb-server mariadb-client mariadb-common >/dev/null 2>&1 || true
        apt-get autoremove -y >/dev/null 2>&1 || true
        ;;
    centos | rhel | almalinux | rocky | ol | alinux | amzn | fedora)
        if command -v dnf >/dev/null 2>&1; then
            dnf remove -y mariadb-server mariadb mariadb-client >/dev/null 2>&1 || true
        else
            yum remove -y mariadb-server mariadb mariadb-client >/dev/null 2>&1 || true
        fi
        ;;
    arch | manjaro | parch)
        pacman -Rns --noconfirm mariadb mariadb-clients >/dev/null 2>&1 || pacman -Rns --noconfirm mariadb >/dev/null 2>&1 || true
        ;;
    opensuse* | sles | opensuse-tumbleweed | opensuse-leap)
        zypper rm -y mariadb mariadb-client mariadb-server >/dev/null 2>&1 || true
        ;;
    alpine)
        apk del mariadb mariadb-client >/dev/null 2>&1 || true
        ;;
    *)
        echo -e "${yellow}当前发行版未内置 MariaDB 卸载命令，请手动检查数据库包${plain}"
        ;;
    esac
}

reset_user() {
    confirm "确定要重置面板的用户名和密码吗？" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi

    read -rp "请设置登录用户名 [默认随机生成]：" config_account
    [[ -z $config_account ]] && config_account=$(gen_random_string 10)
    read -rp "请设置登录密码 [默认随机生成]：" config_password
    [[ -z $config_password ]] && config_password=$(gen_random_string 18)

    read -rp "是否要禁用当前配置的双因素认证？(y/n)：" twoFactorConfirm
    if [[ $twoFactorConfirm != "y" && $twoFactorConfirm != "Y" ]]; then
        ${xui_folder}/x-ui setting -username "${config_account}" -password "${config_password}" -resetTwoFactor false >/dev/null 2>&1
    else
        ${xui_folder}/x-ui setting -username "${config_account}" -password "${config_password}" -resetTwoFactor true >/dev/null 2>&1
        echo -e "双因素认证已禁用。"
    fi

    echo -e "面板登录用户名已重置为：${green} ${config_account} ${plain}"
    echo -e "面板登录密码已重置为：${green} ${config_password} ${plain}"
    echo -e "${green} 请使用新的登录用户名和密码访问 X-UI 面板。请牢记！${plain}"
    confirm_restart
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $(( length * 2 )) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

reset_webbasepath() {
    echo -e "${yellow}正在重置 Web 路径${plain}"

    read -rp "确定要重置 Web 路径吗？(y/n)：" confirm
    if [[ $confirm != "y" && $confirm != "Y" ]]; then
        echo -e "${yellow}操作已取消。${plain}"
        return
    fi

    config_webBasePath=$(gen_random_string 18)

    # 应用新的 Web 路径设置
    ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}" >/dev/null 2>&1

    echo -e "Web 路径已重置为：${green}${config_webBasePath}${plain}"
    echo -e "${green}请使用新的 Web 路径访问面板。${plain}"
    restart
}

reset_config() {
    confirm "确定要重置所有面板设置吗？这将清除面板的端口、路径、证书等配置，但不会删除账户数据和流量数据。" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi

    # 重置面板证书配置
    ${xui_folder}/x-ui cert -reset 2>/dev/null
    # 重置面板设置（端口、路径等）
    ${xui_folder}/x-ui setting -reset

    echo -e "所有面板设置已重置为默认值。"
    echo -e "${yellow}面板将使用默认端口 2053 和随机用户名/密码重新启动。${plain}"
    restart
}

check_config() {
    local info=$(${xui_folder}/x-ui setting -show true)
    if [[ $? != 0 ]]; then
        LOGE "获取当前设置出错，请查看日志"
        show_menu
        return
    fi
    LOGI "${info}"

    local existing_webBasePath=$(echo "$info" | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(echo "$info" | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
    local server_ip=$(curl -s --max-time 3 https://api.ipify.org)
    if [ -z "$server_ip" ]; then
        server_ip=$(curl -s --max-time 3 https://4.ident.me)
    fi

    if [[ -n "$existing_cert" ]]; then
        local domain=$(basename "$(dirname "$existing_cert")")

        if [[ "$domain" =~ ^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
            echo -e "${green}访问地址：https://${domain}:${existing_port}${existing_webBasePath}${plain}"
        else
            echo -e "${green}访问地址：https://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
        fi
    else
        echo -e "${red}⚠ 警告：未配置 SSL 证书！${plain}"
        echo -e "${yellow}您可以为 IP 地址获取 Let's Encrypt 证书（有效期约 6 天，自动续期）。${plain}"
        read -rp "现在为 IP 生成 SSL 证书？[y/N]：" gen_ssl
        if [[ "$gen_ssl" == "y" || "$gen_ssl" == "Y" ]]; then
            stop >/dev/null 2>&1
            ssl_cert_issue_for_ip
            if [[ $? -eq 0 ]]; then
                echo -e "${green}访问地址：https://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
                # ssl_cert_issue_for_ip 已经重启面板，但确保其正在运行
                start >/dev/null 2>&1
            else
                LOGE "IP 证书配置失败。"
                echo -e "${yellow}您可以通过选项 19（SSL 证书管理）重试。${plain}"
                start >/dev/null 2>&1
            fi
        else
            echo -e "${yellow}访问地址：http://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
            echo -e "${yellow}出于安全考虑，请使用选项 19（SSL 证书管理）配置 SSL 证书${plain}"
        fi
    fi
}

set_port() {
    echo -n "输入端口号[1-65535]："
    read -r port
    if [[ -z "${port}" ]]; then
        LOGD "已取消"
        before_show_menu
    else
        ${xui_folder}/x-ui setting -port ${port}
        echo -e "端口已设置，请立即重启面板，并使用新端口 ${green}${port}${plain} 访问 Web 面板"
        confirm_restart
    fi
}

start() {
    check_status
    if [[ $? == 0 ]]; then
        echo ""
        LOGI "面板正在运行，无需重复启动，如需重启请选择重启"
    else
        if [[ $release == "alpine" ]]; then
            rc-service x-ui start
        else
            systemctl start x-ui
        fi
        sleep 2
        check_status
        if [[ $? == 0 ]]; then
            LOGI "x-ui 启动成功"
        else
            LOGE "面板启动失败，可能是因为启动时间超过两秒，请稍后查看日志信息"
        fi
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "面板已停止，无需重复停止！"
    else
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        sleep 2
        check_status
        if [[ $? == 1 ]]; then
            LOGI "x-ui 和 xray 已停止"
        else
            LOGE "面板停止失败，可能是因为停止时间超过两秒，请稍后查看日志信息"
        fi
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

restart() {
    if [[ $release == "alpine" ]]; then
        rc-service x-ui restart
    else
        systemctl restart x-ui
    fi
    sleep 2
    check_status
    if [[ $? == 0 ]]; then
        LOGI "x-ui 和 xray 重启成功"
    else
        LOGE "面板重启失败，可能是因为启动时间超过两秒，请稍后查看日志信息"
    fi
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

restart_xray() {
    systemctl reload x-ui
    LOGI "xray-core 重启信号已发送，请查看日志信息确认 xray 是否重启成功"
    sleep 2
    show_xray_status
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

status() {
    if [[ $release == "alpine" ]]; then
        rc-service x-ui status
    else
        systemctl status x-ui -l
    fi
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    if [[ $release == "alpine" ]]; then
        rc-update add x-ui default
    else
        systemctl enable x-ui
    fi
    if [[ $? == 0 ]]; then
        LOGI "x-ui 设置开机自启成功"
    else
        LOGE "x-ui 设置开机自启失败"
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

disable() {
    if [[ $release == "alpine" ]]; then
        rc-update del x-ui
    else
        systemctl disable x-ui
    fi
    if [[ $? == 0 ]]; then
        LOGI "x-ui 已取消开机自启"
    else
        LOGE "x-ui 取消开机自启失败"
    fi

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

show_log() {
    if [[ $release == "alpine" ]]; then
        echo -e "${green}\t1.${plain} 调试日志"
        echo -e "${green}\t0.${plain} 返回主菜单"
        read -rp "请选择：" choice

        case "$choice" in
        0)
            show_menu
            ;;
        1)
            grep -F 'x-ui[' /var/log/messages
            if [[ $# == 0 ]]; then
                before_show_menu
            fi
            ;;
        *)
            echo -e "${red}无效选项，请选择有效数字。${plain}\n"
            show_log
            ;;
        esac
    else
        echo -e "${green}\t1.${plain} 调试日志"
        echo -e "${green}\t2.${plain} 清除所有日志"
        echo -e "${green}\t0.${plain} 返回主菜单"
        read -rp "请选择：" choice

        case "$choice" in
        0)
            show_menu
            ;;
        1)
            journalctl -u x-ui -e --no-pager -f -p debug
            if [[ $# == 0 ]]; then
                before_show_menu
            fi
            ;;
        2)
            sudo journalctl --rotate
            sudo journalctl --vacuum-time=1s
            echo "所有日志已清除。"
            restart
            ;;
        *)
            echo -e "${red}无效选项，请选择有效数字。${plain}\n"
            show_log
            ;;
        esac
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} 启用 BBR"
    echo -e "${green}\t2.${plain} 禁用 BBR"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -rp "请选择：" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        bbr_menu
        ;;
    2)
        disable_bbr
        bbr_menu
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        bbr_menu
        ;;
    esac
}

disable_bbr() {

    if [[ $(sysctl -n net.ipv4.tcp_congestion_control) != "bbr" ]] || [[ ! $(sysctl -n net.core.default_qdisc) =~ ^(fq|cake)$ ]]; then
        echo -e "${yellow}BBR 当前未启用。${plain}"
        before_show_menu
    fi

    if [ -f "/etc/sysctl.d/99-bbr-x-ui.conf" ]; then
        old_settings=$(head -1 /etc/sysctl.d/99-bbr-x-ui.conf | tr -d '#')
        sysctl -w net.core.default_qdisc="${old_settings%:*}"
        sysctl -w net.ipv4.tcp_congestion_control="${old_settings#*:}"
        rm /etc/sysctl.d/99-bbr-x-ui.conf
        sysctl --system
    else
        # 用 CUBIC 配置替换 BBR
        if [ -f "/etc/sysctl.conf" ]; then
            sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
            sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
            sysctl -p
        fi
    fi

    if [[ $(sysctl -n net.ipv4.tcp_congestion_control) != "bbr" ]]; then
        echo -e "${green}BBR 已成功替换为 CUBIC。${plain}"
    else
        echo -e "${red}替换 BBR 为 CUBIC 失败。请检查系统配置。${plain}"
    fi
}

enable_bbr() {
    if [[ $(sysctl -n net.ipv4.tcp_congestion_control) == "bbr" ]] && [[ $(sysctl -n net.core.default_qdisc) =~ ^(fq|cake)$ ]]; then
        echo -e "${green}BBR 已启用！${plain}"
        before_show_menu
    fi

    # 启用 BBR
    if [ -d "/etc/sysctl.d/" ]; then
        {
            echo "#$(sysctl -n net.core.default_qdisc):$(sysctl -n net.ipv4.tcp_congestion_control)"
            echo "net.core.default_qdisc = fq"
            echo "net.ipv4.tcp_congestion_control = bbr"
        } > "/etc/sysctl.d/99-bbr-x-ui.conf"
        if [ -f "/etc/sysctl.conf" ]; then
            # 备份 sysctl.conf 中的旧设置（如果有）
            sed -i 's/^net.core.default_qdisc/# &/'          /etc/sysctl.conf
            sed -i 's/^net.ipv4.tcp_congestion_control/# &/' /etc/sysctl.conf
        fi
        sysctl --system
    else
        sed -i '/net.core.default_qdisc/d' /etc/sysctl.conf
        sed -i '/net.ipv4.tcp_congestion_control/d' /etc/sysctl.conf
        echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
        echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
        sysctl -p
    fi

    # 验证 BBR 是否已启用
    if [[ $(sysctl -n net.ipv4.tcp_congestion_control) == "bbr" ]]; then
        echo -e "${green}BBR 已成功启用。${plain}"
    else
        echo -e "${red}启用 BBR 失败。请检查系统配置。${plain}"
    fi
}

update_shell() {
    curl -fLRo /usr/bin/x-ui -z /usr/bin/x-ui https://github.com/Sora39831/3x-ui/raw/main/x-ui.sh
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "下载脚本失败，请检查机器是否能连接 GitHub"
        before_show_menu
    else
        chmod +x /usr/bin/x-ui
        LOGI "升级脚本成功，请重新运行脚本"
        before_show_menu
    fi
}

# 0: 运中, 1: 未运行, 2: 未安装
check_status() {
    if [[ $release == "alpine" ]]; then
        if [[ ! -f /etc/init.d/x-ui ]]; then
            if [[ -x "${xui_folder}/x-ui" || -d /etc/x-ui || -d "${xui_folder}" ]]; then
                return 1
            fi
            return 2
        fi
        if [[ $(rc-service x-ui status | grep -F 'status: started' -c) == 1 ]]; then
            return 0
        else
            return 1
        fi
    else
        if [[ ! -f ${xui_service}/x-ui.service ]]; then
            if [[ -x "${xui_folder}/x-ui" || -d /etc/x-ui || -d "${xui_folder}" ]]; then
                return 1
            fi
            return 2
        fi
        temp=$(systemctl status x-ui | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
        if [[ "${temp}" == "running" ]]; then
            return 0
        else
            return 1
        fi
    fi
}

check_enabled() {
    if [[ $release == "alpine" ]]; then
        if [[ $(rc-update show | grep -F 'x-ui' | grep default -c) == 1 ]]; then
            return 0
        else
            return 1
        fi
    else
        temp=$(systemctl is-enabled x-ui)
        if [[ "${temp}" == "enabled" ]]; then
            return 0
        else
            return 1
        fi
    fi
}

check_uninstall() {
    check_status
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "面板已安装，请勿重复安装"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "请先安装面板"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status
    case $? in
    0)
        echo -e "面板状态：${green}运行中${plain}"
        show_enable_status
        ;;
    1)
        echo -e "面板状态：${yellow}未运行${plain}"
        show_enable_status
        ;;
    2)
        echo -e "面板状态：${red}未安装${plain}"
        ;;
    esac
    show_xray_status
}

show_enable_status() {
    check_enabled
    if [[ $? == 0 ]]; then
        echo -e "开机自启：${green}是${plain}"
    else
        echo -e "开机自启：${red}否${plain}"
    fi
}

check_xray_status() {
    count=$(ps -ef | grep "xray-linux" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_xray_status() {
    check_xray_status
    if [[ $? == 0 ]]; then
        echo -e "xray 状态：${green}运行中${plain}"
    else
        echo -e "xray 状态：${red}未运行${plain}"
    fi
}

firewall_menu() {
    echo -e "${green}\t1.${plain} ${green}安装${plain} 防火墙"
    echo -e "${green}\t2.${plain} 端口列表 [带编号]"
    echo -e "${green}\t3.${plain} ${green}开放${plain} 端口"
    echo -e "${green}\t4.${plain} ${red}删除${plain} 列表中的端口"
    echo -e "${green}\t5.${plain} ${green}启用${plain} 防火墙"
    echo -e "${green}\t6.${plain} ${red}禁用${plain} 防火墙"
    echo -e "${green}\t7.${plain} 防火墙状态"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -rp "请选择：" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        install_firewall
        firewall_menu
        ;;
    2)
        ufw status numbered
        firewall_menu
        ;;
    3)
        open_ports
        firewall_menu
        ;;
    4)
        delete_ports
        firewall_wall_menu
        ;;
    5)
        ufw enable
        firewall_menu
        ;;
    6)
        ufw disable
        firewall_menu
        ;;
    7)
        ufw status verbose
        firewall_menu
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        firewall_menu
        ;;
    esac
}

install_firewall() {
    if ! command -v ufw &>/dev/null; then
        echo "ufw 防火墙未安装，正在安装..."
        apt-get update
        apt-get install -y ufw
    else
        echo "ufw 防火墙已安装"
    fi

    # 检查防火墙是否处于非活动状态
    if ufw status | grep -q "Status: active"; then
        echo "防火墙已激活"
    else
        echo "正在激活防火墙..."
        # 开放必要端口
        ufw allow ssh
        ufw allow http
        ufw allow https
        ufw allow 2053/tcp #webPort
        ufw allow 2096/tcp #subport

        # 启用防火墙
        ufw --force enable
    fi
}

open_ports() {
    # 提示用户输入要开放的端口
    read -rp "输入要开放的端口（例如 80,443,2053 或范围 400-500）：" ports

    # 检查输入是否有效
    if ! [[ $ports =~ ^([0-9]+|[0-9]+-[0-9]+)(,([0-9]+|[0-9]+-[0-9]+))*$ ]]; then
        echo "错误：无效输入。请输入逗号分隔的端口列表或端口范围（例如 80,443,2053 或 400-500）。" >&2
        exit 1
    fi

    # 使用 ufw 开放指定端口
    IFS=',' read -ra PORT_LIST <<<"$ports"
    for port in "${PORT_LIST[@]}"; do
        if [[ $port == *-* ]]; then
            # 将范围拆分为起始和结束端口
            start_port=$(echo $port | cut -d'-' -f1)
            end_port=$(echo $port | cut -d'-' -f2)
            # 开放端口范围
            ufw allow $start_port:$end_port/tcp
            ufw allow $start_port:$end_port/udp
        else
            # 开放单个端口
            ufw allow "$port"
        fi
    done

    # 确认端口已开放
    echo "已开放指定端口："
    for port in "${PORT_LIST[@]}"; do
        if [[ $port == *-* ]]; then
            start_port=$(echo $port | cut -d'-' -f1)
            end_port=$(echo $port | cut -d'-' -f2)
            # 检查端口范围是否已成功开放
            (ufw status | grep -q "$start_port:$end_port") && echo "$start_port-$end_port"
        else
            # 检查单个端口是否已成功开放
            (ufw status | grep -q "$port") && echo "$port"
        fi
    done
}

delete_ports() {
    # 显示当前带编号的规则
    echo "当前 UFW 规则："
    ufw status numbered

    # 询问用户删除方式
    echo "您想通过以下哪种方式删除规则："
    echo "1) 规则编号"
    echo "2) 端口号"
    read -rp "请输入选择（1 或 2）：" choice

    if [[ $choice -eq 1 ]]; then
        # 按规则编号删除
        read -rp "输入要删除的规则编号（1, 2 等）：" rule_numbers

        # 验证输入
        if ! [[ $rule_numbers =~ ^([0-9]+)(,[0-9]+)*$ ]]; then
            echo "错误：无效输入。请输入逗号分隔的规则编号列表。" >&2
            exit 1
        fi

        # 将编号拆分为数组
        IFS=',' read -ra RULE_NUMBERS <<<"$rule_numbers"
        for rule_number in "${RULE_NUMBERS[@]}"; do
            # 按编号删除规则
            ufw delete "$rule_number" || echo "删除规则编号 $rule_number 失败"
        done

        echo "已删除所选规则。"

    elif [[ $choice -eq 2 ]]; then
        # 按端口删除
        read -rp "输入要删除的端口（例如 80,443,2053 或范围 400-500）：" ports

        # 验证输入
        if ! [[ $ports =~ ^([0-9]+|[0-9]+-[0-9]+)(,([0-9]+|[0-9]+-[0-9]+))*$ ]]; then
            echo "错误：无效输入。请输入逗号分隔的端口列表或端口范围（例如 80,443,2053 或 400-500）。" >&2
            exit 1
        fi

        # 将端口拆分为数组
        IFS=',' read -ra PORT_LIST <<<"$ports"
        for port in "${PORT_LIST[@]}"; do
            if [[ $port == *-* ]]; then
                # 拆分端口范围
                start_port=$(echo $port | cut -d'-' -f1)
                end_port=$(echo $port | cut -d'-' -f2)
                # 删除端口范围
                ufw delete allow $start_port:$end_port/tcp
                ufw delete allow $start_port:$end_port/udp
            else
                # 删除单个端口
                ufw delete allow "$port"
            fi
        done

        # 确认删除
        echo "已删除指定端口："
        for port in "${PORT_LIST[@]}"; do
            if [[ $port == *-* ]]; then
                start_port=$(echo $port | cut -d'-' -f1)
                end_port=$(echo $port | cut -d'-' -f2)
                # 检查端口范围是否已删除
                (ufw status | grep -q "$start_port:$end_port") || echo "$start_port-$end_port"
            else
                # 检查单个端口是否已删除
                (ufw status | grep -q "$port") || echo "$port"
            fi
        done
    else
        echo "${red}错误：${plain} 无效选择。请输入 1 或 2。" >&2
        exit 1
    fi
}

update_all_geofiles() {
    update_geofiles "main"
}

# Config file path (must match config.GetSettingPath())
SETTING_FILE="/etc/x-ui/x-ui.json"

# Helper: read a value from x-ui.json using python3 or jq
read_geofile_setting() {
    local key="$1"
    if command -v python3 &>/dev/null; then
        python3 -c "
import json, sys
try:
    with open('$SETTING_FILE') as f:
        data = json.load(f)
    print(data.get('geofileUpdate', {}).get('$key', ''))
except: pass
" 2>/dev/null
    elif command -v jq &>/dev/null; then
        jq -r ".geofileUpdate.$key // empty" "$SETTING_FILE" 2>/dev/null
    fi
}

# Helper: write geofileUpdate section to x-ui.json
write_geofile_setting() {
    local enabled="$1"
    local frequency="$2"
    local hour="$3"
    if command -v python3 &>/dev/null; then
        python3 -c "
import json
try:
    with open('$SETTING_FILE') as f:
        data = json.load(f)
except:
    data = {}
data['geofileUpdate'] = {'enabled': $enabled, 'frequency': '$frequency', 'hour': $hour}
with open('$SETTING_FILE', 'w') as f:
    json.dump(data, f, indent=2)
print('ok')
" 2>/dev/null
    else
        tmp=$(mktemp)
        jq ".geofileUpdate = {\"enabled\": $enabled, \"frequency\": \"$frequency\", \"hour\": $hour}" "$SETTING_FILE" > "$tmp" && mv "$tmp" "$SETTING_FILE"
    fi
}

geofile_cron_status() {
    if [ ! -f "$SETTING_FILE" ]; then
        echo -e "${red}x-ui.json not found at $SETTING_FILE${plain}"
        return 1
    fi
    local enabled=$(read_geofile_setting "enabled")
    local frequency=$(read_geofile_setting "frequency")
    local hour=$(read_geofile_setting "hour")
    echo -e "${green}Geofile Scheduled Update:${plain}"
    echo -e "  Enabled:   ${green}${enabled:-false}${plain}"
    echo -e "  Frequency: ${green}${frequency:-daily}${plain}"
    echo -e "  Hour:      ${green}${hour:-4}${plain}"
}

geofile_cron_enable() {
    local frequency="daily"
    local hour="4"
    while [ $# -gt 0 ]; do
        case "$1" in
            --frequency) frequency="$2"; shift 2;;
            --hour) hour="$2"; shift 2;;
            *) shift;;
        esac
    done
    case "$frequency" in
        hourly|every12h|daily|weekly) ;;
        *) echo -e "${red}Invalid frequency: $frequency (must be hourly, every12h, daily, or weekly)${plain}"; return 1;;
    esac
    if ! [ "$hour" -ge 0 ] 2>/dev/null || ! [ "$hour" -le 23 ] 2>/dev/null; then
        echo -e "${red}Invalid hour: $hour (must be 0-23)${plain}"
        return 1
    fi
    write_geofile_setting "true" "$frequency" "$hour"
    echo -e "${green}Geofile scheduled update enabled (frequency=$frequency, hour=$hour)${plain}"
    echo -e "${yellow}Restarting x-ui to apply changes...${plain}"
    systemctl restart x-ui
}

geofile_cron_disable() {
    write_geofile_setting "false" "daily" "4"
    echo -e "${green}Geofile scheduled update disabled${plain}"
    echo -e "${yellow}Restarting x-ui to apply changes...${plain}"
    systemctl restart x-ui
}

update_geofiles() {
    case "${1}" in
      "main") dat_files=(geoip geosite); dat_source="Loyalsoldier/v2ray-rules-dat";;
    esac
    for dat in "${dat_files[@]}"; do
        # 移除后缀获取远程文件名（例如 geoip_IR -> geoip）
        remote_file="${dat%%_*}"
        curl -fLRo ${xui_folder}/bin/${dat}.dat -z ${xui_folder}/bin/${dat}.dat \
            https://github.com/${dat_source}/releases/latest/download/${remote_file}.dat
    done
}

update_geo() {
    echo -e "${green}\t1.${plain} Loyalsoldier (geoip.dat, geosite.dat)"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -rp "请选择：" choice

    case "$choice" in
    0)
        show_menu
        ;;
    1)
        update_geofiles "main"
        echo -e "${green}Loyalsoldier 数据集更新成功！${plain}"
        restart
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        update_geo
        ;;
    esac

    before_show_menu
}

install_acme() {
    # 检查 acme.sh 是否已安装
    if command -v ~/.acme.sh/acme.sh &>/dev/null; then
        LOGI "acme.sh 已安装。"
        return 0
    fi

    LOGI "正在安装 acme.sh..."
    cd ~ || return 1

    curl -s https://get.acme.sh | sh
    if [ $? -ne 0 ]; then
        LOGE "安装 acme.sh 失败。"
        return 1
    else
        LOGI "安装 acme.sh 成功。"
    fi

    return 0
}

ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} 获取 SSL（域名）"
    echo -e "${green}\t2.${plain} 吊销证书"
    echo -e "${green}\t3.${plain} 强制续期"
    echo -e "${green}\t4.${plain} 查看已有域名"
    echo -e "${green}\t5.${plain} 为面板设置证书路径"
    echo -e "${green}\t6.${plain} 为 IP 地址获取 SSL（6 天证书，自动续期）"
    echo -e "${green}\t0.${plain} 返回主菜单"

    read -rp "请选择：" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        ssl_cert_issue
        ssl_cert_issue_main
        ;;
    2)
        local domains=$(find /root/cert/ -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)
        if [ -z "$domains" ]; then
            echo "未找到可吊销的证书。"
        else
            echo "已有域名："
            echo "$domains"
            read -rp "请输入要吊销证书的域名：" domain
            if echo "$domains" | grep -qw "$domain"; then
                ~/.acme.sh/acme.sh --revoke -d ${domain}
                LOGI "域名 $domain 的证书已吊销"
            else
                echo "输入的域名无效。"
            fi
        fi
        ssl_cert_issue_main
        ;;
    3)
        local domains=$(find /root/cert/ -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)
        if [ -z "$domains" ]; then
            echo "未找到可续期的证书。"
        else
            echo "已有域名："
            echo "$domains"
            read -rp "请输入要续期 SSL 证书的域名：" domain
            if echo "$domains" | grep -qw "$domain"; then
                ~/.acme.sh/acme.sh --renew -d ${domain} --force
                LOGI "域名 $domain 的证书已强制续期"
            else
                echo "输入的域名无效。"
            fi
        fi
        ssl_cert_issue_main
        ;;
    4)
        local domains=$(find /root/cert/ -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)
        if [ -z "$domains" ]; then
            echo "未找到证书。"
        else
            echo "已有域名及其路径："
            for domain in $domains; do
                local cert_path="/root/cert/${domain}/fullchain.pem"
                local key_path="/root/cert/${domain}/privkey.pem"
                if [[ -f "${cert_path}" && -f "${key_path}" ]]; then
                    echo -e "域名：${domain}"
                    echo -e "\t证书路径：${cert_path}"
                    echo -e "\t私钥路径：${key_path}"
                else
                    echo -e "域名：${domain} - 证书或密钥缺失。"
                fi
            done
        fi
        ssl_cert_issue_main
        ;;
    5)
        local domains=$(find /root/cert/ -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)
        if [ -z "$domains" ]; then
            echo "未找到证书。"
        else
            echo "可选域名："
            echo "$domains"
            read -rp "请选择一个域名设置面板路径：" domain

            if echo "$domains" | grep -qw "$domain"; then
                local webCertFile="/root/cert/${domain}/fullchain.pem"
                local webKeyFile="/root/cert/${domain}/privkey.pem"

                if [[ -f "${webCertFile}" && -f "${webKeyFile}" ]]; then
                    ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                    echo "域名 $domain 的面板路径已设置"
                    echo "  - 证书文件：$webCertFile"
                    echo "  - 私钥文件：$webKeyFile"
                    restart
                else
                    echo "未找到域名 $domain 的证书或私钥。"
                fi
            else
                echo "输入的域名无效。"
            fi
        fi
        ssl_cert_issue_main
        ;;
    6)
        echo -e "${yellow}Let's Encrypt IP 地址 SSL 证书${plain}"
        echo -e "将使用短期配置文件为服务器 IP 获取证书。"
        echo -e "${yellow}证书有效期约 6 天，通过 acme.sh cron 自动续期。${plain}"
        echo -e "${yellow}80 端口必须开放且可从外网访问。${plain}"
        confirm "是否继续？" "y"
        if [[ $? == 0 ]]; then
            ssl_cert_issue_for_ip
        fi
        ssl_cert_issue_main
        ;;

    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        ssl_cert_issue_main
        ;;
    esac
}

ssl_cert_issue_for_ip() {
    LOGI "开始为服务器 IP 自动生成 SSL 证书..."
    LOGI "使用 Let's Encrypt 短期配置文件（约 6 天有效期，自动续期）"

    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')

    # 获取服务器 IP
    local server_ip=$(curl -s --max-time 3 https://api.ipify.org)
    if [ -z "$server_ip" ]; then
        server_ip=$(curl -s --max-time 3 https://4.ident.me)
    fi

    if [ -z "$server_ip" ]; then
        LOGE "获取服务器 IP 地址失败"
        return 1
    fi

    LOGI "检测到服务器 IP：${server_ip}"

    # 询问可选的 IPv6
    local ipv6_addr=""
    read -rp "是否包含 IPv6 地址？（留空跳过）：" ipv6_addr
    ipv6_addr="${ipv6_addr// /}"  # 去除空格

    # 先检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        LOGI "未找到 acme.sh，正在安装..."
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "安装 acme.sh 失败"
            return 1
        fi
    fi

    # 安装 socat
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update >/dev/null 2>&1 && apt-get install socat -y >/dev/null 2>&1
        ;;
    fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
        dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        ;;
    centos)
        if [[ "${VERSION_ID}" =~ ^7 ]]; then
            yum -y update >/dev/null 2>&1 && yum -y install socat >/dev/null 2>&1
        else
            dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        fi
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat >/dev/null 2>&1
        ;;
    opensuse-tumbleweed | opensuse-leap)
        zypper refresh >/dev/null 2>&1 && zypper -q install -y socat >/dev/null 2>&1
        ;;
    alpine)
        apk add socat curl openssl >/dev/null 2>&1
        ;;
    *)
        LOGW "不支持的系统，无法自动安装 socat"
        ;;
    esac

    # 创建证书目录
    certPath="/root/cert/ip"
    mkdir -p "$certPath"

    # 构建域名参数
    local domain_args="-d ${server_ip}"
    if [[ -n "$ipv6_addr" ]] && is_ipv6 "$ipv6_addr"; then
        domain_args="${domain_args} -d ${ipv6_addr}"
        LOGI "包含 IPv6 地址：${ipv6_addr}"
    fi

    # 选择 HTTP-01 监听端口（默认 80，允许自定义）
    local WebPort=""
    read -rp "用于 ACME HTTP-01 监听的端口（默认 80）：" WebPort
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
        LOGE "无效端口，回退到 80。"
        WebPort=80
    fi
    LOGI "使用端口 ${WebPort} 为 IP 签发证书：${server_ip}"
    if [[ "${WebPort}" -ne 80 ]]; then
        LOGI "提醒：Let's Encrypt 仍然连接 80 端口；请将外部 80 端口转发到 ${WebPort}。"
    fi

    while true; do
        if is_port_in_use "${WebPort}"; then
            LOGI "端口 ${WebPort} 当前被占用。"

            local alt_port=""
            read -rp "请输入另一个端口供 acme.sh 独立监听（留空取消）：" alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                LOGE "端口 ${WebPort} 被占用，无法继续。"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                LOGE "无效端口。"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            LOGI "端口 ${WebPort} 空闲，可以进行独立验证。"
            break
        fi
    done

    # 重载命令 - 续期后重启面板
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null"

    # 使用短期配置文件为 IP 签发证书
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        LOGE "为 IP 签发证书失败：${server_ip}"
        LOGE "请确保端口 ${WebPort} 已开放且服务器可从外网访问"
        # 清理 acme.sh 数据（IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${server_ip} 2>/dev/null
        [[ -n "$ipv6_addr" ]] && rm -rf ~/.acme.sh/${ipv6_addr} 2>/dev/null
        rm -rf ${certPath} 2>/dev/null
        return 1
    else
        LOGI "IP 证书签发成功：${server_ip}"
    fi

    # 安装证书
    # 注意：acme.sh 可能在 reloadcmd 失败时报告 "Reload error" 并返回非零退出码，
    # 但证书文件仍然已安装。我们通过检查文件而非退出码来判断。
    ~/.acme.sh/acme.sh --installcert -d ${server_ip} \
        --key-file "${certPath}/privkey.pem" \
        --fullchain-file "${certPath}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # 验证证书文件存在（不依赖退出码 - reloadcmd 失败会导致非零）
    if [[ ! -f "${certPath}/fullchain.pem" || ! -f "${certPath}/privkey.pem" ]]; then
        LOGE "安装后未找到证书文件"
        # 清理 acme.sh 数据（IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${server_ip} 2>/dev/null
        [[ -n "$ipv6_addr" ]] && rm -rf ~/.acme.sh/${ipv6_addr} 2>/dev/null
        rm -rf ${certPath} 2>/dev/null
        return 1
    fi

    LOGI "证书文件安装成功"

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1
    chmod 600 $certPath/privkey.pem 2>/dev/null
    chmod 644 $certPath/fullchain.pem 2>/dev/null

    # 为面板设置证书路径
    local webCertFile="${certPath}/fullchain.pem"
    local webKeyFile="${certPath}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
        LOGI "面板证书已配置"
        LOGI "  - 证书文件：$webCertFile"
        LOGI "  - 私钥文件：$webKeyFile"
        LOGI "  - 有效期：约 6 天（通过 acme.sh cron 自动续期）"
        echo -e "${green}访问地址：https://${server_ip}:${existing_port}${existing_webBasePath}${plain}"
        LOGI "面板将重启以应用 SSL 证书..."
        restart
        return 0
    else
        LOGE "安装后未找到证书文件"
        return 1
    fi
}

ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # 先检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "未找到 acme.sh，将安装它"
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "安装 acme.sh 失败，请查看日志"
            exit 1
        fi
    fi

    # 安装 socat
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update >/dev/null 2>&1 && apt-get install socat -y >/dev/null 2>&1
        ;;
    fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
        dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        ;;
    centos)
        if [[ "${VERSION_ID}" =~ ^7 ]]; then
            yum -y update >/dev/null 2>&1 && yum -y install socat >/dev/null 2>&1
        else
            dnf -y update >/dev/null 2>&1 && dnf -y install socat >/dev/null 2>&1
        fi
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat >/dev/null 2>&1
        ;;
    opensuse-tumbleweed | opensuse-leap)
        zypper refresh >/dev/null 2>&1 && zypper -q install -y socat >/dev/null 2>&1
        ;;
    alpine)
        apk add socat curl openssl >/dev/null 2>&1
        ;;
    *)
        LOGW "不支持的系统，无法自动安装 socat"
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "安装 socat 失败，请查看日志"
        exit 1
    else
        LOGI "安装 socat 成功..."
    fi

    # 获取域名并验证
    local domain=""
    while true; do
        read -rp "请输入您的域名：" domain
        domain="${domain// /}"  # 去除空格

        if [[ -z "$domain" ]]; then
            LOGE "域名不能为空，请重试。"
            continue
        fi

        if ! is_domain "$domain"; then
            LOGE "无效的域名格式：${domain}，请输入有效的域名。"
            continue
        fi

        break
    done
    LOGD "您的域名是：${domain}，正在检查..."

    # 检查是否已存在证书
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')
    if [ "${currentCert}" == "${domain}" ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "系统已有该域名的证书，无法重复签发。当前证书详情："
        LOGI "$certInfo"
        exit 1
    else
        LOGI "您的域名已准备好签发证书..."
    fi

    # 创建证书目录
    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    # 获取独立服务器端口号
    local WebPort=80
    read -rp "请选择要使用的端口（默认 80）：" WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        LOGE "输入 ${WebPort} 无效，将使用默认端口 80。"
        WebPort=80
    fi
    LOGI "将使用端口：${WebPort} 签发证书。请确保此端口已开放。"

    # 签发证书
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
    if [ $? -ne 0 ]; then
        LOGE "签发证书失败，请查看日志。"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGE "证书签发成功，正在安装证书..."
    fi

    reloadCmd="x-ui restart"

    LOGI "ACME 默认 --reloadcmd 为：${yellow}x-ui restart"
    LOGI "此命令将在每次签发和续期证书时运行。"
    read -rp "是否要修改 ACME 的 --reloadcmd？(y/n)：" setReloadcmd
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} 预设：systemctl reload nginx ; x-ui restart"
        echo -e "${green}\t2.${plain} 输入自定义命令"
        echo -e "${green}\t0.${plain} 保持默认 reloadcmd"
        read -rp "请选择：" choice
        case "$choice" in
        1)
            LOGI "Reloadcmd 设为：systemctl reload nginx ; x-ui restart"
            reloadCmd="systemctl reload nginx ; x-ui restart"
            ;;
        2)
            LOGD "建议将 x-ui restart 放在最后，这样即使其他服务失败也不会报错"
            read -rp "请输入自定义的 reloadcmd（例如：systemctl reload nginx ; x-ui restart）：" reloadCmd
            LOGI "您的 reloadcmd 为：${reloadCmd}"
            ;;
        *)
            LOGI "保持默认 reloadcmd"
            ;;
        esac
    fi

    # 安装证书
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}"

    if [ $? -ne 0 ]; then
        LOGE "安装证书失败，退出。"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGI "安装证书成功，正在启用自动续期..."
    fi

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "自动续期设置失败，证书详情："
        ls -lah cert/*
        chmod 600 $certPath/privkey.pem
        chmod 644 $certPath/fullchain.pem
        exit 1
    else
        LOGI "自动续期设置成功，证书详情："
        ls -lah cert/*
        chmod 600 $certPath/privkey.pem
        chmod 644 $certPath/fullchain.pem
    fi

    # 证书安装成功后提示用户为面板设置证书路径
    read -rp "是否要为面板设置此证书？(y/n)：" setPanel
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            LOGI "域名 $domain 的面板路径已设置"
            LOGI "  - 证书文件：$webCertFile"
            LOGI "  - 私钥文件：$webKeyFile"
            echo -e "${green}访问地址：https://${domain}:${existing_port}${existing_webBasePath}${plain}"
            restart
        else
            LOGE "错误：未找到域名 $domain 的证书或私钥文件。"
        fi
    else
        LOGI "跳过面板路径设置。"
    fi
}

ssl_cert_issue_CF() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    LOGI "****** 使用说明 ******"
    LOGI "请按照以下步骤完成操作："
    LOGI "1. Cloudflare 注册邮箱。"
    LOGI "2. Cloudflare 全局 API 密钥。"
    LOGI "3. 域名。"
    LOGI "4. 证书签发后，将提示您为面板设置证书（可选）。"
    LOGI "5. 脚本还支持安装后自动续期 SSL 证书。"

    confirm "请确认信息并继续？[y/n]" "y"

    if [ $? -eq 0 ]; then
        # 检查 acme.sh
        if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
            echo "未找到 acme.sh，将安装它。"
            install_acme
            if [ $? -ne 0 ]; then
                LOGE "安装 acme.sh 失败，请查看日志。"
                exit 1
            fi
        fi

        CF_Domain=""

        LOGD "请设置域名："
        read -rp "在此输入您的域名：" CF_Domain
        LOGD "您的域名设置为：${CF_Domain}"

        # 设置 Cloudflare API 信息
        CF_GlobalKey=""
        CF_AccountEmail=""
        LOGD "请设置 API 密钥："
        read -rp "在此输入您的密钥：" CF_GlobalKey
        LOGD "您的 API 密钥为：${CF_GlobalKey}"

        LOGD "请设置注册邮箱："
        read -rp "在此输入您的邮箱：" CF_AccountEmail
        LOGD "您的注册邮箱为：${CF_AccountEmail}"

        # 设置默认 CA 为 Let's Encrypt
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        if [ $? -ne 0 ]; then
            LOGE "设置默认 CA（Let's Encrypt）失败，脚本退出..."
            exit 1
        fi

        export CF_Key="${CF_GlobalKey}"
        export CF_Email="${CF_AccountEmail}"

        # 使用 Cloudflare DNS 签发证书
        ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} --log --force
        if [ $? -ne 0 ]; then
            LOGE "证书签发失败，脚本退出..."
            exit 1
        else
            LOGI "证书签发成功，正在安装..."
        fi

         # 安装证书
        certPath="/root/cert/${CF_Domain}"
        if [ -d "$certPath" ]; then
            rm -rf ${certPath}
        fi

        mkdir -p ${certPath}
        if [ $? -ne 0 ]; then
            LOGE "创建目录失败：${certPath}"
            exit 1
        fi

        reloadCmd="x-ui restart"

        LOGI "ACME 默认 --reloadcmd 为：${yellow}x-ui restart"
        LOGI "此命令将在每次签发和续期证书时运行。"
        read -rp "是否要修改 ACME 的 --reloadcmd？(y/n)：" setReloadcmd
        if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
            echo -e "\n${green}\t1.${plain} 预设：systemctl reload nginx ; x-ui restart"
            echo -e "${green}\t2.${plain} 输入自定义命令"
            echo -e "${green}\t0.${plain} 保持默认 reloadcmd"
            read -rp "请选择：" choice
            case "$choice" in
            1)
                LOGI "Reloadcmd 设为：systemctl reload nginx ; x-ui restart"
                reloadCmd="systemctl reload nginx ; x-ui restart"
                ;;
            2)
                LOGD "建议将 x-ui restart 放在最后，这样即使其他服务失败也不会报错"
                read -rp "请输入自定义的 reloadcmd（例如：systemctl reload nginx ; x-ui restart）：" reloadCmd
                LOGI "您的 reloadcmd 为：${reloadCmd}"
                ;;
            *)
                LOGI "保持默认 reloadcmd"
                ;;
            esac
        fi
        ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} \
            --key-file ${certPath}/privkey.pem \
            --fullchain-file ${certPath}/fullchain.pem --reloadcmd "${reloadCmd}"

        if [ $? -ne 0 ]; then
            LOGE "证书安装失败，脚本退出..."
            exit 1
        else
            LOGI "证书安装成功，正在启用自动更新..."
        fi

        # 启用自动更新
        ~/.acme.sh/acme.sh --upgrade --auto-upgrade
        if [ $? -ne 0 ]; then
            LOGE "自动更新设置失败，脚本退出..."
            exit 1
        else
            LOGI "证书已安装并开启自动续期。详情如下："
            ls -lah ${certPath}/*
            chmod 600 ${certPath}/privkey.pem
            chmod 644 ${certPath}/fullchain.pem
        fi

        # 证书安装成功后提示用户为面板设置证书路径
        read -rp "是否要为面板设置此证书？(y/n)：" setPanel
        if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
            local webCertFile="${certPath}/fullchain.pem"
            local webKeyFile="${certPath}/privkey.pem"

            if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
                ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
                LOGI "域名 $CF_Domain 的面板路径已设置"
                LOGI "  - 证书文件：$webCertFile"
                LOGI "  - 私钥文件：$webKeyFile"
                echo -e "${green}访问地址：https://${CF_Domain}:${existing_port}${existing_webBasePath}${plain}"
                restart
            else
                LOGE "错误：未找到域名 $CF_Domain 的证书或私钥文件。"
            fi
        else
            LOGI "跳过面板路径设置。"
        fi
    else
        show_menu
    fi
}

run_speedtest() {
    # 检查 Speedtest 是否已安装
    if ! command -v speedtest &>/dev/null; then
        # 如未安装，确定安装方式
        if command -v snap &>/dev/null; then
            # 使用 snap 安装 Speedtest
            echo "正在使用 snap 安装 Speedtest..."
            snap install speedtest
        else
            # 回退到使用包管理器
            local pkg_manager=""
            local speedtest_install_script=""

            if command -v dnf &>/dev/null; then
                pkg_manager="dnf"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.rpm.sh"
            elif command -v yum &>/dev/null; then
                pkg_manager="yum"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.rpm.sh"
            elif command -v apt-get &>/dev/null; then
                pkg_manager="apt-get"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh"
            elif command -v apt &>/dev/null; then
                pkg_manager="apt"
                speedtest_install_script="https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh"
            fi

            if [[ -z $pkg_manager ]]; then
                echo "错误：未找到包管理器。您可能需要手动安装 Speedtest。"
                return 1
            else
                echo "正在使用 $pkg_manager 安装 Speedtest..."
                curl -s $speedtest_install_script | bash
                $pkg_manager install -y speedtest
            fi
        fi
    fi

    speedtest
}



ip_validation() {
    ipv6_regex="^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$"
    ipv4_regex="^((25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]?|0)\.){3}(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]?|0)$"
}

iplimit_main() {
    echo -e "\n${green}\t1.${plain} 安装 Fail2ban 并配置 IP 限制"
    echo -e "${green}\t2.${plain} 修改封禁时长"
    echo -e "${green}\t3.${plain} 解封所有人"
    echo -e "${green}\t4.${plain} 封禁日志"
    echo -e "${green}\t5.${plain} 封禁指定 IP 地址"
    echo -e "${green}\t6.${plain} 解封指定 IP 地址"
    echo -e "${green}\t7.${plain} 实时日志"
    echo -e "${green}\t8.${plain} 服务状态"
    echo -e "${green}\t9.${plain} 重启服务"
    echo -e "${green}\t10.${plain} 卸载 Fail2ban 和 IP 限制"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -rp "请选择：" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        confirm "是否继续安装 Fail2ban 和 IP 限制？" "y"
        if [[ $? == 0 ]]; then
            install_iplimit
        else
            iplimit_main
        fi
        ;;
    2)
        read -rp "请输入新的封禁时长（分钟）[默认 30]：" NUM
        if [[ $NUM =~ ^[0-9]+$ ]]; then
            create_iplimit_jails ${NUM}
            if [[ $release == "alpine" ]]; then
                rc-service fail2ban restart
            else
                systemctl restart fail2ban
            fi
        else
            echo -e "${red}${NUM} 不是数字！请重试。${plain}"
        fi
        iplimit_main
        ;;
    3)
        confirm "是否继续解封 IP 限制中的所有人？" "y"
        if [[ $? == 0 ]]; then
            fail2ban-client reload --restart --unban 3x-ipl
            truncate -s 0 "${iplimit_banned_log_path}"
            echo -e "${green}所有用户已成功解封。${plain}"
            iplimit_main
        else
            echo -e "${yellow}已取消。${plain}"
        fi
        iplimit_main
        ;;
    4)
        show_banlog
        iplimit_main
        ;;
    5)
        read -rp "输入要封禁的 IP 地址：" ban_ip
        ip_validation
        if [[ $ban_ip =~ $ipv4_regex || $ban_ip =~ $ipv6_regex ]]; then
            fail2ban-client set 3x-ipl banip "$ban_ip"
            echo -e "${green}IP 地址 ${ban_ip} 已成功封禁。${plain}"
        else
            echo -e "${red}IP 地址格式无效！请重试。${plain}"
        fi
        iplimit_main
        ;;
    6)
        read -rp "输入要解封的 IP 地址：" unban_ip
        ip_validation
        if [[ $unban_ip =~ $ipv4_regex || $unban_ip =~ $ipv6_regex ]]; then
            fail2ban-client set 3x-ipl unbanip "$unban_ip"
            echo -e "${green}IP 地址 ${unban_ip} 已成功解封。${plain}"
        else
            echo -e "${red}IP 地址格式无效！请重试。${plain}"
        fi
        iplimit_main
        ;;
    7)
        tail -f /var/log/fail2ban.log
        iplimit_main
        ;;
    8)
        service fail2ban status
        iplimit_main
        ;;
    9)
        if [[ $release == "alpine" ]]; then
            rc-service fail2ban restart
        else
            systemctl restart fail2ban
        fi
        iplimit_main
        ;;
    10)
        remove_iplimit
        iplimit_main
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        iplimit_main
        ;;
    esac
}

install_iplimit() {
    if ! command -v fail2ban-client &>/dev/null; then
        echo -e "${green}Fail2ban 未安装，正在安装...！${plain}\n"

        # 检查操作系统并安装必要的软件包
        case "${release}" in
        ubuntu)
            apt-get update
            if [[ "${os_version}" -ge 24 ]]; then
                apt-get install python3-pip -y
                python3 -m pip install pyasynchat --break-system-packages
            fi
            apt-get install fail2ban -y
            ;;
        debian)
            apt-get update
            if [ "$os_version" -ge 12 ]; then
                apt-get install -y python3-systemd
            fi
            apt-get install -y fail2ban
            ;;
        armbian)
            apt-get update && apt-get install fail2ban -y
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf -y install fail2ban
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum update -y && yum install epel-release -y
                yum -y install fail2ban
            else
                dnf -y update && dnf -y install fail2ban
            fi
            ;;
        arch | manjaro | parch)
            pacman -Syu --noconfirm fail2ban
            ;;
        alpine)
            apk add fail2ban
            ;;
        *)
            echo -e "${red}不支持的操作系统。请检查脚本并手动安装必要的软件包。${plain}\n"
            exit 1
            ;;
        esac

        if ! command -v fail2ban-client &>/dev/null; then
            echo -e "${red}Fail2ban 安装失败。${plain}\n"
            exit 1
        fi

        echo -e "${green}Fail2ban 安装成功！${plain}\n"
    else
        echo -e "${yellow}Fail2ban 已安装。${plain}\n"
    fi

    echo -e "${green}正在配置 IP 限制...${plain}\n"

    # 确保 jail 文件没有冲突
    iplimit_remove_conflicts

    # 检查日志文件是否存在
    if ! test -f "${iplimit_banned_log_path}"; then
        touch ${iplimit_banned_log_path}
    fi

    # 检查服务日志文件是否存在，以免 fail2ban 报错
    if ! test -f "${iplimit_log_path}"; then
        touch ${iplimit_log_path}
    fi

    # 创建 iplimit jail 文件
    # 此处不传递 bantime 以使用默认值
    create_iplimit_jails

    # 启动 fail2ban
    if [[ $release == "alpine" ]]; then
        if [[ $(rc-service fail2ban status | grep -F 'status: started' -c) == 0 ]]; then
            rc-service fail2ban start
        else
            rc-service fail2ban restart
        fi
        rc-update add fail2ban
    else
        if ! systemctl is-active --quiet fail2ban; then
            systemctl start fail2ban
        else
            systemctl restart fail2ban
        fi
        systemctl enable fail2ban
    fi

    echo -e "${green}IP 限制安装并配置成功！${plain}\n"
    before_show_menu
}

remove_iplimit() {
    echo -e "${green}\t1.${plain} 仅移除 IP 限制配置"
    echo -e "${green}\t2.${plain} 卸载 Fail2ban 和 IP 限制"
    echo -e "${green}\t0.${plain} 返回主菜单"
    read -rp "请选择：" num
    case "$num" in
    1)
        rm -f /etc/fail2ban/filter.d/3x-ipl.conf
        rm -f /etc/fail2ban/action.d/3x-ipl.conf
        rm -f /etc/fail2ban/jail.d/3x-ipl.conf
        if [[ $release == "alpine" ]]; then
            rc-service fail2ban restart
        else
            systemctl restart fail2ban
        fi
        echo -e "${green}IP 限制已成功移除！${plain}\n"
        before_show_menu
        ;;
    2)
        rm -rf /etc/fail2ban
        if [[ $release == "alpine" ]]; then
            rc-service fail2ban stop
        else
            systemctl stop fail2ban
        fi
        case "${release}" in
        ubuntu | debian | armbian)
            apt-get remove -y fail2ban
            apt-get purge -y fail2ban -y
            apt-get autoremove -y
            ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf remove fail2ban -y
            dnf autoremove -y
            ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum remove fail2ban -y
                yum autoremove -y
            else
                dnf remove fail2ban -y
                dnf autoremove -y
            fi
            ;;
        arch | manjaro | parch)
            pacman -Rns --noconfirm fail2ban
            ;;
        alpine)
            apk del fail2ban
            ;;
        *)
            echo -e "${red}不支持的操作系统。请手动卸载 Fail2ban。${plain}\n"
            exit 1
            ;;
        esac
        echo -e "${green}Fail2ban 和 IP 限制已成功移除！${plain}\n"
        before_show_menu
        ;;
    0)
        show_menu
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        remove_iplimit
        ;;
    esac
}

show_banlog() {
    local system_log="/var/log/fail2ban.log"

    echo -e "${green}正在检查封禁日志...${plain}\n"

    if [[ $release == "alpine" ]]; then
        if [[ $(rc-service fail2ban status | grep -F 'status: started' -c) == 0 ]]; then
            echo -e "${red}Fail2ban 服务未运行！${plain}\n"
            return 1
        fi
    else
        if ! systemctl is-active --quiet fail2ban; then
            echo -e "${red}Fail2ban 服务未运行！${plain}\n"
            return 1
        fi
    fi

    if [[ -f "$system_log" ]]; then
        echo -e "${green}来自 fail2ban.log 的最近系统封禁活动：${plain}"
        grep "3x-ipl" "$system_log" | grep -E "Ban|Unban" | tail -n 10 || echo -e "${yellow}未找到最近的系统封禁活动${plain}"
        echo ""
    fi

    if [[ -f "${iplimit_banned_log_path}" ]]; then
        echo -e "${green}3X-IPL 封禁日志条目：${plain}"
        if [[ -s "${iplimit_banned_log_path}" ]]; then
            grep -v "INIT" "${iplimit_banned_log_path}" | tail -n 10 || echo -e "${yellow}未找到封禁记录${plain}"
        else
            echo -e "${yellow}封禁日志文件为空${plain}"
        fi
    else
        echo -e "${red}未找到封禁日志文件：${iplimit_banned_log_path}${plain}"
    fi

    echo -e "\n${green}当前 jail 状态：${plain}"
    fail2ban-client status 3x-ipl || echo -e "${yellow}无法获取 jail 状态${plain}"
}

create_iplimit_jails() {
    # 如未传递则使用默认封禁时长 => 30 分钟
    local bantime="${1:-30}"

    # 取消 fail2ban.conf 中 'allowipv6 = auto' 的注释
    sed -i 's/#allowipv6 = auto/allowipv6 = auto/g' /etc/fail2ban/fail2ban.conf

    # 在 Debian 12+ 上，fail2ban 的默认后端应更改为 systemd
    if [[  "${release}" == "debian" && ${os_version} -ge 12 ]]; then
        sed -i '0,/action =/s/backend = auto/backend = systemd/' /etc/fail2ban/jail.conf
    fi

    cat << EOF > /etc/fail2ban/jail.d/3x-ipl.conf
[3x-ipl]
enabled=true
backend=auto
filter=3x-ipl
action=3x-ipl
logpath=${iplimit_log_path}
maxretry=2
findtime=32
bantime=${bantime}m
EOF

    cat << EOF > /etc/fail2ban/filter.d/3x-ipl.conf
[Definition]
datepattern = ^%%Y/%%m/%%d %%H:%%M:%%S
failregex   = \[LIMIT_IP\]\s*Email\s*=\s*<F-USER>.+</F-USER>\s*\|\|\s*Disconnecting OLD IP\s*=\s*<ADDR>\s*\|\|\s*Timestamp\s*=\s*\d+
ignoreregex =
EOF

    cat << EOF > /etc/fail2ban/action.d/3x-ipl.conf
[INCLUDES]
before = iptables-allports.conf

[Definition]
actionstart = <iptables> -N f2b-<name>
              <iptables> -A f2b-<name> -j <returntype>
              <iptables> -I <chain> -p <protocol> -j f2b-<name>

actionstop = <iptables> -D <chain> -p <protocol> -j f2b-<name>
             <actionflush>
             <iptables> -X f2b-<name>

actioncheck = <iptables> -n -L <chain> | grep -q 'f2b-<name>[ \t]'

actionban = <iptables> -I f2b-<name> 1 -s <ip> -j <blocktype>
            echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   BAN   [Email] = <F-USER> [IP] = <ip> banned for <bantime> seconds." >> ${iplimit_banned_log_path}

actionunban = <iptables> -D f2b-<name> -s <ip> -j <blocktype>
              echo "\$(date +"%%Y/%%m/%%d %%H:%%M:%%S")   UNBAN   [Email] = <F-USER> [IP] = <ip> unbanned." >> ${iplimit_banned_log_path}

[Init]
name = default
protocol = tcp
chain = INPUT
EOF

    echo -e "${green}IP 限制 jail 文件已创建，封禁时长为 ${bantime} 分钟。${plain}"
}

iplimit_remove_conflicts() {
    local jail_files=(
        /etc/fail2ban/jail.conf
        /etc/fail2ban/jail.local
    )

    for file in "${jail_files[@]}"; do
        # 检查 jail 文件中是否存在 [3x-ipl] 配置，如存在则移除
        if test -f "${file}" && grep -qw '3x-ipl' ${file}; then
            sed -i "/\[3x-ipl\]/,/^$/d" ${file}
            echo -e "${yellow}正在移除 jail (${file}) 中 [3x-ipl] 的冲突配置！${plain}\n"
        fi
    done
}

SSH_port_forwarding() {
    local URL_lists=(
        "https://api4.ipify.org"
		"https://ipv4.icanhazip.com"
		"https://v4.api.ipinfo.io/ip"
		"https://ipv4.myexternalip.com/raw"
		"https://4.ident.me"
		"https://check-host.net/ip"
    )
    local server_ip=""
    for ip_address in "${URL_lists[@]}"; do
        local response=$(curl -s -w "\n%{http_code}" --max-time 3 "${ip_address}" 2>/dev/null)
        local http_code=$(echo "$response" | tail -n1)
        local ip_result=$(echo "$response" | head -n-1 | tr -d '[:space:]')
        if [[ "${http_code}" == "200" && -n "${ip_result}" ]]; then
            server_ip="${ip_result}"
            break
        fi
    done

    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    local existing_listenIP=$(${xui_folder}/x-ui setting -getListen true | grep -Eo 'listenIP: .+' | awk '{print $2}')
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep -Eo 'cert: .+' | awk '{print $2}')
    local existing_key=$(${xui_folder}/x-ui setting -getCert true | grep -Eo 'key: .+' | awk '{print $2}')

    local config_listenIP=""
    local listen_choice=""

    if [[ -n "$existing_cert" && -n "$existing_key" ]]; then
        echo -e "${green}面板已配置 SSL，安全。${plain}"
        before_show_menu
    fi
    if [[ -z "$existing_cert" && -z "$existing_key" && (-z "$existing_listenIP" || "$existing_listenIP" == "0.0.0.0") ]]; then
        echo -e "\n${red}警告：未找到证书和密钥！面板不安全。${plain}"
        echo "请获取证书或设置 SSH 端口转发。"
    fi

    if [[ -n "$existing_listenIP" && "$existing_listenIP" != "0.0.0.0" && (-z "$existing_cert" && -z "$existing_key") ]]; then
        echo -e "\n${green}当前 SSH 端口转发配置：${plain}"
        echo -e "标准 SSH 命令："
        echo -e "${yellow}ssh -L 2222:${existing_listenIP}:${existing_port} root@${server_ip}${plain}"
        echo -e "\n如果使用 SSH 密钥："
        echo -e "${yellow}ssh -i <sshkeypath> -L 2222:${existing_listenIP}:${existing_port} root@${server_ip}${plain}"
        echo -e "\n连接后，通过以下地址访问面板："
        echo -e "${yellow}http://localhost:2222${existing_webBasePath}${plain}"
    fi

    echo -e "\n请选择："
    echo -e "${green}1.${plain} 设置监听 IP"
    echo -e "${green}2.${plain} 清除监听 IP"
    echo -e "${green}0.${plain} 返回主菜单"
    read -rp "请选择：" num

    case "$num" in
    1)
        if [[ -z "$existing_listenIP" || "$existing_listenIP" == "0.0.0.0" ]]; then
            echo -e "\n未配置 listenIP。请选择："
            echo -e "1. 使用默认 IP (127.0.0.1)"
            echo -e "2. 设置自定义 IP"
            read -rp "请选择（1 或 2）：" listen_choice

            config_listenIP="127.0.0.1"
            [[ "$listen_choice" == "2" ]] && read -rp "输入自定义监听 IP：" config_listenIP

            ${xui_folder}/x-ui setting -listenIP "${config_listenIP}" >/dev/null 2>&1
            echo -e "${green}监听 IP 已设置为 ${config_listenIP}。${plain}"
            echo -e "\n${green}SSH 端口转发配置：${plain}"
            echo -e "标准 SSH 命令："
            echo -e "${yellow}ssh -L 2222:${config_listenIP}:${existing_port} root@${server_ip}${plain}"
            echo -e "\n如果使用 SSH 密钥："
            echo -e "${yellow}ssh -i <sshkeypath> -L 2222:${config_listenIP}:${existing_port} root@${server_ip}${plain}"
            echo -e "\n连接后，通过以下地址访问面板："
            echo -e "${yellow}http://localhost:2222${existing_webBasePath}${plain}"
            restart
        else
            config_listenIP="${existing_listenIP}"
            echo -e "${green}当前监听 IP 已设置为 ${config_listenIP}。${plain}"
        fi
        ;;
    2)
        ${xui_folder}/x-ui setting -listenIP 0.0.0.0 >/dev/null 2>&1
        echo -e "${green}监听 IP 已清除。${plain}"
        restart
        ;;
    0)
        show_menu
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        SSH_port_forwarding
        ;;
    esac
}

show_usage() {
    echo -e "┌────────────────────────────────────────────────────────────────┐
│  ${blue}x-ui 管理菜单用法（子命令）：${plain}                                │
│                                                                │
│  ${blue}x-ui${plain}                       - 管理脚本                      │
│  ${blue}x-ui start${plain}                 - 启动                          │
│  ${blue}x-ui stop${plain}                  - 停止                          │
│  ${blue}x-ui restart${plain}               - 重启                          │
|  ${blue}x-ui restart-xray${plain}          - 重启 Xray                     │
│  ${blue}x-ui status${plain}                - 查看当前状态                  │
│  ${blue}x-ui settings${plain}              - 查看当前设置                  │
│  ${blue}x-ui enable${plain}                - 设置开机自启                  │
│  ${blue}x-ui disable${plain}               - 取消开机自启                  │
│  ${blue}x-ui log${plain}                   - 查看日志                      │
│  ${blue}x-ui banlog${plain}                - 查看 Fail2ban 封禁日志        │
│  ${blue}x-ui update${plain}                - 更新                          │
│  ${blue}x-ui update-all-geofiles${plain}   - 更新所有 geo 文件            │
│  ${blue}x-ui geofile-cron --enable${plain} - 启用 Geofile 定时更新        │
│  ${blue}x-ui geofile-cron --disable${plain} - 禁用 Geofile 定时更新       │
│  ${blue}x-ui geofile-cron --status${plain}  - 查看 Geofile 定时更新状态   │
│  ${blue}x-ui legacy${plain}                - 安装旧版本                    │
│  ${blue}x-ui install${plain}               - 安装                          │
│  ${blue}x-ui uninstall${plain}             - 卸载                          │
└────────────────────────────────────────────────────────────────┘"
}

# Read dbType from /etc/x-ui/x-ui.json using the Go binary
read_json_dbtype() {
    local db_type
    db_type=$(${xui_folder}/x-ui setting -showDbType 2>/dev/null)
    if [ -z "$db_type" ]; then
        echo "sqlite"
    else
        echo "$db_type"
    fi
}

get_database_setting() {
    local key="$1"
    local default_value="$2"
    local json_path="/etc/x-ui/x-ui.json"
    local jq_expr=""

    if [ ! -f "$json_path" ]; then
        echo "$default_value"
        return
    fi

    if command -v jq >/dev/null 2>&1; then
        case "$key" in
        ".dbType")
            jq_expr='.databaseConnection.dbType // .other.dbType // .dbType // "sqlite"'
            ;;
        ".dbHost")
            jq_expr='.databaseConnection.dbHost // .other.dbHost // .dbHost // "127.0.0.1"'
            ;;
        ".dbPort")
            jq_expr='.databaseConnection.dbPort // .other.dbPort // .dbPort // "3306"'
            ;;
        ".dbUser")
            jq_expr='.databaseConnection.dbUser // .other.dbUser // .dbUser // ""'
            ;;
        ".dbPassword")
            jq_expr='.databaseConnection.dbPassword // .other.dbPassword // .dbPassword // ""'
            ;;
        ".dbName")
            jq_expr='.databaseConnection.dbName // .other.dbName // .dbName // "3xui"'
            ;;
        *)
            jq_expr="$key // $default_value"
            ;;
        esac
        jq -r "$jq_expr" "$json_path" 2>/dev/null
        return
    fi

    case "$key" in
    ".dbType")
        grep -o '"dbType"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".dbHost")
        grep -o '"dbHost"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".dbPort")
        grep -o '"dbPort"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".dbUser")
        grep -o '"dbUser"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".dbPassword")
        grep -o '"dbPassword"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".dbName")
        grep -o '"dbName"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    *)
        echo "$default_value"
        ;;
    esac
}

# Show current database status
db_show_status() {
    local current_type=$(read_json_dbtype)
    echo -e "${green}当前数据库类型: ${current_type}${plain}"
    if [ "$current_type" = "mariadb" ]; then
        local host=$(get_database_setting '.dbHost' '127.0.0.1')
        local port=$(get_database_setting '.dbPort' '3306')
        local dbname=$(get_database_setting '.dbName' '3xui')
        echo -e "${green}MariaDB 主机: ${host:-127.0.0.1}:${port:-3306}${plain}"
        echo -e "${green}数据库名: ${dbname:-3xui}${plain}"
    fi
    show_node_status
}

get_node_setting() {
    local key="$1"
    local default_value="$2"
    local json_path="/etc/x-ui/x-ui.json"
    local jq_expr=""

    if [ ! -f "$json_path" ]; then
        echo "$default_value"
        return
    fi

    if command -v jq >/dev/null 2>&1; then
        case "$key" in
        ".nodeRole")
            jq_expr='.other.nodeRole // .nodeRole // "master"'
            ;;
        ".nodeId")
            jq_expr='.other.nodeId // .nodeId // ""'
            ;;
        ".syncInterval")
            jq_expr='.other.syncInterval // .syncInterval // "30"'
            ;;
        ".trafficFlushInterval")
            jq_expr='.other.trafficFlushInterval // .trafficFlushInterval // "10"'
            ;;
        *)
            jq_expr="$key // $default_value"
            ;;
        esac
        jq -r "$jq_expr" "$json_path" 2>/dev/null
        return
    fi

    case "$key" in
    ".nodeRole")
        grep -o '"nodeRole"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".nodeId")
        grep -o '"nodeId"[[:space:]]*:[[:space:]]*"[^"]*"' "$json_path" 2>/dev/null | tail -1 | sed 's/.*"\([^"]*\)"$/\1/' || echo "$default_value"
        ;;
    ".syncInterval")
        grep -o '"syncInterval"[[:space:]]*:[[:space:]]*[^,}]*' "$json_path" 2>/dev/null | tail -1 | awk -F': ' '{print $2}' | tr -d '[:space:]' || echo "$default_value"
        ;;
    ".trafficFlushInterval")
        grep -o '"trafficFlushInterval"[[:space:]]*:[[:space:]]*[^,}]*' "$json_path" 2>/dev/null | tail -1 | awk -F': ' '{print $2}' | tr -d '[:space:]' || echo "$default_value"
        ;;
    *)
        echo "$default_value"
        ;;
    esac
}

show_node_status() {
    local node_role
    local node_id
    local sync_interval
    local flush_interval

    node_role=$(get_node_setting '.nodeRole' '"master"' | tr -d '"')
    node_id=$(get_node_setting '.nodeId' '""' | tr -d '"')
    sync_interval=$(get_node_setting '.syncInterval' '30' | tr -d '"')
    flush_interval=$(get_node_setting '.trafficFlushInterval' '10' | tr -d '"')

    echo -e "${green}节点角色: ${node_role:-master}${plain}"
    echo -e "${green}节点 ID: ${node_id:-<empty>}${plain}"
    echo -e "${green}同步间隔: ${sync_interval:-30}s${plain}"
    echo -e "${green}流量回刷间隔: ${flush_interval:-10}s${plain}"
}

set_node_role() {
    local node_role=""
    local node_id=""

    read -rp "输入节点角色（master/worker）: " node_role
    node_role=$(echo "${node_role}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
    if [ "$node_role" != "master" ] && [ "$node_role" != "worker" ]; then
        echo -e "${red}无效的节点角色${plain}"
        return 1
    fi

    if [ "$node_role" = "worker" ]; then
        read -rp "输入节点 ID: " node_id
        node_id="${node_id// /}"
        if [ -z "$node_id" ]; then
            echo -e "${red}worker 节点必须提供 nodeId${plain}"
            return 1
        fi
        if ! ${xui_folder}/x-ui setting -nodeRole worker -nodeId "$node_id"; then
            echo -e "${red}节点角色更新失败${plain}"
            return 1
        fi
    else
        if ! ${xui_folder}/x-ui setting -nodeRole master; then
            echo -e "${red}节点角色更新失败${plain}"
            return 1
        fi
    fi

    echo -e "${yellow}节点设置已更新，建议重启面板使其完全生效。${plain}"
}

set_node_id() {
    local node_id=""
    local current_role=""

    read -rp "输入节点 ID: " node_id
    node_id="${node_id// /}"
    current_role=$(get_node_setting '.nodeRole' '"master"' | tr -d '"')
    if [ "${current_role}" = "worker" ] && [ -z "${node_id}" ]; then
        echo -e "${red}worker 节点必须提供 nodeId${plain}"
        return 1
    fi
    if ! ${xui_folder}/x-ui setting -nodeId "$node_id"; then
        echo -e "${red}节点 ID 更新失败${plain}"
        return 1
    fi
    echo -e "${yellow}节点 ID 已更新，建议重启面板使其完全生效。${plain}"
}

set_sync_interval() {
    local sync_interval=""

    read -rp "输入同步间隔（秒）: " sync_interval
    sync_interval="${sync_interval// /}"
    if ! [[ "${sync_interval}" =~ ^[1-9][0-9]*$ ]]; then
        echo -e "${red}同步间隔必须为正整数${plain}"
        return 1
    fi
    if ! ${xui_folder}/x-ui setting -syncInterval "${sync_interval}"; then
        echo -e "${red}同步间隔更新失败${plain}"
        return 1
    fi
    echo -e "${yellow}同步间隔已更新，建议重启面板使其完全生效。${plain}"
}

set_traffic_flush_interval() {
    local flush_interval=""

    read -rp "输入流量回刷间隔（秒）: " flush_interval
    flush_interval="${flush_interval// /}"
    if ! [[ "${flush_interval}" =~ ^[1-9][0-9]*$ ]]; then
        echo -e "${red}流量回刷间隔必须为正整数${plain}"
        return 1
    fi
    if ! ${xui_folder}/x-ui setting -trafficFlushInterval "${flush_interval}"; then
        echo -e "${red}流量回刷间隔更新失败${plain}"
        return 1
    fi
    echo -e "${yellow}流量回刷间隔已更新，建议重启面板使其完全生效。${plain}"
}

set_remote_database_connection() {
    local current_type current_host current_port current_user current_name current_pass
    local db_host db_port db_user db_name db_pass effective_pass

    current_type=$(read_json_dbtype)
    current_host=$(get_database_setting '.dbHost' '127.0.0.1')
    current_port=$(get_database_setting '.dbPort' '3306')
    current_user=$(get_database_setting '.dbUser' '')
    current_name=$(get_database_setting '.dbName' '3xui')
    current_pass=$(get_database_setting '.dbPassword' '')

    echo -e "${green}当前远程数据库连接配置：${plain}"
    echo -e "${green}Host: ${current_host:-127.0.0.1}${plain}"
    echo -e "${green}Port: ${current_port:-3306}${plain}"
    echo -e "${green}User: ${current_user:-<empty>}${plain}"
    echo -e "${green}Database: ${current_name:-3xui}${plain}"
    if [ -n "$current_pass" ]; then
        echo -e "${green}Password: <stored>${plain}"
    else
        echo -e "${green}Password: <empty>${plain}"
    fi

    if [ "$current_type" != "mariadb" ]; then
        echo -e "${yellow}当前数据库类型为 ${current_type}。本操作只更新 MariaDB 连接配置，不会自动切换数据库类型。${plain}"
    fi

    ensure_mariadb_client_ready || {
        echo -e "${yellow}已取消安装 MariaDB 客户端，返回数据库菜单${plain}"
        return 1
    }

    echo -e "${green}请输入新的远程数据库连接信息，直接回车保留当前值。${plain}"
    read -rp "远程 MariaDB host [${current_host:-127.0.0.1}]: " db_host
    read -rp "远程 MariaDB port [${current_port:-3306}]: " db_port
    read -rp "业务数据库名 [${current_name:-3xui}]: " db_name
    read -rp "业务用户名 [${current_user}]: " db_user
    read -rsp "业务密码（留空则保持当前密码）: " db_pass
    echo

    db_host=${db_host:-$current_host}
    db_port=${db_port:-$current_port}
    db_name=${db_name:-$current_name}
    db_user=${db_user:-$current_user}

    if [ -z "$db_pass" ]; then
        effective_pass="$current_pass"
    else
        effective_pass="$db_pass"
    fi

    if [ -z "$db_host" ]; then
        echo -e "${red}远程 MariaDB host 不能为空${plain}"
        return 1
    fi
    if ! [[ "${db_port}" =~ ^[0-9]+$ ]] || ((db_port < 1 || db_port > 65535)); then
        echo -e "${red}远程 MariaDB 端口无效，请输入 1-65535 之间的数字${plain}"
        return 1
    fi
    if [ -z "$db_user" ]; then
        echo -e "${red}业务用户名不能为空${plain}"
        return 1
    fi
    if [ -z "$db_name" ]; then
        echo -e "${red}业务数据库名不能为空${plain}"
        return 1
    fi

    echo -e "${green}正在验证远程 MariaDB 业务连接...${plain}"
    if ! test_mariadb_database_connection "$db_host" "$db_port" "$db_name" "$db_user" "$effective_pass"; then
        echo -e "${red}无法使用输入的远程 MariaDB 信息连接到业务数据库，配置未保存${plain}"
        return 1
    fi

    echo -e "${green}正在保存远程 MariaDB 连接配置...${plain}"
    if [ -n "$db_pass" ]; then
        XUI_DB_PASSWORD="$db_pass" ${xui_folder}/x-ui setting -dbHost "$db_host" -dbPort "$db_port" -dbUser "$db_user" -dbName "$db_name" >/dev/null 2>&1
    else
        ${xui_folder}/x-ui setting -dbHost "$db_host" -dbPort "$db_port" -dbUser "$db_user" -dbName "$db_name" >/dev/null 2>&1
    fi
    if [ $? -ne 0 ]; then
        echo -e "${red}远程 MariaDB 连接配置保存失败${plain}"
        return 1
    fi

    if [ "$current_type" = "mariadb" ]; then
        echo -e "${yellow}远程 MariaDB 连接配置已更新，建议重启面板使其完全生效。${plain}"
    else
        echo -e "${yellow}远程 MariaDB 连接配置已更新，当前数据库类型仍为 ${current_type}。${plain}"
    fi
}

validate_tcp_port() {
    local port="$1"
    [[ "$port" =~ ^[0-9]+$ ]] && ((port >= 1 && port <= 65535))
}

is_local_mariadb_host() {
    case "$1" in
    "" | "127.0.0.1" | "localhost" | "::1")
        return 0
        ;;
    *)
        return 1
        ;;
    esac
}

mariadb_server_override_path() {
    local dir=""
    for dir in /etc/mysql/mariadb.conf.d /etc/mysql/conf.d /etc/my.cnf.d /etc/mariadb.conf.d; do
        if [ -d "$dir" ]; then
            echo "${dir}/60-x-ui.cnf"
            return 0
        fi
    done
    echo "/etc/my.cnf"
}

mariadb_server_config_candidates() {
    local override_path
    override_path=$(mariadb_server_override_path)

    local path=""
    for path in \
        /etc/mysql/mariadb.conf.d/50-server.cnf \
        /etc/mysql/mariadb.cnf \
        /etc/mysql/my.cnf \
        /etc/mysql/conf.d/mysql.cnf \
        /etc/my.cnf.d/mariadb-server.cnf \
        /etc/my.cnf.d/server.cnf \
        /etc/mariadb.conf.d/50-server.cnf \
        /etc/my.cnf; do
        if [ -f "$path" ] && [ "$path" != "$override_path" ]; then
            echo "$path"
        fi
    done

    echo "$override_path"
}

ensure_mariadb_override_file() {
    local path
    path=$(mariadb_server_override_path)
    mkdir -p "$(dirname "$path")"
    if [ ! -f "$path" ]; then
        printf "[mysqld]\n" >"$path"
    elif ! grep -q '^\[mysqld\]' "$path" 2>/dev/null; then
        printf "\n[mysqld]\n" >>"$path"
    fi
    echo "$path"
}

upsert_mariadb_mysqld_option() {
    local file="$1"
    local key="$2"
    local value="$3"
    local tmp_file
    tmp_file=$(mktemp)

    awk -v key="$key" -v value="$value" '
    BEGIN {
        in_section = 0
        section_seen = 0
        key_written = 0
    }
    /^\[.*\][[:space:]]*$/ {
        if (in_section && !key_written) {
            print key " = " value
            key_written = 1
        }
        if ($0 == "[mysqld]") {
            in_section = 1
            section_seen = 1
        } else {
            in_section = 0
        }
        print
        next
    }
    {
        if (in_section && $0 ~ "^[[:space:]]*[#;]?[[:space:]]*" key "[[:space:]]*=") {
            if (!key_written) {
                print key " = " value
                key_written = 1
            }
            next
        }
        print
    }
    END {
        if (!section_seen) {
            print "[mysqld]"
        }
        if (!key_written) {
            print key " = " value
        }
    }' "$file" >"$tmp_file"

    cat "$tmp_file" >"$file"
    rm -f "$tmp_file"
}

disable_mariadb_skip_networking() {
    local file=""
    local tmp_file=""
    while IFS= read -r file; do
        [ -f "$file" ] || continue
        tmp_file=$(mktemp)
        awk '
        {
            if ($0 ~ /^[[:space:]]*skip-networking([[:space:]]*=[[:space:]]*.*)?[[:space:]]*$/) {
                print "# x-ui disabled skip-networking to allow managed remote access"
                next
            }
            print
        }' "$file" >"$tmp_file"
        cat "$tmp_file" >"$file"
        rm -f "$tmp_file"
    done < <(mariadb_server_config_candidates)
}

read_mariadb_server_option() {
    local key="$1"
    local default_value="$2"
    local file=""
    local value=""
    local result=""

    while IFS= read -r file; do
        [ -f "$file" ] || continue
        value=$(awk -v key="$key" '
        BEGIN {
            in_section = 0
            value = ""
        }
        /^\[.*\][[:space:]]*$/ {
            in_section = ($0 == "[mysqld]")
            next
        }
        {
            if (in_section && $0 !~ /^[[:space:]]*[#;]/ && $0 ~ "^[[:space:]]*" key "[[:space:]]*=") {
                sub(/^[[:space:]]*[^=]+=[[:space:]]*/, "", $0)
                gsub(/[[:space:]]+$/, "", $0)
                value = $0
            }
        }
        END {
            print value
        }' "$file")
        if [ -n "$value" ]; then
            result="$value"
        fi
    done < <(mariadb_server_config_candidates)

    echo "${result:-$default_value}"
}

restart_mariadb_service() {
    local svc_name=""
    local output=""
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl list-unit-files 2>/dev/null | grep -q "^mariadb.service"; then
            svc_name="mariadb"
        elif systemctl list-unit-files 2>/dev/null | grep -q "^mysql.service"; then
            svc_name="mysql"
        fi
    fi

    if [ -n "$svc_name" ]; then
        output=$(systemctl restart "$svc_name" 2>&1) || {
            echo -e "${red}systemctl restart $svc_name 失败:${plain}" >&2
            echo "$output" >&2
            echo -e "${yellow}systemctl status $svc_name:${plain}" >&2
            systemctl status "$svc_name" --no-pager -l 2>&1 | head -20 >&2
            return 1
        }
        return 0
    fi
    if [[ $release == "alpine" ]]; then
        output=$(rc-service mariadb restart 2>&1) || {
            echo -e "${red}rc-service mariadb restart 失败:${plain}" >&2
            echo "$output" >&2
            return 1
        }
        return $?
    fi

    start_mariadb_service
}

configure_local_mariadb_server_network() {
    local db_port="$1"
    local bind_address="$2"
    local override_file=""

    if ! validate_tcp_port "$db_port"; then
        echo -e "${red}MariaDB 端口无效，请输入 1-65535 之间的数字${plain}"
        return 1
    fi

    override_file=$(ensure_mariadb_override_file) || return 1
    upsert_mariadb_mysqld_option "$override_file" "port" "$db_port"
    upsert_mariadb_mysqld_option "$override_file" "bind-address" "$bind_address"
    disable_mariadb_skip_networking

    if ! restart_mariadb_service; then
        echo -e "${red}重启 MariaDB 失败，请检查配置文件${plain}"
        return 1
    fi
    return 0
}

has_mariadb_cli() {
    command -v mariadb >/dev/null 2>&1 || command -v mysql >/dev/null 2>&1
}

mariadb_cli_bin() {
    if command -v mariadb >/dev/null 2>&1; then
        command -v mariadb
        return 0
    fi
    if command -v mysql >/dev/null 2>&1; then
        command -v mysql
        return 0
    fi
    return 1
}

has_local_mariadb_service() {
    if command -v systemctl >/dev/null 2>&1; then
        # Also verify the server package is actually installed (not just a stale service file)
        if systemctl list-unit-files 2>/dev/null | grep -qE '^(mariadb|mysql)\.service$'; then
            case "${release}" in
                ubuntu | debian | armbian | linuxmint)
                    dpkg -s mariadb-server >/dev/null 2>&1 && return 0
                    # Package missing but service file exists — stale state
                    return 1
                ;;
                centos | rhel | almalinux | rocky | ol | alinux | amzn | fedora)
                    rpm -q mariadb-server >/dev/null 2>&1 && return 0
                    return 1
                ;;
                *)
                    return 0
                ;;
            esac
        fi
    fi
    [[ -f /etc/init.d/mariadb ]]
}

check_mariadb_installed() {
    has_mariadb_cli || has_local_mariadb_service
}

install_mariadb_client() {
    echo -e "${green}正在安装 MariaDB 客户端...${plain}"
    case "${release}" in
    ubuntu | debian | linuxmint)
        apt-get update -y && apt-get install -y mariadb-client
        ;;
    centos | rhel | almalinux | rocky | ol | alinux | amzn)
        if command -v dnf >/dev/null 2>&1; then
            dnf install -y mariadb
        else
            yum install -y mariadb
        fi
        ;;
    fedora)
        dnf install -y mariadb
        ;;
    arch | manjaro)
        pacman -Sy --noconfirm mariadb-clients >/dev/null 2>&1 || pacman -Sy --noconfirm mariadb
        ;;
    opensuse* | sles)
        zypper install -y mariadb-client
        ;;
    alpine)
        apk add mariadb-client
        ;;
    *)
        echo -e "${red}不支持的发行版: ${release}，请手动安装 MariaDB 客户端${plain}"
        return 1
        ;;
    esac
}

install_local_mariadb_server() {
    echo -e "${green}正在安装本地 MariaDB...${plain}"
    case "${release}" in
    ubuntu | debian | linuxmint)
        apt-get update -y && apt-get install -y mariadb-server mariadb-client
        ;;
    centos | rhel | almalinux | rocky | ol | alinux | amzn)
        if command -v dnf >/dev/null 2>&1; then
            dnf install -y mariadb-server mariadb
        else
            yum install -y mariadb-server mariadb
        fi
        ;;
    fedora)
        dnf install -y mariadb-server mariadb
        ;;
    arch | manjaro)
        pacman -Sy --noconfirm mariadb
        mariadb-install-db --user=mysql --basedir=/usr --datadir=/var/lib/mysql >/dev/null 2>&1 || true
        ;;
    opensuse* | sles)
        zypper install -y mariadb-server mariadb-client
        ;;
    alpine)
        apk add mariadb mariadb-client
        mariadb-install-db --user=mysql --basedir=/usr --datadir=/var/lib/mysql >/dev/null 2>&1 || true
        ;;
    *)
        echo -e "${red}不支持的发行版: ${release}，请手动安装 MariaDB${plain}"
        return 1
        ;;
    esac
    local ret=$?
    if [ $ret -eq 0 ]; then
        echo -e "${green}MariaDB 安装成功${plain}"
    else
        echo -e "${red}MariaDB 安装失败${plain}"
    fi
    return $ret
}

start_mariadb_service() {
    local svc_name=""
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl list-unit-files 2>/dev/null | grep -q "^mariadb.service"; then
            svc_name="mariadb"
        elif systemctl list-unit-files 2>/dev/null | grep -q "^mysql.service"; then
            svc_name="mysql"
        fi
    fi
    if [ -n "$svc_name" ]; then
        systemctl start "$svc_name" 2>/dev/null || true
        systemctl enable "$svc_name" 2>/dev/null || true
        return 0
    fi
    if [[ $release == "alpine" ]]; then
        rc-service mariadb start 2>/dev/null
        rc-update add mariadb 2>/dev/null
        return $?
    fi
    return 1
}

ensure_mariadb_client_ready() {
    if has_mariadb_cli; then
        return 0
    fi
    echo -e "${yellow}未检测到 MariaDB 客户端${plain}"
    confirm "是否安装 MariaDB 客户端？" "y" || return 1
    install_mariadb_client || return 1
    has_mariadb_cli
}

ensure_local_mariadb_ready() {
    if ! has_local_mariadb_service; then
        echo -e "${yellow}未检测到本地 MariaDB 服务${plain}"
        confirm "是否安装本地 MariaDB？" "y" || return 1
        install_local_mariadb_server || return 1
        LOCAL_MARIADB_JUST_INSTALLED="1"
    fi
    ensure_mariadb_client_ready || return 1
    start_mariadb_service || true
    return 0
}

test_mariadb_server_connection() {
    local host="$1" port="$2" user="$3" pass="$4"
    local bin
    local -a cmd
    local err_output
    bin=$(mariadb_cli_bin) || return 1
    cmd=("$bin" -h "$host" -P "$port" -u "$user")
    if [[ -n "$pass" ]]; then
        cmd+=("-p$pass")
    fi
    cmd+=(-e "SELECT 1;")
    err_output=$("${cmd[@]}" 2>&1)
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        echo -e "${red}MariaDB 连接失败: ${err_output}${plain}" >&2
        return 1
    fi
}

test_mariadb_database_connection() {
    local host="$1" port="$2" dbname="$3" user="$4" pass="$5"
    local bin
    local -a cmd
    local err_output
    bin=$(mariadb_cli_bin) || return 1
    cmd=("$bin" -h "$host" -P "$port" -u "$user" -D "$dbname")
    if [[ -n "$pass" ]]; then
        cmd+=("-p$pass")
    fi
    cmd+=(-e "SELECT 1;")
    err_output=$("${cmd[@]}" 2>&1)
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        echo -e "${red}MariaDB 连接失败: ${err_output}${plain}" >&2
        return 1
    fi
}

is_safe_mariadb_identifier() {
    [[ "$1" =~ ^[A-Za-z0-9_.-]+$ ]]
}

escape_sql_string() {
    printf "%s" "$1" | sed "s/'/''/g"
}

LOCAL_MARIADB_ADMIN_MODE=""
LOCAL_MARIADB_ADMIN_USER=""
LOCAL_MARIADB_ADMIN_PASS=""
LOCAL_MARIADB_ADMIN_PORT="3306"
LOCAL_MARIADB_JUST_INSTALLED="0"

try_local_mariadb_socket_admin() {
    local bin
    bin=$(mariadb_cli_bin) || return 1
    "$bin" -e "SELECT 1;" >/dev/null 2>&1 || "$bin" -uroot -e "SELECT 1;" >/dev/null 2>&1
}

ensure_local_mariadb_admin_access() {
    local port="${1:-3306}"
    local i
    LOCAL_MARIADB_ADMIN_PORT="$port"

    for ((i = 0; i < 10; i++)); do
        if try_local_mariadb_socket_admin; then
            LOCAL_MARIADB_ADMIN_MODE="socket"
            return 0
        fi
        sleep 1
    done

    if test_mariadb_server_connection "127.0.0.1" "$port" "root" ""; then
        LOCAL_MARIADB_ADMIN_MODE="password"
        LOCAL_MARIADB_ADMIN_USER="root"
        LOCAL_MARIADB_ADMIN_PASS=""
        if [[ "$LOCAL_MARIADB_JUST_INSTALLED" == "1" ]]; then
            echo -e "${green}检测到新安装 MariaDB，已自动使用 root 免密权限初始化数据库。${plain}"
        fi
        return 0
    fi

    local admin_user admin_pass
    echo -e "${yellow}无法通过 root socket 直接连接本地 MariaDB，请输入管理员账号信息。${plain}"
    read -rp "MariaDB 管理员用户名 [root]: " admin_user
    admin_user="${admin_user:-root}"
    read -rsp "MariaDB 管理员密码（可留空）: " admin_pass
    echo

    if ! test_mariadb_server_connection "127.0.0.1" "$port" "$admin_user" "$admin_pass"; then
        echo -e "${red}管理员账号连接失败${plain}"
        return 1
    fi

    LOCAL_MARIADB_ADMIN_MODE="password"
    LOCAL_MARIADB_ADMIN_USER="$admin_user"
    LOCAL_MARIADB_ADMIN_PASS="$admin_pass"
}

run_local_mariadb_admin_sql() {
    local sql="$1"
    local bin
    local -a cmd
    bin=$(mariadb_cli_bin) || return 1

    case "$LOCAL_MARIADB_ADMIN_MODE" in
    socket)
        "$bin" -e "$sql" >/dev/null 2>&1 || "$bin" -uroot -e "$sql" >/dev/null 2>&1
        ;;
    password)
        cmd=("$bin" -h "127.0.0.1" -P "$LOCAL_MARIADB_ADMIN_PORT" -u "$LOCAL_MARIADB_ADMIN_USER")
        if [[ -n "$LOCAL_MARIADB_ADMIN_PASS" ]]; then
            cmd+=("-p$LOCAL_MARIADB_ADMIN_PASS")
        fi
        cmd+=(-e "$sql")
        "${cmd[@]}" >/dev/null 2>&1
        ;;
    *)
        return 1
        ;;
    esac
}

run_local_mariadb_admin_query() {
    local sql="$1"
    local bin
    local -a cmd
    bin=$(mariadb_cli_bin) || return 1

    case "$LOCAL_MARIADB_ADMIN_MODE" in
    socket)
        "$bin" -N -B -e "$sql" 2>/dev/null || "$bin" -uroot -N -B -e "$sql" 2>/dev/null
        ;;
    password)
        cmd=("$bin" -h "127.0.0.1" -P "$LOCAL_MARIADB_ADMIN_PORT" -u "$LOCAL_MARIADB_ADMIN_USER")
        if [[ -n "$LOCAL_MARIADB_ADMIN_PASS" ]]; then
            cmd+=("-p$LOCAL_MARIADB_ADMIN_PASS")
        fi
        cmd+=(-N -B -e "$sql")
        "${cmd[@]}" 2>/dev/null
        ;;
    *)
        return 1
        ;;
    esac
}

ensure_mariadb_database_and_user() {
    local dbname="$1" dbuser="$2" dbpass="$3"
    local escaped_pass
    local sql=""
    local account_host=""

    if ! is_safe_mariadb_identifier "$dbname"; then
        echo -e "${red}业务数据库名仅支持字母、数字、点、下划线和连字符${plain}"
        return 1
    fi
    if ! is_safe_mariadb_identifier "$dbuser"; then
        echo -e "${red}业务用户名仅支持字母、数字、点、下划线和连字符${plain}"
        return 1
    fi

    escaped_pass=$(escape_sql_string "$dbpass")
    sql="CREATE DATABASE IF NOT EXISTS \`${dbname}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

    for account_host in "localhost" "127.0.0.1" "::1"; do
        sql="${sql} CREATE USER IF NOT EXISTS '${dbuser}'@'${account_host}' IDENTIFIED BY '${escaped_pass}';"
        sql="${sql} ALTER USER '${dbuser}'@'${account_host}' IDENTIFIED BY '${escaped_pass}';"
        sql="${sql} GRANT ALL PRIVILEGES ON \`${dbname}\`.* TO '${dbuser}'@'${account_host}';"
    done
    sql="${sql} FLUSH PRIVILEGES;"

    echo -e "${green}正在确保本地 MariaDB 的业务库和业务账号存在...${plain}"
    run_local_mariadb_admin_sql "$sql"
}

CURRENT_LOCAL_DB_PORT=""
CURRENT_LOCAL_DB_USER=""
CURRENT_LOCAL_DB_PASS=""
CURRENT_LOCAL_DB_NAME=""

load_local_mariadb_business_context() {
    local db_type=""
    local db_host=""

    db_type=$(read_json_dbtype)
    if [ "$db_type" != "mariadb" ]; then
        echo -e "${red}当前数据库类型不是 MariaDB${plain}"
        return 1
    fi

    db_host=$(get_database_setting '.dbHost' '127.0.0.1')
    if ! is_local_mariadb_host "$db_host"; then
        echo -e "${red}当前面板使用的是远程 MariaDB，无法管理本机 MariaDB 远程访问${plain}"
        return 1
    fi

    CURRENT_LOCAL_DB_PORT=$(get_database_setting '.dbPort' '3306')
    CURRENT_LOCAL_DB_USER=$(get_database_setting '.dbUser' '')
    CURRENT_LOCAL_DB_PASS=$(get_database_setting '.dbPassword' '')
    CURRENT_LOCAL_DB_NAME=$(get_database_setting '.dbName' '3xui')

    if ! validate_tcp_port "$CURRENT_LOCAL_DB_PORT"; then
        echo -e "${red}当前配置中的本地 MariaDB 端口无效${plain}"
        return 1
    fi
    if [ -z "$CURRENT_LOCAL_DB_USER" ]; then
        echo -e "${red}当前配置缺少 MariaDB 业务用户名${plain}"
        return 1
    fi
    if [ -z "$CURRENT_LOCAL_DB_NAME" ]; then
        echo -e "${red}当前配置缺少 MariaDB 业务数据库名${plain}"
        return 1
    fi
    if ! is_safe_mariadb_identifier "$CURRENT_LOCAL_DB_USER"; then
        echo -e "${red}当前配置中的 MariaDB 业务用户名不支持自动远程授权${plain}"
        return 1
    fi
    if ! is_safe_mariadb_identifier "$CURRENT_LOCAL_DB_NAME"; then
        echo -e "${red}当前配置中的 MariaDB 业务数据库名不支持自动远程授权${plain}"
        return 1
    fi

    ensure_local_mariadb_ready || return 1
    ensure_local_mariadb_admin_access "$CURRENT_LOCAL_DB_PORT" || return 1
    return 0
}

list_remote_mariadb_hosts() {
    local escaped_user=""
    escaped_user=$(escape_sql_string "$CURRENT_LOCAL_DB_USER")
    run_local_mariadb_admin_query "SELECT Host FROM mysql.user WHERE User = '${escaped_user}' AND Host NOT IN ('localhost', '127.0.0.1', '::1') ORDER BY Host;"
}

show_mariadb_remote_access_status() {
    local bind_address=""
    local server_port=""
    local hosts=""

    load_local_mariadb_business_context || return 1

    bind_address=$(read_mariadb_server_option "bind-address" "127.0.0.1")
    server_port=$(read_mariadb_server_option "port" "$CURRENT_LOCAL_DB_PORT")
    hosts=$(list_remote_mariadb_hosts | sed '/^$/d')

    echo -e "${green}MariaDB 服务端口: ${server_port}${plain}"
    echo -e "${green}MariaDB 监听地址: ${bind_address}${plain}"
    if [ "$bind_address" = "127.0.0.1" ]; then
        echo -e "${yellow}远程访问状态: 已关闭${plain}"
    else
        echo -e "${yellow}远程访问状态: 已开启${plain}"
    fi
    if [ -n "$hosts" ]; then
        echo -e "${green}允许的远程 IP:${plain}"
        printf '%s\n' "$hosts"
    else
        echo -e "${yellow}允许的远程 IP: <empty>${plain}"
    fi
}

add_mariadb_remote_ip_grant() {
    local remote_ip="$1"
    local escaped_user=""
    local escaped_pass=""
    local escaped_ip=""

    if ! is_ip "$remote_ip"; then
        echo -e "${red}仅支持输入单个 IP 地址${plain}"
        return 1
    fi

    load_local_mariadb_business_context || return 1
    escaped_user=$(escape_sql_string "$CURRENT_LOCAL_DB_USER")
    escaped_pass=$(escape_sql_string "$CURRENT_LOCAL_DB_PASS")
    escaped_ip=$(escape_sql_string "$remote_ip")

    run_local_mariadb_admin_sql "CREATE USER IF NOT EXISTS '${escaped_user}'@'${escaped_ip}' IDENTIFIED BY '${escaped_pass}'; ALTER USER '${escaped_user}'@'${escaped_ip}' IDENTIFIED BY '${escaped_pass}'; GRANT ALL PRIVILEGES ON \`${CURRENT_LOCAL_DB_NAME}\`.* TO '${escaped_user}'@'${escaped_ip}'; FLUSH PRIVILEGES;"
}

remove_mariadb_remote_ip_grant() {
    local remote_ip="$1"
    local escaped_user=""
    local escaped_ip=""

    if ! is_ip "$remote_ip"; then
        echo -e "${red}仅支持输入单个 IP 地址${plain}"
        return 1
    fi

    load_local_mariadb_business_context || return 1
    escaped_user=$(escape_sql_string "$CURRENT_LOCAL_DB_USER")
    escaped_ip=$(escape_sql_string "$remote_ip")

    run_local_mariadb_admin_sql "DROP USER IF EXISTS '${escaped_user}'@'${escaped_ip}'; FLUSH PRIVILEGES;"
}

set_local_mariadb_port() {
    local current_port=""
    local current_bind=""
    local new_port=""

    load_local_mariadb_business_context || return 1

    current_port=$(read_mariadb_server_option "port" "$CURRENT_LOCAL_DB_PORT")
    current_bind=$(read_mariadb_server_option "bind-address" "127.0.0.1")
    read -rp "本地 MariaDB port [${current_port:-3306}]: " new_port
    new_port="${new_port// /}"
    new_port="${new_port:-$current_port}"

    if ! validate_tcp_port "$new_port"; then
        echo -e "${red}本地 MariaDB 端口无效，请输入 1-65535 之间的数字${plain}"
        return 1
    fi
    if ! configure_local_mariadb_server_network "$new_port" "$current_bind"; then
        return 1
    fi
    if ! ${xui_folder}/x-ui setting -dbPort "$new_port" >/dev/null 2>&1; then
        echo -e "${red}写入面板数据库端口配置失败${plain}"
        return 1
    fi

    echo -e "${yellow}本地 MariaDB 端口已更新为 ${new_port}，建议重启面板使其完全生效。${plain}"
}

enable_mariadb_remote_access() {
    local hosts=""

    load_local_mariadb_business_context || return 1
    hosts=$(list_remote_mariadb_hosts | sed '/^$/d')
    if [ -z "$hosts" ]; then
        echo -e "${red}请先添加至少一个允许的远程 IP${plain}"
        return 1
    fi
    if ! configure_local_mariadb_server_network "$CURRENT_LOCAL_DB_PORT" "0.0.0.0"; then
        return 1
    fi

    echo -e "${yellow}MariaDB 远程访问已开启，仅已授权的远程 IP 可连接。${plain}"
}

disable_mariadb_remote_access() {
    local hosts=""
    local host=""

    load_local_mariadb_business_context || return 1
    hosts=$(list_remote_mariadb_hosts | sed '/^$/d')
    if [ -n "$hosts" ]; then
        while IFS= read -r host; do
            [ -n "$host" ] || continue
            remove_mariadb_remote_ip_grant "$host" >/dev/null 2>&1 || true
        done <<<"$hosts"
    fi
    if ! configure_local_mariadb_server_network "$CURRENT_LOCAL_DB_PORT" "127.0.0.1"; then
        return 1
    fi

    echo -e "${yellow}MariaDB 远程访问已关闭，并已清理远程 IP 授权。${plain}"
}

show_mariadb_remote_ips() {
    local hosts=""

    load_local_mariadb_business_context || return 1
    hosts=$(list_remote_mariadb_hosts | sed '/^$/d')
    if [ -z "$hosts" ]; then
        echo -e "${yellow}当前没有已授权的远程 IP${plain}"
        return 0
    fi

    echo -e "${green}当前已授权的远程 IP:${plain}"
    printf '%s\n' "$hosts"
}

add_mariadb_remote_ip() {
    local remote_ip=""

    read -rp "输入允许连接 MariaDB 的远程 IP: " remote_ip
    remote_ip="${remote_ip// /}"
    if ! add_mariadb_remote_ip_grant "$remote_ip"; then
        echo -e "${red}添加 MariaDB 允许 IP 失败${plain}"
        return 1
    fi

    echo -e "${yellow}已添加 MariaDB 允许 IP: ${remote_ip}${plain}"
}

remove_mariadb_remote_ip() {
    local remote_ip=""

    read -rp "输入要移除的远程 IP: " remote_ip
    remote_ip="${remote_ip// /}"
    if ! remove_mariadb_remote_ip_grant "$remote_ip"; then
        echo -e "${red}移除 MariaDB 允许 IP 失败${plain}"
        return 1
    fi

    echo -e "${yellow}已移除 MariaDB 允许 IP: ${remote_ip}${plain}"
}

# Switch to MariaDB
db_switch_to_mariadb() {
    local current_type=$(read_json_dbtype)
    if [ "$current_type" = "mariadb" ]; then
        echo -e "${yellow}当前已经是 MariaDB${plain}"
        db_menu
        return
    fi

    local mariadb_mode_choice mariadb_mode
    local db_host db_port db_user db_pass db_name

    read -rp "MariaDB 部署位置 [1=本地 MariaDB, 2=远程 MariaDB，默认 1]: " mariadb_mode_choice
    case "${mariadb_mode_choice:-1}" in
    2)
        mariadb_mode="remote"
        ;;
    *)
        mariadb_mode="local"
        ;;
    esac

    if [[ "${mariadb_mode}" == "remote" ]]; then
        ensure_mariadb_client_ready || {
            echo -e "${yellow}已取消安装 MariaDB 客户端，返回数据库菜单${plain}"
            db_menu
            return
        }

        echo -e "${green}请输入远程 MariaDB 业务连接信息（直接回车使用默认值）：${plain}"
        read -rp "远程 MariaDB host [127.0.0.1]: " db_host
        read -rp "远程 MariaDB port [3306]: " db_port
        read -rp "业务数据库名 [3xui]: " db_name
        read -rp "业务用户名: " db_user
        read -rsp "业务密码: " db_pass
        echo

        db_host=${db_host:-127.0.0.1}
        db_port=${db_port:-3306}
        db_name=${db_name:-3xui}
        while ! [[ "${db_port}" =~ ^[0-9]+$ ]] || ((db_port < 1 || db_port > 65535)); do
            echo -e "${red}远程 MariaDB 端口无效，请输入 1-65535 之间的数字${plain}"
            read -rp "远程 MariaDB port [3306]: " db_port
            db_port=${db_port:-3306}
        done
        if [[ -z "$db_user" || -z "$db_pass" ]]; then
            echo -e "${red}业务用户名和业务密码不能为空${plain}"
            db_menu
            return
        fi

        echo -e "${green}正在验证远程 MariaDB 业务连接...${plain}"
        if ! test_mariadb_database_connection "$db_host" "$db_port" "$db_name" "$db_user" "$db_pass"; then
            echo -e "${red}无法使用输入的远程 MariaDB 信息连接到业务数据库${plain}"
            db_menu
            return
        fi
    else
        db_host="127.0.0.1"
        read -rp "本地 MariaDB port [3306]: " db_port
        read -rp "业务数据库名 [3xui]: " db_name
        read -rp "业务用户名: " db_user
        read -rsp "业务密码: " db_pass
        echo

        db_port=${db_port:-3306}
        db_name=${db_name:-3xui}
        if ! validate_tcp_port "$db_port"; then
            echo -e "${red}本地 MariaDB 端口无效，请输入 1-65535 之间的数字${plain}"
            db_menu
            return
        fi
        if [[ -z "$db_user" || -z "$db_pass" ]]; then
            echo -e "${red}业务用户名和业务密码不能为空${plain}"
            db_menu
            return
        fi

        ensure_local_mariadb_ready || {
            echo -e "${yellow}本地 MariaDB 未准备完成，返回数据库菜单${plain}"
            db_menu
            return
        }
        configure_local_mariadb_server_network "$db_port" "127.0.0.1" || {
            db_menu
            return
        }
        ensure_local_mariadb_admin_access "$db_port" || {
            db_menu
            return
        }
        ensure_mariadb_database_and_user "$db_name" "$db_user" "$db_pass" || {
            db_menu
            return
        }

        echo -e "${green}正在验证本地 MariaDB 业务连接...${plain}"
        if ! test_mariadb_database_connection "$db_host" "$db_port" "$db_name" "$db_user" "$db_pass"; then
            echo -e "${red}无法使用创建后的本地 MariaDB 业务账号连接数据库${plain}"
            db_menu
            return
        fi
    fi

    echo -e "${green}正在配置 MariaDB 连接...${plain}"
    XUI_DB_PASSWORD="$db_pass" ${xui_folder}/x-ui setting -dbHost "$db_host" -dbPort "$db_port" -dbUser "$db_user" -dbName "$db_name" >/dev/null 2>&1

    echo -e "${green}正在迁移数据从 SQLite 到 MariaDB...${plain}"
    ${xui_folder}/x-ui migrate-db -direction sqlite-to-mariadb

    if [ $? -eq 0 ]; then
        echo -e "${green}数据库切换成功，正在重启面板...${plain}"
        ${xui_folder}/x-ui setting -dbType mariadb >/dev/null 2>&1
        restart
    else
        echo -e "${red}数据迁移失败，保持 SQLite 不变${plain}"
        restart
    fi
}

# Switch to SQLite
db_switch_to_sqlite() {
    local current_type=$(read_json_dbtype)
    if [ "$current_type" = "sqlite" ]; then
        echo -e "${yellow}当前已经是 SQLite${plain}"
        db_menu
        return
    fi

    echo -e "${green}正在迁移数据从 MariaDB 到 SQLite...${plain}"
    ${xui_folder}/x-ui migrate-db -direction mariadb-to-sqlite

    if [ $? -eq 0 ]; then
        echo -e "${green}数据库切换成功，正在重启面板...${plain}"
        ${xui_folder}/x-ui setting -dbType sqlite >/dev/null 2>&1
        restart
    else
        echo -e "${red}数据迁移失败，保持 MariaDB 不变${plain}"
        db_menu
    fi
}

# Database management menu
backup_db() {
    echo -e "${green}Creating database backup...${plain}"
    local xui_bin="${xui_folder}/x-ui"
    if [[ ! -x "$xui_bin" ]]; then
        echo -e "${red}x-ui binary not found at $xui_bin${plain}"
        return 1
    fi
    "$xui_bin" backup
}

restore_db() {
    local backup_file="$1"
    if [[ -z "$backup_file" ]]; then
        echo -e "${red}Usage: x-ui restore <backup-filename>${plain}"
        list_backups
        return 1
    fi
    local backup_dir="/etc/x-ui/backups"
    local full_path="${backup_dir}/${backup_file}"
    if [[ ! -f "$full_path" ]]; then
        echo -e "${red}Backup file not found: $full_path${plain}"
        list_backups
        return 1
    fi
    echo -e "${yellow}WARNING: Restore will stop the panel and replace the database.${plain}"
    read -p "Continue? (y/n) " confirm
    if [[ "$confirm" != "y" ]]; then
        echo "Cancelled."
        return 0
    fi
    echo "Stopping panel..."
    stop
    echo "Restoring from $backup_file..."
    local xui_bin="${xui_folder}/x-ui"
    "$xui_bin" restore --file="$backup_file"
    echo "Starting panel..."
    start
    echo -e "${green}Restore completed.${plain}"
}

list_backups() {
    local backup_dir="/etc/x-ui/backups"
    if [[ ! -d "$backup_dir" ]]; then
        echo "No backups found."
        return 0
    fi
    local count=$(ls -1 "$backup_dir" 2>/dev/null | wc -l)
    if [[ "$count" -eq 0 ]]; then
        echo "No backups found."
        return 0
    fi
    echo -e "${green}Backups in ${backup_dir}:${plain}"
    ls -lh "$backup_dir" | awk 'NR>1 {print $5, $6, $7, $8, $9}'
}

db_menu() {
    local current_type=$(read_json_dbtype)

    echo -e "
╔────────────────────────────────────────────────╗
│   ${green}数据库管理${plain}                                    │
│────────────────────────────────────────────────│
│   ${green}0.${plain} 返回主菜单                                │
│   ${green}1.${plain} 查看当前数据库类型（当前: ${current_type}）   │
│   ${green}2.${plain} 切换到 MariaDB                             │
│   ${green}3.${plain} 切换到 SQLite                               │
│   ${green}4.${plain} 查看当前节点设置                            │
│   ${green}5.${plain} 设置节点角色                                │
│   ${green}6.${plain} 设置节点 ID                                 │
│   ${green}7.${plain} 设置同步间隔                                │
│   ${green}8.${plain} 设置流量回刷间隔                            │
│   ${green}9.${plain} 设置远程数据库连接                          │
│  ${green}10.${plain} 设置本地 MariaDB 端口                       │
│  ${green}11.${plain} 查看 MariaDB 远程访问状态                   │
│  ${green}12.${plain} 开启 MariaDB 远程访问                       │
│  ${green}13.${plain} 关闭 MariaDB 远程访问                       │
│  ${green}14.${plain} 查看 MariaDB 允许 IP                        │
│  ${green}15.${plain} 添加 MariaDB 允许 IP                        │
│  ${green}16.${plain} 移除 MariaDB 允许 IP                        │
│  ${green}17.${plain} 创建数据库备份                              │
│  ${green}18.${plain} 从备份恢复数据库                            │
│  ${green}19.${plain} 列出所有备份                                │
╚════════════════════════════════════════════════╝
"
    read -rp "请输入选择 [0-19]：" num
    case "${num}" in
    0)
        show_menu
        ;;
    1)
        db_show_status
        db_menu
        ;;
    2)
        db_switch_to_mariadb
        ;;
    3)
        db_switch_to_sqlite
        ;;
    4)
        show_node_status
        db_menu
        ;;
    5)
        set_node_role
        db_menu
        ;;
    6)
        set_node_id
        db_menu
        ;;
    7)
        set_sync_interval
        db_menu
        ;;
    8)
        set_traffic_flush_interval
        db_menu
        ;;
    9)
        set_remote_database_connection
        db_menu
        ;;
    10)
        set_local_mariadb_port
        db_menu
        ;;
    11)
        show_mariadb_remote_access_status
        db_menu
        ;;
    12)
        enable_mariadb_remote_access
        db_menu
        ;;
    13)
        disable_mariadb_remote_access
        db_menu
        ;;
    14)
        show_mariadb_remote_ips
        db_menu
        ;;
    15)
        add_mariadb_remote_ip
        db_menu
        ;;
    16)
        remove_mariadb_remote_ip
        db_menu
        ;;
    17)
        backup_db
        db_menu
        ;;
    18)
        list_backups
        echo ""
        read -p "Enter backup filename to restore: " restore_filename
        restore_db "$restore_filename"
        db_menu
        ;;
    19)
        list_backups
        db_menu
        ;;
    *)
        echo -e "${red}无效选项，请选择有效数字。${plain}\n"
        db_menu
        ;;
    esac
}

show_menu() {
    echo -e "
╔────────────────────────────────────────────────╗
│   ${green}3X-UI 面板管理脚本${plain}                          │
│   ${green}0.${plain} 退出脚本                                 │
│────────────────────────────────────────────────│
│   ${green}1.${plain} 安装                                      │
│   ${green}2.${plain} 更新                                      │
│   ${green}3.${plain} 更新菜单                                  │
│   ${green}4.${plain} 安装旧版本                                │
│   ${green}5.${plain} 卸载                                      │
│────────────────────────────────────────────────│
│   ${green}6.${plain} 重置用户名和密码                          │
│   ${green}7.${plain} 重置 Web 路径                             │
│   ${green}8.${plain} 重置设置                                  │
│   ${green}9.${plain} 修改端口                                  │
│  ${green}10.${plain} 查看当前设置                              │
│────────────────────────────────────────────────│
│  ${green}11.${plain} 启动                                      │
│  ${green}12.${plain} 停止                                      │
│  ${green}13.${plain} 重启                                      │
|  ${green}14.${plain} 重启 Xray                                 │
│  ${green}15.${plain} 查看状态                                  │
│  ${green}16.${plain} 日志管理                                  │
│────────────────────────────────────────────────│
│  ${green}17.${plain} 设置开机自启                              │
│  ${green}18.${plain} 取消开机自启                              │
│────────────────────────────────────────────────│
│  ${green}19.${plain} SSL 证书管理                              │
│  ${green}20.${plain} Cloudflare SSL 证书                       │
│  ${green}21.${plain} IP 限制管理                               │
│  ${green}22.${plain} 防火墙管理                                │
│  ${green}23.${plain} SSH 端口转发管理                          │
│────────────────────────────────────────────────│
│  ${green}24.${plain} BBR 管理                                  │
│  ${green}25.${plain} 更新 Geo 文件                             │
│  ${green}26.${plain} 网速测试 (Speedtest)                      │
│────────────────────────────────────────────────│
│  ${green}27.${plain} 数据库管理                                │
╚────────────────────────────────────────────────╝
"
    show_status
    echo && read -rp "请输入选择 [0-27]：" num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && update_menu
        ;;
    4)
        check_install && legacy_version
        ;;
    5)
        check_install && uninstall
        ;;
    6)
        check_install && reset_user
        ;;
    7)
        check_install && reset_webbasepath
        ;;
    8)
        check_install && reset_config
        ;;
    9)
        check_install && set_port
        ;;
    10)
        check_install && check_config
        ;;
    11)
        check_install && start
        ;;
    12)
        check_install && stop
        ;;
    13)
        check_install && restart
        ;;
    14)
        check_install && restart_xray
        ;;
    15)
        check_install && status
        ;;
    16)
        check_install && show_log
        ;;
    17)
        check_install && enable
        ;;
    18)
        check_install && disable
        ;;
    19)
        ssl_cert_issue_main
        ;;
    20)
        ssl_cert_issue_CF
        ;;
    21)
        iplimit_main
        ;;
    22)
        firewall_menu
        ;;
    23)
        SSH_port_forwarding
        ;;
    24)
        bbr_menu
        ;;
    25)
        update_geo
        ;;
    26)
        run_speedtest
        ;;
    27)
        check_install && db_menu
        ;;
    *)
        LOGE "请输入正确的数字 [0-27]"
        ;;
    esac
}

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start 0
        ;;
    "stop")
        check_install 0 && stop 0
        ;;
    "restart")
        check_install 0 && restart 0
        ;;
    "restart-xray")
        check_install 0 && restart_xray 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "settings")
        check_install 0 && check_config 0
        ;;
    "enable")
        check_install 0 && enable 0
        ;;
    "disable")
        check_install 0 && disable 0
        ;;
    "log")
        check_install 0 && show_log 0
        ;;
    "banlog")
        check_install 0 && show_banlog 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "legacy")
        check_install 0 && legacy_version 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    "update-all-geofiles")
        check_install 0 && update_all_geofiles 0 && restart 0
        ;;
    "geofile-cron")
        shift
        case "${1}" in
            "--enable") geofile_cron_enable "${@}";;
            "--disable") geofile_cron_disable;;
            "--status") geofile_cron_status;;
            *) echo -e "${red}Usage: x-ui geofile-cron [--enable --frequency daily --hour 4 | --disable | --status]${plain}";;
        esac
        ;;
    "backup")
        check_install 0 && backup_db
        ;;
    "restore")
        check_install 0 && restore_db "${2}"
        ;;
    "list-backups")
        check_install 0 && list_backups
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
