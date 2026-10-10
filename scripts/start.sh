#!/bin/bash
# RepoWiki 启动脚本（源码/解压运行用；安装包用户直接双击 .app 或 exe 即可）
cd "$(dirname "$0")" || exit 1

if [ ! -x "./repowiki" ]; then
    echo "未找到可执行文件 repowiki，请确认已完整解压压缩包。"
    exit 1
fi

# 首次运行若未配置 API Key，程序会在浏览器里打开引导页让你填写；
# 端口被占用时会自动改用空闲端口，浏览器地址始终与监听地址一致。
./repowiki
