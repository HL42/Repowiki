#!/bin/bash
# RepoWiki 跨平台构建脚本
# 产物：
#   macOS  → RepoWiki-<版本>-arm64.dmg / -amd64.dmg（内含 RepoWiki.app，拖进应用程序）
#   Windows→ repowiki-<版本>-windows-amd64.zip（内含无控制台窗口的 repowiki.exe）
#
# 用法: bash scripts/build.sh [版本号]     （默认版本 dev）

set -e

VERSION="${1:-dev}"
CFBUNDLE_VERSION="${VERSION#v}"          # Info.plist 用纯数字版本，去掉前导 v
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
RELEASE_DIR="$PROJECT_DIR/release/$VERSION"
FRONTEND_DIR="$PROJECT_DIR/frontend"
BINARY_NAME="repowiki"
LDFLAGS="-s -w -X main.Version=$VERSION"

echo "=== RepoWiki 构建 ==="
echo "版本: $VERSION"

# 1. 构建前端
echo "[1/5] 构建前端..."
cd "$FRONTEND_DIR"
[ -d "node_modules" ] || npm install
npm run build
cd "$PROJECT_DIR"

# 2. 清理
echo "[2/5] 清理旧产物..."
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# 把通用资源复制到目标目录（frontend/dist、examples、classify_rules、.env.example）
copy_resources() {
    local dest="$1"
    mkdir -p "$dest/frontend/dist"
    cp -r "$FRONTEND_DIR/dist/"* "$dest/frontend/dist/"
    cp -r "$PROJECT_DIR/examples" "$dest/"
    cp "$PROJECT_DIR/classify_rules.json" "$dest/" 2>/dev/null || true
    cp "$PROJECT_DIR/.env.example" "$dest/"
}

# 3. macOS .app + .dmg
echo "[3/5] 构建 macOS 原生壳 + .app + .dmg..."

# 先编译通用（arm64 + x86_64）的 WKWebView 壳，两个 .app 共用同一份
SHELL_SRC="$PROJECT_DIR/scripts/macapp/main.swift"
SHELL_ARM="$RELEASE_DIR/shell-arm64"
SHELL_AMD="$RELEASE_DIR/shell-amd64"
SHELL_UNIV="$RELEASE_DIR/RepoWiki-shell"
swiftc -O -swift-version 5 -target arm64-apple-macosx12.0  "$SHELL_SRC" -o "$SHELL_ARM"
swiftc -O -swift-version 5 -target x86_64-apple-macosx12.0 "$SHELL_SRC" -o "$SHELL_AMD"
lipo -create "$SHELL_ARM" "$SHELL_AMD" -output "$SHELL_UNIV"
rm -f "$SHELL_ARM" "$SHELL_AMD"

for ARCH in arm64 amd64; do
    APP_DIR="$RELEASE_DIR/RepoWiki-$ARCH.app"
    BIN_DIR="$APP_DIR/Contents/MacOS"
    RES_DIR="$APP_DIR/Contents/Resources"
    mkdir -p "$BIN_DIR" "$RES_DIR"

    echo "  darwin/$ARCH..."
    # 后端放在 Resources（exeDir 即 Resources，frontend/dist 就在它旁边，定位正确）
    CGO_ENABLED=0 GOOS=darwin GOARCH=$ARCH \
        go build -ldflags="$LDFLAGS" -o "$RES_DIR/$BINARY_NAME" ./cmd/...
    chmod +x "$RES_DIR/$BINARY_NAME"
    # 原生壳作为 CFBundleExecutable，负责拉起后端并开独立窗口
    cp "$SHELL_UNIV" "$BIN_DIR/RepoWiki"
    chmod +x "$BIN_DIR/RepoWiki"

    copy_resources "$RES_DIR"

    cat > "$APP_DIR/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>RepoWiki</string>
  <key>CFBundleDisplayName</key><string>RepoWiki</string>
  <key>CFBundleIdentifier</key><string>com.repowiki.app</string>
  <key>CFBundleVersion</key><string>${CFBUNDLE_VERSION}</string>
  <key>CFBundleShortVersionString</key><string>${CFBUNDLE_VERSION}</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleExecutable</key><string>RepoWiki</string>
  <key>LSMinimumSystemVersion</key><string>10.15</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>LSApplicationCategoryType</key><string>public.app-category.developer-tools</string>
</dict>
</plist>
EOF

    # 组装 dmg：staging 里放 app + 指向 /Applications 的快捷方式，便于拖拽安装
    STAGE="$RELEASE_DIR/stage-$ARCH"
    rm -rf "$STAGE"
    mkdir -p "$STAGE"
    cp -R "$APP_DIR" "$STAGE/"
    ln -s /Applications "$STAGE/Applications"

    DMG="$RELEASE_DIR/RepoWiki-${VERSION}-${ARCH}.dmg"
    hdiutil create -volname "RepoWiki" -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null
    rm -rf "$STAGE"
    echo "  → $DMG"
done
rm -f "$SHELL_UNIV"

# 4. Windows exe（无控制台窗口）
echo "[4/5] 构建 Windows exe..."
WIN_DIR="$RELEASE_DIR/repowiki-${VERSION}-windows-amd64"
mkdir -p "$WIN_DIR"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    go build -ldflags="$LDFLAGS -H windowsgui" -o "$WIN_DIR/$BINARY_NAME.exe" ./cmd/...
copy_resources "$WIN_DIR"

cd "$RELEASE_DIR"
zip -qr "repowiki-${VERSION}-windows-amd64.zip" "repowiki-${VERSION}-windows-amd64/"
rm -rf "$WIN_DIR"
cd "$PROJECT_DIR"
echo "  → $RELEASE_DIR/repowiki-${VERSION}-windows-amd64.zip"

# 5. 完成
echo "[5/5] 完成"
echo ""
ls -lh "$RELEASE_DIR"/*.dmg "$RELEASE_DIR"/*.zip 2>/dev/null
echo ""
echo "上传到 GitHub Release 即可分发:"
echo "  Mac (Apple Silicon): RepoWiki-${VERSION}-arm64.dmg"
echo "  Mac (Intel):         RepoWiki-${VERSION}-amd64.dmg"
echo "  Windows:             repowiki-${VERSION}-windows-amd64.zip"
