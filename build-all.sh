#!/bin/bash

echo "============================================"
echo "Gigabot Remote Updater - Build All (Mac)"
echo "============================================"
echo ""

# Verificar que Go está instalado
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go no está instalado o no está en PATH"
    exit 1
fi

# Guardar directorio del script
cd "$(dirname "$0")"

echo "[1/4] Compilando Deployer (Mac)..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o deployer-mac ./deployer-src/main.go
if [ $? -ne 0 ]; then
    echo "[ERROR] Fallo compilando deployer"
    exit 1
fi
echo "[OK] deployer-mac creado"

echo ""
echo "[2/4] Compilando Nexo (para VPS Windows - cross-compile)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o nexo.exe ./nexo-src/main.go
if [ $? -ne 0 ]; then
    echo "[ERROR] Fallo compilando nexo"
    exit 1
fi
echo "[OK] nexo.exe creado (para VPS Windows)"

echo ""
echo "[3/4] Compilando Updater (Mac ARM64)..."
go build -o updater-mac ./updater-src/main.go
if [ $? -ne 0 ]; then
    echo "[ERROR] Fallo compilando updater-mac"
    exit 1
fi
echo "[OK] updater-mac creado (para Mac M1/M2/M3/M4)"

echo ""
echo "[4/4] Generando claves Ed25519 (si no existen)..."
if [ ! -f deploy-private.key ]; then
    echo "Generando nuevo par de claves..."
    cd keys-src
    go run genkeys.go
    if [ $? -ne 0 ]; then
        echo "[AVISO] No se pudieron generar claves automáticamente"
        echo "Usa: openssl genpkey -algorithm Ed25519 -out deploy-private.key"
        echo "     openssl pkey -in deploy-private.key -pubout -out deploy-public.key"
    else
        cp deploy-private.key ../
        cp deploy-public.key ../
    fi
    cd ..
else
    echo "[OK] Claves ya existen"
fi

echo ""
echo "============================================"
echo "BUILD COMPLETADO"
echo "============================================"
echo ""
echo "Archivos generados:"
echo "  - deployer-mac        (para subir desde Mac M1/M2/M3/M4)"
echo "  - nexo.exe            (para instalar en VPS Windows)"
echo "  - updater-mac         (para correr en Mac M4 destino)"
echo "  - deploy-private.key  (clave privada - GUARDAR SEGURA)"
echo "  - deploy-public.key   (clave pública - distribuir a VPS y Mac)"
echo ""
echo "Próximos pasos:"
echo "1. Copiar nexo.exe, deploy-public.key y config.json al VPS Windows"
echo "2. En VPS: .\\nexo.exe (o instalar como servicio)"
echo "3. En Mac M4 destino: ./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot"
echo "4. Desde este Mac: ./deployer-mac https://tu-vps:8443 TU-TOKEN deploy-private.key"
echo ""
