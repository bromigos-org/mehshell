package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func capturePrompt(t *testing.T) string {
	t.Helper()
	oldArgs, oldStdout := os.Args, os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Args, os.Stdout = oldArgs, oldStdout }()
	os.Args, os.Stdout = []string{"mehshell", "0", "5", "80"}, writer
	main()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestRuntimeTerminalFallbackPreservesDesktopSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(home); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	for _, directory := range []string{"config/mehshell", ".git/refs/heads", "bin"} {
		if err := os.MkdirAll(filepath.Join(home, directory), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// Deterministic clean repository; no external repository or runtime is inspected.
	for path, value := range map[string]string{
		".git/HEAD": "ref: refs/heads/example\n", ".git/refs/heads/example": strings.Repeat("a", 40) + "\n",
		"go.mod": "module example.org/fixture\ngo 1.22\n", "bin/git": "#!/bin/sh\nprintf '# branch.oid aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\\000# branch.head example\\000'\n",
	} {
		if err := os.WriteFile(filepath.Join(home, path), []byte(value), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", filepath.Join(home, "bin"))
	var config strings.Builder
	config.WriteString("icons = true\nstyle = rainbow\ncolor_75 = #159b09\ncolor_76 = #39ff14\n")
	for segment := range defaultConfig().Segments {
		fmt.Fprintf(&config, "%s = %t\n", segment, segment == "dir" || segment == "git" || segment == "go" || segment == "os")
	}
	original := config.String()
	if err := os.WriteFile(configPath(), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"linux", "dumb", "xterm-kitty"} {
		t.Run(term, func(t *testing.T) {
			t.Setenv("TERM", term)
			output := capturePrompt(t)
			if term == "xterm-kitty" {
				if !strings.Contains(output, "\ue0b0") || !strings.Contains(output, "\ue627") || !strings.Contains(output, "#39ff14") || !strings.Contains(output, "❯") {
					t.Fatalf("desktop style, icons or colours changed: %q", output)
				}
				return
			}
			for _, char := range output {
				if char > 127 {
					t.Fatalf("unsupported built-in glyph %q in %q", char, output)
				}
			}
			if !strings.Contains(output, "go 1.22") || !strings.Contains(output, "clean") || !strings.Contains(output, ">") {
				t.Fatalf("text labels or prompt mark missing: %q", output)
			}
			if term == "dumb" && (strings.Contains(output, "%F{") || strings.Contains(output, "%K{") || strings.ContainsRune(output, '\x1b')) {
				t.Fatalf("colour in dumb output: %q", output)
			}
			if term == "linux" {
				for _, match := range regexp.MustCompile(`%[FK]\{([^}]+)\}`).FindAllStringSubmatch(output, -1) {
					if len(match[1]) != 1 || match[1][0] < '0' || match[1][0] > '7' {
						t.Fatalf("unsupported console colour: %q", match[1])
					}
				}
			}
		})
	}
	data, err := os.ReadFile(configPath())
	if err != nil || string(data) != original {
		t.Fatal("terminal fallback changed the user's config", err)
	}
}

func runZsh(t *testing.T, script string) string {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is required for generated-hook integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, zsh, "-f", "-c", script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh hooks failed: %v\n%s", err, output)
	}
	return string(output)
}

func TestTerminalHooksIsolateCacheAndUseAsciiMarks(t *testing.T) {
	for _, term := range []string{"linux", "dumb"} {
		t.Run(term, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CACHE_HOME", home)
			t.Setenv("ZDOTDIR", home)
			t.Setenv("TERM", term)
			desktopCache := filepath.Join(home, "mehshell-prompt-cache")
			if err := os.WriteFile(desktopCache, []byte("PROMPT='DESKTOP-❯-\\ue0b0'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cache := filepath.Join(home, "mehshell-prompt-cache-"+term)
			if err := os.WriteFile(cache, []byte("# mehshell instant prompt v2\nPROMPT='TTY-CACHE'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := defaultConfig()
			cfg.Colours["76"] = "#39ff14"
			init := zshInitScript(cfg)
			for _, char := range init {
				if char > 127 {
					t.Fatalf("unsupported built-in glyph in generated hooks: %q", char)
				}
			}
			// Stub ZLE only: no terminal is opened, focused or modified.
			script := "zle() { :; }\n" + init + "\nprint -r -- $PROMPT\n"
			suffix := " "
			if term == "linux" {
				suffix = "%f "
			}
			script += "PROMPT='path>name >" + suffix + "'\nKEYMAP=vicmd\n_mehshell_zle_keymap_select\nprint -r -- $PROMPT\n"
			script += "KEYMAP=main\n_mehshell_zle_keymap_select\nprint -r -- $PROMPT\n"
			script += "unset _mehshell_transient\n_mehshell_accept_line\nprint -r -- $PROMPT\n"
			output := runZsh(t, script)
			lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
			if len(lines) != 4 || lines[0] != "TTY-CACHE" || lines[1] != "path>name <"+suffix || lines[2] != "path>name >"+suffix {
				t.Fatalf("cache or vi hook: %q", output)
			}
			if lines[3] != fc(cGreen, ">")+" " {
				t.Fatalf("transient prompt: %q", lines[3])
			}
		})
	}
}
