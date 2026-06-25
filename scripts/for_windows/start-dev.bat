@echo off
chcp 65001 >nul
:: 关闭命令回显，切换 UTF-8 编码避免中文乱码
:: 创建日期: 2026-04-25
:: 功能: 一键启动 SesameSauce 开发环境（minIO + Go 后端 + Vite 前端）

:: ==================== 可配置变量 ====================
set "MINIO_BIN=minio.exe"
:: minio.exe 名称。如果已在系统 PATH 中，保持 minio.exe 即可；
:: 否则改为绝对路径，如 set "MINIO_BIN=D:\Tools\minio.exe"

set "MINIO_DATA_DIR=minio-data"
:: minIO 数据目录，相对于仓库根目录

set "MINIO_CONSOLE_PORT=9001"
:: minIO Console 管理界面端口

set "MINIO_API_PORT=9000"
:: minIO S3 API 端口

:: ==================== 计算仓库根目录 ====================
cd /d "%~dp0"
cd ..
set "REPO_ROOT=%CD%"
:: REPO_ROOT 为脚本所在目录的父级，即仓库根目录

:: ==================== 依赖检查 ====================
where /q "%MINIO_BIN%" 2>nul
if errorlevel 1 (
    if not exist "%MINIO_BIN%" (
        echo [错误] 找不到 minio.exe。请确保 minio.exe 在系统 PATH 中，或修改脚本里的 MINIO_BIN 为绝对路径。
        pause
        exit /b 1
    )
)

where /q go 2>nul
if errorlevel 1 (
    echo [错误] 找不到 go 命令。请确保 Go 已安装并加入 PATH。
    pause
    exit /b 1
)

where /q npm 2>nul
if errorlevel 1 (
    echo [错误] 找不到 npm 命令。请确保 Node.js 已安装并加入 PATH。
    pause
    exit /b 1
)

:: ==================== 创建 minIO 数据目录 ====================
if not exist "%REPO_ROOT%\%MINIO_DATA_DIR%" (
    mkdir "%REPO_ROOT%\%MINIO_DATA_DIR%"
    echo [信息] 已创建 minIO 数据目录: %REPO_ROOT%\%MINIO_DATA_DIR%
)

:: ==================== 启动三个独立终端窗口 ====================

:: 1) 启动 minIO
start "MinIO" cmd /k "cd /d "%REPO_ROOT%" && "%MINIO_BIN%" server "%REPO_ROOT%\%MINIO_DATA_DIR%" --console-address :%MINIO_CONSOLE_PORT%"

:: 2) 等待 minIO API 就绪后再启动后端
::    后端 InitMinio() 启动时会立即连接 localhost:9000，若尚未监听将导致崩溃
echo [信息] 等待 minIO 启动（约 3 秒）...
timeout /t 3 /nobreak >nul

:: 3) 启动 Go 后端
start "Backend" cmd /k "cd /d "%REPO_ROOT%\backend" && go run ./cmd/api"

:: 4) 启动前端 Vite（与后端并行，不依赖 minIO）
start "Frontend" cmd /k "cd /d "%REPO_ROOT%\frontend" && npm run dev"

:: ==================== 提示信息 ====================
echo.
echo ========================================
echo  开发环境已启动，共 3 个独立窗口：
echo    - MinIO   : API %MINIO_API_PORT% / Console %MINIO_CONSOLE_PORT%
echo    - Backend : 以实际配置或终端输出为准
echo    - Frontend: 5173 (vite 默认)
echo ========================================
echo.
echo 停止方法：
echo   1) 分别在各窗口按 Ctrl+C，然后关闭窗口
echo   2) 或运行 scripts\stop-dev.bat 一键停止
pause
