#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
blue='\033[0;34m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

xui_folder="${XUI_MAIN_FOLDER:=/usr/local/x-ui}"
xui_service="${XUI_SERVICE:=/etc/systemd/system}"

# 检查 root 权限
[[ $EUID -ne 0 ]] && echo -e "${red}错误：${plain} 请使用 root 权限运行此脚本 \n " && exit 1

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

arch() {
    case "$(uname -m)" in
        x86_64 | x64 | amd64) echo 'amd64' ;;
        i*86 | x86) echo '386' ;;
        armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
        armv7* | armv7 | arm) echo 'armv7' ;;
        armv6* | armv6) echo 'armv6' ;;
        armv5* | armv5) echo 'armv5' ;;
        s390x) echo 's390x' ;;
        *) echo -e "${green}不支持的 CPU 架构！${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "架构：$(arch)"

# 基本辅助函数
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

# 端口检测辅助函数
is_port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -ltn 2>/dev/null | awk -v p=":${port}$" '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v netstat >/dev/null 2>&1; then
        netstat -lnt 2>/dev/null | awk -v p=":${port} " '$4 ~ p {exit 0} END {exit 1}'
        return
    fi
    if command -v lsof >/dev/null 2>&1; then
        lsof -nP -iTCP:${port} -sTCP:LISTEN >/dev/null 2>&1 && return 0
    fi
    return 1
}

install_base() {
    case "${release}" in
        ubuntu | debian | armbian)
            apt-get update && apt-get install -y -q cron curl tar tzdata socat ca-certificates openssl
        ;;
        fedora | amzn | virtuozzo | rhel | almalinux | rocky | ol)
            dnf -y update && dnf install -y -q curl tar tzdata socat ca-certificates openssl
        ;;
        centos)
            if [[ "${VERSION_ID}" =~ ^7 ]]; then
                yum -y update && yum install -y curl tar tzdata socat ca-certificates openssl
            else
                dnf -y update && dnf install -y -q curl tar tzdata socat ca-certificates openssl
            fi
        ;;
        arch | manjaro | parch)
            pacman -Syu && pacman -Syu --noconfirm curl tar tzdata socat ca-certificates openssl
        ;;
        opensuse-tumbleweed | opensuse-leap)
            zypper refresh && zypper -q install -y curl tar timezone socat ca-certificates openssl
        ;;
        alpine)
            apk update && apk add curl tar tzdata socat ca-certificates openssl
        ;;
        *)
            apt-get update && apt-get install -y -q curl tar tzdata socat ca-certificates openssl
        ;;
    esac
}

gen_random_string() {
    local length="$1"
    openssl rand -base64 $(( length * 2 )) \
        | tr -dc 'a-zA-Z0-9' \
        | head -c "$length"
}

install_acme() {
    echo -e "${green}正在安装 acme.sh 用于 SSL 证书管理...${plain}"
    cd ~ || return 1
    curl -s https://get.acme.sh | sh >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${red}安装 acme.sh 失败${plain}"
        return 1
    else
        echo -e "${green}acme.sh 安装成功${plain}"
    fi
    return 0
}

setup_ssl_certificate() {
    local domain="$1"
    local server_ip="$2"
    local existing_port="$3"
    local existing_webBasePath="$4"

    echo -e "${green}正在配置 SSL 证书...${plain}"

    # 检查 acme.sh 是否已安装
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${yellow}安装 acme.sh 失败，跳过 SSL 配置${plain}"
            return 1
        fi
    fi

    # 创建证书目录
    local certPath="/root/cert/${domain}"
    mkdir -p "$certPath"

    # 签发证书
    echo -e "${green}正在为 ${domain} 签发 SSL 证书...${plain}"
    echo -e "${yellow}注意：80 端口必须开放且可从外网访问${plain}"

    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force >/dev/null 2>&1
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport 80 --force

    if [ $? -ne 0 ]; then
        echo -e "${yellow}为 ${domain} 签发证书失败${plain}"
        echo -e "${yellow}请确保 80 端口已开放，稍后可通过以下命令重试：x-ui${plain}"
        rm -rf ~/.acme.sh/${domain} 2>/dev/null
        rm -rf "$certPath" 2>/dev/null
        return 1
    fi

    # 安装证书
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem \
        --reloadcmd "systemctl restart x-ui" >/dev/null 2>&1

    if [ $? -ne 0 ]; then
        echo -e "${yellow}安装证书失败${plain}"
        return 1
    fi

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1
    # 安全权限：私钥仅所有者可读
    chmod 600 $certPath/privkey.pem 2>/dev/null
    chmod 644 $certPath/fullchain.pem 2>/dev/null

    # 为面板设置证书
    local webCertFile="/root/cert/${domain}/fullchain.pem"
    local webKeyFile="/root/cert/${domain}/privkey.pem"

    if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
        ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" >/dev/null 2>&1
        echo -e "${green}SSL 证书安装并配置成功！${plain}"
        return 0
    else
        echo -e "${yellow}未找到证书文件${plain}"
        return 1
    fi
}

# 签发 Let's Encrypt IP 证书（短期配置文件，约 6 天有效期）
# 需要 acme.sh 且 80 端口开放用于 HTTP-01 验证
setup_ip_certificate() {
    local ipv4="$1"
    local ipv6="$2"  # 可选

    echo -e "${green}正在配置 Let's Encrypt IP 证书（短期配置文件）...${plain}"
    echo -e "${yellow}注意：IP 证书有效期约 6 天，将自动续期。${plain}"
    echo -e "${yellow}默认监听 80 端口。如选择其他端口，请确保外部 80 端口转发到该端口。${plain}"

    # 检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        install_acme
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        fi
    fi

    # 验证 IP 地址
    if [[ -z "$ipv4" ]]; then
        echo -e "${red}需要提供 IPv4 地址${plain}"
        return 1
    fi

    if ! is_ipv4 "$ipv4"; then
        echo -e "${red}无效的 IPv4 地址：$ipv4${plain}"
        return 1
    fi

    # 创建证书目录
    local certDir="/root/cert/ip"
    mkdir -p "$certDir"

    # 构建域名参数
    local domain_args="-d ${ipv4}"
    if [[ -n "$ipv6" ]] && is_ipv6 "$ipv6"; then
        domain_args="${domain_args} -d ${ipv6}"
        echo -e "${green}包含 IPv6 地址：${ipv6}${plain}"
    fi

    # 设置重载命令用于自动续期（添加 || true 以防首次安装时失败）
    local reloadCmd="systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null || true"

    # 选择 HTTP-01 监听端口（默认 80，可自定义）
    local WebPort=""
    read -rp "用于 ACME HTTP-01 监听的端口（默认 80）：" WebPort
    WebPort="${WebPort:-80}"
    if ! [[ "${WebPort}" =~ ^[0-9]+$ ]] || ((WebPort < 1 || WebPort > 65535)); then
        echo -e "${red}无效端口，回退到 80。${plain}"
        WebPort=80
    fi
    echo -e "${green}使用端口 ${WebPort} 进行独立验证。${plain}"
    if [[ "${WebPort}" -ne 80 ]]; then
        echo -e "${yellow}提醒：Let's Encrypt 仍然连接 80 端口；请将外部 80 端口转发到 ${WebPort}。${plain}"
    fi

    # 确保所选端口可用
    while true; do
        if is_port_in_use "${WebPort}"; then
            echo -e "${yellow}端口 ${WebPort} 已被占用。${plain}"

            local alt_port=""
            read -rp "请输入另一个端口供 acme.sh 独立监听（留空取消）：" alt_port
            alt_port="${alt_port// /}"
            if [[ -z "${alt_port}" ]]; then
                echo -e "${red}端口 ${WebPort} 被占用，无法继续。${plain}"
                return 1
            fi
            if ! [[ "${alt_port}" =~ ^[0-9]+$ ]] || ((alt_port < 1 || alt_port > 65535)); then
                echo -e "${red}无效端口。${plain}"
                return 1
            fi
            WebPort="${alt_port}"
            continue
        else
            echo -e "${green}端口 ${WebPort} 空闲，可以进行独立验证。${plain}"
            break
        fi
    done

    # 使用短期配置文件签发证书
    echo -e "${green}正在为 ${ipv4} 签发 IP 证书...${plain}"
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force >/dev/null 2>&1

    ~/.acme.sh/acme.sh --issue \
        ${domain_args} \
        --standalone \
        --server letsencrypt \
        --certificate-profile shortlived \
        --days 6 \
        --httpport ${WebPort} \
        --force

    if [ $? -ne 0 ]; then
        echo -e "${red}签发 IP 证书失败${plain}"
        echo -e "${yellow}请确保端口 ${WebPort} 可达（或从外部 80 端口转发）${plain}"
        # 清理 acme.sh 数据（IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${ipv4} 2>/dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2>/dev/null
        rm -rf ${certDir} 2>/dev/null
        return 1
    fi

    echo -e "${green}证书签发成功，正在安装...${plain}"

    # 安装证书
    # 注意：acme.sh 可能在 reloadcmd 失败时报告 "Reload error" 并返回非零退出码，
    # 但证书文件仍然已安装。我们通过检查文件而非退出码来判断。
    ~/.acme.sh/acme.sh --installcert -d ${ipv4} \
        --key-file "${certDir}/privkey.pem" \
        --fullchain-file "${certDir}/fullchain.pem" \
        --reloadcmd "${reloadCmd}" 2>&1 || true

    # 验证证书文件存在（不依赖退出码 - reloadcmd 失败会导致非零）
    if [[ ! -f "${certDir}/fullchain.pem" || ! -f "${certDir}/privkey.pem" ]]; then
        echo -e "${red}安装后未找到证书文件${plain}"
        # 清理 acme.sh 数据（IPv4 和 IPv6）
        rm -rf ~/.acme.sh/${ipv4} 2>/dev/null
        [[ -n "$ipv6" ]] && rm -rf ~/.acme.sh/${ipv6} 2>/dev/null
        rm -rf ${certDir} 2>/dev/null
        return 1
    fi

    echo -e "${green}证书文件安装成功${plain}"

    # 启用 acme.sh 自动升级（确保 cron 任务运行）
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1

    # 安全权限：私钥仅所有者可读
    chmod 600 ${certDir}/privkey.pem 2>/dev/null
    chmod 644 ${certDir}/fullchain.pem 2>/dev/null

    # 为面板配置证书路径
    echo -e "${green}正在为面板设置证书路径...${plain}"
    ${xui_folder}/x-ui cert -webCert "${certDir}/fullchain.pem" -webCertKey "${certDir}/privkey.pem"

    if [ $? -ne 0 ]; then
        echo -e "${yellow}警告：无法自动设置证书路径${plain}"
        echo -e "${yellow}证书文件位于：${plain}"
        echo -e "  证书：${certDir}/fullchain.pem"
        echo -e "  密钥：${certDir}/privkey.pem"
    else
        echo -e "${green}证书路径配置成功${plain}"
    fi

    echo -e "${green}IP 证书安装并配置成功！${plain}"
    echo -e "${green}证书有效期约 6 天，通过 acme.sh cron 任务自动续期。${plain}"
    echo -e "${yellow}acme.sh 将在证书到期前自动续期并重载 x-ui。${plain}"
    return 0
}

# 综合手动 SSL 证书签发（通过 acme.sh）
ssl_cert_issue() {
    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep 'webBasePath:' | awk -F': ' '{print $2}' | tr -d '[:space:]' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep 'port:' | awk -F': ' '{print $2}' | tr -d '[:space:]')

    # 检查 acme.sh
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "未找到 acme.sh，正在安装..."
        cd ~ || return 1
        curl -s https://get.acme.sh | sh
        if [ $? -ne 0 ]; then
            echo -e "${red}安装 acme.sh 失败${plain}"
            return 1
        else
            echo -e "${green}acme.sh 安装成功${plain}"
        fi
    fi

    # 获取域名并验证
    local domain=""
    while true; do
        read -rp "请输入您的域名：" domain
        domain="${domain// /}"  # 去除空格

        if [[ -z "$domain" ]]; then
            echo -e "${red}域名不能为空，请重试。${plain}"
            continue
        fi

        if ! is_domain "$domain"; then
            echo -e "${red}无效的域名格式：${domain}，请输入有效的域名。${plain}"
            continue
        fi

        break
    done
    echo -e "${green}您的域名是：${domain}，正在检查...${plain}"

    # 检查是否已存在证书
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')
    if [ "${currentCert}" == "${domain}" ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        echo -e "${red}系统已有该域名的证书，无法重复签发。${plain}"
        echo -e "${yellow}当前证书信息：${plain}"
        echo "$certInfo"
        return 1
    else
        echo -e "${green}您的域名已准备好签发证书...${plain}"
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
        echo -e "${yellow}输入 ${WebPort} 无效，将使用默认端口 80。${plain}"
        WebPort=80
    fi
    echo -e "${green}将使用端口：${WebPort} 签发证书。请确保此端口已开放。${plain}"

    # 临时停止面板
    echo -e "${yellow}正在临时停止面板...${plain}"
    systemctl stop x-ui 2>/dev/null || rc-service x-ui stop 2>/dev/null

    # 签发证书
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
    ~/.acme.sh/acme.sh --issue -d ${domain} --listen-v6 --standalone --httpport ${WebPort} --force
    if [ $? -ne 0 ]; then
        echo -e "${red}签发证书失败，请查看日志。${plain}"
        rm -rf ~/.acme.sh/${domain}
        systemctl start x-ui 2>/dev/null || rc-service x-ui start 2>/dev/null
        return 1
    else
        echo -e "${green}证书签发成功，正在安装证书...${plain}"
    fi

    # 设置重载命令
    reloadCmd="systemctl restart x-ui || rc-service x-ui restart"
    echo -e "${green}ACME 默认 --reloadcmd 为：${yellow}systemctl restart x-ui || rc-service x-ui restart${plain}"
    echo -e "${green}此命令将在每次签发和续期证书时运行。${plain}"
    read -rp "是否要修改 ACME 的 --reloadcmd？(y/n)：" setReloadcmd
    if [[ "$setReloadcmd" == "y" || "$setReloadcmd" == "Y" ]]; then
        echo -e "\n${green}\t1.${plain} 预设：systemctl reload nginx ; systemctl restart x-ui"
        echo -e "${green}\t2.${plain} 输入自定义命令"
        echo -e "${green}\t0.${plain} 保持默认 reloadcmd"
        read -rp "请选择：" choice
        case "$choice" in
        1)
            echo -e "${green}Reloadcmd 设为：systemctl reload nginx ; systemctl restart x-ui${plain}"
            reloadCmd="systemctl reload nginx ; systemctl restart x-ui"
            ;;
        2)
            echo -e "${yellow}建议将 x-ui restart 放在最后，这样即使其他服务失败也不会报错${plain}"
            read -rp "请输入自定义的 reloadcmd：" reloadCmd
            echo -e "${green}Reloadcmd 设为：${reloadCmd}${plain}"
            ;;
        *)
            echo -e "${green}保持默认 reloadcmd${plain}"
            ;;
        esac
    fi

    # 安装证书
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem --reloadcmd "${reloadCmd}"

    if [ $? -ne 0 ]; then
        echo -e "${red}安装证书失败，退出。${plain}"
        rm -rf ~/.acme.sh/${domain}
        systemctl start x-ui 2>/dev/null || rc-service x-ui start 2>/dev/null
        return 1
    else
        echo -e "${green}安装证书成功，正在启用自动续期...${plain}"
    fi

    # 启用自动续期
    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        echo -e "${yellow}自动续期设置有问题，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        chmod 600 $certPath/privkey.pem 2>/dev/null
        chmod 644 $certPath/fullchain.pem 2>/dev/null
    else
        echo -e "${green}自动续期设置成功，证书详情：${plain}"
        ls -lah /root/cert/${domain}/
        chmod 600 $certPath/privkey.pem 2>/dev/null
        chmod 644 $certPath/fullchain.pem 2>/dev/null
    fi

    # 证书安装成功后提示用户为面板设置证书路径
    read -rp "是否要为面板设置此证书？(y/n)：" setPanel
    if [[ "$setPanel" == "y" || "$setPanel" == "Y" ]]; then
        local webCertFile="/root/cert/${domain}/fullchain.pem"
        local webKeyFile="/root/cert/${domain}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile"
            echo -e "${green}面板证书路径已设置${plain}"
            echo -e "${green}证书文件：$webCertFile${plain}"
            echo -e "${green}私钥文件：$webKeyFile${plain}"
            echo ""
            echo -e "${green}访问地址：https://${domain}:${existing_port}/${existing_webBasePath}${plain}"
            echo -e "${yellow}面板将重启以应用 SSL 证书...${plain}"
            systemctl restart x-ui 2>/dev/null || rc-service x-ui restart 2>/dev/null
        else
            echo -e "${red}错误：未找到域名 ${domain} 的证书或私钥文件。${plain}"
        fi
    else
        echo -e "${yellow}跳过面板路径设置。${plain}"
    fi

    return 0
}

# 可复用的交互式 SSL 设置（域名或 IP）
# 设置全局 `SSL_HOST` 以供访问地址使用
prompt_and_setup_ssl() {
    local panel_port="$1"
    local web_base_path="$2"   # 预期不含前导斜杠
    local server_ip="$3"

    local ssl_choice=""

    echo -e "${yellow}选择 SSL 证书配置方式：${plain}"
    echo -e "${green}1.${plain} Let's Encrypt 域名证书（90 天有效期，自动续期）"
    echo -e "${green}2.${plain} Let's Encrypt IP 证书（6 天有效期，自动续期）"
    echo -e "${green}3.${plain} 自定义 SSL 证书（指定已有文件路径）"
    echo -e "${green}4.${plain} Cloudflare SSL 证书（通配符证书，DNS 验证）"
    echo -e "${blue}注意：${plain} 选项 1 和 2 需要开放 80 端口。选项 3 需要手动指定路径。选项 4 需要 Cloudflare API 密钥。"
    read -rp "请选择（默认 2 使用 IP）：" ssl_choice
    ssl_choice="${ssl_choice// /}"  # 去除空格

    # 如果输入为空或无效（非 1、3 或 4），默认为 2（IP 证书）
    if [[ "$ssl_choice" != "1" && "$ssl_choice" != "3" && "$ssl_choice" != "4" ]]; then
        ssl_choice="2"
    fi

    case "$ssl_choice" in
    1)
        # 用户选择 Let's Encrypt 域名选项
        echo -e "${green}使用 Let's Encrypt 域名证书...${plain}"
        ssl_cert_issue
        # 从证书中提取使用的域名
        local cert_domain=$(~/.acme.sh/acme.sh --list 2>/dev/null | tail -1 | awk '{print $1}')
        if [[ -n "${cert_domain}" ]]; then
            SSL_HOST="${cert_domain}"
            echo -e "${green}✓ SSL 证书配置成功，域名：${cert_domain}${plain}"
        else
            echo -e "${yellow}SSL 配置可能已完成，但域名提取失败${plain}"
            SSL_HOST="${server_ip}"
        fi
        ;;
    2)
        # 用户选择 Let's Encrypt IP 证书选项
        echo -e "${green}使用 Let's Encrypt IP 证书（短期配置文件）...${plain}"

        # 询问可选的 IPv6
        local ipv6_addr=""
        read -rp "是否包含 IPv6 地址？（留空跳过）：" ipv6_addr
        ipv6_addr="${ipv6_addr// /}"  # 去除空格

        # 停止面板（需要 80 端口）
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop >/dev/null 2>&1
        else
            systemctl stop x-ui >/dev/null 2>&1
        fi

        setup_ip_certificate "${server_ip}" "${ipv6_addr}"
        if [ $? -eq 0 ]; then
            SSL_HOST="${server_ip}"
            echo -e "${green}✓ Let's Encrypt IP 证书配置成功${plain}"
        else
            echo -e "${red}✗ IP 证书配置失败。请检查 80 端口是否已开放。${plain}"
            SSL_HOST="${server_ip}"
        fi
        ;;
    3)
        # 用户选择自定义路径（用户提供）
        echo -e "${green}使用自定义已有证书...${plain}"
        local custom_cert=""
        local custom_key=""
        local custom_domain=""

        # 3.1 请求域名以组成面板 URL
        read -rp "请输入证书对应的域名：" custom_domain
        custom_domain="${custom_domain// /}" # 去除空格

        # 3.2 证书路径循环
        while true; do
            read -rp "输入证书路径（关键字：.crt / fullchain）：" custom_cert
            # 去除引号
            custom_cert=$(echo "$custom_cert" | tr -d '"' | tr -d "'")

            if [[ -f "$custom_cert" && -r "$custom_cert" && -s "$custom_cert" ]]; then
                break
            elif [[ ! -f "$custom_cert" ]]; then
                echo -e "${red}错误：文件不存在！请重试。${plain}"
            elif [[ ! -r "$custom_cert" ]]; then
                echo -e "${red}错误：文件存在但不可读（请检查权限）！${plain}"
            else
                echo -e "${red}错误：文件为空！${plain}"
            fi
        done

        # 3.3 私钥路径循环
        while true; do
            read -rp "输入私钥路径（关键字：.key / privatekey）：" custom_key
            # 去除引号
            custom_key=$(echo "$custom_key" | tr -d '"' | tr -d "'")

            if [[ -f "$custom_key" && -r "$custom_key" && -s "$custom_key" ]]; then
                break
            elif [[ ! -f "$custom_key" ]]; then
                echo -e "${red}错误：文件不存在！请重试。${plain}"
            elif [[ ! -r "$custom_key" ]]; then
                echo -e "${red}错误：文件存在但不可读（请检查权限）！${plain}"
            else
                echo -e "${red}错误：文件为空！${plain}"
            fi
        done

        # 3.4 通过 x-ui 二进制文件应用设置
        ${xui_folder}/x-ui cert -webCert "$custom_cert" -webCertKey "$custom_key" >/dev/null 2>&1

        # 设置 SSL_HOST 用于组成面板 URL
        if [[ -n "$custom_domain" ]]; then
            SSL_HOST="$custom_domain"
        else
            SSL_HOST="${server_ip}"
        fi

        echo -e "${green}✓ 自定义证书路径已应用。${plain}"
        echo -e "${yellow}注意：您需要自行管理这些文件的续期。${plain}"

        systemctl restart x-ui >/dev/null 2>&1 || rc-service x-ui restart >/dev/null 2>&1
        ;;
    4)
        # Cloudflare SSL 证书（通配符，DNS 验证）
        echo -e "${green}使用 Cloudflare SSL 证书...${plain}"
        echo -e "${yellow}需要以下信息：${plain}"
        echo -e "  1. Cloudflare 注册邮箱"
        echo -e "  2. Cloudflare 全局 API 密钥"
        echo -e "  3. 域名（证书将包含主域名和通配符 *.域名）"

        local cf_domain=""
        local cf_key=""
        local cf_email=""

        read -rp "请输入域名：" cf_domain
        cf_domain="${cf_domain// /}"
        if [[ -z "$cf_domain" ]]; then
            echo -e "${red}域名不能为空，跳过 SSL 配置。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        read -rp "请输入 Cloudflare 全局 API 密钥：" cf_key
        cf_key="${cf_key// /}"
        if [[ -z "$cf_key" ]]; then
            echo -e "${red}API 密钥不能为空，跳过 SSL 配置。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        read -rp "请输入 Cloudflare 注册邮箱：" cf_email
        cf_email="${cf_email// /}"
        if [[ -z "$cf_email" ]]; then
            echo -e "${red}邮箱不能为空，跳过 SSL 配置。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        # 确保 acme.sh 已安装
        if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
            echo -e "${yellow}未找到 acme.sh，正在安装...${plain}"
            install_acme
            if [ $? -ne 0 ]; then
                echo -e "${red}安装 acme.sh 失败，跳过 SSL 配置。${plain}"
                SSL_HOST="${server_ip}"
                return 1
            fi
        fi

        # 设置 Let's Encrypt 为默认 CA
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt --force
        if [ $? -ne 0 ]; then
            echo -e "${red}设置默认 CA 失败，跳过 SSL 配置。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        # 导出 Cloudflare 凭证
        export CF_Key="${cf_key}"
        export CF_Email="${cf_email}"

        # 使用 Cloudflare DNS 签发证书（通配符 + 主域名）
        echo -e "${yellow}正在通过 Cloudflare DNS 签发证书...${plain}"
        ~/.acme.sh/acme.sh --issue --dns dns_cf -d "${cf_domain}" -d "*.${cf_domain}" --log --force
        if [ $? -ne 0 ]; then
            echo -e "${red}证书签发失败，请检查 Cloudflare API 密钥和域名是否正确。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        # 安装证书
        local certPath="/root/cert/${cf_domain}"
        rm -rf "${certPath}"
        mkdir -p "${certPath}"
        if [ $? -ne 0 ]; then
            echo -e "${red}创建目录失败：${certPath}${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        local reloadCmd="x-ui restart"
        ~/.acme.sh/acme.sh --installcert -d "${cf_domain}" -d "*.${cf_domain}" \
            --key-file "${certPath}/privkey.pem" \
            --fullchain-file "${certPath}/fullchain.pem" --reloadcmd "${reloadCmd}"

        if [ $? -ne 0 ]; then
            echo -e "${red}证书安装失败。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        # 启用自动续期
        ~/.acme.sh/acme.sh --upgrade --auto-upgrade >/dev/null 2>&1

        chmod 600 "${certPath}/privkey.pem"
        chmod 644 "${certPath}/fullchain.pem"

        echo -e "${green}✓ Cloudflare SSL 证书签发并安装成功。${plain}"

        # 为面板设置证书
        local webCertFile="${certPath}/fullchain.pem"
        local webKeyFile="${certPath}/privkey.pem"

        if [[ -f "$webCertFile" && -f "$webKeyFile" ]]; then
            ${xui_folder}/x-ui cert -webCert "$webCertFile" -webCertKey "$webKeyFile" >/dev/null 2>&1
            echo -e "${green}✓ 面板证书已设置。${plain}"
        else
            echo -e "${red}未找到证书或私钥文件。${plain}"
            SSL_HOST="${server_ip}"
            return 1
        fi

        SSL_HOST="${cf_domain}"
        echo -e "${green}✓ Cloudflare SSL 证书配置完成。${plain}"
        echo -e "${yellow}注意：证书支持自动续期，无需手动管理。${plain}"

        systemctl restart x-ui >/dev/null 2>&1 || rc-service x-ui restart >/dev/null 2>&1
        ;;
    *)
        echo -e "${red}无效选项。跳过 SSL 配置。${plain}"
        SSL_HOST="${server_ip}"
        ;;
    esac
}

config_after_install() {
    # 检测 x-ui 是否已安装：优先检查 x-ui.json（设置文件），其次检查 x-ui.db
    local db_folder="${XUI_DB_FOLDER:-/etc/x-ui}"
    local is_fresh_install=false
    if [[ ! -f "${db_folder}/x-ui.json" && ! -f "${db_folder}/x-ui.db" ]]; then
        is_fresh_install=true
    fi

    local existing_webBasePath=$(${xui_folder}/x-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}' | sed 's#^/##')
    local existing_port=$(${xui_folder}/x-ui setting -show true | grep -Eo 'port: .+' | awk '{print $2}')
    # 通过检查 cert: 行是否存在且之后有内容来正确检测空证书
    local existing_cert=$(${xui_folder}/x-ui setting -getCert true | grep 'cert:' | awk -F': ' '{print $2}' | tr -d '[:space:]')
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

    if [[ "$is_fresh_install" == "true" ]]; then
        # 全新安装：用户输入或随机生成凭据
        echo -e "${yellow}设置面板凭据（输入 rd 或留空将自动生成）：${plain}"

        read -rp "请输入用户名 [默认 admin]：" config_username
        config_username="${config_username// /}"
        if [[ -z "$config_username" || "$config_username" == "rd" ]]; then
            config_username="admin"
        fi

        read -rp "请输入密码 [默认 admin]：" config_password
        config_password="${config_password// /}"
        if [[ -z "$config_password" || "$config_password" == "rd" ]]; then
            config_password="admin"
        fi

        read -rp "请输入 Web 路径（不含前导 /）：" config_webBasePath
        config_webBasePath="${config_webBasePath// /}"
        config_webBasePath="${config_webBasePath#/}"   # 去除前导斜杠
        if [[ -z "$config_webBasePath" || "$config_webBasePath" == "rd" ]]; then
            config_webBasePath=$(gen_random_string 18)
            echo -e "${green}已生成随机 Web 路径：${config_webBasePath}${plain}"
        fi

        read -rp "是否要自定义面板端口？（否则将使用随机端口）[y/n]：" config_confirm
        if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
            read -rp "请设置面板端口：" config_port
            echo -e "${yellow}您的面板端口为：${config_port}${plain}"
        else
            local config_port=$(shuf -i 1024-62000 -n 1)
            echo -e "${yellow}已生成随机端口：${config_port}${plain}"
        fi

        ${xui_folder}/x-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"

        echo ""
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}        SSL 证书配置（必需）              ${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${yellow}出于安全考虑，所有面板都需要配置 SSL 证书。${plain}"
        echo -e "${yellow}Let's Encrypt 现已支持域名和 IP 地址！${plain}"
        echo ""

        prompt_and_setup_ssl "${config_port}" "${config_webBasePath}" "${server_ip}"

        # 显示最终凭据和访问信息
        echo ""
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}           面板安装完成！                 ${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${green}用户名：    ${config_username}${plain}"
        echo -e "${green}密码：      ${config_password}${plain}"
        echo -e "${green}端口：      ${config_port}${plain}"
        echo -e "${green}Web路径：   ${config_webBasePath}${plain}"
        echo -e "${green}访问地址：  https://${SSL_HOST}:${config_port}/${config_webBasePath}${plain}"
        echo -e "${green}═══════════════════════════════════════════${plain}"
        echo -e "${yellow}⚠ 重要：请安全保存这些凭据！${plain}"
        echo -e "${yellow}⚠ SSL 证书：已启用并配置${plain}"
    else
        # 已有安装（存在 x-ui.json 或 x-ui.db）：保留所有配置，不重新输入
        local config_webBasePath="${existing_webBasePath}"
        if [[ ${#config_webBasePath} -lt 4 ]]; then
            config_webBasePath=$(gen_random_string 18)
            echo -e "${yellow}WebBasePath 缺失或过短，正在生成新的...${plain}"
            ${xui_folder}/x-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}新 WebBasePath：${config_webBasePath}${plain}"
        fi

        if [[ -n "${existing_cert}" ]]; then
            echo -e "${green}SSL 证书已配置。${plain}"
        fi
        echo -e "${green}访问地址：https://${server_ip}:${existing_port}/${config_webBasePath}${plain}"
    fi

    ${xui_folder}/x-ui migrate
}

get_releases() {
    local releases_json
    releases_json=$(curl -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases")
    if [[ -z "$releases_json" ]]; then
        echo -e "${yellow}正在尝试通过 IPv4 获取版本...${plain}"
        releases_json=$(curl -4 -Ls "https://api.github.com/repos/Sora39831/3x-ui/releases")
        if [[ -z "$releases_json" ]]; then
            echo -e "${red}获取 x-ui 版本失败，可能是 GitHub API 限制，请稍后重试${plain}"
            exit 1
        fi
    fi

    # Parse first non-prerelease tag_name
    latest_stable=$(echo "$releases_json" | tr '{' '\n' | grep -B5 '"prerelease": false' | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')

    # Parse first prerelease tag_name
    latest_prerelease=$(echo "$releases_json" | tr '{' '\n' | grep -B5 '"prerelease": true' | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')

    if [[ -z "$latest_stable" && -z "$latest_prerelease" ]]; then
        echo -e "${red}获取 x-ui 版本失败${plain}"
        exit 1
    fi
}

select_version() {
    if [[ -n "$latest_stable" && -n "$latest_prerelease" ]]; then
        echo ""
        echo -e "${green}请选择要安装的版本：${plain}"
        echo -e "${green}1)${plain} 最新稳定版: ${latest_stable}"
        echo -e "${green}2)${plain} 最新预发布版: ${latest_prerelease}"
        read -rp "请输入选择 [1-2]: " version_choice
        while [[ "$version_choice" != "1" && "$version_choice" != "2" ]]; do
            read -rp "无效输入，请重新输入 [1-2]: " version_choice
        done
        if [[ "$version_choice" == "1" ]]; then
            tag_version="$latest_stable"
        else
            tag_version="$latest_prerelease"
        fi
    elif [[ -n "$latest_stable" ]]; then
        tag_version="$latest_stable"
    else
        tag_version="$latest_prerelease"
    fi
}

install_x-ui() {
    cd ${xui_folder%/x-ui}/

    # 下载资源
    if [ $# == 0 ]; then
        get_releases
        select_version
        echo -e "获取到 x-ui 版本：${tag_version}，开始安装..."
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz https://github.com/Sora39831/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui 失败，请确保服务器可以访问 GitHub${plain}"
            exit 1
        fi
    else
        tag_version=$1
        tag_version_numeric=${tag_version#v}
        min_version="2.3.5"

        if [[ "$(printf '%s\n' "$min_version" "$tag_version_numeric" | sort -V | head -n1)" != "$min_version" ]]; then
            echo -e "${red}请使用更新的版本（至少 v2.3.5）。安装已取消。${plain}"
            exit 1
        fi

        url="https://github.com/Sora39831/3x-ui/releases/download/${tag_version}/x-ui-linux-$(arch).tar.gz"
        echo -e "开始安装 x-ui $1"
        curl -4fLRo ${xui_folder}-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui $1 失败，请检查版本是否存在${plain}"
            exit 1
        fi
    fi
    curl -4fLRo /usr/bin/x-ui-temp https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}下载 x-ui.sh 失败${plain}"
        exit 1
    fi

    # 停止 x-ui 服务并删除旧资源
    if [[ -e ${xui_folder}/ ]]; then
        if [[ $release == "alpine" ]]; then
            rc-service x-ui stop
        else
            systemctl stop x-ui
        fi
        rm ${xui_folder}/ -rf
    fi

    # 解压资源并设置权限
    tar zxvf x-ui-linux-$(arch).tar.gz
    rm x-ui-linux-$(arch).tar.gz -f

    cd x-ui
    chmod +x x-ui
    chmod +x x-ui.sh

    # 检查系统架构并重命名文件
    if [[ $(arch) == "armv5" || $(arch) == "armv6" || $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x x-ui bin/xray-linux-$(arch)

    # 更新 x-ui 命令行工具并设置权限
    mv -f /usr/bin/x-ui-temp /usr/bin/x-ui
    chmod +x /usr/bin/x-ui
    mkdir -p /var/log/x-ui
    config_after_install

    # Etckeeper 兼容性
    if [ -d "/etc/.git" ]; then
        if [ -f "/etc/.gitignore" ]; then
            if ! grep -q "x-ui/x-ui.db" "/etc/.gitignore"; then
                echo "" >> "/etc/.gitignore"
                echo "x-ui/x-ui.db" >> "/etc/.gitignore"
                echo -e "${green}已将 x-ui.db 添加到 /etc/.gitignore 以支持 etckeeper${plain}"
            fi
        else
            echo "x-ui/x-ui.db" > "/etc/.gitignore"
            echo -e "${green}已创建 /etc/.gitignore 并添加 x-ui.db 以支持 etckeeper${plain}"
        fi
    fi

    if [[ $release == "alpine" ]]; then
        curl -4fLRo /etc/init.d/x-ui https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.rc
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui.rc 失败${plain}"
            exit 1
        fi
        chmod +x /etc/init.d/x-ui
        rc-update add x-ui
        rc-service x-ui start
    else
        # 安装 systemd 服务文件
        service_installed=false

        if [ -f "x-ui.service" ]; then
            echo -e "${green}在解压文件中找到 x-ui.service，正在安装...${plain}"
            cp -f x-ui.service ${xui_service}/ >/dev/null 2>&1
            if [[ $? -eq 0 ]]; then
                service_installed=true
            fi
        fi

        if [ "$service_installed" = false ]; then
            case "${release}" in
                ubuntu | debian | armbian)
                    if [ -f "x-ui.service.debian" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.debian，正在安装...${plain}"
                        cp -f x-ui.service.debian ${xui_service}/x-ui.service >/dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                ;;
                arch | manjaro | parch)
                    if [ -f "x-ui.service.arch" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.arch，正在安装...${plain}"
                        cp -f x-ui.service.arch ${xui_service}/x-ui.service >/dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                ;;
                *)
                    if [ -f "x-ui.service.rhel" ]; then
                        echo -e "${green}在解压文件中找到 x-ui.service.rhel，正在安装...${plain}"
                        cp -f x-ui.service.rhel ${xui_service}/x-ui.service >/dev/null 2>&1
                        if [[ $? -eq 0 ]]; then
                            service_installed=true
                        fi
                    fi
                ;;
            esac
        fi

        # 如果 tar.gz 中未找到服务文件，从 GitHub 下载
        if [ "$service_installed" = false ]; then
            echo -e "${yellow}tar.gz 中未找到服务文件，正在从 GitHub 下载...${plain}"
            case "${release}" in
                ubuntu | debian | armbian)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.service.debian >/dev/null 2>&1
                ;;
                arch | manjaro | parch)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.service.arch >/dev/null 2>&1
                ;;
                *)
                    curl -4fLRo ${xui_service}/x-ui.service https://raw.githubusercontent.com/Sora39831/3x-ui/main/x-ui.service.rhel >/dev/null 2>&1
                ;;
            esac

            if [[ $? -ne 0 ]]; then
                echo -e "${red}从 GitHub 安装 x-ui.service 失败${plain}"
                exit 1
            fi
            service_installed=true
        fi

        if [ "$service_installed" = true ]; then
            echo -e "${green}正在配置 systemd 服务...${plain}"
            chown root:root ${xui_service}/x-ui.service >/dev/null 2>&1
            chmod 644 ${xui_service}/x-ui.service >/dev/null 2>&1
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl restart x-ui
        else
            echo -e "${red}安装 x-ui.service 文件失败${plain}"
            exit 1
        fi
    fi

    echo -e "${green}x-ui ${tag_version}${plain} 安装完成，正在运行..."
    echo -e ""
    echo -e "┌───────────────────────────────────────────────────────┐
│  ${blue}x-ui 管理菜单用法（子命令）：${plain}                        │
│                                                       │
│  ${blue}x-ui${plain}              - 管理脚本                       │
│  ${blue}x-ui start${plain}        - 启动                           │
│  ${blue}x-ui stop${plain}         - 停止                           │
│  ${blue}x-ui restart${plain}      - 重启                           │
│  ${blue}x-ui status${plain}       - 查看状态                       │
│  ${blue}x-ui settings${plain}     - 查看当前设置                   │
│  ${blue}x-ui enable${plain}       - 设置开机自启                   │
│  ${blue}x-ui disable${plain}      - 取消开机自启                   │
│  ${blue}x-ui log${plain}          - 查看日志                       │
│  ${blue}x-ui banlog${plain}       - 查看 Fail2ban 封禁日志         │
│  ${blue}x-ui update${plain}       - 更新                           │
│  ${blue}x-ui legacy${plain}       - 安装旧版本                     │
│  ${blue}x-ui install${plain}      - 安装                           │
│  ${blue}x-ui uninstall${plain}    - 卸载                           │
└───────────────────────────────────────────────────────┘"
}

echo -e "${green}正在执行...${plain}"
install_base
install_x-ui $1
