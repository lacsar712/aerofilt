package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lacsar712/aerofilt/internal/app"
	"github.com/lacsar712/aerofilt/internal/config"
)

func main() {
	cfgPath := flag.String("config", "", "optional JSON config path")
	addr := flag.String("addr", "", "listen address override")
	flag.Parse()
	cfg, err := config.LoadJSON(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *addr != "" {
		cfg.ListenAddr = *addr
	}
	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	defer func() { _ = application.Close() }()
	if err := application.SeedHead(time.Now().UTC()); err != nil {
		log.Fatalf("seed: %v", err)
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: application.AttachHTTP(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		fmt.Printf("aerofilt listening on %s plant=%s\n", cfg.ListenAddr, cfg.PlantID)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
