#!/bin/bash

# ==================== CPU 性能守护 ====================
GUARD_LOG="/var/log/sesame-cpu-guard.log"
GUARD_PID_FILE="/var/run/sesame-cpu-guard.pid"
CPU_THRESHOLD=85
LOAD_THRESHOLD=10.0
TARGETS=("sesame-test" "go build")

log_guard() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$GUARD_LOG"
}

# 递归杀进程树（先子后父，防止 go build 子进程变孤儿）
kill_tree() {
    local pid=$1
    [ -z "$pid" ] && return
    [ "$pid" = "$$" ] && return
    [ ! -d "/proc/$pid" ] && return
    
    # 先杀所有子进程
    for child in $(pgrep -P "$pid" 2>/dev/null); do
        kill_tree "$child"
    done
    
    # 再杀自己
    cmd=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ')
    log_guard "KILL pid=$pid cmd='$cmd'"
    kill -15 "$pid" 2>/dev/null
    sleep 1
    if [ -d "/proc/$pid" ]; then
        kill -9 "$pid" 2>/dev/null
        log_guard "FORCE KILL pid=$pid"
    fi
}

# 启动守护（防重复）
if [ -f "$GUARD_PID_FILE" ]; then
    OLD_PID=$(cat "$GUARD_PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        log_guard "Guard already running (pid=$OLD_PID)"
    else
        rm -f "$GUARD_PID_FILE"
    fi
fi

if [ ! -f "$GUARD_PID_FILE" ]; then
    (
        while true; do
            CPU=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1 | cut -d'.' -f1)
            LOAD=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')
            
            OVERLOAD=0
            [ -n "$CPU" ] && [ "$CPU" -gt "$CPU_THRESHOLD" ] 2>/dev/null && OVERLOAD=1
            awk "BEGIN {exit !($LOAD >= $LOAD_THRESHOLD)}" 2>/dev/null && OVERLOAD=1
            
            if [ "$OVERLOAD" -eq 1 ]; then
                log_guard "OVERLOAD CPU=${CPU}% LOAD=${LOAD}, killing targets..."
                for target in "${TARGETS[@]}"; do
                    pids=$(pgrep -f "$target" 2>/dev/null)
                    for pid in $pids; do
                        kill_tree "$pid"
                    done
                done
            fi
            sleep 5
        done
    ) & disown
    
    NEW_PID=$!
    echo "$NEW_PID" > "$GUARD_PID_FILE"
    log_guard "Guard started (pid=$NEW_PID)"
fi
# =====================================================

cd /root/sesame/Sesame-sauce || exit 1
git pull

# 默认参数：全更新
TARGET="${1:-all}"
# | 命令                  | 效果                  |
# | ------------------- | ------------------- |
# | `./script.sh`       | 默认全更新（back + front） |
# | `./script.sh all`   | 全更新                 |
# | `./script.sh back`  | 只更新后端               |
# | `./script.sh front` | 只更新前端               |

case "$TARGET" in
    back|backend)
        /root/sesame/Sesame-sauce/scripts/updateBackend.sh
        ;;
    front|frontend)
        /root/sesame/Sesame-sauce/scripts/updateFrontend.sh
        ;;
    all|*)
        /root/sesame/Sesame-sauce/scripts/updateBackend.sh
        /root/sesame/Sesame-sauce/scripts/updateFrontend.sh
        ;;
esac