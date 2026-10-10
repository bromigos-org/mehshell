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

	var b strings.Builder
	b.WriteString("zmodload zsh/datetime 2>/dev/null\n")
	b.WriteString("typeset -gi _mehshell_ts=0\n")
	if cfg.InstantPrompt {
		b.WriteString("[[ -r \"${XDG_CACHE_HOME:-$HOME/.cache}/mehshell-prompt-cache\" ]] && source \"${XDG_CACHE_HOME:-$HOME/.cache}/mehshell-prompt-cache\"\n")
	}
	b.WriteString("_mehshell_preexec() { _mehshell_ts=$EPOCHSECONDS }\n")
	b.WriteString("_mehshell_precmd() {\n")
	b.WriteString("  local e=$? d=0\n")
	b.WriteString("  (( _mehshell_ts > 0 )) && d=$(( EPOCHSECONDS - _mehshell_ts ))\n")
	b.WriteString("  _mehshell_ts=0\n")
	b.WriteString(fmt.Sprintf("  local _out=\"$(%s $e $d $COLUMNS)\"\n", bin))
	b.WriteString("  eval \"$_out\"\n")
	if cfg.InstantPrompt {
		b.WriteString("  print -r -- \"$_out\" >| \"${XDG_CACHE_HOME:-$HOME/.cache}/mehshell-prompt-cache\" 2>/dev/null\n")
	}
	b.WriteString("}\n")
	if cfg.ViMode {
		b.WriteString("_mehshell_zle_keymap_select() {\n")
		b.WriteString("  [[ $KEYMAP == vicmd ]] && PROMPT=\"${PROMPT/❯/❮}\" || PROMPT=\"${PROMPT/❮/❯}\"\n")
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("}\n")
		b.WriteString("zle -N zle-keymap-select _mehshell_zle_keymap_select\n")
	}
	if cfg.TransientPrompt {
		b.WriteString("_mehshell_accept_line() {\n")
		b.WriteString(fmt.Sprintf("  PROMPT=$'%s '\n", colourPrompt("%F{76}❯%f", cfg)))
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("  zle .accept-line\n")
		b.WriteString("}\n")
		b.WriteString("zle -N accept-line _mehshell_accept_line\n")
	}
	b.WriteString("preexec_functions+=(_mehshell_preexec)\n")
	b.WriteString("precmd_functions+=(_mehshell_precmd)")
	return b.String()
}
