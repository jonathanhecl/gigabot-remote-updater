package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	fmt.Println("Generando par de claves Ed25519 para Gigabot Updater...")

	// Generar par de claves
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generando claves: %v\n", err)
		os.Exit(1)
	}

	// Guardar clave privada en formato PEM (PKCS#8)
	privateKeyBytes := privateKey.Seed()
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	if err := os.WriteFile("deploy-private.key", privateKeyPEM, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error escribiendo clave privada: %v\n", err)
		os.Exit(1)
	}

	// Guardar clave pública en formato PEM (PKIX)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKey,
	})

	if err := os.WriteFile("deploy-public.key", publicKeyPEM, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error escribiendo clave pública: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Claves generadas exitosamente:")
	fmt.Printf("  - deploy-private.key (GUARDAR EN LUGAR SEGURO)\n")
	fmt.Printf("  - deploy-public.key (distribuir a VPS y Mac)\n")
	fmt.Println()
	fmt.Println("Información de la clave pública:")
	fmt.Printf("  Algoritmo: Ed25519\n")
	fmt.Printf("  Tamaño: %d bytes\n", len(publicKey))
	fmt.Printf("  Base64: %s\n", base64.StdEncoding.EncodeToString(publicKey))
	fmt.Println()
	fmt.Println("IMPORTANTE:")
	fmt.Println("  - La clave privada NUNCA debe compartirse o subirse al VPS")
	fmt.Println("  - Solo la máquina de desarrollo debe tener la clave privada")
	fmt.Println("  - El VPS y el Mac solo necesitan la clave pública")
}
