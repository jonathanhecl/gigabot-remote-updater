# Gigabot Remote Updater

Sistema de actualización remota para Gigabot: compila en Windows/Mac → sube a VPS Windows → Mac M4 descarga y actualiza automáticamente.

**Arquitectura:** 3 apps standalone que no modifican el proyecto Gigabot original.

```
gigabot-remote-updater/
├── deployer-src/          # Código fuente del deployer
├── nexo-src/              # Código fuente del servidor VPS
├── updater-src/           # Código fuente del updater para Mac
├── keys-src/              # Generador de claves Ed25519
├── build-all.bat          # Script para compilar todo (Windows)
├── build-all.sh           # Script para compilar todo (Mac/Linux)
├── deployer.exe           # Binario Windows (generado)
├── deployer-mac           # Binario Mac (generado)
├── nexo.exe               # Binario VPS Windows (generado)
└── updater-mac            # Binario Mac (generado)
```

---

## Componentes

### 1. Deployer (Windows)
Compila Gigabot para Mac (arm64), lo firma con Ed25519 y lo sube al VPS.

**Uso:**
```bash
# Desde la raíz del proyecto
go build -o deployer.exe ./deployer-src/main.go

# Ejecutar deploy
.\deployer.exe https://tu-vps.com:8443 TU-TOKEN deploy-private.key
```

### 2. Nexo (VPS Windows)
Servidor HTTP que recibe binarios, valida firma Ed25519 + checksum, y sirve actualizaciones.

**Configuración:**
Crea un archivo `config.json` en el mismo directorio que `nexo.exe`:
```json
{
  "token": "tu-token-ultra-secreto-minimo-32-caracteres",
  "public_key_path": "deploy-public.key",
  "port": "8443",
  "storage_dir": "./storage"
}
```

**Instalación en VPS:**
```powershell
# Crear directorio
mkdir C:\GigabotNexo
cd C:\GigabotNexo

# Copiar archivos (desde tu máquina de desarrollo)
copy \\ruta\nexo.exe .
copy \\ruta\deploy-public.key .
copy \\ruta\config.json .

# Ejecutar directamente (para probar)
.\nexo.exe

# O instalar como servicio de Windows
New-Service -Name "GigabotNexo" -BinaryPathName "C:\GigabotNexo\nexo.exe" -StartupType Automatic
Start-Service -Name "GigabotNexo"

# Ver logs
Get-Content C:\GigabotNexo\logs\nexo.log -Tail 20 -Wait
```

**Variables de entorno (fallback opcional):**
Si prefieres no usar `config.json`, el nexo puede cargar la configuración desde estas variables:
- `NEXO_TOKEN` - Token de autenticación
- `NEXO_PUBLIC_KEY` - Ruta a la clave pública
- `NEXO_PORT` - Puerto (default: 8443)
- `NEXO_STORAGE` - Directorio de storage (default: ./storage)
- `NEXO_CONFIG` - Ruta alternativa al config.json (si quieres otro nombre/ubicación)

**Endpoints:**
- `POST /upload` - Recibe binario firmado (token + firma requeridos)
- `GET /latest` - Retorna metadata de última versión
- `GET /download` - Descarga el binario
- `GET /health` - Health check

### 3. Updater (Mac M4)

El `updater-mac` es un ejecutable que llevas al Mac M4 (sí, puede ir en un pendrive). Su trabajo es:

1. **Lanzar Gigabot** como proceso hijo
2. **Esperar 5 minutos** y consultar al VPS si hay versión nueva
3. **Si hay update**: descarga → verifica que es legítimo → mata el gigabot viejo → reemplaza el archivo → reinicia

**Anatomía simple:**
```
updater-mac (wrapper/padre)
    └── gigabot (hijo, tu app)
         (cada 5 minutos el padre chequea si hay update)
```

**Instalación práctica (desde pendrive o scp):**

```bash
# En el Mac M4, abrir terminal
mkdir -p ~/gigabot
cd ~/gigabot

# Copiar archivos (desde pendrive, scp, o como prefieras)
# Necesitas: updater-mac + deploy-public.key + gigabot (tu app)
cp /Volumes/MiPendrive/updater-mac .
cp /Volumes/MiPendrive/deploy-public.key .
cp /ruta/a/tu/gigabot .

# Hacer ejecutables
chmod +x updater-mac gigabot

# Ejecutar (esto mantiene todo corriendo y actualizado)
./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot
```

**¿Qué pasa después?**
- El updater queda corriendo en primer plano (o en background si usas `&`)
- Mantiene gigabot vivo (si se cae, lo reinicia)
- Cada 5 minutos pregunta al VPS: "¿Hay versión nueva?"
- Si hay: lo descarga, verifica la firma, y actualiza automáticamente
- **Tú no haces nada más en el Mac M4**, todo es automático

**Para dejarlo corriendo permanentemente (LaunchAgent):**
```bash
# Crear archivo de servicio
mkdir -p ~/Library/LaunchAgents
cat > ~/Library/LaunchAgents/com.gigabot.updater.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gigabot.updater</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/tuusuario/gigabot/updater-mac</string>
        <string>https://tu-vps:8443</string>
        <string>/Users/tuusuario/gigabot/deploy-public.key</string>
        <string>/Users/tuusuario/gigabot/gigabot</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF

# Activar
launchctl load ~/Library/LaunchAgents/com.gigabot.updater.plist
```

Ahora el updater se inicia automáticamente cuando enciendes el Mac.

---

## Seguridad (5 Capas)

1. **Token de autenticación**: Solo desarrolladores autorizados pueden subir
2. **Firma Ed25519**: Cada binario va firmado, el VPS y Mac verifican
3. **Checksum SHA256**: Integridad del archivo verificada
4. **Sandbox**: Descarga a temp primero, verificación completa antes de reemplazar
5. **HTTPS**: Usar reverse proxy (nginx/caddy) con Let's Encrypt en producción

---

## Setup Completo

### Paso 1: Generar Claves (una vez)

**Opción A - Automática (desde este directorio):**
```bash
cd gigabot-remote-updater
./build-all.sh        # En Mac/Linux
# o
.\build-all.bat       # En Windows
```

**Opción B - OpenSSL (recomendado para producción):**
```bash
openssl genpkey -algorithm Ed25519 -out deploy-private.key
openssl pkey -in deploy-private.key -pubout -out deploy-public.key
```

Guarda `deploy-private.key` en tu máquina de desarrollo (¡nunca la compartas!).
Copia `deploy-public.key` al VPS y al Mac.

### Paso 2: Compilar Todo

Desde la raíz del proyecto:
```bash
cd gigabot-remote-updater
.\build-all.bat       # En Windows
# o
./build-all.sh        # En Mac/Linux
```

Esto genera:
- `deployer.exe`   - Para subir actualizaciones (Windows)
- `deployer-mac`   - Para subir actualizaciones (Mac)
- `nexo.exe`       - Para el VPS Windows
- `updater-mac`    - Para el Mac (compilado para arm64)

### Paso 3: Instalar en VPS Windows

Ver sección "Nexo (VPS Windows)" arriba.

### Paso 4: Instalar en Mac M4

```bash
# En el Mac, crear estructura
mkdir -p ~/gigabot
cd ~/gigabot

# Copiar archivos (desde tu máquina)
scp updater-mac deploy-public.key .
scp ~/tu-gigabot-compilado ./gigabot  # o compilar directo en Mac

# Hacer ejecutables
chmod +x updater-mac gigabot

# Iniciar (usar screen, tmux o launchd para mantenerlo corriendo)
./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot
```

**Opcional - LaunchAgent (auto-inicio en Mac):**
```xml
<!-- ~/Library/LaunchAgents/com.gigabot.updater.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gigabot.updater</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/tuusuario/gigabot/updater-mac</string>
        <string>https://tu-vps:8443</string>
        <string>/Users/tuusuario/gigabot/deploy-public.key</string>
        <string>/Users/tuusuario/gigabot/gigabot</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

```bash
launchctl load ~/Library/LaunchAgents/com.gigabot.updater.plist
```

### Paso 5: Deploy

Cuando quieras actualizar Gigabot en el Mac:

```bash
# Desde tu máquina de desarrollo Windows
.\deployer.exe https://tu-vps:8443 TU-TOKEN-SUPER-SECRETO deploy-private.key

# Desde tu máquina de desarrollo Mac
./deployer-mac https://tu-vps:8443 TU-TOKEN-SUPER-SECRETO deploy-private.key
```

El Mac automáticamente:
1. Detectará la nueva versión (dentro de 5 minutos, o configurable)
2. Descargará y verificará firma + checksum
3. Matará el proceso gigabot actual
4. Reemplazará el binario
5. Reiniciará con la nueva versión

---

## Troubleshooting

### "Firma inválida" en el VPS
- Verifica que usaste la clave privada correcta al firmar
- Verifica que el VPS tiene la clave pública correcta

### "Permission denied" en Mac
```bash
# Quitar atributos extendidos que bloquean ejecución
xattr -c ~/gigabot/gigabot
xattr -c ~/gigabot/updater-mac
```

### No se actualiza automáticamente
- Verifica que el updater está corriendo: `ps aux | grep updater`
- Revisa logs del updater (se imprimen a stdout)
- Verifica conectividad al VPS: `curl https://tu-vps:8443/health`

### VPS no recibe el upload
- Verifica firewall (puerto 8443 abierto)
- Verifica token: debe coincidir exactamente
- Revisa logs en VPS: `Get-Content C:\GigabotNexo\logs\nexo.log`

---

## Notas de Desarrollo

- El deployer compila automáticamente con `GOOS=darwin GOARCH=arm64`
- El updater usa polling cada 5 minutos (modificable en código)
- Cada versión es identificada por timestamp: `YYYYMMDD-HHMMSS`
- Rollback automático si la nueva versión no inicia
