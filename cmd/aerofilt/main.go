// Command aerofilt runs the Biofilter filter backwash window & temperature control service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lacsar712/aerofilt/internal/app"
	"github.com/lacsar712/aerofilt/internal/config"
	"github.com/lacsar712/aerofilt/internal/web"
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

	if err := application.SeedSensors(time.Now().UTC()); err != nil {
		log.Fatalf("seed: %v", err)
	}

	static := web.Handler()
	if cfg.StaticDir != "" {
		if st, err := os.Stat(cfg.StaticDir); err == nil && st.IsDir() {
			static = http.FileServer(http.Dir(cfg.StaticDir))
		} else if abs, err := filepath.Abs(cfg.StaticDir); err == nil {
			if st, err := os.Stat(abs); err == nil && st.IsDir() {
				static = http.FileServer(http.Dir(abs))
			}
		}
	}

	handler := application.AttachHTTP(static)
	fmt.Printf("aerofilt listening on %s filter=%s\n", cfg.ListenAddr, cfg.FilterID)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatal(err)
	}
}
