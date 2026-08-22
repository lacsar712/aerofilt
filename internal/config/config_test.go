package config

import "testing"

func TestDefaultValidates(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestClampHeaterKW(t *testing.T) {
	cfg := Default()
	if cfg.ClampHeaterKW(999) != cfg.HeaterMaxKW {
		t.Fatal("clamp failed")
	}
}
