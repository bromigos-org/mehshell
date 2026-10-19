package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrandAndSessionIdentity(t *testing.T) {
	cfg := defaultConfig()
	t.Setenv("TERM", "dumb")
	t.Setenv("MEHSHELL_BROMIGOS", "1")
	if effectiveBrand(cfg) != "bromigos" || segBrand(cfg, false) != "bromigOS" {
		t.Fatal("missing plain bromigOS identity")
	}
	cfg.Brand = "none"
	if segBrand(cfg, true) != "" {
		t.Fatal("branding cannot be disabled")
	}
	cfg.Brand = "os"
	if effectiveBrand(cfg) != "os" {
		t.Fatal("explicit portable choice ignored")
	}
	cfg.Brand = "auto"
	t.Setenv("MEHSHELL_BROMIGOS", "0")
	if effectiveBrand(cfg) != "os" {
		t.Fatal("portable fallback ignored")
	}
	for _, row := range []struct {
		uid        int
		user, host string
		remote     bool
		want       string
	}{
		{1000, "test", "desktop", false, ""}, {0, "root", "desktop", false, "ROOT"},
		{1000, "test", "server", true, "ssh:test@server"}, {0, "root", "server", true, "ROOT@server"},
	} {
		if got := sessionLabel(row.uid, row.user, row.host, row.remote); got != row.want {
			t.Errorf("got %q, want %q", got, row.want)
		}
	}
}

func TestGameMarkerAndTextFailure(t *testing.T) {
	t.Setenv("TERM", "dumb")
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	path := filepath.Join(runtime, "bromigos-game-on.json")
	for _, row := range []struct{ value, want string }{{"{broken", ""}, {"{}", ""}, {`{"paused":"false","leds":{}}`, "GAME"}} {
		if err := os.WriteFile(path, []byte(row.value), 0600); err != nil {
			t.Fatal(err)
		}
		if got := segGame(); got != row.want {
			t.Fatalf("got %q, want %q", got, row.want)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if segGame() != "" {
		t.Fatal("stale game indicator")
	}
	cfg := defaultConfig()
	if promptMark(cfg, 0) != ">" || promptMark(cfg, 1) != "exit:1 >" {
		t.Fatal("failure must remain visible without colour")
	}
	cfg.Segments["status"] = false
	if promptMark(cfg, 1) != ">" {
		t.Fatal("status option ignored")
	}
}

func TestInterruptedCommandIsNeutralAcrossTerminals(t *testing.T) {
	for _, term := range []string{"xterm-kitty", "linux", "dumb"} {
		t.Run(term, func(t *testing.T) {
			t.Setenv("TERM", term)
			cfg := defaultConfig()
			if got := promptMark(cfg, 130); got != fc(cGray, "interrupted "+promptArrow()) {
				t.Fatalf("interruption should be neutral: %q", got)
			}
			if got := fittedPromptMark(cfg, 130, 4); got != fc(cGray, promptArrow()) {
				t.Fatalf("narrow interruption should not use a failure marker: %q", got)
			}
			if got := promptMark(cfg, 1); got != fc(cRed, "exit:1 "+promptArrow()) {
				t.Fatalf("ordinary failure lost its status: %q", got)
			}
			cfg.Segments["status"] = false
			if got := promptMark(cfg, 130); got != fc(cGray, promptArrow()) {
				t.Fatalf("interruption ignored the status option: %q", got)
			}
		})
	}
}

func TestConfigSetPreservesThemeAndSymlink(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "dotfiles-config")
	original := "# personal setting\nstyle = lean\ncolor_76 = #39ff14\ngit = false\n"
	if err := os.WriteFile(target, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, configPath()); err != nil {
		t.Fatal(err)
	}
	if err := setOption("style", "rainbow"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != strings.Replace(original, "style = lean", "style = rainbow", 1) {
		t.Fatalf("unrelated content changed: %q %v", data, err)
	}
	if info, err := os.Lstat(configPath()); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced", err)
	}
	before := string(data)
	for _, row := range [][2]string{{"style", "unknown"}, {"git", "perhaps"}, {"not-a-setting", "true"}, {"color_76", "#ffffff"}} {
		if setOption(row[0], row[1]) == nil {
			t.Fatalf("accepted unsupported setting %v", row)
		}
	}
	data, err = os.ReadFile(target)
	if err != nil || string(data) != before {
		t.Fatal("failed edit changed config", err)
	}
	if err := setOption("brand", "bromigos"); err != nil {
		t.Fatal(err)
	}
	if loadConfig().Brand != "bromigos" {
		t.Fatal("new setting not read")
	}
}

func TestPreviewDoesNotInterpretPromptText(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	text := fc(cGreen, "100% $(example) 'literal'")
	if got := previewText(text, false); got != "100% $(example) 'literal'" {
		t.Fatal(got)
	}
	if got := previewText(text, true); !strings.HasPrefix(got, "\x1b[38;5;76m") || !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatal(got)
	}
}

func TestLiteralColourTextAndHomeBoundary(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	cfg := defaultConfig()
	cfg.Colours["75"] = "#159b09"
	if got := previewText(colourPrompt(fc(cCyan, "%F{75}notes"), cfg), false); got != "%F{75}notes" {
		t.Fatal(got)
	}
	if got := previewText(segDir("/home/user-other", "/home/user"), false); got != "/home/user-other" {
		t.Fatal(got)
	}
	if got := previewText(segDir("/home/user/project", "/home/user"), false); got != "~/project" {
		t.Fatal(got)
	}
	for width := 1; width < 40; width++ {
		if visibleWidth(fittedPromptMark(cfg, 255, width)) > max(0, width-2) {
			t.Fatalf("transient prompt exceeds %d columns", width)
		}
	}
}
