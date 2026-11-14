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
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
    -ldflags="-s -w" \
    -tags "desktop,production" \
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

# Copy ICU data dylibs. PostgreSQL's ICU libs reference these via @loader_path,
# so they won't appear in the absolute-path dependency scan below.
for icu_lib in /opt/homebrew/opt/icu4c@*/lib/libicudata*.dylib /usr/local/opt/icu4c@*/lib/libicudata*.dylib; do
    if [ -f "$icu_lib" ]; then
        cp -L "$icu_lib" "$PG_BUNDLE/lib/" 2>/dev/null || true
    fi
done

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
    otool -L "$binary" 2>/dev/null | tail -n +2 | awk '{print $1}' | grep -E '^(/opt/homebrew|/usr/local).+\.dylib$' || true
}

# Copy all dependency dylibs and fix references
fix_binary() {
    local binary="$1"
    local lib_dest="$PG_BUNDLE/lib"
    local loader_ref="@loader_path/../lib"

    if [ "$(dirname "$binary")" = "$lib_dest" ]; then
        loader_ref="@loader_path"
    fi

    for dep in $(collect_deps "$binary"); do
        local dep_name=$(basename "$dep")
        local dep_target="$lib_dest/$dep_name"
        local dep_ref="$loader_ref/$dep_name"
        if [ ! -f "$dep_target" ]; then
            cp -L "$dep" "$dep_target" 2>/dev/null || true
            chmod 755 "$dep_target" 2>/dev/null || true
            install_name_tool -id "@loader_path/$dep_name" "$dep_target" 2>/dev/null || true
            # Rewrite the parent reference before recursing so nested calls can't
            # contaminate the current dependency target via bash's dynamic scope.
            install_name_tool -change "$dep" "$dep_ref" "$binary" 2>/dev/null || true
            # Recursively fix this library's deps too
            fix_binary "$dep_target"
            continue
        fi
        # Use @loader_path so PG binaries resolve libs relative to themselves
        install_name_tool -change "$dep" "$dep_ref" "$binary" 2>/dev/null || true
    done

    # Set the rpath
    install_name_tool -add_rpath "@loader_path/../lib" "$binary" 2>/dev/null || true
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

# Ad-hoc codesign all bundled binaries and libraries
echo "  Signing bundled binaries..."
for bin in "$PG_BUNDLE"/bin/*; do
    codesign --force -s - "$bin" 2>/dev/null || true
done
for lib in "$PG_BUNDLE"/lib/*.dylib "$PG_BUNDLE"/lib/postgresql/*.dylib; do
    [ -f "$lib" ] && codesign --force -s - "$lib" 2>/dev/null || true
done
echo "  ✓ Binaries signed"

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

# Copy app icon
ICON_SRC="$SCRIPT_DIR/AppIcon.icns"
if [ -f "$ICON_SRC" ]; then
    cp "$ICON_SRC" "$RESOURCES/AppIcon.icns"
    echo "  ✓ App icon copied"
else
    echo "  (No AppIcon.icns found, skipping)"
fi

# ── Create DMG ───────────────────────────────────────────────
echo ""
echo "▸ Creating DMG..."

DMG_PATH="$BUILD_DIR/Folio.dmg"
DMG_CREATED=0
if command -v hdiutil >/dev/null 2>&1; then
    if hdiutil create \
        -volname "Folio" \
        -srcfolder "$APP_DIR" \
        -ov \
        -format UDZO \
        "$DMG_PATH" 2>/dev/null; then
        DMG_CREATED=1
    else
        echo "  ! DMG creation failed; app bundle is still ready at $APP_DIR"
    fi
else
    echo "  ! hdiutil not available; skipping DMG creation"
fi

echo ""
echo "╔══════════════════════════════════════╗"
echo "║            Build Complete!           ║"
echo "╚══════════════════════════════════════╝"
echo ""
echo "  App:  $APP_DIR"
if [ "$DMG_CREATED" -eq 1 ]; then
    echo "  DMG:  $DMG_PATH"
else
    echo "  DMG:  skipped"
fi
echo "  Size: $(du -sh "$APP_DIR" | awk '{print $1}')"
echo ""
echo "  To test: open \"$APP_DIR\""
if [ "$DMG_CREATED" -eq 1 ]; then
    echo "  To distribute: share $DMG_PATH"
fi
echo ""
