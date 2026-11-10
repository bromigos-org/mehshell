package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigReloadAndSafeColours(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
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
	t.Setenv("TERM", "xterm-kitty")
	for _, rainbow := range []bool{false, true} {
		s := renderPowerlineLeft([]seg{{text: fc(cBlue, "50%"), bg: 31}}, rainbow)
		if visibleWidth(s) != 6 || !strings.Contains(s, "50%%") {
			t.Fatal(s, visibleWidth(s))
		}
	}
}

func TestTransientUsesReloadedColour(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	cfg := defaultConfig()
	cfg.Colours["76"] = "#39ff14"
	init := zshInitScript(cfg)
	if !strings.Contains(init, "PROMPT=${_mehshell_transient:-$'%F{#39ff14}❯%f '}") {
		t.Fatal(init)
	}
}

// config.example and the file written by "config init" document the same
// options; a new option has to reach both.
func TestExampleConfigMatchesGeneratedDefault(t *testing.T) {
	example, err := os.ReadFile("config.example")
	if err != nil {
		t.Fatal(err)
	}
	settings := func(text string) []string {
		var lines []string
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Option lines, and the comment lines that describe each style.
			if !strings.HasPrefix(line, "#") || strings.HasPrefix(line, "#   ") {
				lines = append(lines, line)
			}
		}
		return lines
	}
	got, want := settings(string(example)), settings(defaultConfigFile)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("config.example:\n%s\n\ngenerated default:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	listed := "\n" + strings.Join(want, "\n") + "\n"
	for name := range defaultConfig().Segments {
		if !strings.Contains(listed, "\n"+name+" = true\n") {
			t.Errorf("segment %q is missing from the generated default", name)
		}
	}
}
