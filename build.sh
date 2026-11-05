#!/usr/bin/env bash
# ============================================================
#  Net Tool Box - Linux/macOS 构建脚本（网页版 + 命令行 TUI 版）
#  产出两个免安装单文件程序（需要本机已安装 Go）
# ============================================================
set -euo pipefail

mkdir -p dist

echo "正在构建【网页版】..."
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/nettoolbox .

echo "正在构建【命令行 TUI 版】..."
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/nettoolbox-cli ./cmd/nettui

echo "构建完成："
echo "  网页版    dist/nettoolbox"
echo "  命令行版  dist/nettoolbox-cli"
