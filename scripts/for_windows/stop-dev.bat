@echo off
chcp 65001 >nul
:: 关闭命令回显，切换 UTF-8 编码避免中文乱码
:: 创建日期: 2026-04-25
:: 功能: 一键停止 SesameSauce 开发环境（minIO + Go 后端 + Vite 前端）

echo 正在停止 SesameSauce 开发环境...

:: 按窗口标题终止，避免误杀其他项目进程
taskkill /F /FI "WINDOWTITLE eq MinIO" 2>nul
:: minIO 进程兜底
taskkill /F /IM minio.exe 2>nul

taskkill /F /FI "WINDOWTITLE eq Backend" 2>nul
:: go run 的编译守护进程兜底
taskkill /F /IM go.exe 2>nul

taskkill /F /FI "WINDOWTITLE eq Frontend" 2>nul
:: vite 的 node 进程兜底
taskkill /F /IM node.exe 2>nul

echo 已尝试停止所有服务。
pause
