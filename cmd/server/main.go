package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mehrshad/dns-split/internal/config"
	"github.com/mehrshad/dns-split/internal/proxy"
	"github.com/mehrshad/dns-split/internal/rewriter"
)

func main() {
	configPath := flag.String("config", "configs/server.yaml", "path to server config file")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rw := rewriter.New(cfg.Mappings)
	srv := proxy.NewServer(cfg.Listen, cfg.Upstream, rw)

	if err := srv.Start(); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down server...")
	srv.Shutdown()
}
