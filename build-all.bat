@echo off
chcp 65001 >nul
echo ============================================
echo Gigabot Remote Updater - Build All
echo ============================================
echo.

REM Verificar que Go está instalado
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go no está instalado o no está en PATH
    exit /b 1
)

echo [1/4] Compilando Deployer (Windows)...
cd deployer
go build -o ..\deployer.exe main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando deployer
    exit /b 1
)
cd ..
echo [OK] deployer.exe creado

echo.
echo [2/4] Compilando Nexo (VPS Windows)...
cd nexo
go build -o ..\nexo.exe main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando nexo
    exit /b 1
)
cd ..
echo [OK] nexo.exe creado

echo.
echo [3/4] Compilando Updater (Mac ARM64)...
cd updater-mac
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -o ..\updater-mac main.go
if %errorlevel% neq 0 (
    echo [ERROR] Fallo compilando updater-mac
    exit /b 1
)
set GOOS=
set GOARCH=
set CGO_ENABLED=
cd ..
echo [OK] updater-mac creado (para Mac M1/M2/M3/M4)

echo.
echo [4/4] Generando claves Ed25519 (si no existen)...
if not exist deploy-private.key (
    echo Generando nuevo par de claves...
    cd keys
    go run genkeys.go
    if %errorlevel% neq 0 (
        echo [AVISO] No se pudieron generar claves automáticamente
        echo Usa: openssl genpkey -algorithm Ed25519 -out deploy-private.key
        echo      openssl pkey -in deploy-private.key -pubout -out deploy-public.key
    ) else (
        copy deploy-private.key ..\
        copy deploy-public.key ..\
    )
    cd ..
) else (
    echo [OK] Claves ya existen
)

echo.
echo ============================================
echo BUILD COMPLETADO
echo ============================================
echo.
echo Archivos generados:
echo   - deployer.exe      (para subir desde Windows/Mac)
echo   - nexo.exe          (para instalar en VPS Windows)
echo   - updater-mac       (para correr en Mac M4)
echo   - deploy-private.key (clave privada - GUARDAR SEGURA)
echo   - deploy-public.key  (clave pública - distribuir a VPS y Mac)
echo.
echo Próximos pasos:
echo 1. Copiar nexo.exe y deploy-public.key al VPS Windows
echo 2. En VPS: .
exo.exe (o instalar como servicio)
echo 3. En Mac: ./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot
echo 4. Desde dev: .\deployer.exe https://tu-vps:8443 TOKEN deploy-private.key
echo.
