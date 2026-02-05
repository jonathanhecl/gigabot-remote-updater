package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Config struct {
	VpsHost     string
	Token       string
	PrivateKey  string
	ProjectPath string
	BinaryName  string
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Uso: deployer <vps-host> <token> <private-key-file> [project-path]")
		fmt.Println("Ejemplo: deployer https://tu-vps.com:8443 mi-token-secreto deploy-private.key")
		fmt.Println("         deployer https://tu-vps.com:8443 mi-token-secreto deploy-private.key C:\\proyectos\\gigabot")
		os.Exit(1)
	}

	// Detectar directorio del ejecutable (donde está deployer.exe)
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detectando ruta del ejecutable: %v\n", err)
		os.Exit(1)
	}
	execDir := filepath.Dir(execPath)

	// Usar parámetro opcional o el directorio del ejecutable
	projectPath := execDir
	if len(os.Args) >= 5 {
		projectPath = os.Args[4]
	}

	config := Config{
		VpsHost:     os.Args[1],
		Token:       os.Args[2],
		PrivateKey:  os.Args[3],
		ProjectPath: projectPath,
		BinaryName:  "gigabot-mac",
	}

	fmt.Printf("Deployer desde: %s\n", execDir)
	fmt.Printf("Proyecto a compilar: %s\n", config.ProjectPath)

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(config Config) error {
	// Verificar clave privada
	privateKeyPEM, err := os.ReadFile(config.PrivateKey)
	if err != nil {
		return fmt.Errorf("no se puede leer la clave privada: %w", err)
	}

	buildTime := time.Now().Format("2006-01-02 15:04:05")
	version := time.Now().Format("20060102-150405")

	fmt.Println("Compilando Gigabot para Mac (darwin/arm64)...")
	fmt.Printf("Version: %s\n", version)

	// Compilar para Mac M4 (arm64)
	binaryPath := filepath.Join(config.ProjectPath, config.BinaryName)

	// Ejecutar go build con cross-compilation
	cmd := exec.Command("go", "build",
		"-ldflags", fmt.Sprintf("-X main.BuildTime=%s -X main.Version=%s", buildTime, version),
		"-o", config.BinaryName,
		"cmd/gigabot/main.go")
	cmd.Dir = config.ProjectPath
	cmd.Env = append(os.Environ(),
		"GOOS=darwin",
		"GOARCH=arm64",
		"CGO_ENABLED=0",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Ejecutando: go build -ldflags '...' -o %s cmd/gigabot/main.go\n", config.BinaryName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error compilando: %w", err)
	}

	fmt.Println("Compilación exitosa!")

	// Calcular checksum
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("no se puede leer el binario: %w", err)
	}

	checksum := sha256.Sum256(binaryData)
	checksumHex := fmt.Sprintf("%x", checksum)
	fmt.Printf("Checksum: %s\n", checksumHex)

	// Firmar el binario con Ed25519
	signature, err := signBinary(privateKeyPEM, binaryData)
	if err != nil {
		return fmt.Errorf("error al firmar: %w", err)
	}
	fmt.Println("Firma generada")

	// Preparar metadata
	metadata := map[string]string{
		"version":    version,
		"build_time": buildTime,
		"checksum":   checksumHex,
		"platform":   "darwin/arm64",
		"signature":  base64.StdEncoding.EncodeToString(signature),
	}

	metadataJSON, _ := json.Marshal(metadata)

	fmt.Printf("Subiendo a VPS (%s)...\n", config.VpsHost)

	// Crear multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Token
	_ = writer.WriteField("token", config.Token)
	// Version
	_ = writer.WriteField("version", version)
	// Metadata
	_ = writer.WriteField("metadata", string(metadataJSON))

	// Archivo
	part, err := writer.CreateFormFile("file", config.BinaryName)
	if err != nil {
		return fmt.Errorf("error creando form file: %w", err)
	}

	_, err = io.Copy(part, bytes.NewReader(binaryData))
	if err != nil {
		return fmt.Errorf("error copiando archivo: %w", err)
	}

	writer.Close()

	// Enviar request
	url := config.VpsHost + "/upload"
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("error creando request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error del servidor (%d): %s", resp.StatusCode, string(body))
	}

	fmt.Println("Deploy exitoso!")
	fmt.Printf("Respuesta: %s\n", string(body))

	return nil
}

func signBinary(privateKeyPEM []byte, data []byte) ([]byte, error) {
	// Parsear la clave privada Ed25519 desde formato PEM
	// El formato es: -----BEGIN PRIVATE KEY----- ... -----END PRIVATE KEY-----

	// Extraer la parte base64 del PEM
	start := bytes.Index(privateKeyPEM, []byte("-----BEGIN PRIVATE KEY-----"))
	end := bytes.Index(privateKeyPEM, []byte("-----END PRIVATE KEY-----"))

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("formato PEM inválido")
	}

	// Extraer solo la parte base64
	base64Data := privateKeyPEM[start+27 : end]
	// Limpiar newlines y espacios
	base64Data = bytes.ReplaceAll(base64Data, []byte("\n"), []byte{})
	base64Data = bytes.ReplaceAll(base64Data, []byte("\r"), []byte{})

	privateKeyBytes, err := base64.StdEncoding.DecodeString(string(base64Data))
	if err != nil {
		return nil, fmt.Errorf("error decodificando base64: %w", err)
	}

	// Para Ed25519 en formato PKCS#8, necesitamos extraer la clave privada
	// El formato DER contiene: version (1) + algorithm identifier + private key octet string
	// La clave Ed25519 tiene 32 bytes

	// Buscar la clave privada en el DER (últimos 32 bytes para Ed25519)
	if len(privateKeyBytes) < 32 {
		return nil, fmt.Errorf("clave privada demasiado corta")
	}

	// Los últimos 32 bytes son la clave privada Ed25519
	seed := privateKeyBytes[len(privateKeyBytes)-32:]

	privateKey := ed25519.NewKeyFromSeed(seed)
	signature := ed25519.Sign(privateKey, data)

	return signature, nil
}
