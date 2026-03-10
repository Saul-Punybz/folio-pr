#!/bin/bash
set -euo pipefail

# Build Folio.app — Self-contained macOS application
# Bundles: Go binary (with embedded frontend+migrations) + PostgreSQL + pgvector
#
# Requirements (build machine only):
#   brew install postgresql@17 pgvector node
#
# Usage:
#   bash scripts/build-macos.sh
#
# Output:
#   build/Folio.dmg

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_DIR/build"
APP_DIR="$BUILD_DIR/Folio.app"
CONTENTS="$APP_DIR/Contents"
MACOS="$CONTENTS/MacOS"
RESOURCES="$CONTENTS/Resources"
PG_BUNDLE="$RESOURCES/pg"

echo "╔══════════════════════════════════════╗"
echo "║     Building Folio.app for macOS     ║"
echo "╚══════════════════════════════════════╝"

# Clean previous build
rm -rf "$BUILD_DIR"
mkdir -p "$MACOS" "$RESOURCES" "$PG_BUNDLE"/{bin,lib,share}

# ── Step 1: Build Frontend ────────────────────────────────────
echo ""
echo "▸ Step 1/5: Building frontend..."
cd "$PROJECT_DIR/frontend"
npm run build --silent 2>&1
cd "$PROJECT_DIR"
echo "  ✓ Frontend built"

# ── Step 2: Build Go Binary ──────────────────────────────────
echo ""
echo "▸ Step 2/5: Building Go binary (arm64)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
    -ldflags="-s -w" \
    -o "$MACOS/Folio" \
    ./cmd/app
echo "  ✓ Binary built ($(du -h "$MACOS/Folio" | awk '{print $1}'))"

# ── Step 3: Bundle PostgreSQL ────────────────────────────────
echo ""
echo "▸ Step 3/5: Bundling PostgreSQL..."

# Find PostgreSQL from Homebrew
PG_PREFIX=""
for ver in 17 16 15; do
    candidate="/opt/homebrew/opt/postgresql@${ver}"
    if [ -d "$candidate" ]; then
        PG_PREFIX="$candidate"
        echo "  Found PostgreSQL @${ver} at $PG_PREFIX"
        break
    fi
    candidate="/usr/local/opt/postgresql@${ver}"
    if [ -d "$candidate" ]; then
        PG_PREFIX="$candidate"
        echo "  Found PostgreSQL @${ver} at $PG_PREFIX"
        break
    fi
done

if [ -z "$PG_PREFIX" ]; then
    echo "  ✗ PostgreSQL not found! Install with: brew install postgresql@17"
    exit 1
fi

# Copy essential PG binaries
for bin in postgres initdb pg_ctl pg_isready createdb psql; do
    if [ -f "$PG_PREFIX/bin/$bin" ]; then
        cp "$PG_PREFIX/bin/$bin" "$PG_BUNDLE/bin/"
    fi
done

# Copy PG libraries
cp "$PG_PREFIX/lib/"*.dylib "$PG_BUNDLE/lib/" 2>/dev/null || true
if [ -d "$PG_PREFIX/lib/postgresql" ]; then
    cp -R "$PG_PREFIX/lib/postgresql" "$PG_BUNDLE/lib/"
fi

# Copy PG share data (needed by initdb)
PG_SHARE=""
for d in "$PG_PREFIX/share/postgresql@"* "$PG_PREFIX/share/postgresql" "$PG_PREFIX/share"; do
    if [ -d "$d" ] && [ -f "$d/postgres.bki" ] 2>/dev/null || [ -d "$d/timezone" ] 2>/dev/null; then
        PG_SHARE="$d"
        break
    fi
done
if [ -n "$PG_SHARE" ]; then
    cp -R "$PG_SHARE" "$PG_BUNDLE/share/postgresql"
else
    # Try the versioned share
    PG_VER=$(basename "$PG_PREFIX" | sed 's/postgresql@//')
    if [ -d "$PG_PREFIX/share/postgresql@${PG_VER}" ]; then
        cp -R "$PG_PREFIX/share/postgresql@${PG_VER}" "$PG_BUNDLE/share/postgresql"
    fi
fi

# Copy pgvector extension if available
PGVECTOR_LIB=$(find /opt/homebrew /usr/local -name "vector.dylib" -path "*/postgresql*" 2>/dev/null | head -1)
if [ -n "$PGVECTOR_LIB" ]; then
    mkdir -p "$PG_BUNDLE/lib/postgresql/"
    cp "$PGVECTOR_LIB" "$PG_BUNDLE/lib/postgresql/"
    echo "  ✓ pgvector bundled"
fi

# Copy pgvector SQL files
PGVECTOR_SQL_DIR=$(dirname "$PGVECTOR_LIB" 2>/dev/null)/../share/*/extension 2>/dev/null
for ext_dir in /opt/homebrew/share/postgresql@*/extension /usr/local/share/postgresql@*/extension; do
    if [ -d "$ext_dir" ]; then
        mkdir -p "$PG_BUNDLE/share/postgresql/extension/"
        cp "$ext_dir"/vector* "$PG_BUNDLE/share/postgresql/extension/" 2>/dev/null || true
        break
    fi
done

echo "  ✓ PostgreSQL bundled"

# ── Step 4: Fix Dynamic Library Paths ────────────────────────
echo ""
echo "▸ Step 4/5: Fixing library paths..."

# Collect all Homebrew dependencies
collect_deps() {
    local binary="$1"
    otool -L "$binary" 2>/dev/null | grep -o '/opt/homebrew[^ ]*\|/usr/local[^ ]*' | grep '\.dylib' || true
}

# Copy all dependency dylibs and fix references
fix_binary() {
    local binary="$1"
    local lib_dest="$PG_BUNDLE/lib"

    for dep in $(collect_deps "$binary"); do
        local dep_name=$(basename "$dep")
        if [ ! -f "$lib_dest/$dep_name" ]; then
            cp "$dep" "$lib_dest/$dep_name" 2>/dev/null || true
            chmod 755 "$lib_dest/$dep_name" 2>/dev/null || true
            # Recursively fix this library's deps too
            fix_binary "$lib_dest/$dep_name"
        fi
        install_name_tool -change "$dep" "@executable_path/../Resources/pg/lib/$dep_name" "$binary" 2>/dev/null || true
    done

    # Set the rpath
    install_name_tool -add_rpath "@executable_path/../Resources/pg/lib" "$binary" 2>/dev/null || true
}

# Fix all PG binaries
for bin in "$PG_BUNDLE"/bin/*; do
    fix_binary "$bin"
done

# Fix all dylibs
for lib in "$PG_BUNDLE"/lib/*.dylib; do
    fix_binary "$lib"
done

# Fix dylibs in postgresql subdir
for lib in "$PG_BUNDLE"/lib/postgresql/*.dylib "$PG_BUNDLE"/lib/postgresql/*.so; do
    [ -f "$lib" ] && fix_binary "$lib"
done

echo "  ✓ Library paths fixed"

# ── Step 5: Create .app Bundle Metadata ──────────────────────
echo ""
echo "▸ Step 5/5: Creating app bundle..."

# Info.plist
cat > "$CONTENTS/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleExecutable</key>
    <string>Folio</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>bz.puny.folio</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Folio</string>
    <key>CFBundleDisplayName</key>
    <string>Folio</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSArchitecturePriority</key>
    <array>
        <string>arm64</string>
    </array>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2026 Puny.bz. All rights reserved.</string>
</dict>
</plist>
PLIST

# Create a simple app icon (blue circle with F)
# For production, replace with proper .icns file
if command -v sips &>/dev/null; then
    # Create a 512x512 icon using Python
    python3 -c "
from PIL import Image, ImageDraw, ImageFont
import sys

try:
    img = Image.new('RGBA', (512, 512), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    # Draw rounded rectangle background
    draw.rounded_rectangle([20, 20, 492, 492], radius=80, fill=(88, 80, 236))
    # Draw letter F
    try:
        font = ImageFont.truetype('/System/Library/Fonts/SFCompact.ttf', 280)
    except:
        font = ImageFont.load_default()
    draw.text((256, 240), 'F', fill='white', font=font, anchor='mm')
    img.save('$RESOURCES/icon.png')
    print('Icon created')
except ImportError:
    # No PIL, create a placeholder
    sys.exit(1)
" 2>/dev/null || echo "  (Pillow not installed, skipping icon generation)"
fi

# ── Create DMG ───────────────────────────────────────────────
echo ""
echo "▸ Creating DMG..."

DMG_PATH="$BUILD_DIR/Folio.dmg"
hdiutil create \
    -volname "Folio" \
    -srcfolder "$APP_DIR" \
    -ov \
    -format UDZO \
    "$DMG_PATH" 2>/dev/null

echo ""
echo "╔══════════════════════════════════════╗"
echo "║            Build Complete!           ║"
echo "╚══════════════════════════════════════╝"
echo ""
echo "  App:  $APP_DIR"
echo "  DMG:  $DMG_PATH"
echo "  Size: $(du -sh "$APP_DIR" | awk '{print $1}')"
echo ""
echo "  To test: open \"$APP_DIR\""
echo "  To distribute: share $DMG_PATH"
echo ""
