#!/usr/bin/env bash

# StudyBuddy AI - Go静的解析スクリプト

# OpenCVのpkg-configパスを設定
export PKG_CONFIG_PATH="/opt/homebrew/lib/pkgconfig:$PKG_CONFIG_PATH"

# GOPATHとツールのパスを設定
export PATH="$PATH:$(go env GOPATH)/bin:/opt/homebrew/bin"

echo "=== Go静的解析実行 ==="

# go vet
echo "📋 go vet実行中..."
go vet ./... || echo "⚠️  go vetで問題が見つかりました"

echo ""

# golangci-lint（staticcheckを含む）
echo "🔧 golangci-lint実行中..."
if command -v golangci-lint &> /dev/null; then
    golangci-lint run ./... || echo "⚠️  golangci-lintで問題が見つかりました"
else
    echo "⚠️  golangci-lintがインストールされていません"
    echo "インストール: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin"
fi

echo ""
echo "✅ 静的解析完了"