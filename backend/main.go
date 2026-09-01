package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"podtainer/internal/api"
	"podtainer/internal/config"
	"podtainer/internal/webui"
)

func main() {
	port := flag.Int("p", 8080, "port to listen on")
	flag.Parse()

	cfg, err := config.Load(*port)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	mux := api.NewMux(cfg)

	frontend, err := webui.FS()
	if err != nil {
		log.Fatalf("webui: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(frontend)))

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	log.Printf("podtainer listening on %s (stacks: %s, quadlets: %s)", addr, cfg.StacksDir, cfg.QuadletDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}
