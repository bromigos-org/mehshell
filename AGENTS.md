# PROJECT KNOWLEDGE BASE

**Generated:** 2026-03-14
**Commit:** 0e4c334
**Branch:** master

## OVERVIEW

Fast, parallelized zsh prompt engine. Single Go binary, stdlib-only, ~6ms per prompt. Outputs `PROMPT` variable via `eval "$(mehshell $e $d $COLUMNS)"`. Supports three styles (lean/classic/rainbow), instant prompt, transient prompt, vi mode, icons toggle, and config file at `~/.config/mehshell/config`.

## STRUCTURE

```
mehshell/
├── main.go              # Entry point, CLI dispatch, segment orchestration (196 lines)
├── config.go            # Config struct, loadConfig(), defaultConfig(), configPath() (137 lines)
├── segments.go          # All 20 seg*() implementations + hasMarkerUp() (532 lines)
├── render.go            # Color constants, fc(), powerline rendering, visibleWidth() (156 lines)
├── git.go               # gitBranch() zero-fork detection, gitDirty() with 150ms timeout (101 lines)
├── init.go              # zshInitScript() — generates zsh hooks (55 lines)
├── go.mod               # Go 1.22, zero dependencies
├── config.example       # Reference config with all options documented
├── .goreleaser.yml      # Cross-platform release builds + Homebrew tap
├── .github/workflows/
│   └── release.yml      # Tag → GoReleaser → optional AUR publish pipeline
├── aur/
│   ├── PKGBUILD         # Arch Linux package definition
│   └── LICENSE          # MIT license for AUR
├── README.md
└── .gitignore           # Excludes: mehshell binary, dist/
```

Six-file layout, single `main` package. No `internal/`, `cmd/`, `pkg/` — intentional at ~1200 lines.

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add/modify prompt segment | `segments.go` for `seg*()` impl, `main.go` for registration | Follow `seg*()` pattern, gate with `on["name"]`, register via `add()` |
| Add config option | `config.go` — struct + `defaultConfig()` + `loadConfig()` switch | Also update `defaultConfigFile` const and `config.example` |
| Change rendering / styles | `render.go` | `renderPowerlineLeft/Right()` for classic/rainbow, `joinSegs()` for lean |
| Prompt layout (line1/line2) | `main.go` lines 162-195 | Style switch at L169, padding calc, `PROMPT` output |
| Git detection | `git.go` | `gitBranch()` zero-fork via `.git/HEAD`, `gitDirty()` 150ms timeout |
| Zsh init script | `init.go` | Dynamic generation based on config (transient, instant, vi mode) |
| Color constants | `render.go` L9-18 | Zsh 256-color codes: `cCyan=75`, `cBlue=39`, etc. |
| Version injection | `main.go` L13 | `var Version = "dev"`, set via ldflags: `-X main.Version={{.Version}}` |
| Release config | `.goreleaser.yml` | Linux + macOS, amd64 + arm64, CGO off, Homebrew tap |
| CI pipeline | `.github/workflows/release.yml` | Checks on push; tag → GoReleaser → optional AUR publish |
| AUR package | `aur/PKGBUILD` | Update `pkgver` manually or via CI |

## ARCHITECTURE

### Execution Flow
1. CLI dispatch: `--version`, `init zsh`, `config init|path`, or prompt generation
2. `loadConfig()` reads `~/.config/mehshell/config`, merges with defaults
3. `add()` closure spawns goroutines for each enabled segment via `sync.WaitGroup`
4. Each `seg*()` returns formatted string or `""` (skip)
5. Mutex-protected append to `left`/`right` slices, sorted by `order` after `wg.Wait()`
6. Style switch: lean (box-drawing + text), classic (powerline dark bg), rainbow (powerline per-segment colors)
7. Right-aligned line 1 + prompt char line 2 → `PROMPT=$'...'`

### Segment Registration Pattern
```go
// main.go — add() closure wraps goroutine + WaitGroup + mutex
add(&left, ORDER, BG_COLOR, func() string { return segFoo(args) })
```
- `order` int controls display position
- `bg` int is background color for powerline styles (ignored in lean)
- Return `""` to hide segment
- Gate with `if on["name"] { add(...) }`

### File Responsibilities
| File | Role | Key Exports |
|------|------|-------------|
| `main.go` | Orchestration — CLI routing, concurrency, prompt assembly | `main()`, `add()` closure |
| `config.go` | Configuration — loading, parsing, defaults | `config` struct, `loadConfig()`, `configPath()` |
| `segments.go` | Segments — all 20 implementations | `seg*()` functions, `hasMarkerUp()` |
| `render.go` | Rendering — colors, styles, width calculation | `fc()`, `seg` struct, `renderPowerline*()`, `visibleWidth()` |
| `git.go` | Git — branch detection, dirty state | `gitBranch()`, `gitDirty()` |
| `init.go` | Zsh — hook script generation | `zshInitScript()` |

### Segment Functions
| Function | File:Line | Side | Order | BG | Trigger |
|----------|-----------|------|-------|----|---------|
| `segOS` | segments.go:16 | left | 0 | 24 | Always (reads `/etc/os-release`); hidden when `icons=false` |
| `segDir` | segments.go:44 | left | 1 | 31 | Always |
| `segGit` | segments.go:55 | left | 2 | 97 | `.git` found walking up |
| `segNode` | segments.go:74 | left | 3 | 34 | `package.json` in cwd |
| `segPython` | segments.go:98 | left | 4 | 136 | Marker files walking up |
| `segGo` | segments.go:121 | left | 5 | 37 | `go.mod` in cwd |
| `segRust` | segments.go:142 | left | 6 | 130 | `Cargo.toml` in cwd |
| `segRuby` | segments.go:184 | left | 7 | 124 | Marker files walking up |
| `segJava` | segments.go:210 | left | 8 | 94 | `pom.xml`, `build.gradle`, `build.gradle.kts` in cwd |
| `segConda` | segments.go:245 | right | 0 | 34 | `$CONDA_DEFAULT_ENV` set (not "base") |
| `segVenv` | segments.go:257 | right | 1 | 34 | `$VIRTUAL_ENV` set |
| `segK8s` | segments.go:269 | right | 2 | 25 | K8s manifest in cwd (skaffold, helmfile, Chart, kustomization) |
| `segTerraform` | segments.go:306 | right | 3 | 57 | `.tf` files in cwd |
| `segDocker` | segments.go:341 | right | 4 | 25 | Dockerfile/compose in cwd |
| `segAWS` | segments.go:366 | right | 5 | 166 | `$AWS_PROFILE` set |
| `segAzure` | segments.go:378 | right | 6 | 25 | `$AZURE_DEFAULTS_GROUP` set |
| `segGCloud` | segments.go:390 | right | 7 | 34 | `$CLOUDSDK_CORE_PROJECT` / `$GCLOUD_PROJECT` / `$GOOGLE_CLOUD_PROJECT` set |
| `segBattery` | segments.go:408 | right | 8 | 22 | Battery present (Linux `/sys/`, macOS `pmset`) |
| `segDuration` | segments.go:498 | right | 9 | 136 | Duration ≥ 3s |
| `segTime` | segments.go:514 | right | 10 | 238 | Always |

### External Commands & Timeouts
| Segment | Command | Timeout | Fallback |
|---------|---------|---------|----------|
| `segNode` | `node --version` | 100ms | `.node-version`, `.nvmrc` files first |
| `segPython` | `python --version` | 100ms | `.python-version` file first |
| `segRust` | `rustc --version` | 100ms | `rust-toolchain`, `.rust-version`, `rust-toolchain.toml` first |
| `segRuby` | `ruby --version` | 100ms | `.ruby-version` file first |
| `segJava` | `java -version` | 100ms | `.java-version` file first |
| `segTerraform` | `terraform version` | 100ms | `.terraform-version` file first |
| `segBattery` (macOS) | `pmset -g batt` | 100ms | — |
| `gitDirty` | `git status --porcelain` | **150ms** | Returns `""` on timeout |

### Styles
| Style | Description | Rendering |
|-------|-------------|-----------|
| `lean` (default) | Colored text, no backgrounds | `╭─ segments...` / `╰─❯` with `joinSegs()` |
| `classic` | Powerline with dark grey (238) background | `renderPowerlineLeft/Right()` with `rainbow=false` |
| `rainbow` | Powerline with per-segment colored backgrounds | `renderPowerlineLeft/Right()` with `rainbow=true`, uses `contrastFg()` |

## CONVENTIONS

- **Naming**: `seg*()` for segments, `git*()` for git helpers, short vars (`cwd`, `wg`, `mu`, `cfg`, `on`)
- **Colors**: Use `fc(cColor, text)` helper — never raw `%F{N}` strings
- **Error handling**: Return `""` on any error. Never panic, never log.
- **External commands**: Always wrap in `context.WithTimeout` (100-150ms)
- **File reads over forks**: Prefer `os.ReadFile` over `exec.Command` when possible
- **Section headers**: ASCII box drawing (`// ── Section ──────`)
- **No dependencies**: Stdlib only. Do not add external modules.
- **Icons toggle**: Every segment showing an icon must check the `icons bool` param and provide a text fallback
- **Segment signature**: `seg*(cwd string, icons bool) string` for most; env-only segments take `(icons bool)`
- **Config changes**: Update three places — `config` struct, `defaultConfig()`, `loadConfig()` switch, plus `defaultConfigFile` const and `config.example`

## ANTI-PATTERNS

- **No blocking commands**: Every `exec.Command` MUST have a context timeout
- **No `log.*` or `fmt.Println` to stderr**: Silent operation only (stdout is eval'd by zsh)
- **Don't print to stdout arbitrarily**: Output is `eval`'d — stray output breaks the shell
- **No blank identifier for important errors**: Don't use `_ = err` to swallow errors that affect correctness
- **No external dependencies**: Do not add modules to go.mod

## COMMANDS

```bash
# Build
go build

# Run (simulates prompt generation)
./mehshell 0 5 120

# Init script
./mehshell init zsh

# Create default config
./mehshell config init

# Show config path
./mehshell config path

# Version
./mehshell --version

# Release (after checks pass; never move an existing tag)
git tag v<version>
git push origin v<version>
```

## NOTES

- **Binary in repo**: Compiled `mehshell` ELF exists at root (gitignored but present). Don't confuse with source.
- **Tests**: `go test -race ./...` covers config reload, colour validation and rendering.
- **No linter config**: No golangci-lint, relies on `gofmt` defaults.
- **Licence**: MIT, with the original author credit in `LICENSE` and `aur/LICENSE`.
- **Nerd Font required**: Icons render as boxes without a patched font. `icons=false` switches to text labels.
- **Zsh only**: `init` subcommand only supports `zsh`. No bash/fish/etc.
- **CI secrets**: Optional publishing uses `HOMEBREW_TAP_TOKEN` for `bromigos-org/homebrew-tap` and `AUR_SSH_PRIVATE_KEY`. Enable each route with `HOMEBREW_PUBLISH_ENABLED` or `AUR_PUBLISH_ENABLED` only after the new account is ready. Binary releases need no additional secrets.
- **Versioning**: Releases run only on version tags. Homebrew and AUR publishing are optional until the operator creates their accounts and secrets.
- **Config format**: `key = value` (key=value also works). Boolean values: `true`/`1`/`yes` → true, anything else → false.
- **Style values**: Only `lean`, `classic`, `rainbow` are valid. Invalid values default to lean.
