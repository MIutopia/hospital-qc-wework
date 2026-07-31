@echo off
REM ===================================================
REM 住院病例质控系统 - Windows 构建脚本
REM 用法: build.bat [env]
REM   env: dev / prod（默认 dev）
REM ===================================================

setlocal enabledelayedexpansion

set ENV=%1
if "%ENV%"=="" set ENV=dev

echo ========================================
echo 住院病例质控系统 - 构建脚本
echo 环境: %ENV%
echo ========================================

REM ----- 后端构建 -----
echo [1/3] 构建后端服务...
cd /d "%~dp0..\backend"

REM 设置 Go 环境
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

REM 构建
go build -ldflags="-s -w" -o ..\deploy\out\hospital-qc.exe .\cmd\server\main.go
if %ERRORLEVEL% neq 0 (
    echo [失败] 后端构建失败
    exit /b 1
)
echo [成功] 后端构建完成: deploy\out\hospital-qc.exe

REM 复制配置文件
copy config.yaml ..\deploy\out\config.yaml /Y >nul
echo [成功] 配置文件已复制

REM ----- 前端构建 -----
echo [2/3] 构建前端...
cd /d "%~dp0..\frontend"

if not exist "node_modules" (
    echo [信息] 正在安装前端依赖...
    call npm install
    if !ERRORLEVEL! neq 0 (
        echo [失败] 前端依赖安装失败
        exit /b 1
    )
)

call npm run build
if %ERRORLEVEL% neq 0 (
    echo [失败] 前端构建失败
    exit /b 1
)
echo [成功] 前端构建完成: frontend\dist\

REM ----- 复制部署文件 -----
echo [3/3] 整理部署包...
cd /d "%~dp0.."

if not exist "deploy\out" mkdir deploy\out

REM 复制前端产物
xcopy /E /I /Y frontend\dist deploy\out\dist >nul

REM 复制 Nginx 配置
copy deploy\nginx.conf deploy\out\nginx.conf /Y >nul

REM 复制 SQL 脚本
xcopy /E /I /Y sql deploy\out\sql >nul

REM 复制部署手册
copy docs\deploy.md deploy\out\deploy.md /Y >nul 2>nul

echo ========================================
echo 构建完成！部署包位置: deploy\out\
echo ========================================
echo.
echo 部署步骤:
echo 1. 将 deploy\out\ 目录下所有文件复制到服务器
echo 2. 使用 NSSM 注册服务: nssm install HospitalQC D:\app\hospital-qc\hospital-qc.exe
echo 3. 配置 Nginx（参考 nginx.conf）
echo 4. 执行 SQL 脚本建库
echo 5. 启动服务: nssm start HospitalQC
echo 6. 启动 Nginx: nginx
echo.

endlocal
