package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func effectiveBrand(cfg config) string {
	if cfg.Brand != "auto" && cfg.Brand != "" {
		return cfg.Brand
	}
	if value := os.Getenv("MEHSHELL_BROMIGOS"); value != "" {
		if value == "1" {
			return "bromigos"
		}
		return "os"
	}
	if info, err := os.Stat("/usr/share/bromigos/default/zsh/bromigos.zsh"); err == nil && info.Mode().IsRegular() {
		return "bromigos"
	}
	return "os"
}

func segBrand(cfg config, icons bool) string {
	switch effectiveBrand(cfg) {
	case "bromigos":
		return fc(cGreen, "bromigOS")
	case "os":
		return segOS(icons)
	default:
		return ""
	}
}

func sessionLabel(uid int, user, host string, remote bool) string {
	if uid == 0 {
		if remote {
			return "ROOT@" + host
		}
		return "ROOT"
	}
	if remote {
		if user == "" {
			user = fmt.Sprintf("uid:%d", uid)
		}
		return "ssh:" + user + "@" + host
	}
	return ""
}

func segSession() string {
	remote := os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
	host := ""
	if remote {
		if value, err := os.Hostname(); err == nil {
			host = strings.SplitN(value, ".", 2)[0]
		}
		if host == "" {
			host = "remote"
		}
	}
	colour := cOrange
	if os.Geteuid() == 0 {
		colour = cRed
	}
	return fc(colour, sessionLabel(os.Geteuid(), os.Getenv("USER"), host, remote))
}

// The existing game CLI uses this local recovery marker as its saved on/off state.
// Never contact Hyprland, a lab service or a credential provider while drawing a prompt.
func segGame() string {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	path := filepath.Join(runtime, "bromigos-game-on.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 65537))
	if err != nil || len(data) > 65536 {
		return ""
	}
	var marker map[string]json.RawMessage
	if json.Unmarshal(data, &marker) != nil || len(marker) == 0 {
		return ""
	}
	return fc(cOrange, "GAME")
}

func promptMark(cfg config, code int) string {
	colour, label := cGreen, ""
	if code != 0 {
		colour = cRed
		if cfg.Segments["status"] {
			label = fmt.Sprintf("exit:%d ", code)
		}
	}
	return fc(colour, label+promptArrow())
}

func fittedPromptMark(cfg config, code, columns int) string {
	mark := promptMark(cfg, code)
	if visibleWidth(mark) > columns-2 {
		colour := cGreen
		if code != 0 {
			colour = cRed
		}
		label := promptArrow()
		if code != 0 && cfg.Segments["status"] && columns >= 4 {
			label = "!" + label
		}
		mark = fc(colour, label)
	}
	return fitPromptText(mark, columns-2)
}
