package config_test

import (
	"testing"
	"time"

	"github.com/lacsar712/aerofilt/internal/config"
)

func TestDefaultValid(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.WashCloseWindow != 45*time.Second {
		t.Fatalf("window=%s", cfg.WashCloseWindow)
	}
}