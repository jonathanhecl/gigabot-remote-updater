#!/bin/bash

# Script para iniciar el updater de Gigabot en Mac
# Colocar en ~/gigabot/start-gigabot.sh

set -e

GIGABOT_DIR="$HOME/gigabot"
VPS_HOST="${1:-https://tu-vps:8443}"
PUBLIC_KEY="${2:-deploy-public.key}"
GIGABOT_BIN="${3:-./gigabot}"

cd "$GIGABOT_DIR"

# Verificar que existe updater-mac
if [ ! -f "./updater-mac" ]; then
    echo "Error: updater-mac no encontrado en $GIGABOT_DIR"
    exit 1
fi

# Verificar que existe la clave pública
if [ ! -f "$PUBLIC_KEY" ]; then
    echo "Error: $PUBLIC_KEY no encontrado en $GIGABOT_DIR"
    exit 1
fi

# Poner permisos ejecutables
chmod +x ./updater-mac

# Si existe gigabot, ponerle permisos también
if [ -f "$GIGABOT_BIN" ]; then
    chmod +x "$GIGABOT_BIN"
fi

echo "Iniciando Gigabot Updater..."
echo "VPS: $VPS_HOST"
echo "Binario: $GIGABOT_BIN"
echo ""

# Limpiar atributos extendidos que pueden bloquear ejecución
xattr -c ./updater-mac 2>/dev/null || true
xattr -c "$GIGABOT_BIN" 2>/dev/null || true

# Iniciar el updater (descargará gigabot si no existe)
exec ./updater-mac "$VPS_HOST" "$PUBLIC_KEY" "$GIGABOT_BIN"
