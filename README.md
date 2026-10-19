# mehshell 🐚

A fast zsh prompt engine for [bromigOS](https://bromigos.org), written in Go.
Created by [blackflame007](https://github.com/blackflame007).
See the [bromigOS manual](https://github.com/bromigos-org/bromigOS/blob/main/manual/src/shell.md) for desktop setup.

## Prompt Example

![Prompt styles](docs/screenshots/prompt_styles_screenshot.png)

## ✨ Features

- 🚀 Single Go binary with bounded local probes.
- 🔀 All segments run as parallel goroutines.
- 🔥 Git status check with a 150ms command budget; slow or failed reads show status?.
- 🧠 Local Git metadata provides branch and operation labels without a network request.
- 🖥️ OS icon auto-detection for Arch, Ubuntu, Fedora, Debian, NixOS, and macOS.
- 📦 Runtime version detection for Node.js, Python, Go, Rust, Ruby, and Java when marker files are present.
- ☁️ Cloud context support for Kubernetes, AWS, Azure, and GCP profiles.
- 🏗️ Terraform workspace and Docker context detection.
- 🐍 Environment detection for Conda and virtualenv.
- 🔋 Battery level with charge state icons.
- ⏱️ Command duration (3s+), time, and exit code coloring.
- ✏️ Vi mode indicator (swaps prompt char on keymap change).
- 🪄 Transient prompt (collapses previous prompt on Enter).
- ⚡ Instant prompt (caches a successful prompt for zero-latency shell startup).
- ➡️ Right-aligned segments on the first line.
- 🎨 Nerd Font icons.
- 🔄 Segment, style and colour changes reload at the next prompt.
- ⚙️ Configurable via `~/.config/mehshell/config`.
- Narrow-window layouts, counted Git changes, worktree and operation labels.
- bromigOS branding, SSH/root identity, readable failures and local game-mode state.
- `doctor`, `preview` and `config set` commands. See [bromigOS integration](docs/bromigos-integration.md).

## 📥 Install

### bromigOS

With the signed bromigOS repository configured (packages from
[pkgs.bromigos.org](https://pkgs.bromigos.org)):

```bash
sudo pacman -S mehshell
bromigos shell prompt mehshell
```

### Homebrew (macOS & Linux)

```bash
brew tap bromigos-org/tap
brew install mehshell
```

### AUR (Arch Linux)

The `mehshell` source package is published by the release workflow. Its maintainer
credit is `bromigOS <packages@bromigos.org>`. The AUR account `blackflame` publishes it.

```bash
paru -S mehshell
```

The old `blackflame007/tap` Homebrew tap and `mehshell-bin` AUR package are legacy
installation routes.

### Binary releases

Download a prebuilt binary from [GitHub Releases](https://github.com/bromigos-org/mehshell/releases/latest):

```bash
# Linux (x86_64)
curl -sL https://github.com/bromigos-org/mehshell/releases/latest/download/mehshell_linux_amd64.tar.gz | tar xz && mv mehshell ~/.local/bin/

# Linux (ARM64)
curl -sL https://github.com/bromigos-org/mehshell/releases/latest/download/mehshell_linux_arm64.tar.gz | tar xz && mv mehshell ~/.local/bin/

# macOS (Apple Silicon)
curl -sL https://github.com/bromigos-org/mehshell/releases/latest/download/mehshell_darwin_arm64.tar.gz | tar xz && mv mehshell ~/.local/bin/

# macOS (Intel)
curl -sL https://github.com/bromigos-org/mehshell/releases/latest/download/mehshell_darwin_amd64.tar.gz | tar xz && mv mehshell ~/.local/bin/
```

### From source

```bash
go install github.com/bromigos-org/mehshell@latest
```

## 🔌 Zsh Integration

Add this to your `.zshrc`:

```zsh
eval "$(mehshell init zsh)"
```

A red `exit:N` reports a command that failed in this shell. Opening a terminal,
pressing Enter on an empty line, or cancelling unfinished input with Ctrl+C leaves
a normal prompt. Interrupting a running command shows a grey `interrupted` label.
After updating mehshell, open a new shell or run the init line again to load its hooks.

### Linux text consoles

On a Linux text console (`TERM=linux`), mehshell automatically uses text labels,
plain `>` and `<` prompt marks, and the console's basic eight colours. The prompt
uses a plain layout even if your desktop style is `classic` or `rainbow`.
With `TERM=dumb`, it also leaves out colour formatting.

Your saved font, icon, style and colour settings stay unchanged. Desktop terminals
keep using them. Normal, instant, transient and vi-mode prompts use the same fallback;
console and desktop prompts have separate caches. You do not need to change your
`mehshell init zsh` line. mehshell does not load or change the console font.

[Terminal support](docs/terminal-support.md) explains the behaviour and its tests.

## 🛠️ Configuration

Generate a default config file:

```bash
mehshell config init
```

This creates `~/.config/mehshell/config` (or `$XDG_CONFIG_HOME/mehshell/config`). See [config.example](config.example) for the full default config.

### Features

| Option             | Default | Description                                                                                                                                                          |
| ------------------ | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `transient_prompt` | `true`  | Collapse previous prompts to `❯` on Enter. Set `false` to preserve full prompts with timestamps in scrollback.                                                       |
| `instant_prompt`   | `true`  | Show cached prompt immediately on shell startup.                                                                                                                     |
| `vi_mode`          | `true`  | Swap prompt character on vi keymap change (`❯`/`❮`).                                                                                                                 |
| `icons`            | `true`  | Show Nerd Font icons. Set `false` for text labels (`go`, `py`, `rs`, etc.).                                                                                          |
| `brand`            | `auto` | Identity label: `auto`, `bromigos`, `os` or `none`. |
| `status`           | `true` | Show the exit code after a failed command. |
| `style`            | `lean`  | Prompt style: `lean` (colored text, no backgrounds), `classic` (powerline with dark grey background), or `rainbow` (powerline with per-segment colored backgrounds). |

### Left Segments

| Option   | Default | Description                                               |
| -------- | ------- | --------------------------------------------------------- |
| `os`     | `true`  | bromigOS badge or OS identity, chosen by `brand`. |
| `session` | `true` | Root or SSH session identity. |
| `dir`    | `true`  | Current working directory.                                |
| `git`    | `true`  | Branch, change counts, upstream, operations and linked worktrees.          |
| `node`   | `true`  | Node.js version when `package.json` is present.           |
| `python` | `true`  | Python version when marker files are found.               |
| `go`     | `true`  | Go version from `go.mod`.                                 |
| `rust`   | `true`  | Rust version when `Cargo.toml` is present.                |
| `ruby`   | `true`  | Ruby version when marker files are found.                 |
| `java`   | `true`  | Java version when `pom.xml` or `build.gradle` is present. |

### Right Segments

| Option      | Default | Description                                        |
| ----------- | ------- | -------------------------------------------------- |
| `game`      | `true` | GAME while bromigOS has a saved game-mode state. |
| `conda`     | `true`  | Conda environment name (skips `base`).             |
| `venv`      | `true`  | Python virtualenv name.                            |
| `k8s`       | `true`  | Kubernetes context when manifest files are in cwd. |
| `terraform` | `true`  | Terraform version when `.tf` files are present.    |
| `docker`    | `true`  | Docker context when Dockerfile/compose is present. |
| `aws`       | `true`  | AWS profile from `$AWS_PROFILE`.                   |
| `azure`     | `true`  | Azure resource group from `$AZURE_DEFAULTS_GROUP`. |
| `gcloud`    | `true`  | GCP project from `$CLOUDSDK_CORE_PROJECT`.         |
| `battery`   | `true`  | Battery level with charge state icon.              |
| `duration`  | `true`  | Command duration (shown when ≥ 3s).                |
| `time`      | `true`  | Current time.                                      |

Set any segment to `false` to disable it. Segment, style and colour changes appear
at the next prompt. Restart zsh after changing `transient_prompt`,
`instant_prompt` or `vi_mode`, which install shell hooks.

To see where your config lives:

```bash
mehshell config path
```

## 📊 Measure your prompt

Run `mehshell doctor --timings` inside a project to measure local segments. Results
depend on repository size, storage and installed runtimes. No remote fetch is made.

## 🆚 Feature Comparison with Powerlevel10k

mehshell is intentionally minimal. p10k is a full-featured theme engine. Pick the right tool for your workflow.

| Feature                  | mehshell                        | p10k                            |
| ------------------------ | ------------------------------- | ------------------------------- |
| **Architecture**         | Single Go binary                | Zsh scripts + gitstatusd daemon |
| **Dependencies**         | Zero (Go stdlib only)           | gitstatus binary (downloaded)   |
| **Configuration**        | `~/.config/mehshell/config`     | `~/.p10k.zsh` + config wizard   |
| **Async rendering**      | Goroutines (parallel, bounded commands)           | Zsh workers + gitstatusd        |
| **Git branch detection** | Zero-fork (reads `.git/HEAD`)   | gitstatusd (libgit2)            |
| **Git dirty check**      | `git status` with 150ms timeout | Async, never blocks             |
| **Instant prompt**       | ✓ (cache-based)                 | ✓                               |
| **Transient prompt**     | ✓                               | ✓                               |
| **Vi mode**              | ✓ (prompt char swap)            | ✓ (full indicator)              |
| **Custom segments**      | Add in source                   | Public API (`p10k segment`)     |
| **Total segments**       | 23 switches                     | 67+                             |

### Segment Coverage

| Segment    | mehshell           | p10k                                       |
| ---------- | ------------------ | ------------------------------------------ |
| OS icon    | ✓                  | ✓                                          |
| Directory  | ✓ (adaptive truncation) | ✓ (smart truncation)                       |
| Git        | ✓ (counts, upstream, operations, worktrees) | ✓ (branch, ahead/behind, stash, conflicts) |
| Node.js    | ✓                  | ✓ (+ nvm, nodenv, package name)            |
| Python     | ✓                  | ✓ (+ pyenv, poetry)                        |
| Go         | ✓                  | ✓ (+ goenv)                                |
| Rust       | ✓                  | ✓                                          |
| Ruby       | ✓                  | ✓ (+ rvm, chruby)                          |
| Java       | ✓                  | ✓ (+ jenv)                                 |
| Conda      | ✓                  | ✓                                          |
| Virtualenv | ✓                  | ✓                                          |
| Kubernetes | ✓                  | ✓ (+ show-on-command)                      |
| Terraform  | ✓                  | ✓                                          |
| Docker     | ✓                  | ✓                                          |
| AWS        | ✓                  | ✓ (+ Elastic Beanstalk)                    |
| Azure      | ✓                  | ✓                                          |
| GCP        | ✓                  | ✓                                          |
| Battery    | ✓                  | ✓                                          |
| Duration   | ✓                  | ✓                                          |
| Time       | ✓                  | ✓ (+ date)                                 |
| CPU / RAM  | —                  | ✓                                          |
| Vi mode    | ✓ (prompt char)    | ✓ (full indicator)                         |

> **Note**: p10k is in limited maintenance mode — no new features are in development.

## 📋 Requirements

- Go 1.22+ (build only)
- A Nerd Font for optional desktop-terminal icons; text consoles use text labels
- zsh

## 📄 License

MIT

Unicode width data uses Unicode License V3; see [LICENSE-UNICODE](LICENSE-UNICODE).

### Theme colours

`color_75 = #159b09` replaces a zsh palette index with a hex colour.
The same form works for any foreground or background index. bromigOS writes
these values when you change theme. Linux text consoles use basic colours instead;
`TERM=dumb` disables colour formatting.

## Publishing releases

Push a new version tag after the Go checks pass. The workflow publishes the Linux
and macOS binaries, updates `bromigos-org/homebrew-tap`, and builds and publishes
the AUR source package `mehshell`. It never moves an existing tag.

For Homebrew, create a fine-grained personal access token in GitHub Settings →
Developer settings → Personal access tokens → Fine-grained tokens → Generate new
token. Choose `bromigos-org` as Resource owner. Choose Only select repositories and
select `homebrew-tap`. Grant Repository permissions → Contents → Read and write.
Leave other permissions at their defaults. Complete any organisation approval.
See [GitHub’s token guide](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens).
Set an expiry and save the token as a secret without putting it in a command:

```bash
gh secret set HOMEBREW_TAP_TOKEN -R bromigos-org/mehshell
```

Paste the token at the hidden prompt. The workflow checks the token's push
permission on the tap before releasing. It prints no token value.

AUR publishing uses `AUR_SSH_PRIVATE_KEY`, already configured for `blackflame`.
To add a future bromigOS co-maintainer, register at
[aur.archlinux.org](https://aur.archlinux.org) with `packages@bromigos.org` when
registration is open. Generate a separate SSH key, add its public key in the account's
profile, and add the new account as co-maintainer of `mehshell`.

```bash
ssh-keygen -t ed25519 -f ~/.ssh/aur-bromigos-future -C packages@bromigos.org
cat ~/.ssh/aur-bromigos-future.pub
```

Only the public key goes in the AUR profile. The package repository is
`ssh://aur@aur.archlinux.org/mehshell.git`. A source build and tests must pass
before the workflow pushes its PKGBUILD and `.SRCINFO`.
