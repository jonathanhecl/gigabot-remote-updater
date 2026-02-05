package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Config struct {
	VpsHost       string
	CheckInterval time.Duration
	GigabotPath   string
	TempDir       string
}

type Metadata struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	Checksum  string `json:"checksum"`
	Platform  string `json:"platform"`
	Signature string `json:"signature"`
}

type Updater struct {
	config     Config
	publicKey  ed25519.PublicKey
	currentVer string
	gigabotCmd *exec.Cmd
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Uso: updater-mac <vps-host> <public-key-file> <gigabot-path>")
		fmt.Println("Ejemplo: updater-mac https://tu-vps.com:8443 deploy-public.key ./gigabot")
		os.Exit(1)
	}

	publicKeyPEM, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error leyendo clave pública: %v\n", err)
		os.Exit(1)
	}

	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parseando clave pública: %v\n", err)
		os.Exit(1)
	}

	updater := &Updater{
		config: Config{
			VpsHost:       os.Args[1],
			CheckInterval: 5 * time.Minute,
			GigabotPath:   os.Args[3],
			TempDir:       os.TempDir(),
		},
		publicKey:  publicKey,
		currentVer: "",
	}

	fmt.Println("Updater Mac iniciado")
	fmt.Printf("VPS: %s\n", updater.config.VpsHost)
	fmt.Printf("Gigabot: %s\n", updater.config.GigabotPath)
	fmt.Printf("Intervalo de chequeo: %s\n", updater.config.CheckInterval)

	if err := updater.run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error fatal: %v\n", err)
		os.Exit(1)
	}
}

func (u *Updater) run() error {
	for {
		needsUpdate, metadata, err := u.checkUpdate()
		if err != nil {
			fmt.Printf("Error chequeando actualización: %v\n", err)
			time.Sleep(u.config.CheckInterval)
			continue
		}

		if !needsUpdate {
			fmt.Printf("Versión actual (%s) es la última. Esperando...\n", u.currentVer)

			if u.gigabotCmd == nil || (u.gigabotCmd.ProcessState != nil && u.gigabotCmd.ProcessState.Exited()) {
				if err := u.startGigabot(); err != nil {
					fmt.Printf("Error iniciando Gigabot: %v\n", err)
				}
			}

			time.Sleep(u.config.CheckInterval)
			continue
		}

		fmt.Printf("Nueva versión disponible: %s (actual: %s)\n", metadata.Version, u.currentVer)

		if err := u.downloadAndUpdate(metadata); err != nil {
			fmt.Printf("Error actualizando: %v\n", err)
			time.Sleep(u.config.CheckInterval)
			continue
		}

		u.currentVer = metadata.Version
		fmt.Printf("Actualización a %s completada exitosamente\n", metadata.Version)

		time.Sleep(u.config.CheckInterval)
	}
}

func (u *Updater) checkUpdate() (bool, *Metadata, error) {
	resp, err := http.Get(u.config.VpsHost + "/latest")
	if err != nil {
		return false, nil, fmt.Errorf("error consultando VPS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("error HTTP %d", resp.StatusCode)
	}

	var metadata Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return false, nil, fmt.Errorf("error decodificando metadata: %w", err)
	}

	if u.currentVer == "" {
		return true, &metadata, nil
	}

	if metadata.Version != u.currentVer {
		return true, &metadata, nil
	}

	return false, &metadata, nil
}

func (u *Updater) downloadAndUpdate(metadata *Metadata) error {
	tempPath := filepath.Join(u.config.TempDir, "gigabot-new")

	fmt.Printf("Descargando nueva versión a %s...\n", tempPath)

	resp, err := http.Get(u.config.VpsHost + "/download")
	if err != nil {
		return fmt.Errorf("error descargando: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error HTTP %d descargando", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo datos: %w", err)
	}

	checksum := sha256.Sum256(data)
	checksumHex := fmt.Sprintf("%x", checksum)
	if checksumHex != metadata.Checksum {
		return fmt.Errorf("checksum inválido: esperado %s, recibido %s", metadata.Checksum, checksumHex)
	}
	fmt.Println("Checksum verificado")

	sigBytes, err := base64.StdEncoding.DecodeString(metadata.Signature)
	if err != nil {
		return fmt.Errorf("error decodificando firma: %w", err)
	}

	if !ed25519.Verify(u.publicKey, data, sigBytes) {
		return fmt.Errorf("firma Ed25519 inválida - posible ataque de inyección")
	}
	fmt.Println("Firma Ed25519 verificada")

	if err := os.WriteFile(tempPath, data, 0755); err != nil {
		return fmt.Errorf("error guardando archivo temporal: %w", err)
	}

	// Verificar si existe el binario actual
	_, err = os.Stat(u.config.GigabotPath)
	gigabotExists := err == nil

	if gigabotExists {
		// Flujo de actualización: detener, backup, reemplazar
		fmt.Println("Deteniendo Gigabot actual...")
		if err := u.stopGigabot(); err != nil {
			fmt.Printf("Advertencia: error deteniendo Gigabot: %v\n", err)
		}

		time.Sleep(2 * time.Second)

		backupPath := u.config.GigabotPath + ".backup"
		if err := os.Rename(u.config.GigabotPath, backupPath); err != nil {
			return fmt.Errorf("error haciendo backup: %w", err)
		}

		if err := os.Rename(tempPath, u.config.GigabotPath); err != nil {
			os.Rename(backupPath, u.config.GigabotPath)
			return fmt.Errorf("error reemplazando binario: %w", err)
		}

		exec.Command("xattr", "-c", u.config.GigabotPath).Run()
		fmt.Println("Binario reemplazado exitosamente")

		if err := u.startGigabot(); err != nil {
			os.Remove(u.config.GigabotPath)
			os.Rename(backupPath, u.config.GigabotPath)
			return fmt.Errorf("error iniciando nueva versión, rollback realizado: %w", err)
		}

		os.Remove(backupPath)
	} else {
		// Primera instalación: simplemente mover, poner +x y ejecutar
		fmt.Println("Gigabot no existe, realizando primera instalación...")

		if err := os.Rename(tempPath, u.config.GigabotPath); err != nil {
			return fmt.Errorf("error guardando binario: %w", err)
		}

		// Asegurar permisos ejecutables
		if err := os.Chmod(u.config.GigabotPath, 0755); err != nil {
			fmt.Printf("Advertencia: error poniendo permisos ejecutables: %v\n", err)
		}

		exec.Command("xattr", "-c", u.config.GigabotPath).Run()
		fmt.Println("Binario instalado exitosamente")

		if err := u.startGigabot(); err != nil {
			return fmt.Errorf("error iniciando Gigabot: %w", err)
		}
	}

	return nil
}

func (u *Updater) startGigabot() error {
	fmt.Printf("Iniciando Gigabot: %s\n", u.config.GigabotPath)

	cmd := exec.Command(u.config.GigabotPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error iniciando proceso: %w", err)
	}

	u.gigabotCmd = cmd
	fmt.Printf("Gigabot iniciado con PID %d\n", cmd.Process.Pid)

	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Printf("Gigabot terminó con error: %v\n", err)
		} else {
			fmt.Println("Gigabot terminó normalmente")
		}
		u.gigabotCmd = nil
	}()

	return nil
}

func (u *Updater) stopGigabot() error {
	if u.gigabotCmd == nil || u.gigabotCmd.Process == nil {
		return nil
	}

	fmt.Printf("Enviando señal de terminación a PID %d...\n", u.gigabotCmd.Process.Pid)

	if err := u.gigabotCmd.Process.Signal(os.Interrupt); err != nil {
		u.gigabotCmd.Process.Kill()
	}

	done := make(chan error, 1)
	go func() {
		done <- u.gigabotCmd.Wait()
	}()

	select {
	case <-done:
		fmt.Println("Gigabot detenido")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout esperando, forzando kill...")
		u.gigabotCmd.Process.Kill()
	}

	u.gigabotCmd = nil
	return nil
}

func parsePublicKey(publicKeyPEM []byte) (ed25519.PublicKey, error) {
	beginIdx := -1
	endIdx := -1

	for i := 0; i < len(publicKeyPEM)-26; i++ {
		if string(publicKeyPEM[i:i+26]) == "-----BEGIN PUBLIC KEY-----" {
			beginIdx = i + 26
		}
		if string(publicKeyPEM[i:i+24]) == "-----END PUBLIC KEY-----" {
			endIdx = i
		}
	}

	if beginIdx != -1 && endIdx != -1 && beginIdx < endIdx {
		publicKeyPEM = publicKeyPEM[beginIdx:endIdx]
	}

	clean := ""
	for _, c := range string(publicKeyPEM) {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			clean += string(c)
		}
	}

	keyBytes, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("error decodificando base64: %w", err)
	}

	if len(keyBytes) == 32 {
		return ed25519.PublicKey(keyBytes), nil
	}

	if len(keyBytes) > 32 {
		return ed25519.PublicKey(keyBytes[len(keyBytes)-32:]), nil
	}

	return nil, fmt.Errorf("clave pública inválida: %d bytes", len(keyBytes))
}
