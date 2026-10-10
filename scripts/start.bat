@echo off
chcp 65001 >nul
cd /d "%~dp0"

rem 源码/解压运行用；安装包用户直接双击 repowiki.exe 即可（无控制台窗口）
if not exist "repowiki.exe" (
    echo 未找到 repowiki.exe，请确认已完整解压压缩包。
    pause
    exit /b 1
)

rem 首次运行未配置 API Key 时，程序会在浏览器里打开引导页让你填写；
rem 端口被占用会自动换空闲端口。
repowiki.exe
