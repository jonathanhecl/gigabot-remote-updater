#!/bin/bash

# Script de instalación del LaunchAgent para Gigabot Updater en macOS
# Uso: ./install-launchagent.sh [usuario]

set -e

USERNAME="${1:-$(whoami)}"
GIGABOT_DIR="$HOME/gigabot"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"
PLIST_NAME="com.gigabot.updater.plist"
PLIST_SOURCE="${2:-./$PLIST_NAME}"

echo "======================================"
echo "Instalador LaunchAgent - Gigabot Updater"
echo "======================================"
echo "Usuario: $USERNAME"
echo "Directorio: $GIGABOT_DIR"
echo ""

# Verificar que existe el plist fuente
if [ ! -f "$PLIST_SOURCE" ]; then
    echo "Error: No se encuentra $PLIST_SOURCE"
    echo "Asegúrate de estar en el directorio donde está el archivo .plist"
    exit 1
fi

# Crear directorios necesarios
echo "[1/4] Creando directorios..."
mkdir -p "$GIGABOT_DIR/logs"
mkdir -p "$LAUNCH_AGENTS_DIR"

# Verificar que existe updater-mac
if [ ! -f "$GIGABOT_DIR/updater-mac" ]; then
    echo "[AVISO] updater-mac no encontrado en $GIGABOT_DIR"
    echo "Copia updater-mac y deploy-public.key antes de continuar:"
    echo "  cp updater-mac deploy-public.key $GIGABOT_DIR/"
fi

# Verificar que existe deploy-public.key
if [ ! -f "$GIGABOT_DIR/deploy-public.key" ]; then
    echo "[AVISO] deploy-public.key no encontrado en $GIGABOT_DIR"
fi

# Copiar y modificar el plist
echo "[2/4] Configurando LaunchAgent..."
PLIST_TEMP="/tmp/$PLIST_NAME"

# Reemplazar USERNAME por el usuario real
sed "s/USERNAME/$USERNAME/g" "$PLIST_SOURCE" > "$PLIST_TEMP"

# Copiar a LaunchAgents
cp "$PLIST_TEMP" "$LAUNCH_AGENTS_DIR/$PLIST_NAME"
rm "$PLIST_TEMP"

# Establecer permisos correctos
chmod 644 "$LAUNCH_AGENTS_DIR/$PLIST_NAME"

echo "[3/4] Cargando servicio..."

# Descargar si ya estaba cargado
launchctl unload "$LAUNCH_AGENTS_DIR/$PLIST_NAME" 2>/dev/null || true

# Cargar el nuevo servicio
launchctl load "$LAUNCH_AGENTS_DIR/$PLIST_NAME"

echo "[4/4] Verificando instalación..."
if launchctl list | grep -q "com.gigabot.updater"; then
    echo "✅ LaunchAgent instalado y cargado correctamente"
    echo ""
    echo "Estado del servicio:"
    launchctl list | grep "com.gigabot.updater"
else
    echo "⚠️  El servicio no aparece en launchctl list"
    echo "   Intenta reiniciar o revisa los logs en ~/gigabot/logs/"
fi

echo ""
echo "======================================"
echo "Instalación completada"
echo "======================================"
echo ""
echo "Comandos útiles:"
echo "  launchctl list | grep gigabot     # Ver estado"
echo "  tail -f ~/gigabot/logs/updater.log  # Ver logs"
echo "  launchctl unload ~/Library/LaunchAgents/$PLIST_NAME  # Detener"
echo "  launchctl load ~/Library/LaunchAgents/$PLIST_NAME   # Iniciar"
echo ""
