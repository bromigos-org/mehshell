package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	TransientPrompt bool
	InstantPrompt   bool
	ViMode          bool
	Icons           bool
	Style           string
	Colours         map[string]string
	Segments        map[string]bool
}

func defaultConfig() config {
	return config{
		TransientPrompt: true,
		InstantPrompt:   true,
		ViMode:          true,
		Icons:           true,
		Style:           "lean",
		Colours:         make(map[string]string),
		Segments: map[string]bool{
			"os": true, "dir": true, "git": true, "node": true,
			"python": true, "go": true, "rust": true, "ruby": true,
			"java": true, "conda": true, "venv": true, "k8s": true,
			"terraform": true, "docker": true, "aws": true, "azure": true,
			"gcloud": true, "battery": true, "duration": true, "time": true,
		},
	}
}

func configPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mehshell", "config")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "mehshell", "config")
}

func loadConfig() config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		on := val == "true" || val == "1" || val == "yes"
		switch key {
		case "transient_prompt":
			cfg.TransientPrompt = on
		case "instant_prompt":
			cfg.InstantPrompt = on
		case "vi_mode":
			cfg.ViMode = on
		case "icons":
			cfg.Icons = on
		case "style":
			switch val {
			case "lean", "classic", "rainbow":
				cfg.Style = val
			}
		default:
			if strings.HasPrefix(key, "color_") && validColour(val) {
				cfg.Colours[strings.TrimPrefix(key, "color_")] = val
			}
			if _, ok := cfg.Segments[key]; ok {
				cfg.Segments[key] = on
			}
		}
	}
	return cfg
}

const defaultConfigFile = `# mehshell configuration
# Optional theme colours: color_<256-colour index> = #RRGGBB
# color_75 = #159b09

#
# After changing, restart your shell or run: source <(mehshell init zsh)
# All options default to true if not specified.

# ── Features ────────────────────────────────────────────

# Collapse previous prompts to a simple ">" on Enter.
# Set to false to preserve full prompts with timestamps in scrollback.
transient_prompt = true

# Show cached prompt immediately on shell startup.
instant_prompt = true

# Swap prompt character on vi keymap change.
vi_mode = true

# Show Nerd Font icons. Set to false for text labels instead.
# TERM=linux and TERM=dumb always use text labels and a plain prompt layout.
icons = true

# Prompt style: lean, classic, or rainbow.
#   lean    - colored text, no backgrounds (default)
#   classic - powerline arrows with segment-colored backgrounds
#   rainbow - powerline arrows with cycling rainbow backgrounds
style = lean

# ── Left segments ───────────────────────────────────────

os = true
dir = true
git = true
node = true
python = true
go = true
rust = true
ruby = true
java = true

# ── Right segments ──────────────────────────────────────

conda = true
venv = true
k8s = true
terraform = true
docker = true
aws = true
azure = true
gcloud = true
battery = true
duration = true
time = true
`

// Only hexadecimal colours can enter zsh's evaluated prompt output.
func validColour(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func colourPrompt(s string, cfg config) string {
	if basicTerminal() {
		return s
	}
	for index, colour := range cfg.Colours {
		for _, prefix := range []string{"%F{", "%K{"} {
			s = strings.ReplaceAll(s, prefix+index+"}", prefix+colour+"}")
		}
	}
	return s
}
