#!/bin/bash
# start.sh - SesameSauce 一键启动脚本
# 功能：启动后端、MinIO、nginx、Docker 服务，并开放防火墙端口
# 创建日期：2026-06-06
# 用法：./start.sh

set -euo pipefail

# ==================== 配置变量 ====================
BASE_DIR="/root/sesame"
PROJECT_DIR="$BASE_DIR/Sesame-sauce"
BACKEND_DIR="$PROJECT_DIR/backend"
LOG_DIR="$BASE_DIR/logs"
DOCKER_BASE="$BASE_DIR/dockers"

BACKEND_BIN="$BACKEND_DIR/sesame.exe"
BACKEND_LOG="$LOG_DIR/backend.log"
MINIO_LOG="$LOG_DIR/minio.log"
START_LOG="$LOG_DIR/start.log"

MINIO_USER='zhongfabai'
MINIO_PASS='9@olw1efacz$po!d'
MINIO_DATA="$BASE_DIR/data/minio"

NGINX_CONF_SRC="$PROJECT_DIR/deploy/nginx.conf"
NGINX_CONF_DST="/etc/nginx/sites-available/sesame-sauce"
NGINX_CONF_LINK="/etc/nginx/sites-enabled/sesame-sauce"

# 需要对外放行的端口
PORTS=(80 8080 9001)

# ==================== 日志初始化 ====================
mkdir -p "$LOG_DIR"
# 将脚本自身的所有输出同时记录到日志文件
exec > >(tee -a "$START_LOG") 2>&1

echo "========================================"
echo " SesameSauce 服务启动脚本"
echo " 时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "========================================"

# ==================== 后端服务 ====================
echo "[步骤 1/5] 启动后端服务..."

if [[ ! -f "$BACKEND_BIN" ]]; then
    echo "[信息] 后端可执行文件不存在，开始编译..."
    cd "$BACKEND_DIR"
    go build -o ./sesame.exe ./cmd/api/main.go
    echo "[成功] 编译完成: $BACKEND_BIN"
else
    echo "[信息] 后端可执行文件已存在: $BACKEND_BIN"
fi

# 如果已在运行，先停止旧进程
if pgrep -f "sesame.exe" > /dev/null 2>&1; then
    echo "[警告] 检测到 sesame.exe 正在运行，先停止旧进程..."
    pkill -f "sesame.exe" || true
    sleep 2
fi

cd "$BACKEND_DIR"
nohup ./sesame.exe > "$BACKEND_LOG" 2>&1 &
echo "[成功] 后端已启动 (PID: $!), 日志: $BACKEND_LOG"

# ==================== MinIO ====================
echo "[步骤 2/5] 启动 MinIO..."

if pgrep -f "minio server" > /dev/null 2>&1; then
    echo "[警告] 检测到 MinIO 正在运行，先停止旧进程..."
    pkill -f "minio server" || true
    sleep 2
fi

export MINIO_ROOT_USER="$MINIO_USER"
export MINIO_ROOT_PASSWORD="$MINIO_PASS"

nohup minio server "$MINIO_DATA" --console-address ":9001" > "$MINIO_LOG" 2>&1 &
echo "[成功] MinIO 已启动 (PID: $!), 日志: $MINIO_LOG"

# ==================== nginx（前端） ====================
echo "[步骤 3/5] 检查并加载 nginx 配置..."

# 确保 nginx 站点配置已部署
if [[ ! -f "$NGINX_CONF_LINK" ]]; then
    echo "[信息] nginx 站点配置未启用，正在部署..."
    cp "$NGINX_CONF_SRC" "$NGINX_CONF_DST"
    ln -sf "$NGINX_CONF_DST" "$NGINX_CONF_LINK"
    echo "[成功] 已部署 nginx 配置: $NGINX_CONF_LINK"
fi

# 测试并重载 nginx
nginx -t
if systemctl is-active --quiet nginx; then
    systemctl reload nginx
    echo "[成功] nginx 已重载"
else
    systemctl start nginx
    echo "[成功] nginx 已启动"
fi

# ==================== Docker 服务 ====================
echo "[步骤 4/5] 启动 Docker 服务..."

for service in kafka redis zookeeper es; do
    compose_dir="$DOCKER_BASE/$service"
    if [[ -f "$compose_dir/docker-compose.yml" || -f "$compose_dir/docker-compose.yaml" ]]; then
        echo "[信息] 启动 $service ..."
        cd "$compose_dir"
        docker compose up -d
        echo "[成功] $service 已启动"
    else
        echo "[警告] $compose_dir 下未找到 docker-compose 文件，跳过"
    fi
done

# ==================== 防火墙 ====================
echo "[步骤 5/5] 开放防火墙端口..."

if ! command -v ufw >/dev/null 2>&1; then
    echo "[错误] 未找到 ufw，跳过防火墙配置"
else
    for port in "${PORTS[@]}"; do
        ufw allow "$port/tcp" >/dev/null
        echo "[成功] 已放行端口 $port/tcp"
    done
    echo ""
    echo "--- 当前 ufw 状态 ---"
    ufw status | grep -E "(Status|${PORTS[*]// /|})" || ufw status
fi

echo ""
echo "========================================"
echo " 所有服务启动完成！"
echo "========================================"
