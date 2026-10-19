package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Start a separate interactive shell on a new pseudoterminal, with only the
// generated test configuration. Never read the user's zshrc or use a live TTY.
func runStatusPTY(t *testing.T, home, setup, actions string) {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is required for generated-hook integration tests")
	}
	setup += `
observe_prompt_ready() { print -r -- "$PROMPT" >> "$HOME/prompt-events" }
add-zle-hook-widget line-init observe_prompt_ready
# A harmless optional startup probe can fail without a user command failing.
false
`
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte(setup), 0600); err != nil {
		t.Fatal(err)
	}
	script := `zmodload zsh/zpty || exit 77
zmodload zsh/zselect || exit 77
zpty -b status-test zsh -d -i
wait_for_prompt() {
  repeat 150; do
    if [[ -r "$HOME/prompt-events" ]]; then
      local -a events=("${(@f)$(<"$HOME/prompt-events")}")
      (( $#events >= $1 )) && return 0
    fi
    zselect -t 1
  done
  print -u2 -- "Timed out waiting for prompt $1"
  return 1
}
{
  wait_for_prompt 1 || exit 1
` + actions + `
  zpty -w status-test exit
} always {
  zpty -d status-test 2>/dev/null
}
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, zsh, "-f", "-c", script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("disposable shell failed: %v\n%s", err, output)
	}
}

func loggedStatuses(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	var statuses []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("malformed prompt invocation: %q", line)
		}
		statuses = append(statuses, fields[0])
	}
	return strings.Join(statuses, ",")
}

func TestInteractivePromptDistinguishesCommandsFromInput(t *testing.T) {
	home, bin := hookSandbox(t)
	cfg := defaultConfig()
	cfg.InstantPrompt = false
	runStatusPTY(t, home, zshInitScriptForBinary(cfg, bin), `
  # Blank Enter and cancelling an unfinished command are not failed commands.
  zpty -w -n status-test $'\n'
  wait_for_prompt 2 || exit 1
  zpty -w -n status-test $'unfinished input\003'
  wait_for_prompt 3 || exit 1
  zpty -w status-test '(exit 17)'
  wait_for_prompt 4 || exit 1
  zpty -w -n status-test $'\n'
  wait_for_prompt 5 || exit 1
  # Ctrl+C after execution starts must keep SIGINT's status for the renderer.
  zpty -w status-test 'print running > "$HOME/running"; sleep 10'
  repeat 150; do
    [[ -e "$HOME/running" ]] && break
    zselect -t 1
  done
  [[ -e "$HOME/running" ]] || exit 1
  zselect -t 5
  zpty -w -n status-test $'\003'
  wait_for_prompt 6 || exit 1
`)
	if got := loggedStatuses(t, home); got != "0,0,0,17,0,130" {
		t.Fatalf("startup, Enter, input cancellation, failure, Enter, interruption: got %s", got)
	}
}

func TestNamedPrecmdDoesNotHideUserCommandFailure(t *testing.T) {
	home, bin := hookSandbox(t)
	cfg := defaultConfig()
	cfg.InstantPrompt = false
	setup := "precmd() { return 9 }\n" + zshInitScriptForBinary(cfg, bin)
	runStatusPTY(t, home, setup, `
  zpty -w status-test '(exit 23)'
  wait_for_prompt 2 || exit 1
`)
	if got := loggedStatuses(t, home); got != "0,23" {
		t.Fatalf("named precmd changed startup or user-command status: %s", got)
	}
}

func TestInstantPromptRejectsLegacyFailureCache(t *testing.T) {
	home, bin := hookSandbox(t)
	cache := filepath.Join(home, "mehshell-prompt-cache-dumb")
	if err := os.WriteFile(cache, []byte("PROMPT='exit:1 > '\n"), 0600); err != nil {
		t.Fatal(err)
	}
	init := zshInitScriptForBinary(defaultConfig(), bin)
	output := runZsh(t, "PROMPT='neutral > '\n"+init+"\nprint -r -- \"$PROMPT\"\n")
	if output != "neutral > \n" {
		t.Fatalf("an old shell's cached failure appeared on startup: %q", output)
	}
}

func TestFailedCommandDoesNotPoisonNextShellInstantPrompt(t *testing.T) {
	home, bin := hookSandbox(t)
	code := `#!/bin/sh
printf "PROMPT='status:%s > '\n" "$1"
`
	if err := os.WriteFile(bin, []byte(code), 0700); err != nil {
		t.Fatal(err)
	}
	init := zshInitScriptForBinary(defaultConfig(), bin)
	output := runZsh(t, init+`
false
_mehshell_precmd
_mehshell_preexec
(exit 19)
_mehshell_precmd
print -r -- "$PROMPT"
`)
	if output != "status:19 > \n" {
		t.Fatalf("the current shell lost a real command failure: %q", output)
	}
	cache, err := os.ReadFile(filepath.Join(home, "mehshell-prompt-cache-dumb"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(cache), "# mehshell instant prompt v2\n") || strings.Contains(string(cache), "status:19") {
		t.Fatalf("failure persisted in the cross-session cache: %q", cache)
	}
	if next := runZsh(t, init+"\nprint -r -- \"$PROMPT\"\n"); next != "status:0 > \n" {
		t.Fatalf("a new shell inherited the previous shell's command status: %q", next)
	}
}
