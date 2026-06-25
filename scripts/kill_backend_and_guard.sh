#!/bin/bash
# stopSesame.sh - 停止 sesame-test 和 CPU guard

echo "=== 停止 sesame-test ==="
for pid in $(pgrep -f "sesame-test" 2>/dev/null); do
    cmd=$(cat /proc/$pid/cmdline 2>/dev/null | tr '\0' ' ')
    echo "  kill sesame-test pid=$pid"
    kill -15 "$pid" 2>/dev/null
done

sleep 2
for pid in $(pgrep -f "sesame-test" 2>/dev/null); do
    echo "  force kill sesame-test pid=$pid"
    kill -9 "$pid" 2>/dev/null
done

echo "=== 停止 CPU guard ==="
GUARD_PID_FILE="/var/run/sesame-cpu-guard.pid"
if [ -f "$GUARD_PID_FILE" ]; then
    GUARD_PID=$(cat "$GUARD_PID_FILE")
    if kill -0 "$GUARD_PID" 2>/dev/null; then
        echo "  kill guard pid=$GUARD_PID"
        kill "$GUARD_PID"
    else
        echo "  guard already dead"
    fi
    rm -f "$GUARD_PID_FILE"
else
    echo "  guard pid file not found"
fi

echo "=== 检查残留 ==="
pgrep -f "sesame-test" && echo "  [WARN] sesame-test still running!" || echo "  sesame-test: clean"
pgrep -f "sesame-cpu-guard" && echo "  [WARN] guard still running!" || echo "  guard: clean"

echo "=== done ==="