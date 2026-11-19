#!/bin/bash
# Instala o actualiza Folio en una Mac y lo deja corriendo.
#
#   curl -fsSL https://github.com/Saul-Punybz/folio-pr/releases/latest/download/instalar.sh | bash
#
# O con el zip a mano:  bash instalar.sh ~/Downloads/Folio.zip
#
# Qué hace: comprueba el chip, baja Folio.zip (curl no le pone la marca de
# cuarentena, así que Gatekeeper no lo bloquea), cierra el Folio que esté
# abierto, reemplaza la app (los datos en Application Support/Folio2 no se
# tocan), lo deja arrancando solo al iniciar sesión y lo abre.
#
# Variables para probar sin tocar la Mac real:
#   FOLIO_DEST=<carpeta>   dónde copiar Folio.app
#   FOLIO_NO_START=1       no cierra, no instala arranque, no abre
set -euo pipefail

URL="${FOLIO_ZIP_URL:-https://github.com/Saul-Punybz/folio-pr/releases/latest/download/Folio.zip}"
LABEL="pr.folio.app"
AGENT="$HOME/Library/LaunchAgents/$LABEL.plist"
APPURL="http://127.0.0.1:47615"

ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
fail() { printf '\n  \033[31m✗ %s\033[0m\n\n' "$1"; exit 1; }

echo
echo "  Instalando Folio…"
echo

# 1. Chip y sistema
[ "$(uname -s)" = "Darwin" ] || fail "Esto es solo para Mac."
[ "$(uname -m)" = "arm64" ] || fail "Folio necesita una Mac con chip de Apple (M1 o más nuevo). Esta es Intel."
ok "Mac con chip de Apple, macOS $(sw_vers -productVersion)"

# 2. Conseguir el zip
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
if [ "${1:-}" != "" ]; then
  [ -f "$1" ] || fail "No encuentro $1"
  cp "$1" "$TMP/Folio.zip"
  ok "Usando $1"
else
  curl -fL --progress-bar -o "$TMP/Folio.zip" "$URL" || fail "No se pudo bajar Folio. Revisa la conexión a internet."
  ok "Descargado"
fi
ditto -x -k "$TMP/Folio.zip" "$TMP/x" || fail "El zip está dañado. Bájalo otra vez."
NEW="$(find "$TMP/x" -maxdepth 2 -name 'Folio.app' -type d | head -1)"
[ -n "$NEW" ] || fail "El zip no trae Folio.app"
xattr -cr "$NEW"
codesign --verify --deep "$NEW" 2>/dev/null || fail "La app vino incompleta. Bájala otra vez."

# 3. Destino: /Applications si se puede escribir, si no ~/Applications
if [ -n "${FOLIO_DEST:-}" ]; then
  DEST="$FOLIO_DEST"
elif [ -w /Applications ]; then
  DEST="/Applications"
else
  DEST="$HOME/Applications"
fi
mkdir -p "$DEST"
APP="$DEST/Folio.app"
BIN="$APP/Contents/MacOS/Folio"

# 4. Cerrar el Folio abierto (para actualizar sin pisar la base)
if [ -z "${FOLIO_NO_START:-}" ]; then
  launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
  if pgrep -f "Folio.app/Contents/MacOS/Folio" >/dev/null; then
    pkill -TERM -f "Folio.app/Contents/MacOS/Folio" || true
    for _ in $(seq 1 30); do
      pgrep -f "Folio.app/Contents/MacOS/Folio" >/dev/null || break
      sleep 1
    done
    pgrep -f "Folio.app/Contents/MacOS/Folio" >/dev/null && fail "Folio no se quiso cerrar. Ciérralo en Ajustes → Cerrar Folio y corre esto otra vez."
    ok "Folio cerrado para actualizar"
  fi
fi

# 5. Copiar
rm -rf "$APP"
ditto "$NEW" "$APP"
xattr -cr "$APP"
ok "Instalado en $APP"

if [ -n "${FOLIO_NO_START:-}" ]; then
  echo
  echo "  (FOLIO_NO_START: no se arranca)"
  exit 0
fi

# 6. Arranque solo al iniciar sesión (en segundo plano, sin abrir navegador).
#    Se reinicia solo si se cae; "Cerrar Folio" lo deja cerrado.
mkdir -p "$HOME/Library/LaunchAgents"
cat > "$AGENT" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key>
  <array><string>$BIN</string><string>app</string></array>
  <key>EnvironmentVariables</key>
  <dict><key>FOLIO_NO_BROWSER</key><string>1</string></dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>ThrottleInterval</key><integer>30</integer>
  <key>ProcessType</key><string>Background</string>
</dict>
</plist>
EOF
plutil -lint -s "$AGENT" || fail "No se pudo preparar el arranque automático."
launchctl bootstrap "gui/$(id -u)" "$AGENT" || fail "No se pudo activar el arranque automático."
ok "Arranca solo al encender la Mac"

# 7. Esperar a que responda y abrirlo
printf '  … esperando a Folio (la primera vez tarda hasta un minuto)'
for _ in $(seq 1 90); do
  if curl -s -o /dev/null --max-time 2 "$APPURL/"; then
    echo
    ok "Folio está corriendo"
    open "$APPURL"
    echo
    echo "  Listo. Se abrió el navegador en $APPURL"
    echo "  Si es la primera vez, crea tu acceso con tu correo y una contraseña."
    echo
    exit 0
  fi
  printf '.'
  sleep 1
done
echo
fail "Folio no respondió. El registro está en ~/Library/Application Support/Folio2/logs/folio.log"
