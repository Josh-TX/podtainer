package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"podtainer/internal/api"
	"podtainer/internal/auth"
	"podtainer/internal/config"
	"podtainer/internal/shellsvc"
	"podtainer/internal/webui"
)

var version = "dev"

func main() {
	port := flag.Int("p", 8080, "port to listen on")
	showVersion := flag.Bool("v", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(*port)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	a := auth.New(cfg.AuthFile)
	shellMgr := shellsvc.NewManager()
	execMgr := shellsvc.NewManager()
	mux := api.NewMux(cfg, a, shellMgr, execMgr)

	frontend, err := webui.FS()
	if err != nil {
		log.Fatalf("webui: %v", err)
	}
	mux.Handle("/", webui.Handler(frontend))

	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	log.Printf("podtainer listening on %s (stacks: %s, quadlets: %s)", addr, cfg.StacksDir, cfg.QuadletDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}
