@echo off
chcp 65001 >nul
REM 一键编译：把整个项目打包成单文件 exe（双击即用、不依赖 Go 安装）。
REM 用法：直接双击本文件，或在终端运行 build.bat。

cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo [错误] 没找到 Go。请先安装 Go 1.23+：https://go.dev/dl/
  echo 安装后重新打开终端再运行本脚本。
  pause
  exit /b 1
)

REM 先关掉正在运行的实例，否则 Go 无法覆盖正在运行的 exe，会留下 clash节点工具.exe~ 备份。
taskkill /IM "clash节点工具.exe" /F >nul 2>nul

echo 正在下载依赖...
go mod tidy
if errorlevel 1 ( echo [错误] go mod tidy 失败 & pause & exit /b 1 )

echo 正在编译 clash节点工具.exe ...
go build -ldflags "-s -w" -o "clash节点工具.exe" ./cmd/clash-node-pipeline
if errorlevel 1 ( echo [错误] 编译失败 & pause & exit /b 1 )

REM 清理可能残留的旧备份。
if exist "clash节点工具.exe~" del /f /q "clash节点工具.exe~" >nul 2>nul

echo.
echo ==================================================
echo  编译完成！
echo  生成文件: clash节点工具.exe
echo  双击它即可打开图形界面，无需再敲命令。
echo ==================================================
pause
