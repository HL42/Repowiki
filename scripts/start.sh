#!/bin/bash
# RepoWiki 启动脚本 (Mac/Linux)
cd "$(dirname "$0")"

if [ ! -f ".env" ]; then
    echo "============================================"
    echo "  首次运行 — 需要 DeepSeek API Key"
    echo "  免费注册: https://platform.deepseek.com"
    echo "============================================"
    echo ""
    read -p "请输入 API Key: " APIKEY
    if [ -z "$APIKEY" ]; then
        echo "未输入 API Key，退出。"
        exit 1
    fi
    cat > .env << EOF
DEEPSEEK_API_KEY=$APIKEY
DEEPSEEK_MODEL=deepseek-chat
PORT=8080
WIKI_ROOT=./knowledge-base
EOF
    echo "配置已保存。"
fi

echo "正在启动 RepoWiki..."
echo "浏览器打开 http://localhost:8080"
echo "按 Ctrl+C 停止"
echo ""

sleep 1 && open http://localhost:8080 2>/dev/null &
./repowiki
