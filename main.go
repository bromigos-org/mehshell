package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "doctor":
			os.Exit(runDoctor(os.Args[2:]))
		case "preview":
			os.Exit(runPreview(os.Args[2:]))
		case "--help", "-h":
			fmt.Println("mehshell: init zsh | doctor [--json] [--timings] | preview [--width N] [--style lean|classic|rainbow] | config init|path|set KEY VALUE | --version")
			return
		case "--version", "-v":
			fmt.Println("mehshell", Version)
			return
		case "init":
			if len(os.Args) > 2 && os.Args[2] == "zsh" {
				cfg := loadConfig()
				fmt.Println(zshInitScript(cfg))
			} else {
				fmt.Fprintln(os.Stderr, "usage: mehshell init zsh")
				os.Exit(1)
			}
			return
		case "config":
			if len(os.Args) > 2 && os.Args[2] == "set" {
				os.Exit(runConfigSet(os.Args[3:]))
			}
			if len(os.Args) > 2 && os.Args[2] == "init" {
				p := configPath()
				if _, err := os.Stat(p); err == nil {
					fmt.Fprintln(os.Stderr, "config already exists:", p)
					os.Exit(1)
				}
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
					os.Exit(1)
				}
				if err := os.WriteFile(p, []byte(defaultConfigFile), 0644); err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
					os.Exit(1)
				}
				fmt.Println("created", p)
			} else if len(os.Args) > 2 && os.Args[2] == "path" {
				fmt.Println(configPath())
			} else {
				fmt.Fprintln(os.Stderr, "usage: mehshell config init|path|set KEY VALUE")
				os.Exit(1)
			}
			return
		}
	}

	cfg := loadConfig()

	exitCode := 0
	duration := 0
	columns := 80

	if len(os.Args) > 1 {
		exitCode, _ = strconv.Atoi(os.Args[1])
	}
	if len(os.Args) > 2 {
		duration, _ = strconv.Atoi(os.Args[2])
	}
	if len(os.Args) > 3 {
		columns, _ = strconv.Atoi(os.Args[3])
	}
	if columns <= 0 {
		columns = 80
	}

	line1, line2 := renderPrompt(cfg, exitCode, duration, columns)
	fmt.Printf("_mehshell_transient=$'%s '\n", escShell(colourPrompt(fittedPromptMark(cfg, exitCode, columns), cfg)))
	fmt.Printf("_mehshell_display=$'\\n%s\\n%s '\n", escShell(colourPrompt(line1, cfg)), escShell(colourPrompt(line2, cfg)))
	fmt.Println("if [[ -o promptsubst ]]; then PROMPT='${_mehshell_display}'; else PROMPT=$_mehshell_display; fi")
}

func renderPrompt(cfg config, exitCode, duration, columns int) (string, string) {
	on := cfg.Segments
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var left, right []seg

	add := func(dst *[]seg, order int, bg int, fn func() string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if t := fn(); t != "" {
				mu.Lock()
				*dst = append(*dst, seg{t, order, bg})
				mu.Unlock()
			}
		}()
	}

	icons := cfg.Icons
	if basicTerminal() {
		icons = false
		cfg.Style = "console"
	}
	if on["session"] {
		add(&left, -2, 166, segSession)
	}
	if on["game"] {
		add(&right, -1, 166, segGame)
	}
	if on["os"] {
		add(&left, 0, 24, func() string { return segBrand(cfg, icons) })
	}
	if on["dir"] {
		add(&left, 1, 31, func() string { return segDir(cwd, home) })
	}
	if on["git"] {
		add(&left, 2, 97, func() string { return segGit(cwd, icons) })
	}
	if on["node"] {
		add(&left, 3, 34, func() string { return segNode(cwd, icons) })
	}
	if on["python"] {
		add(&left, 4, 136, func() string { return segPython(cwd, icons) })
	}
	if on["go"] {
		add(&left, 5, 37, func() string { return segGo(cwd, icons) })
	}
	if on["rust"] {
		add(&left, 6, 130, func() string { return segRust(cwd, icons) })
	}
	if on["ruby"] {
		add(&left, 7, 124, func() string { return segRuby(cwd, icons) })
	}
	if on["java"] {
		add(&left, 8, 94, func() string { return segJava(cwd, icons) })
	}
	if on["conda"] {
		add(&right, 0, 34, func() string { return segConda(icons) })
	}
	if on["venv"] {
		add(&right, 1, 34, func() string { return segVenv(icons) })
	}
	if on["k8s"] {
		add(&right, 2, 25, func() string { return segK8s(cwd, home, icons) })
	}
	if on["terraform"] {
		add(&right, 3, 57, func() string { return segTerraform(cwd, icons) })
	}
	if on["docker"] {
		add(&right, 4, 25, func() string { return segDocker(cwd, icons) })
	}
	if on["aws"] {
		add(&right, 5, 166, func() string { return segAWS(icons) })
	}
	if on["azure"] {
		add(&right, 6, 25, func() string { return segAzure(icons) })
	}
	if on["gcloud"] {
		add(&right, 7, 34, func() string { return segGCloud(icons) })
	}
	if on["battery"] {
		add(&right, 8, 22, func() string { return segBattery(icons) })
	}
	if on["duration"] {
		add(&right, 9, 136, func() string { return segDuration(duration) })
	}
	if on["time"] {
		add(&right, 10, 238, func() string { return segTime() })
	}

	wg.Wait()

	sort.Slice(left, func(i, j int) bool { return left[i].order < left[j].order })
	sort.Slice(right, func(i, j int) bool { return right[i].order < right[j].order })

	line1 := layoutPrompt(left, right, columns, cfg.Style)
	line2 := fittedPromptMark(cfg, exitCode, columns)
	if cfg.Style == "lean" && visibleWidth(line2)+4 <= columns {
		line2 = fc(cCyan, "╰─") + line2
	}
	return line1, fitPromptText(line2, columns-2)
}
