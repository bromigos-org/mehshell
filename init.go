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
	return zshInitScriptForBinary(cfg, bin)
}

// A separate builder lets integration tests use a disposable prompt executable.
func zshInitScriptForBinary(cfg config, bin string) string {
	quotedBin := "'" + strings.ReplaceAll(bin, "'", "'\\''") + "'"

	cacheName := "mehshell-prompt-cache"
	if basicTerminal() {
		cacheName += "-" + os.Getenv("TERM")
	}
	cache := "${XDG_CACHE_HOME:-$HOME/.cache}/" + cacheName

	var b strings.Builder
	b.WriteString("zmodload zsh/datetime 2>/dev/null\n")
	b.WriteString("zmodload zsh/zle 2>/dev/null\n")
	b.WriteString("zmodload zsh/zleparameter 2>/dev/null\n")
	b.WriteString("autoload -Uz add-zle-hook-widget\n")
	b.WriteString("typeset -gi _mehshell_ts=${_mehshell_ts:-0}\n")
	// Expand only this fixed parameter reference when PROMPT_SUBST is enabled.
	// Data inside the parameter (paths and branches) is not recursively evaluated.
	b.WriteString("_mehshell_publish_prompt() {\n")
	b.WriteString("  [[ $PROMPT != '${_mehshell_display}' ]] && _mehshell_display=$PROMPT\n")
	b.WriteString("  if [[ $1 == on ]]; then PROMPT='${_mehshell_display}'; else PROMPT=$_mehshell_display; fi\n")
	b.WriteString("  return 0\n}\n")
	if cfg.InstantPrompt {
		b.WriteString(fmt.Sprintf("if [[ -z ${_mehshell_initialized:-} && -r \"%s\" ]]; then\n", cache))
		// Older caches can contain another command's failure. Only reuse caches
		// written by the successful-command-only policy below.
		b.WriteString("  local _mehshell_cache_header\n")
		b.WriteString(fmt.Sprintf("  IFS= read -r _mehshell_cache_header < \"%s\"\n", cache))
		b.WriteString("  if [[ $_mehshell_cache_header == '# mehshell instant prompt v2' ]]; then\n")
		b.WriteString(fmt.Sprintf("    source \"%s\"\n", cache))
		b.WriteString("    _mehshell_publish_prompt ${options[promptsubst]}\n  fi\n")
		b.WriteString("  unset _mehshell_cache_header\nfi\n")
	}
	b.WriteString("typeset -g _mehshell_initialized=1\n")
	b.WriteString("_mehshell_preexec() { _mehshell_ts=$EPOCHSECONDS; return 0 }\n")
	b.WriteString("_mehshell_precmd() {\n")
	b.WriteString("  local e=$? d=0 _mehshell_subst=${options[promptsubst]}\n")
	b.WriteString("  emulate -L zsh\n")
	b.WriteString("  if (( _mehshell_ts > 0 )); then d=$(( EPOCHSECONDS - _mehshell_ts )); else e=0; fi\n")
	b.WriteString("  _mehshell_ts=0\n")
	b.WriteString(fmt.Sprintf("  local _out=\"$(%s $e $d $COLUMNS)\"\n", quotedBin))
	b.WriteString("  eval \"$_out\"\n")
	b.WriteString("  _mehshell_publish_prompt $_mehshell_subst\n")
	if cfg.InstantPrompt {
		b.WriteString("  if (( e == 0 )); then\n")
		b.WriteString(fmt.Sprintf("    { print -r -- '# mehshell instant prompt v2'; print -r -- \"$_out\"; } >| \"%s\" 2>/dev/null\n", cache))
		b.WriteString("  fi\n")
	}
	b.WriteString("  return 0\n}\n")
	// Upgrade a former direct widget binding only when it is still ours. The
	// hook manager preserves any widget installed by another shell plugin.
	b.WriteString("[[ ${widgets[zle-keymap-select]:-} == user:_mehshell_zle_keymap_select ]] && zle -D zle-keymap-select\n")
	b.WriteString("add-zle-hook-widget -d keymap-select _mehshell_zle_keymap_select\n")
	if cfg.ViMode {
		b.WriteString("_mehshell_zle_keymap_select() {\n")
		b.WriteString("  local _mehshell_subst=${options[promptsubst]}\n")
		b.WriteString("  emulate -L zsh\n")
		b.WriteString("  [[ $_mehshell_subst == on ]] && setopt promptsubst\n")
		b.WriteString("  [[ $PROMPT == '${_mehshell_display}' ]] && PROMPT=$_mehshell_display\n")
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
		b.WriteString("  _mehshell_publish_prompt $_mehshell_subst\n")
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("}\n")
		b.WriteString("add-zle-hook-widget keymap-select _mehshell_zle_keymap_select\n")
	}
	b.WriteString("[[ ${widgets[accept-line]:-} == user:_mehshell_accept_line ]] && zle -A .accept-line accept-line\n")
	b.WriteString("add-zle-hook-widget -d line-finish _mehshell_line_finish\n")
	if cfg.TransientPrompt {
		b.WriteString("_mehshell_line_finish() {\n")
		b.WriteString("  local _mehshell_subst=${options[promptsubst]}\n")
		b.WriteString("  emulate -L zsh\n")
		b.WriteString("  [[ $_mehshell_subst == on ]] && setopt promptsubst\n")
		b.WriteString(fmt.Sprintf("  PROMPT=${_mehshell_transient:-$'%s '}\n", colourPrompt(fc(cGreen, promptArrow()), cfg)))
		b.WriteString("  _mehshell_publish_prompt $_mehshell_subst\n")
		b.WriteString("  zle reset-prompt\n")
		b.WriteString("}\n")
		b.WriteString("add-zle-hook-widget line-finish _mehshell_line_finish\n")
		// Retain the old function for callers that saved its widget themselves;
		// new installations never replace the accept-line widget.
		b.WriteString("_mehshell_accept_line() { _mehshell_line_finish; zle .accept-line }\n")
	} else {
		b.WriteString("_mehshell_line_finish() { return 0 }\n")
	}
	b.WriteString("typeset -ga preexec_functions precmd_functions\n")
	b.WriteString("preexec_functions=(_mehshell_preexec \"${(@)preexec_functions:#_mehshell_preexec}\")\n")
	b.WriteString("precmd_functions=(_mehshell_precmd \"${(@)precmd_functions:#_mehshell_precmd}\")\n")
	b.WriteString("true")
	return b.String()
}
