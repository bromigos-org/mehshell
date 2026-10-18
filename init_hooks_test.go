package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generatedPromptForTest(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "prompt-output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	oldArgs, oldOutput := os.Args, os.Stdout
	os.Args, os.Stdout = []string{"mehshell", "7", "0", "240"}, file
	defer func() { os.Args, os.Stdout = oldArgs, oldOutput }()
	main()
	output, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func hookSandbox(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", home)
	t.Setenv("XDG_CACHE_HOME", home)
	t.Setenv("TERM", "dumb")
	t.Setenv("HOOK_LOG", filepath.Join(home, "calls"))
	bin := filepath.Join(home, "prompt with ' quote")
	code := `#!/bin/sh
printf '%s %s %s\n' "$1" "$2" "$3" >> "$HOOK_LOG"
printf "PROMPT='fresh > '\n_mehshell_transient='transient > '\n"
`
	if err := os.WriteFile(bin, []byte(code), 0700); err != nil {
		t.Fatal(err)
	}
	return home, bin
}

func TestHookReloadPreservesOthersAndCommandStatus(t *testing.T) {
	home, bin := hookSandbox(t)
	cfg := defaultConfig()
	cfg.InstantPrompt = false
	init := zshInitScriptForBinary(cfg, bin)
	script := `zmodload zsh/zle
zmodload zsh/zleparameter
other_preexec() { return 0 }
other_precmd() { return 0 }
other_accept() { zle .accept-line }
other_keymap() { return 0 }
other_finish() { return 0 }
zle -N accept-line other_accept
zle -N zle-keymap-select other_keymap
zle -N zle-line-finish other_finish
preexec_functions=(other_preexec _mehshell_preexec _mehshell_preexec)
precmd_functions=(other_precmd _mehshell_precmd _mehshell_precmd)
` + init + "\n" + init + `
print -r -- "preexec=${(j:,:)preexec_functions}"
print -r -- "precmd=${(j:,:)precmd_functions}"
print -r -- "accept=${widgets[accept-line]}"
zstyle -a zle-keymap-select widgets selected_hooks
print -r -- "keymap=${(j:,:)selected_hooks}"
zstyle -a zle-line-finish widgets finish_hooks
print -r -- "finish=${(j:,:)finish_hooks}"
_mehshell_ts=$(( EPOCHSECONDS - 7 ))
COLUMNS=82
(exit 17)
_mehshell_precmd
print -r -- "hook-status=$?"
`
	out := runZsh(t, script)
	for _, want := range []string{"preexec=_mehshell_preexec,other_preexec", "precmd=_mehshell_precmd,other_precmd", "accept=user:other_accept", "user:other_keymap", "user:other_finish", "hook-status=0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	for _, name := range []string{":_mehshell_zle_keymap_select", ":_mehshell_line_finish"} {
		if strings.Count(out, name) != 1 {
			t.Fatalf("duplicate widget hook %q: %q", name, out)
		}
	}
	log, err := os.ReadFile(filepath.Join(home, "calls"))
	if err != nil || string(log) != "17 7 82\n" {
		t.Fatalf("command status/duration lost: %q (%v)", log, err)
	}
}

func TestReloadDisablesOnlyOwnedWidgetHooksAndKeepsTimestamp(t *testing.T) {
	_, bin := hookSandbox(t)
	cfg := defaultConfig()
	cfg.InstantPrompt = false
	enabled := zshInitScriptForBinary(cfg, bin)
	cfg.ViMode, cfg.TransientPrompt = false, false
	disabled := zshInitScriptForBinary(cfg, bin)
	out := runZsh(t, `zmodload zsh/zle
zmodload zsh/zleparameter
custom_accept() { zle .accept-line }
custom_keymap() { return 0 }
zle -N accept-line custom_accept
zle -N zle-keymap-select custom_keymap
`+enabled+"\n_mehshell_ts=123\n"+disabled+`
print -r -- "timestamp=$_mehshell_ts accept=${widgets[accept-line]}"
zstyle -a zle-keymap-select widgets keymap_hooks
print -r -- "keymap=${(j:,:)keymap_hooks}"
zstyle -a zle-line-finish widgets finish_hooks
print -r -- "finish=${(j:,:)finish_hooks}"
`)
	if !strings.Contains(out, "timestamp=123 accept=user:custom_accept") || !strings.Contains(out, "user:custom_keymap") || strings.Contains(out, ":_mehshell_") {
		t.Fatalf("reload changed foreign widgets or kept disabled hooks: %q", out)
	}
}

func TestReloadDoesNotSourceStaleInstantCache(t *testing.T) {
	home, bin := hookSandbox(t)
	if err := os.WriteFile(filepath.Join(home, "mehshell-prompt-cache-dumb"), []byte("PROMPT='cached > '\n"), 0600); err != nil {
		t.Fatal(err)
	}
	init := zshInitScriptForBinary(defaultConfig(), bin)
	out := runZsh(t, init+"\nprint -r -- $PROMPT\nPROMPT='current > '\n"+init+"\nprint -r -- $PROMPT\n")
	if out != "cached > \ncurrent > \n" {
		t.Fatalf("reload sourced stale cache: %q", out)
	}
}

func TestTransientHookWorksWithoutReplacingAcceptLineInDisposablePTY(t *testing.T) {
	home, bin := hookSandbox(t)
	cfg := defaultConfig()
	cfg.InstantPrompt = false
	setup := `zmodload zsh/zle
custom_accept() { print -r -- accepted >> "$HOME/events"; zle .accept-line }
zle -N accept-line custom_accept
` + zshInitScriptForBinary(cfg, bin) + `
observe_finished_prompt() { print -r -- "$PROMPT" >> "$HOME/events" }
add-zle-hook-widget line-finish observe_finished_prompt
`
	path := filepath.Join(home, "setup.zsh")
	if err := os.WriteFile(path, []byte(setup), 0600); err != nil {
		t.Fatal(err)
	}
	// zpty allocates a new pseudoterminal; it never opens or modifies a live TTY.
	script := fmt.Sprintf(`zmodload zsh/zpty || exit 77
zmodload zsh/zselect || exit 77
zpty -b prompt-test zsh -f -i
zpty -w prompt-test "source '%s'"
zpty -w prompt-test false
zpty -w prompt-test exit
repeat 100; do
  zpty -t prompt-test || break
  zselect -t 1
done
zpty -d prompt-test 2>/dev/null
true
`, path)
	runZsh(t, script)
	data, err := os.ReadFile(filepath.Join(home, "events"))
	if err != nil || strings.Count(string(data), "accepted\n") != 2 || strings.Count(string(data), "transient > \n") != 2 {
		t.Fatalf("foreign accept widget or transient redraw did not run: %q (%v)", data, err)
	}
	log, err := os.ReadFile(filepath.Join(home, "calls"))
	if err != nil || !strings.Contains(string(log), "1 0 ") {
		t.Fatalf("PTY prompt did not receive false's exit status: %q (%v)", log, err)
	}
}

func TestPromptSubstitutionNeverExecutesDirectoryText(t *testing.T) {
	home, bin := hookSandbox(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	text.WriteString("brand=none\nstyle=lean\n")
	for name := range defaultConfig().Segments {
		fmt.Fprintf(&text, "%s=%t\n", name, name == "dir" || name == "git")
	}
	if err := os.WriteFile(configPath(), []byte(text.String()), 0600); err != nil {
		t.Fatal(err)
	}
	name := "repo-$(touch marker)-`touch backtick`-%F{1}"
	directory := filepath.Join(home, name)
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	branch := "feature-$(touch${IFS}branch_marker)-`touch${IFS}branch_backtick`"
	if err := os.Mkdir(filepath.Join(directory, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commands := filepath.Join(home, "bin")
	if err := os.Mkdir(commands, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commands, "git"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", commands+":/usr/bin:/bin")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Error(err)
		}
	})
	for _, term := range []string{"linux", "dumb", "xterm-kitty"} {
		t.Run(term, func(t *testing.T) {
			t.Setenv("TERM", term)
			generated := generatedPromptForTest(t)
			quoted := "'" + strings.ReplaceAll(generated, "'", "'\\''") + "'"
			if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' "+quoted+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cache := "mehshell-prompt-cache"
			if term != "xterm-kitty" {
				cache += "-" + term
			}
			if err := os.WriteFile(filepath.Join(home, cache), []byte(generated), 0600); err != nil {
				t.Fatal(err)
			}
			for _, option := range []string{"setopt", "unsetopt"} {
				direct := runZsh(t, option+" promptsubst\n"+generated+"\nprint -P -- \"$PROMPT\"\nprint -r -- option=$options[promptsubst]\n")
				if !strings.Contains(direct, name) || !strings.Contains(direct, branch) {
					t.Fatalf("directory was not rendered literally with %s: %q", option, direct)
				}
				wantOption := "off"
				if option == "setopt" {
					wantOption = "on"
				}
				if !strings.Contains(direct, "option="+wantOption) {
					t.Fatal("direct prompt changed the user's PROMPT_SUBST option")
				}
				if term != "dumb" && !strings.ContainsRune(direct, '\x1b') {
					t.Fatal("parameter wrapper stopped normal prompt colour expansion")
				}
			}
			init := zshInitScriptForBinary(defaultConfig(), bin)
			script := "setopt promptsubst\n" + init + `
print -P -- "$PROMPT"
(exit 7)
_mehshell_precmd
print -P -- "$PROMPT"
zle() { return 0 }
KEYMAP=vicmd
_mehshell_zle_keymap_select
print -P -- "$PROMPT"
_mehshell_line_finish
print -P -- "$PROMPT"
print -r -- "option=$options[promptsubst]"
`
			output := runZsh(t, script)
			if strings.Count(output, name) != 3 || !strings.Contains(output, "option=on") {
				t.Fatalf("cache/runtime/vi hook did not preserve literal prompt text: %q", output)
			}
			for _, filename := range []string{"marker", "backtick", "branch_marker", "branch_backtick"} {
				if _, err := os.Stat(filepath.Join(directory, filename)); !os.IsNotExist(err) {
					t.Fatalf("prompt text executed a command and created %s", filename)
				}
			}
		})
	}
}
