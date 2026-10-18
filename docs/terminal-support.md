# Terminal support

mehshell adapts its own symbols and colours to the terminal. It does not select a font
or change saved user options.

| Terminal | Prompt symbols and layout | Colours |
|---|---|---|
| Linux text console, `TERM=linux` | Text segment labels, `>` and `<`, plain layout | Basic eight-colour palette |
| `TERM=dumb` | Text segment labels, `>` and `<`, plain layout | No colour formatting |
| Other terminals | Configured icons and `lean`, `classic` or `rainbow` layout | Configured palette and theme colours |

The fallback covers normal prompts, the transient prompt after Enter, and vi-mode
prompt marks. A clean Git tree uses the word `clean` on basic terminals. Actual file
and branch names keep their text; their character coverage depends on the console
font. A Nerd Font installed for a desktop terminal does not change the Linux console
font.

## Instant prompt caches

Startup reads the cache for its terminal class. It never reads a desktop prompt into
a Linux console or dumb terminal. Subsequent prompts write the matching cache.

| Terminal | Cache filename under `$XDG_CACHE_HOME` or `~/.cache` |
|---|---|
| Desktop | `mehshell-prompt-cache` |
| Linux console | `mehshell-prompt-cache-linux` |
| Dumb terminal | `mehshell-prompt-cache-dumb` |

Changing `TERM` after shell initialisation is not a supported way to change terminal
capabilities. Start a new shell in the intended terminal so its hooks and cache match.

## Verification

Run `go test -race ./...` with zsh installed. The runtime tests use temporary homes,
configuration and repository fixtures. Hook tests run `zsh -f` with a temporary home
and a stubbed line editor. They check cache isolation, transient prompts, vi mode,
colour limits, preserved desktop settings and unchanged configuration. No test opens
or modifies a real console.
