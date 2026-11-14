#!/bin/bash
# Arma build/Folio.app (Apple Silicon) y build/Folio.zip.
#
# La app no depende de nada instalado en la Mac de destino: lleva dentro el
# binario de Folio (frontend + migraciones embebidos) y un Postgres 18
# relocalizable (binarios de zonky/EDB, solo @loader_path y /usr/lib|/System).
#
# Uso:   bash scripts/build-macos.sh        (o: make mac)
#
# Variables opcionales:
#   FOLIO_OUT=dir              carpeta de salida (por defecto build/)
#   FOLIO_SIGN_IDENTITY="Developer ID Application: …"   firmar con Developer ID
#   FOLIO_NOTARY_PROFILE=perfil  notarizar (xcrun notarytool store-credentials)
# Sin FOLIO_SIGN_IDENTITY se firma ad-hoc (Gatekeeper pedirá "Abrir igualmente").
#
# Requisitos de la máquina que compila: Go, curl, (npm solo si falta frontend/dist).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUT_DIR="${FOLIO_OUT:-$PROJECT_DIR/build}"
CACHE_DIR="$PROJECT_DIR/build/pg-cache"
STAGE="$OUT_DIR/.staging"
APP="$STAGE/Folio.app"
CONTENTS="$APP/Contents"
PG="$CONTENTS/Resources/pg"

# ── Postgres empaquetado (versiones y sumas fijas) ────────────────────
PG_VERSION="18.3.0"
ZONKY_JAR="embedded-postgres-binaries-darwin-arm64v8-$PG_VERSION.jar"
ZONKY_URL="https://repo1.maven.org/maven2/io/zonky/test/postgres/embedded-postgres-binaries-darwin-arm64v8/$PG_VERSION/$ZONKY_JAR"
ZONKY_SHA="a18ec9ec6a905e04f5a08a8f7870bddadd635feb54dee332a3daf80fada3cb1a"
# zonky solo trae initdb/pg_ctl/postgres; pg_dump (respaldos) sale del mismo
# build de EDB del que zonky saca los suyos (mismas libs, misma firma EDB).
EDB_ZIP="edb.zip"
EDB_URL="https://get.enterprisedb.com/postgresql/postgresql-18.3-1-osx-binaries.zip"
EDB_SHA="15ee2a45489eba14f8c7376b05f3df3892c07f36d6f8d344ec2a422df97a5e27"

APP_VERSION="${FOLIO_VERSION:-2.0.0}"

step() { echo ""; echo "▸ $*"; }

fetch() { # fetch <url> <archivo> <sha256>
    local url="$1" file="$CACHE_DIR/$2" sha="$3"
    if [ ! -f "$file" ] || [ "$(shasum -a 256 "$file" | awk '{print $1}')" != "$sha" ]; then
        echo "  descargando $(basename "$file")…"
        curl -fL --retry 3 -o "$file.part" "$url"
        mv "$file.part" "$file"
    fi
    local got
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
    if [ "$got" != "$sha" ]; then
        echo "  ✗ suma SHA-256 no coincide para $file ($got)"; exit 1
    fi
}

rm -rf "$STAGE"
mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources" "$PG" "$CACHE_DIR"

# ── 1. Frontend ───────────────────────────────────────────────────────
step "1/6 Frontend"
if [ -f "$PROJECT_DIR/frontend/dist/index.html" ] || [ -d "$PROJECT_DIR/frontend/dist/inbox" ]; then
    echo "  usando frontend/dist existente"
else
    (cd "$PROJECT_DIR/frontend" && npm install --silent && npm run build --silent)
fi

# ── 2. Binario Go ─────────────────────────────────────────────────────
step "2/6 Binario (darwin/arm64)"
(cd "$PROJECT_DIR" && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
    -trimpath -ldflags="-s -w" -o "$CONTENTS/MacOS/Folio" ./cmd/folio)
echo "  ✓ $(du -h "$CONTENTS/MacOS/Folio" | awk '{print $1}')"

# ── 3. Postgres ───────────────────────────────────────────────────────
step "3/6 Postgres $PG_VERSION"
fetch "$ZONKY_URL" "$ZONKY_JAR" "$ZONKY_SHA"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
unzip -q -o "$CACHE_DIR/$ZONKY_JAR" 'postgres-darwin-arm_64.txz' -d "$TMP"
tar -xJf "$TMP/postgres-darwin-arm_64.txz" -C "$PG"

if [ ! -f "$CACHE_DIR/pg_dump-$PG_VERSION" ]; then
    fetch "$EDB_URL" "$EDB_ZIP" "$EDB_SHA"
    unzip -q -o "$CACHE_DIR/$EDB_ZIP" 'pgsql/bin/pg_dump' -d "$TMP"
    cp "$TMP/pgsql/bin/pg_dump" "$CACHE_DIR/pg_dump-$PG_VERSION"
fi
install -m 755 "$CACHE_DIR/pg_dump-$PG_VERSION" "$PG/bin/pg_dump"

# Solo arm64: quita la mitad x86_64 de los binarios universales.
while IFS= read -r -d '' f; do
    if lipo -archs "$f" 2>/dev/null | grep -q x86_64; then
        lipo -thin arm64 "$f" -output "$f.thin" && mv "$f.thin" "$f"
    fi
done < <(find "$PG" -type f \( -perm +111 -o -name '*.dylib' -o -name '*.so' \) -print0)

# Ninguna referencia a Homebrew ni /usr/local.
if find "$PG" "$CONTENTS/MacOS" -type f \( -perm +111 -o -name '*.dylib' -o -name '*.so' \) \
        -exec otool -L {} \; 2>/dev/null | grep -qE "/opt/homebrew|/usr/local"; then
    echo "  ✗ hay binarios que apuntan a /opt/homebrew o /usr/local"; exit 1
fi
echo "  ✓ $(du -sh "$PG" | awk '{print $1}'), sin rutas de Homebrew"

# ── 4. Info.plist e ícono ─────────────────────────────────────────────
step "4/6 Info.plist"
cp "$SCRIPT_DIR/AppIcon.icns" "$CONTENTS/Resources/AppIcon.icns"
cat > "$CONTENTS/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>                 <string>Folio</string>
    <key>CFBundleDisplayName</key>          <string>Folio</string>
    <key>CFBundleIdentifier</key>           <string>pr.folio.app</string>
    <key>CFBundleExecutable</key>           <string>Folio</string>
    <key>CFBundleIconFile</key>             <string>AppIcon</string>
    <key>CFBundlePackageType</key>          <string>APPL</string>
    <key>CFBundleVersion</key>              <string>$APP_VERSION</string>
    <key>CFBundleShortVersionString</key>   <string>$APP_VERSION</string>
    <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
    <key>LSMinimumSystemVersion</key>       <string>13.0</string>
    <key>LSArchitecturePriority</key>       <array><string>arm64</string></array>
    <key>NSHighResolutionCapable</key>      <true/>
</dict>
</plist>
PLIST
printf 'APPL????' > "$CONTENTS/PkgInfo"

# ── 5. Firma (de adentro hacia afuera) ────────────────────────────────
step "5/6 Firma"
xattr -cr "$APP"
if [ -n "${FOLIO_SIGN_IDENTITY:-}" ]; then
    SIGN=(--force --timestamp --options runtime -s "$FOLIO_SIGN_IDENTITY")
    echo "  Developer ID: $FOLIO_SIGN_IDENTITY"
else
    SIGN=(--force -s -)
    echo "  ad-hoc (sin FOLIO_SIGN_IDENTITY)"
fi
# Librerías primero, luego ejecutables de Postgres, luego Folio, luego el bundle.
find "$PG" -type f \( -name '*.dylib' -o -name '*.so' \) -print0 | xargs -0 codesign "${SIGN[@]}"
find "$PG/bin" -type f -perm +111 -print0 | xargs -0 codesign "${SIGN[@]}"
codesign "${SIGN[@]}" "$CONTENTS/MacOS/Folio"
codesign "${SIGN[@]}" --deep "$APP"
codesign --verify --deep --strict "$APP"
echo "  ✓ firma verificada"

# ── 6. Salida: Folio.app + Folio.zip ──────────────────────────────────
step "6/6 Folio.app y Folio.zip"
rm -rf "$OUT_DIR/Folio.app" "$OUT_DIR/Folio.zip"
mv "$APP" "$OUT_DIR/Folio.app"
rm -rf "$STAGE"
ditto -c -k --keepParent "$OUT_DIR/Folio.app" "$OUT_DIR/Folio.zip"

if [ -n "${FOLIO_SIGN_IDENTITY:-}" ] && [ -n "${FOLIO_NOTARY_PROFILE:-}" ]; then
    echo "  notarizando…"
    xcrun notarytool submit "$OUT_DIR/Folio.zip" --keychain-profile "$FOLIO_NOTARY_PROFILE" --wait
    xcrun stapler staple "$OUT_DIR/Folio.app"
    rm -f "$OUT_DIR/Folio.zip"
    ditto -c -k --keepParent "$OUT_DIR/Folio.app" "$OUT_DIR/Folio.zip"
fi

echo ""
echo "  ✓ $OUT_DIR/Folio.app  ($(du -sh "$OUT_DIR/Folio.app" | awk '{print $1}'))"
echo "  ✓ $OUT_DIR/Folio.zip  ($(du -h "$OUT_DIR/Folio.zip" | awk '{print $1}'))"
