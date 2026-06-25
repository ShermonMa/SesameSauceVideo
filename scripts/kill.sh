#!/bin/bash
# kill.sh - SesameSauce 一键停止脚本
# 功能：停止后端、MinIO、Docker 服务，并关闭防火墙端口（不停止 nginx）
# 创建日期：2026-06-06
# 用法：./kill.sh

set -euo pipefail

# ==================== 配置变量 ====================
BASE_DIR="/root/sesame"
DOCKER_BASE="$BASE_DIR/dockers"
LOG_DIR="$BASE_DIR/logs"
KILL_LOG="$LOG_DIR/kill.log"

# 需要关闭的端口
PORTS=(80 8080 9000 9001)

# ==================== 日志初始化 ====================
mkdir -p "$LOG_DIR"
# 将脚本自身的所有输出同时记录到日志文件
exec > >(tee -a "$KILL_LOG") 2>&1

echo "========================================"
echo " SesameSauce 服务停止脚本"
echo " 时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "========================================"

# ==================== 停止后端 ====================
echo "[步骤 1/4] 停止后端服务..."
if pgrep -f "sesame.exe" > /dev/null 2>&1; then
    pkill -f "sesame.exe" || true
    echo "[成功] 后端已停止"
else
    echo "[信息] 后端未在运行"
fi

# ==================== 停止 MinIO ====================
echo "[步骤 2/4] 停止 MinIO..."
if pgrep -f "minio server" > /dev/null 2>&1; then
    pkill -f "minio server" || true
    echo "[成功] MinIO 已停止"
else
    echo "[信息] MinIO 未在运行"
fi

# ==================== 停止 Docker 服务 ====================
echo "[步骤 3/4] 停止 Docker 服务..."

for service in kafka redis zookeeper es; do
    compose_dir="$DOCKER_BASE/$service"
    if [[ -f "$compose_dir/docker-compose.yml" || -f "$compose_dir/docker-compose.yaml" ]]; then
        echo "[信息] 停止 $service ..."
        cd "$compose_dir"
        docker compose down
        echo "[成功] $service 已停止"
    else
        echo "[警告] $compose_dir 下未找到 docker-compose 文件，跳过"
    fi
done

# ==================== 关闭防火墙 ====================
echo "[步骤 4/4] 关闭防火墙端口..."

if ! command -v ufw >/dev/null 2>&1; then
    echo "[错误] 未找到 ufw，跳过防火墙配置"
else
    for port in "${PORTS[@]}"; do
        ufw delete allow "$port/tcp" >/dev/null 2>&1 || true
        echo "[成功] 已关闭端口 $port/tcp"
    done
    echo ""
    echo "--- 当前 ufw 状态 ---"
    ufw status | grep -E "(Status|${PORTS[*]// /|})" || ufw status
fi

echo ""
echo "========================================"
echo " 所有服务已停止（nginx 保持运行）"
echo "========================================"
