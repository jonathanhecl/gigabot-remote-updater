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
	"path/filepath"
	"time"
)

type Config struct {
	Token         string `json:"token"`
	PublicKeyPath string `json:"public_key_path"`
	Port          string `json:"port"`
	StorageDir    string `json:"storage_dir"`
}

type Server struct {
	storageDir string
	publicKey  ed25519.PublicKey
	token      string
	port       string
}

type Metadata struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	Checksum  string `json:"checksum"`
	Platform  string `json:"platform"`
	Signature string `json:"signature"`
}

func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Valores por defecto
	if config.Token == "" {
		config.Token = "default-token-cambiar-en-produccion"
	}
	if config.PublicKeyPath == "" {
		config.PublicKeyPath = "deploy-public.key"
	}
	if config.Port == "" {
		config.Port = "8443"
	}
	if config.StorageDir == "" {
		config.StorageDir = "./storage"
	}

	return &config, nil
}

func main() {
	// Intentar cargar desde JSON primero, luego fallback a env vars
	var config *Config
	var err error

	configPath := os.Getenv("NEXO_CONFIG")
	if configPath == "" {
		configPath = "config.json"
	}

	if _, err = os.Stat(configPath); err == nil {
		config, err = loadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error cargando config.json: %v\n", err)
			fmt.Println("Usando variables de entorno como fallback...")
			config = nil
		} else {
			fmt.Printf("Configuración cargada desde: %s\n", configPath)
		}
	}

	// Si no hay config JSON, usar variables de entorno
	if config == nil {
		config = &Config{
			Token:         os.Getenv("NEXO_TOKEN"),
			PublicKeyPath: os.Getenv("NEXO_PUBLIC_KEY"),
			Port:          os.Getenv("NEXO_PORT"),
			StorageDir:    os.Getenv("NEXO_STORAGE"),
		}
		// Aplicar defaults
		if config.Token == "" {
			config.Token = "default-token-cambiar-en-produccion"
		}
		if config.PublicKeyPath == "" {
			config.PublicKeyPath = "deploy-public.key"
		}
		if config.Port == "" {
			config.Port = "8443"
		}
		if config.StorageDir == "" {
			config.StorageDir = "./storage"
		}
	}

	// Cargar clave pública
	publicKeyPEM, err := os.ReadFile(config.PublicKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error leyendo clave pública: %v\n", err)
		fmt.Println("Creando directorios y generando clave de ejemplo...")

		// Crear directorios
		os.MkdirAll(config.StorageDir, 0755)
		os.MkdirAll("./logs", 0755)

		// Generar par de claves de ejemplo
		if err := generateExampleKeys(config.PublicKeyPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error generando claves: %v\n", err)
			os.Exit(1)
		}

		publicKeyPEM, _ = os.ReadFile(config.PublicKeyPath)
	}

	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parseando clave pública: %v\n", err)
		os.Exit(1)
	}

	// Crear directorios si no existen
	os.MkdirAll(config.StorageDir, 0755)
	os.MkdirAll("./logs", 0755)

	server := &Server{
		storageDir: config.StorageDir,
		publicKey:  publicKey,
		token:      config.Token,
		port:       config.Port,
	}

	http.HandleFunc("/upload", server.handleUpload)
	http.HandleFunc("/latest", server.handleLatest)
	http.HandleFunc("/download", server.handleDownload)
	http.HandleFunc("/health", server.handleHealth)

	fmt.Printf("Nexo Server iniciado en puerto %s\n", config.Port)
	fmt.Printf("Storage: %s\n", config.StorageDir)
	fmt.Printf("Token configurado: %s...\n", config.Token[:min(10, len(config.Token))])

	if err := http.ListenAndServe(":"+config.Port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error iniciando servidor: %v\n", err)
		os.Exit(1)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar token
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, "Error parseando formulario", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token != s.token {
		s.log("Intento de upload con token inválido")
		http.Error(w, "Token inválido", http.StatusUnauthorized)
		return
	}

	// Obtener metadata
	metadataJSON := r.FormValue("metadata")
	var metadata Metadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		http.Error(w, "Metadata inválida", http.StatusBadRequest)
		return
	}

	// Obtener archivo
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error obteniendo archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Leer archivo
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error leyendo archivo", http.StatusInternalServerError)
		return
	}

	// Verificar checksum
	checksum := sha256.Sum256(data)
	checksumHex := fmt.Sprintf("%x", checksum)
	if checksumHex != metadata.Checksum {
		s.log(fmt.Sprintf("Checksum inválido. Esperado: %s, Recibido: %s", metadata.Checksum, checksumHex))
		http.Error(w, "Checksum inválido", http.StatusBadRequest)
		return
	}

	// Verificar firma
	sigBytes, err := base64.StdEncoding.DecodeString(metadata.Signature)
	if err != nil {
		http.Error(w, "Firma inválida (base64)", http.StatusBadRequest)
		return
	}

	if !ed25519.Verify(s.publicKey, data, sigBytes) {
		s.log("Firma Ed25519 inválida")
		http.Error(w, "Firma inválida", http.StatusUnauthorized)
		return
	}

	// Guardar archivo
	binaryPath := filepath.Join(s.storageDir, "latest.bin")
	if err := os.WriteFile(binaryPath, data, 0755); err != nil {
		http.Error(w, "Error guardando archivo", http.StatusInternalServerError)
		return
	}

	// Guardar metadata
	metadataPath := filepath.Join(s.storageDir, "latest.json")
	metadataBytes, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		http.Error(w, "Error guardando metadata", http.StatusInternalServerError)
		return
	}

	s.log(fmt.Sprintf("Upload exitoso - versión %s", metadata.Version))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Upload exitoso",
		"version": metadata.Version,
	})
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	metadataPath := filepath.Join(s.storageDir, "latest.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "No hay versiones disponibles", http.StatusNotFound)
			return
		}
		http.Error(w, "Error leyendo metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	binaryPath := filepath.Join(s.storageDir, "latest.bin")
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "No hay binario disponible", http.StatusNotFound)
			return
		}
		http.Error(w, "Error leyendo binario", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=gigabot-mac")
	w.Write(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) log(msg string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, msg)
	fmt.Print(logLine)

	// También escribir a archivo de log
	logFile := filepath.Join("./logs", "nexo.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(logLine)
	}
}

func parsePublicKey(publicKeyPEM []byte) (ed25519.PublicKey, error) {
	// Extraer parte base64 del PEM
	// Buscar begin/end
	beginIdx := -1
	endIdx := -1

	for i := 0; i < len(publicKeyPEM)-10; i++ {
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

	// Limpiar
	publicKeyPEM = []byte(string(publicKeyPEM))
	publicKeyPEM = []byte(trimSpaceAndNewlines(string(publicKeyPEM)))

	keyBytes, err := base64.StdEncoding.DecodeString(string(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("error decodificando base64: %w", err)
	}

	// Para Ed25519, la clave pública es de 32 bytes
	// En formato PKIX, puede venir con metadata
	if len(keyBytes) == 32 {
		return ed25519.PublicKey(keyBytes), nil
	}

	// Si es más largo, buscar los últimos 32 bytes (la clave real)
	if len(keyBytes) > 32 {
		return ed25519.PublicKey(keyBytes[len(keyBytes)-32:]), nil
	}

	return nil, fmt.Errorf("clave pública inválida: %d bytes", len(keyBytes))
}

func generateExampleKeys(publicKeyPath string) error {
	// Generar par de claves Ed25519
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	// Guardar clave pública en formato PEM simple
	publicKeyBase64 := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	publicKeyPEM := fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----\n",
		insertNewlines(publicKeyBase64, 64))

	err = os.WriteFile(publicKeyPath, []byte(publicKeyPEM), 0644)
	if err != nil {
		return err
	}

	// Guardar clave privada
	privateKeyPath := "deploy-private.key"
	privateKeyBase64 := base64.StdEncoding.EncodeToString(privateKey.Seed())
	privateKeyPEM := fmt.Sprintf("-----BEGIN PRIVATE KEY-----\n%s\n-----END PRIVATE KEY-----\n",
		insertNewlines(privateKeyBase64, 64))

	err = os.WriteFile(privateKeyPath, []byte(privateKeyPEM), 0600)
	if err != nil {
		return err
	}

	fmt.Printf("Claves de ejemplo generadas:\n")
	fmt.Printf("  - Clave pública: %s\n", publicKeyPath)
	fmt.Printf("  - Clave privada: %s\n", privateKeyPath)
	fmt.Println("IMPORTANTE: En producción, genera tus propias claves con OpenSSL:")
	fmt.Println("  openssl genpkey -algorithm Ed25519 -out deploy-private.key")
	fmt.Println("  openssl pkey -in deploy-private.key -pubout -out deploy-public.key")

	return nil
}

func insertNewlines(s string, every int) string {
	var result string
	for i := 0; i < len(s); i += every {
		end := i + every
		if end > len(s) {
			end = len(s)
		}
		result += s[i:end] + "\n"
	}
	return result
}

func trimSpaceAndNewlines(s string) string {
	result := ""
	for _, c := range s {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			result += string(c)
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
