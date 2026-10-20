// Command server runs the SkyPulse live demo as an HTTP server. This is the
// entry point used when the project is deployed to a host that runs a Go HTTP
// server listening on $PORT (for example Vercel). It serves the handler in the
// server package and never touches the database; persisting snapshots is the
// job of the command-line collector in the repository root (see main.go).
package main

import (
	"log"
	"net/http"
	"os"

	"arch3/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", server.Handler)

	addr := ":" + port
	log.Printf("SkyPulse demo server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
