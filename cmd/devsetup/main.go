// devsetup crea credenciales de desarrollo locales; no configura proveedores externos.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func main() {
	if _, err := os.Stat("go.mod"); err != nil {
		fmt.Fprintln(os.Stderr, "Ejecutar desde la raíz del proyecto")
		os.Exit(1)
	}
	if _, err := os.Stat(".env"); err == nil {
		fmt.Println(".env ya existe; se conserva sin cambios")
		return
	}
	keys := []string{"POSTGRES_PASSWORD", "CUSTOMERS_DB_PASSWORD", "RENTALS_DB_PASSWORD", "EMPLOYEES_DB_PASSWORD", "MONGO_ROOT_PASSWORD", "MONGO_FLEET_PASSWORD", "REDIS_PASSWORD", "RABBITMQ_PASSWORD", "AUTH_ADMIN_TOKEN", "AUTH_CLIENT_TOKEN", "AUTH_GYM_TOKEN", "AUTH_SERVICE_TOKEN"}
	var text strings.Builder
	text.WriteString("# Desarrollo local: no versionar este archivo.\nAPP_PORT=8080\nJAEGER_PORT=16686\nSEED_DEMO=true\n")
	for _, key := range keys {
		value := make([]byte, 24)
		if _, err := rand.Read(value); err != nil {
			fmt.Fprintln(os.Stderr, "No se pudieron generar credenciales")
			os.Exit(1)
		}
		fmt.Fprintf(&text, "%s=%s\n", key, hex.EncodeToString(value))
	}
	file, err := os.OpenFile(".env", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "No se pudo crear .env sin sobrescribir archivos")
		os.Exit(1)
	}
	if _, err = file.WriteString(text.String()); err != nil {
		file.Close()
		fmt.Fprintln(os.Stderr, "No se pudo guardar .env")
		os.Exit(1)
	}
	file.Close()
	fmt.Println(".env creado con credenciales aleatorias. Contiene solamente configuración local; contratos externos pendientes.")
}
