@echo off
chcp 65001 >nul
echo ============================================
echo Gigabot Remote Updater - Build All
echo ============================================
echo.

REM Verificar que Go esta instalado
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go no esta instalado o no esta en PATH
    exit /b 1
)

echo [1/5] Compilando Deployer (Windows)...
go build -o deployer.exe ./deployer-src/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando deployer
    exit /b 1
)
echo [OK] deployer.exe creado

echo.
echo [2/5] Compilando Deployer (Mac M1/M2/M3/M4)...
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -o deployer-mac ./deployer-src/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando deployer-mac
    exit /b 1
)
set GOOS=
set GOARCH=
set CGO_ENABLED=
echo [OK] deployer-mac creado

echo.
echo [3/5] Compilando Nexo (VPS Windows)...
go build -o nexo.exe ./nexo-src/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando nexo
    exit /b 1
)
echo [OK] nexo.exe creado

echo.
echo [4/5] Compilando Updater (Mac ARM64)...
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -o updater-mac ./updater-src/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando updater-mac
    exit /b 1
)
set GOOS=
set GOARCH=
set CGO_ENABLED=
echo [OK] updater-mac creado (para Mac M1/M2/M3/M4)

echo.
echo [5/5] Generando claves Ed25519 (si no existen)...
if not exist deploy-private.key (
    echo Generando nuevo par de claves...
    go run ./keys-src/genkeys.go
    if %errorlevel% neq 0 (
        echo [AVISO] No se pudieron generar claves automaticamente
        echo Usa: openssl genpkey -algorithm Ed25519 -out deploy-private.key
        echo      openssl pkey -in deploy-private.key -pubout -out deploy-public.key
    )
) else (
    echo [OK] Claves ya existen
)

echo.
echo ============================================
echo BUILD COMPLETADO
echo ============================================
echo.
echo Archivos generados:
echo   - deployer.exe      (para subir desde Windows)
echo   - deployer-mac      (para subir desde Mac M1/M2/M3/M4)
echo   - nexo.exe          (para instalar en VPS Windows)
echo   - updater-mac       (para correr en Mac M4)
echo   - deploy-private.key (clave privada - GUARDAR SEGURA)
echo   - deploy-public.key  (clave publica - distribuir a VPS y Mac)
echo.
echo Proximos pasos:
echo 1. Copiar nexo.exe y deploy-public.key al VPS Windows
echo 2. En VPS: .\nexo.exe (o instalar como servicio)
echo 3. En Mac: ./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot
echo 4. Desde dev: .\deployer.exe https://tu-vps:8443 TOKEN deploy-private.key
echo.
pause
