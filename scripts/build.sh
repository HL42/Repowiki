#!/bin/bash
# RepoWiki 跨平台构建脚本
# 用法: bash scripts/build.sh [版本号]

set -e

VERSION="${1:-dev}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
RELEASE_DIR="$PROJECT_DIR/release/$VERSION"
FRONTEND_DIR="$PROJECT_DIR/frontend"
BINARY_NAME="repowiki"

echo "=== RepoWiki 构建 ==="
echo "版本: $VERSION"

# 1. 构建前端
echo "[1/4] 构建前端..."
cd "$FRONTEND_DIR"
[ -d "node_modules" ] || npm install
npm run build
cd "$PROJECT_DIR"

# 2. 清理
echo "[2/4] 清理旧产物..."
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# 3. 交叉编译
echo "[3/4] 交叉编译..."
LDFLAGS="-s -w -X main.Version=$VERSION"

for PLATFORM in "darwin/arm64" "darwin/amd64" "windows/amd64"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"
    echo "  $GOOS/$GOARCH..."
    OUTPUT="$RELEASE_DIR/$BINARY_NAME"
    [ "$GOOS" = "windows" ] && OUTPUT="$OUTPUT.exe"
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
        go build -ldflags="$LDFLAGS" -o "$OUTPUT" ./cmd/...
    echo "  OK"
done

# 4. 打包
echo "[4/4] 打包..."
PACKAGES=(
    "darwin-arm64:darwin"
    "darwin-amd64:darwin"
    "windows-amd64:windows"
)

for ITEM in "${PACKAGES[@]}"; do
    IFS=':' read -r PKG_NAME GOOS <<< "$ITEM"
    BIN="$BINARY_NAME"
    [ "$GOOS" = "windows" ] && BIN="$BINARY_NAME.exe"

    PKG_DIR="$RELEASE_DIR/$PKG_NAME"
    mkdir -p "$PKG_DIR/frontend/dist"

    cp "$RELEASE_DIR/$BIN" "$PKG_DIR/"
    [ "$GOOS" != "windows" ] && chmod +x "$PKG_DIR/$BIN"
    cp -r "$FRONTEND_DIR/dist/"* "$PKG_DIR/frontend/dist/"
    cp "$PROJECT_DIR/.env.example" "$PKG_DIR/"
    cp "$PROJECT_DIR/classify_rules.json" "$PKG_DIR/" 2>/dev/null || true

    if [ "$GOOS" = "windows" ]; then
        cp "$PROJECT_DIR/scripts/start.bat" "$PKG_DIR/"
    else
        cp "$PROJECT_DIR/scripts/start.sh" "$PKG_DIR/start.command"
        chmod +x "$PKG_DIR/start.command"
    fi

    cd "$RELEASE_DIR"
    zip -qr "repowiki-${VERSION}-${PKG_NAME}.zip" "$PKG_NAME/"
    cd "$PROJECT_DIR"
    echo "  $PKG_NAME OK"
done

echo ""
echo "=== 构建完成 ==="
ls -lh "$RELEASE_DIR"/*.zip 2>/dev/null
echo ""
echo "上传到 GitHub Release 即可分发:"
echo "  Mac:   repowiki-${VERSION}-darwin-arm64.zip"
echo "  Win:   repowiki-${VERSION}-windows-amd64.zip"
