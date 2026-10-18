package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func zshInitScript(cfg config) string {
	bin := "mehshell"
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			bin = resolved
		} else {
			bin = exe
		}
	}

	cacheName := "mehshell-prompt-cache"
	if basicTerminal() {
		cacheName += "-" + os.Getenv("TERM")
	}
	cache := "${XDG_CACHE_HOME:-$HOME/.cache}/" + cacheName

	var b strings.Builder
	b.WriteString("zmodload zsh/datetime 2>/dev/null\n")
	b.WriteString("typeset -gi _mehshell_ts=0\n")
	if cfg.InstantPrompt {
		b.WriteString(fmt.Sprintf("[[ -r \"%s\" ]] && source \"%s\"\n", cache, cache))
	}
	b.WriteString("_mehshell_preexec() { _mehshell_ts=$EPOCHSECONDS }\n")
	b.WriteString("_mehshell_precmd() {\n")
	b.WriteString("  local e=$? d=0\n")
	b.WriteString("  (( _mehshell_ts > 0 )) && d=$(( EPOCHSECONDS - _mehshell_ts ))\n")
	b.WriteString("  _mehshell_ts=0\n")
	b.WriteString(fmt.Sprintf("  local _out=\"$(%s $e $d $COLUMNS)\"\n", bin))
	b.WriteString("  eval \"$_out\"\n")
	if cfg.InstantPrompt {
		b.WriteString(fmt.Sprintf("  print -r -- \"$_out\" >| \"%s\" 2>/dev/null\n", cache))
	}
	b.WriteString("}\n")
	if cfg.ViMode {
		b.WriteString("_mehshell_zle_keymap_select() {\n")
		if basicTerminal() {
			b.WriteString("  local _mark='>'\n  [[ $KEYMAP == vicmd ]] && _mark='<'\n")
			suffix := "%f "
			if os.Getenv("TERM") == "dumb" {
				suffix = " "
			}
			// Change only the final prompt mark, preserving > or < in paths.
			b.WriteString(fmt.Sprintf("  PROMPT=\"${PROMPT%%[<>]%s}${_mark}%s\"\n", suffix, suffix))
		} else {
			b.WriteString("  [[ $KEYMAP == vicmd ]] && PROMPT=\"${PROMPT/❯/❮}\" || PROMPT=\"${PROMPT/❮/❯}\"\n")
		}
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("}\n")
		b.WriteString("zle -N zle-keymap-select _mehshell_zle_keymap_select\n")
	}
	if cfg.TransientPrompt {
		b.WriteString("_mehshell_accept_line() {\n")
		b.WriteString(fmt.Sprintf("  PROMPT=${_mehshell_transient:-$'%s '}\n", colourPrompt(fc(cGreen, promptArrow()), cfg)))
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("  zle .accept-line\n")
		b.WriteString("}\n")
		b.WriteString("zle -N accept-line _mehshell_accept_line\n")
	}
	b.WriteString("preexec_functions+=(_mehshell_preexec)\n")
	b.WriteString("precmd_functions+=(_mehshell_precmd)")
	return b.String()
}
