package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func validOption(key, value string) bool {
	switch key {
	case "style":
		return value == "lean" || value == "classic" || value == "rainbow"
	case "brand":
		return value == "auto" || value == "bromigos" || value == "os" || value == "none"
	}
	_, segment := defaultConfig().Segments[key]
	if !segment && key != "icons" && key != "instant_prompt" && key != "transient_prompt" && key != "vi_mode" {
		return false
	}
	return value == "true" || value == "false" || value == "yes" || value == "no" || value == "1" || value == "0"
}

func setOption(key, value string) error {
	if !validOption(key, value) {
		return errors.New("unsupported option or value; use doctor to see the available settings")
	}
	path := configPath()
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return errors.New("the config symlink target is unavailable")
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if info, err := lock.Stat(); err != nil || !info.Mode().IsRegular() {
		return errors.New("config lock must be a regular file")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another config change is running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	mode := os.FileMode(0600)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return errors.New("config must be a small regular file")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	before, _, err := readPromptConfig(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(before), "\n")
	found := false
	for i, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			lines[i], found = key+" = "+value, true
		}
	}
	text := strings.Join(lines, "\n")
	if !found {
		text = strings.TrimRight(text, "\n") + "\n" + key + " = " + value + "\n"
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mehshell-config-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.WriteString(text)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	current, _, err := readPromptConfig(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, before) {
		return errors.New("config changed while saving; try again")
	}
	return os.Rename(f.Name(), path)
}

func runConfigSet(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mehshell config set KEY VALUE")
		return 2
	}
	if err := setOption(args[0], args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "Config unchanged:", err)
		return 1
	}
	fmt.Println("Saved", args[0]+" = "+args[1])
	if args[0] == "instant_prompt" || args[0] == "transient_prompt" || args[0] == "vi_mode" {
		fmt.Println("Open a new shell to use the changed hooks.")
	} else {
		fmt.Println("The next prompt uses this setting.")
	}
	return 0
}

// Convert our own zsh colour tokens for preview, without evaluating any shell text.
func previewText(text string, colour bool) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "%%") {
			out.WriteByte('%')
			i += 2
			continue
		}
		if strings.HasPrefix(text[i:], "%f") || strings.HasPrefix(text[i:], "%k") {
			if colour {
				if text[i+1] == 'f' {
					out.WriteString("\x1b[39m")
				} else {
					out.WriteString("\x1b[49m")
				}
			}
			i += 2
			continue
		}
		if strings.HasPrefix(text[i:], "%F{") || strings.HasPrefix(text[i:], "%K{") {
			if end := strings.IndexByte(text[i+3:], '}'); end >= 0 {
				value := text[i+3 : i+3+end]
				base := 38
				if text[i+1] == 'K' {
					base = 48
				}
				if colour {
					if validColour(value) {
						n, _ := strconv.ParseUint(value[1:], 16, 24)
						fmt.Fprintf(&out, "\x1b[%d;2;%d;%d;%dm", base, n>>16, (n>>8)&255, n&255)
					} else if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 255 {
						if basicTerminal() && n < 8 {
							fmt.Fprintf(&out, "\x1b[%dm", base-8+n)
						} else {
							fmt.Fprintf(&out, "\x1b[%d;5;%dm", base, n)
						}
					}
				}
				i += 4 + end
				continue
			}
		}
		out.WriteByte(text[i])
		i++
	}
	if colour {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func runPreview(args []string) int {
	flags := flag.NewFlagSet("mehshell preview", flag.ContinueOnError)
	width := flags.Int("width", 80, "Terminal width, from 10 to 4096 columns")
	style := flags.String("style", "", "Try lean, classic or rainbow without saving")
	plain := flags.Bool("plain", false, "Leave out colour escapes")
	code := flags.Int("exit-code", 0, "Show an example command exit status")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *width < 10 || *width > 4096 || (*style != "" && !validOption("style", *style)) || *code < 0 || *code > 255 {
		fmt.Fprintln(os.Stderr, "Choose width 10..4096, a supported style and exit code 0..255.")
		return 2
	}
	cfg := loadConfig()
	if *style != "" {
		cfg.Style = *style
	}
	line1, line2 := renderPrompt(cfg, *code, 0, *width)
	info, err := os.Stdout.Stat()
	colour := err == nil && info.Mode()&os.ModeCharDevice != 0 && !*plain && os.Getenv("TERM") != "dumb"
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		colour = false
	}
	fmt.Println("Prompt preview (no shell changes)")
	fmt.Println(previewText(colourPrompt(line1, cfg), colour))
	fmt.Println(previewText(colourPrompt(line2, cfg), colour))
	return 0
}
