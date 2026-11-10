// apidocs permite revisar Swagger sin bases de datos, tokens ni servicios activos.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Gerpeirone/Car-Cruds/docs"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8090", "Dirección del visor de documentación")
	flag.Parse()
	server := &http.Server{Addr: *address, Handler: docs.Handler(), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("Documentación Swagger: http://%s/api/", *address)
	log.Fatal(server.ListenAndServe())
}
