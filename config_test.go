package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigReloadAndSafeColours(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "mehshell"), 0700); err != nil {
		t.Fatal(err)
	}
	path := configPath()
	for _, colour := range []string{"#39ff14", "#3fe0c5"} {
		if err := os.WriteFile(path, []byte("color_75 = "+colour+"\ngit = false\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg := loadConfig()
		if cfg.Segments["git"] || colourPrompt("%F{75}test%f", cfg) != "%F{"+colour+"}test%f" {
			t.Fatal(cfg)
		}
	}
	for _, value := range []string{"#12345", "$(id)", "#12345g", "#123456}';id"} {
		if validColour(value) {
			t.Fatal(value)
		}
	}
}
func TestStylesAndWidths(t *testing.T) {
	for _, rainbow := range []bool{false, true} {
		s := renderPowerlineLeft([]seg{{text: fc(cBlue, "50%"), bg: 31}}, rainbow)
		if visibleWidth(s) != 6 || !strings.Contains(s, "50%%") {
			t.Fatal(s, visibleWidth(s))
		}
	}
}
