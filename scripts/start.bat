@echo off
chcp 65001 >nul
cd /d "%~dp0"

if not exist ".env" (
    echo ============================================
    echo   首次运行 — 需要 DeepSeek API Key
    echo   免费注册: https://platform.deepseek.com
    echo ============================================
    echo.
    set /p APIKEY="请输入 API Key: "
    if "!APIKEY!"=="" (
        echo 未输入 API Key，退出。
        pause
        exit /b 1
    )
    (
        echo DEEPSEEK_API_KEY=!APIKEY!
        echo DEEPSEEK_MODEL=deepseek-chat
        echo PORT=8080
        echo WIKI_ROOT=./knowledge-base
    ) > .env
    echo 配置已保存。
)

echo 正在启动 RepoWiki...
echo 浏览器打开 http://localhost:8080
echo 关闭此窗口即可停止。
echo.

start "" http://localhost:8080
repowiki.exe
