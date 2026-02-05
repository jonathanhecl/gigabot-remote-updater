# Distribución de Archivos por Destino

## Resumen de Qué Va a Dónde

### 🖥️ VPS Windows (Nexo - El servidor)
**Ubicación:** Tu VPS Windows en la nube
**Archivos a copiar:**
```
C:\GigabotNexo\
├── nexo.exe              # Servidor HTTP que recibe/valida
├── deploy-public.key     # Clave pública para verificar firmas
└── config.json           # Configuración (token, puerto, etc.)
```

**Comandos:**
```powershell
mkdir C:\GigabotNexo
# Copiar los 3 archivos anteriores
.\nexo.exe  # Ejecutar
```

---

### 🍎 Mac M4 (Destino final - El que se actualiza automáticamente)
**Ubicación:** Tu Mac M4 local/remoto
**Archivos a copiar:**
```
~/gigabot/  (o /opt/gigabot/)
├── updater-mac         # Proceso wrapper que mantiene gigabot actualizado
├── deploy-public.key   # Clave pública para verificar descargas
└── gigabot            # El binario de tu app (será reemplazado automáticamente)
```

**Comandos:**
```bash
mkdir -p ~/gigabot
cd ~/gigabot
chmod +x updater-mac gigabot
./updater-mac https://tu-vps:8443 deploy-public.key ./gigabot
```

---

### 💻 Máquina de Desarrollo Windows
**Ubicación:** Tu PC Windows donde compilas
**Archivos necesarios:**
```
gigabot-remote-updater/
├── deployer.exe         # Ejecutable precompilado para Windows
└── deploy-private.key   # Clave privada (¡nunca compartir!)
```

**Para subir una actualización:**
```bash
cd gigabot-remote-updater
.\deployer.exe https://tu-vps:8443 TU-TOKEN deploy-private.key
```

---

### 🍎 Máquina de Desarrollo Mac M1/M2/M3/M4
**Ubicación:** Tu Mac donde compilas
**Archivos necesarios:**
```
gigabot-remote-updater/
├── deployer-mac         # Lo compilas tú (ver abajo)
└── deploy-private.key   # Clave privada (¡nunca compartir!)
```

**Compilar deployer en Mac (una sola vez):**
```bash
cd gigabot-remote-updater
./build-all.sh
```

**Para subir una actualización:**
```bash
cd gigabot-remote-updater
./deployer-mac https://tu-vps:8443 TU-TOKEN deploy-private.key
```

**Nota:** Si compilas desde Mac, el deployer-mac automáticamente compila para Mac ARM64 y lo sube al VPS.

---

## Flujo de Deploy Paso a Paso

### 1. Preparar VPS (una sola vez)
```powershell
# En el VPS Windows
mkdir C:\GigabotNexo
copy nexo.exe C:\GigabotNexo\
copy deploy-public.key C:\GigabotNexo\
# Crear config.json con tu token secreto
.\nexo.exe
```

### 2. Preparar Mac M4 (una sola vez)
```bash
# En el Mac M4
mkdir -p ~/gigabot
# Copiar updater-mac y deploy-public.key
# Copiar gigabot inicial
chmod +x ~/gigabot/*
~/gigabot/updater-mac https://tu-vps:8443 ~/gigabot/deploy-public.key ~/gigabot/gigabot
```

### 3. Subir Actualizaciones (cuando quieras actualizar)
```bash
# Desde tu máquina de desarrollo (Windows o Mac)
# El deployer compila gigabot para Mac, lo firma y lo sube al VPS
./deployer-mac https://tu-vps:8443 TU-TOKEN-SECRETO deploy-private.key   # En Mac
# o
.\deployer.exe https://tu-vps:8443 TU-TOKEN-SECRETO deploy-private.key  # En Windows
```

El Mac M4 automáticamente detectará la nueva versión en ~5 minutos y se actualizará solo.

---

## Notas Importantes

- **deploy-private.key**: SOLO en tu máquina de desarrollo (nunca en VPS ni Mac)
- **deploy-public.key**: En VPS y en Mac (para verificar)
- **updater-mac**: Corre como "wrapper" - lanza gigabot y lo mantiene actualizado
- **No necesitas tocar el Mac M4** para actualizar, todo es automático después del primer setup
