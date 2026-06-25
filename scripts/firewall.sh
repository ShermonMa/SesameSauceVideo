#!/bin/bash
# firewall.sh - SesameSauce 前端防火墙开关脚本
# 功能：基于 deploy/nginx.conf 中的 listen 端口，使用 ufw 开关前端对外防火墙
# 用法：./firewall.sh [on|off|status]
# 创建日期：2026-05-29

# 用法示例
# sudo ./firewall.sh on      # 放行前端端口
# sudo ./firewall.sh off     # 关闭前端端口
# ./firewall.sh status       # 查看状态（无需 sudo）

set -euo pipefail

NGINX_CONF="deploy/nginx.conf"

# 提取 nginx.conf 中的 listen 端口（默认 80）
get_frontend_port() {
    local port
    if [[ -f "$NGINX_CONF" ]]; then
        port=$(grep -m1 '^\s*listen' "$NGINX_CONF" | sed -E 's/.*listen\s+([0-9]+).*/\1/')
    fi
    echo "${port:-80}"
}

# 检查是否以 root 或 sudo 运行
check_sudo() {
    if [[ $EUID -ne 0 ]]; then
        echo "[错误] 需要 root 或 sudo 权限来操作防火墙。" >&2
        echo "用法：sudo $0 $1" >&2
        exit 1
    fi
}

# 开关防火墙
ufw_toggle() {
    local action=$1  # allow 或 delete allow
    local port
    port=$(get_frontend_port)

    if ! command -v ufw >/dev/null 2>&1; then
        echo "[错误] 未找到 ufw。请先安装：sudo apt install ufw" >&2
        exit 1
    fi

    echo "[信息] 前端对外端口: $port (来源: $NGINX_CONF)"

    if [[ "$action" == "allow" ]]; then
        ufw allow "$port/tcp" >/dev/null
        echo "[成功] 已放行端口 $port/tcp"
    else
        # ufw delete 语法比较特殊
        ufw delete allow "$port/tcp" >/dev/null 2>&1 || true
        echo "[成功] 已关闭端口 $port/tcp"
    fi

    echo ""
    echo "--- 当前 ufw 状态 ---"
    ufw status numbered | grep -E "(Status|$port)" || ufw status
}

# 查看状态
ufw_status() {
    local port
    port=$(get_frontend_port)

    if ! command -v ufw >/dev/null 2>&1; then
        echo "[错误] 未找到 ufw。请先安装：sudo apt install ufw" >&2
        exit 1
    fi

    echo "[信息] 前端对外端口: $port (来源: $NGINX_CONF)"
    echo ""
    echo "--- 当前 ufw 状态 ---"
    ufw status | grep -E "(Status|$port)" || ufw status
}

# 帮助信息
usage() {
    echo "用法: $0 [on|off|status]"
    echo ""
    echo "命令:"
    echo "  on      放行前端对外端口（基于 deploy/nginx.conf）"
    echo "  off     关闭前端对外端口"
    echo "  status  查看防火墙状态和端口放行情况"
    echo ""
    echo "示例:"
    echo "  sudo $0 on"
    echo "  sudo $0 off"
    echo "  $0 status"
}

# 主逻辑
case "${1:-}" in
    on)
        check_sudo "$1"
        ufw_toggle "allow"
        ;;
    off)
        check_sudo "$1"
        ufw_toggle "delete"
        ;;
    status)
        ufw_status
        ;;
    *)
        usage
        exit 1
        ;;
esac
