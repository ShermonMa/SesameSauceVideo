#!/bin/bash


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