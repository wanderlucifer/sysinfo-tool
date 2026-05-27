@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ==========================================
echo  系统信息采集工具 - 构建脚本
echo ==========================================
echo.

cd /d "%~dp0"

REM Set Go module proxy
set GOPROXY=https://goproxy.cn,direct

REM Clean old builds
if exist "release" rmdir /s /q release
mkdir release

REM ==========================================
REM Build 64-bit version
REM ==========================================
echo [1/2] 正在编译 64位 版本...
set GOARCH=amd64
set GOOS=windows
go build -ldflags="-s -w -H=windowsgui" -o release\sysinfo-tool_x64.exe 2>&1
if %errorlevel% equ 0 (
    echo   ✓ 64位版本编译成功
) else (
    echo   ✗ 64位版本编译失败
    pause
    exit /b 1
)

REM ==========================================
REM Build 32-bit version (XP compatible)
REM ==========================================
echo [2/2] 正在编译 32位 版本 (兼容Windows XP)...
set GOARCH=386
set GOOS=windows
REM Go 1.18+ supports GOAMD64, for 386 no specific flag needed
REM For XP compatibility, set the minimum OS version
go build -ldflags="-s -w -H=windowsgui" -o release\sysinfo-tool_x86.exe 2>&1
if %errorlevel% equ 0 (
    echo   ✓ 32位版本编译成功
) else (
    echo   ✗ 32位版本编译失败
    pause
    exit /b 1
)

echo.
echo ==========================================
echo  构建完成！
echo.
echo  输出文件:
echo    release\sysinfo-tool_x64.exe  (64位)
echo    release\sysinfo-tool_x86.exe  (32位,兼容XP)
echo ==========================================

REM Show file sizes
echo.
echo  文件大小:
for %%f in (release\*.exe) do (
    set size=%%~zf
    set /a sizeMB=!size!/1024/1024
    echo    %%~nxf - !sizeMB! MB
)

pause
