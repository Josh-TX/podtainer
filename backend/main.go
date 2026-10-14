package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"podtainer/internal/api"
	"podtainer/internal/auth"
	"podtainer/internal/config"
	"podtainer/internal/shellsvc"
	"podtainer/internal/webui"
)

var version = "dev" // set via -ldflags "-X main.version=..."

func main() {
	cfg, err := config.Load(os.Args[1:], version)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	a := auth.New(cfg.AuthFile)
	shellMgr := shellsvc.NewManager()
	execMgr := shellsvc.NewManager()
	mux := api.NewMux(cfg, a, shellMgr, execMgr)

	if webui.Embedded() {
		frontend, err := webui.FS()
		if err != nil {
			log.Fatalf("webui: %v", err)
		}
		mux.Handle("/", webui.Handler(frontend))
	} else {
		log.Printf("no frontend embedded, use the Vite dev server (or build with -tags embed_frontend)")
	}

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	log.Printf("podtainer listening on %s (stacks: %s, quadlets: %s)", addr, cfg.StacksDir, cfg.QuadletDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}
