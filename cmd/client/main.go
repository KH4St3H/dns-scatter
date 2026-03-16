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
	configPath := flag.String("config", "configs/client.yaml", "path to client config file")
	flag.Parse()

	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rw := rewriter.New(cfg.Mappings)
	cli := proxy.NewClient(cfg.Listen, cfg.Server, rw)

	if err := cli.Start(); err != nil {
		log.Fatalf("failed to start client: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down client...")
	cli.Shutdown()
}
