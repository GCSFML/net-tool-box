@echo off
REM ============================================================
REM  Net Tool Box - Windows 构建脚本（网页版 + 命令行 TUI 版）
REM  产出两个免安装单文件 exe（需要本机已安装 Go）
REM ============================================================
setlocal

set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
set GOTOOLCHAIN=local

if not exist dist mkdir dist

echo 正在构建【网页版】...
go build -trimpath -ldflags "-s -w" -o dist\nettoolbox.exe .
if %errorlevel% neq 0 goto fail

echo 正在构建【命令行 TUI 版】...
go build -trimpath -ldflags "-s -w" -o dist\nettoolbox-cli.exe ./cmd/nettui
if %errorlevel% neq 0 goto fail

echo.
echo 构建完成（免安装，双击/命令行直接运行）：
echo   网页版    dist\nettoolbox.exe
echo   命令行版  dist\nettoolbox-cli.exe
goto end

:fail
echo 构建失败！
exit /b 1

:end
endlocal
