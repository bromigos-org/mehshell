# A prompt for bromigOS

mehshell follows bromigOS's theme and shows a small `bromigOS` text badge. The badge
works on text consoles too. Standalone installations keep their OS icon.

```sh
bromigos shell doctor
bromigos shell preview --width 40
bromigos shell configure style lean
bromigos shell configure brand bromigos
```

The same tools work directly as `mehshell doctor`, `mehshell preview` and
`mehshell config set KEY VALUE`. They configure mehshell without switching the chosen
prompt engine. Edits preserve comments, theme colours and symlinks to dotfiles.
Style, segment and brand changes appear at the next prompt. Hook options take effect
in a new shell.

## Branding and session context

`brand` accepts `auto`, `bromigos`, `os` or `none`. Auto detects the packaged bromigOS
shell defaults or `MEHSHELL_BROMIGOS=1`. Set that variable to `0` to use the portable
default. Explicit brand choices take precedence. The existing `os` switch hides or
shows the identity segment.

SSH sessions show `ssh:user@host`; root shows `ROOT`. Ordinary local sessions omit
the extra identity. A failed command shows `exit:N`, including on plain terminals.
Only a command executed in the current shell supplies that status. Shell startup,
blank Enter and Ctrl+C while editing clear a stale failure. Interrupting a running
command (status 130) shows a grey `interrupted` label instead of an error.
`GAME` appears while bromigOS's local game-mode record is present. No compositor,
server or API is contacted for these indicators. Disable them separately with
`session = false`, `status = false` or `game = false`.

## Narrow terminals and Git

The prompt shortens long paths and branch labels as a window narrows. Clock, battery
and runtime details give way before the current folder and session identity.
Small layouts use plain separators. Width measurement handles combining characters,
wide characters and common emoji sequences. Emoji rendering can still differ by
terminal and font.

Git shows staged (`+`), modified (`!`) and untracked (`?`) file counts, conflicts,
ahead/behind counts, active operations and `wt:` for linked worktrees. Detached HEAD
and new repositories have explicit labels. The command has a 150 ms budget and a
2 MiB output limit. `status?` means the read failed or exceeded its budget; it never
claims the tree is clean. `upstream?` means the local comparison is unavailable.
Ahead/behind counts use the last locally known upstream. No fetch runs. Optional
index writes and fsmonitor hooks are disabled.

## Preview and diagnostics

`mehshell preview --style rainbow --width 60 --exit-code 1` tries a layout and a
failure marker without saving it. `--plain` leaves out colour escapes. Preview output
is never evaluated as shell code.

`doctor --json` returns structured diagnostics. `doctor --timings` measures local
project and runtime segments. It does not print segment contents or probe lab
services and credential providers. Configuration warnings identify a line and
setting without echoing its value. Font glyph coverage and installed shell hooks
need separate inspection; doctor reports what the next generated prompt will use.

Reloading `mehshell init zsh` keeps one copy of its hooks and preserves other
plugins' widgets. Transient prompts use zsh's line-finish hook rather than replacing
the accept-line widget.

## Tests and data

`go test -race ./...` covers temporary Git repositories, worktrees and conflicts;
configuration edits; desktop and console layouts; diagnostics; and isolated zsh
hooks. Tests never change a live shell or desktop.

Width ranges derive from Unicode 17's East Asian Width data. Its permission notice
ships in [LICENSE-UNICODE](../LICENSE-UNICODE). No Go module dependency is added.
